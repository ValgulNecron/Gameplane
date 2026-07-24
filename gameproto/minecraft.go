package gameproto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// minecraftMaxPacketSize is the maximum allowed packet size. Minecraft packets
// sent at the handshake stage are much smaller; this prevents unbounded
// allocation from a malicious length prefix.
const minecraftMaxPacketSize = 512

// classifyMinecraftHandshake reads and parses the Handshake packet to determine
// whether the connection is a Join (login state) or Status (status state).
func classifyMinecraftHandshake(r io.Reader) (Kind, *MinecraftClassifyResult, error) {
	// Wrap the reader in a bufio reader if needed to get ByteReader interface.
	br, ok := r.(io.ByteReader)
	if !ok {
		// Try to wrap in bytes.Reader if it's a []byte slice.
		if buf, ok := r.(*bytes.Buffer); ok {
			br = buf
		} else {
			// Fallback: read into a buffer first.
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, r); err != nil {
				return Unknown, nil, fmt.Errorf("buffer read: %w", err)
			}
			br = bytes.NewReader(buf.Bytes())
		}
	}

	// Read the frame length (VarInt).
	length, err := readMinecraftVarInt(br)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Unknown, nil, err
		}
		return Unknown, nil, fmt.Errorf("read handshake length: %w", err)
	}

	if length <= 0 || length > minecraftMaxPacketSize {
		return Unknown, nil, fmt.Errorf("handshake packet length %d out of range", length)
	}

	// Read the packet data into a buffer.
	frame := make([]byte, length)
	if _, err = io.ReadFull(r, frame); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Unknown, nil, fmt.Errorf("read handshake frame: %w", err)
		}
		return Unknown, nil, fmt.Errorf("read handshake frame: %w", err)
	}

	// Parse the handshake packet.
	frameReader := bytes.NewReader(frame)

	// Read packet ID (must be 0x00 for Handshake).
	packetID, err := readMinecraftVarInt(frameReader)
	if err != nil {
		return Unknown, nil, fmt.Errorf("read packet id: %w", err)
	}
	if packetID != 0x00 {
		return Unknown, nil, fmt.Errorf("expected handshake packet id 0x00, got 0x%02x", packetID)
	}

	// Read protocol version (VarInt).
	protocolVersion, err := readMinecraftVarInt(frameReader)
	if err != nil {
		return Unknown, nil, fmt.Errorf("read protocol version: %w", err)
	}

	// Read server address (string).
	serverAddr, err := readMinecraftString(frameReader)
	if err != nil {
		return Unknown, nil, fmt.Errorf("read server address: %w", err)
	}

	// Read server port (uint16 big-endian).
	var portBytes [2]byte
	if _, err := io.ReadFull(frameReader, portBytes[:]); err != nil {
		return Unknown, nil, fmt.Errorf("read server port: %w", err)
	}
	port := binary.BigEndian.Uint16(portBytes[:])

	// Read next state (VarInt): 1 = Status, 2 = Login.
	nextState, err := readMinecraftVarInt(frameReader)
	if err != nil {
		return Unknown, nil, fmt.Errorf("read next state: %w", err)
	}

	result := &MinecraftClassifyResult{
		ProtocolVersion: protocolVersion,
		NextState:       nextState,
		ServerAddr:      serverAddr + ":" + strconv.Itoa(int(port)),
	}

	switch nextState {
	case 1: // Status
		return Status, result, nil
	case 2: // Login
		return Join, result, nil
	default:
		return Unknown, result, nil
	}
}

// buildMinecraftStatusResponse builds a JSON status response packet.
// Format: packet_id (0x00) + string(json).
func buildMinecraftStatusResponse(jsonPayload string) ([]byte, error) {
	var inner bytes.Buffer

	// Packet ID (0x00 for Status Response).
	writeMinecraftVarInt(&inner, 0x00)

	// JSON response as a string.
	if err := writeMinecraftString(&inner, jsonPayload); err != nil {
		return nil, fmt.Errorf("encode status response: %w", err)
	}

	// Frame the packet with a length prefix.
	return frameMinecraftPacket(inner.Bytes()), nil
}

// buildMinecraftLoginDisconnect builds a Disconnect packet for the login state.
// Format: packet_id (0x00) + chat(reason).
// The reason is a chat component, which for simplicity we encode as a plain JSON string.
func buildMinecraftLoginDisconnect(reason string) ([]byte, error) {
	var inner bytes.Buffer

	// Packet ID (0x00 for Disconnect in the Login state).
	writeMinecraftVarInt(&inner, 0x00)

	// Reason as a chat component (JSON string).
	chatJSON := `{"text":"` + escapeJSONString(reason) + `"}`
	if err := writeMinecraftString(&inner, chatJSON); err != nil {
		return nil, fmt.Errorf("encode disconnect reason: %w", err)
	}

	return frameMinecraftPacket(inner.Bytes()), nil
}

// readMinecraftVarInt reads a VarInt (5-byte max) from r.
func readMinecraftVarInt(r io.ByteReader) (int32, error) {
	var result uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return int32(result), nil
		}
	}
	return 0, errors.New("varint too long")
}

// writeMinecraftVarInt writes a VarInt to w.
func writeMinecraftVarInt(w *bytes.Buffer, v int32) {
	uv := uint32(v)
	for {
		b := byte(uv & 0x7f)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		w.WriteByte(b)
		if uv == 0 {
			return
		}
	}
}

// readMinecraftString reads a UTF-8 string from r, prefixed by a VarInt length.
func readMinecraftString(r io.ByteReader) (string, error) {
	length, err := readMinecraftVarInt(r)
	if err != nil {
		return "", err
	}
	if length < 0 || length > 32767 { // 32KB max string
		return "", fmt.Errorf("string length %d out of range", length)
	}

	// Convert ByteReader to Reader so we can use ReadFull.
	// If r is already a Reader, use it directly; otherwise wrap it.
	var rr io.Reader
	if reader, ok := r.(io.Reader); ok {
		rr = reader
	} else {
		// Fallback: read bytes one by one.
		buf := make([]byte, length)
		for i := 0; i < int(length); i++ {
			b, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			buf[i] = b
		}
		return string(buf), nil
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(rr, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// writeMinecraftString writes a UTF-8 string to w, prefixed by a VarInt length.
func writeMinecraftString(w *bytes.Buffer, s string) error {
	if len(s) > 32767 {
		return fmt.Errorf("string too long: %d > 32767", len(s))
	}
	writeMinecraftVarInt(w, int32(len(s)))
	w.WriteString(s)
	return nil
}

// frameMinecraftPacket frames packet data by prefixing it with a VarInt length.
func frameMinecraftPacket(data []byte) []byte {
	var out bytes.Buffer
	writeMinecraftVarInt(&out, int32(len(data)))
	out.Write(data)
	return out.Bytes()
}

// escapeJSONString escapes a string for use in a JSON string literal.
// Handles backslashes, quotes, and control characters.
func escapeJSONString(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}
