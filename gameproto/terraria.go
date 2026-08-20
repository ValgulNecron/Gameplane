package gameproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Terraria message types.
const (
	terrariaConnectRequest     = 1
	terrariaDisconnect         = 2
	terrariaContinueConnecting = 3
	terrariaPasswordRequired   = 37
)

// terrariaMaxPacketSize is the maximum allowed Terraria packet size.
// The 2-byte length field is part of the packet, so max is 65535.
// We cap at a reasonable value to prevent allocation from length injection.
const terrariaMaxPacketSize = 65535

// classifyTerrariaConnect reads and parses the first Terraria message to
// determine if it's a ConnectRequest (which is always classified as Join).
// Terraria has no out-of-band status ping, so we only care about connection
// attempts. The caller-provided bufio.Reader is used directly, preserving any
// pipelined data in its internal buffer for the caller to consume later.
func classifyTerrariaConnect(br *bufio.Reader) (Kind, *TerrariaClassifyResult, error) {
	// Accumulate consumed bytes
	var consumed bytes.Buffer

	// Read the message frame header (2-byte length LE + 1-byte type).
	var header [3]byte
	if _, err := io.ReadFull(br, header[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Unknown, nil, err
		}
		return Unknown, nil, fmt.Errorf("read terraria frame header: %w", err)
	}

	// Record the header bytes
	consumed.Write(header[:])

	// The length includes the 2-byte length field itself.
	totalLength := binary.LittleEndian.Uint16(header[:2])
	messageType := header[2]

	// Validate frame length.
	if totalLength < 3 {
		return Unknown, nil, fmt.Errorf("terraria frame too short: %d < 3", totalLength)
	}
	if totalLength > terrariaMaxPacketSize {
		return Unknown, nil, fmt.Errorf("terraria frame too long: %d > %d", totalLength, terrariaMaxPacketSize)
	}

	// The payload is everything after the 3-byte header.
	payloadLength := int(totalLength) - 3
	payload := make([]byte, payloadLength)
	if payloadLength > 0 {
		if _, err := io.ReadFull(br, payload); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return Unknown, nil, fmt.Errorf("read terraria frame payload: %w", err)
			}
			return Unknown, nil, fmt.Errorf("read terraria frame payload: %w", err)
		}
	}

	// Record the payload bytes
	consumed.Write(payload)

	// Only ConnectRequest (type 1) indicates a join attempt.
	if messageType != terrariaConnectRequest {
		return Unknown, &TerrariaClassifyResult{
			Consumed: consumed.Bytes(),
		}, nil
	}

	// Parse the ConnectRequest payload.
	result, err := parseTerrariaConnectRequest(payload)
	if err != nil {
		return Unknown, nil, fmt.Errorf("parse terraria ConnectRequest: %w", err)
	}

	// Store consumed bytes
	result.Consumed = consumed.Bytes()

	return Join, result, nil
}

// parseTerrariaConnectRequest parses the payload of a ConnectRequest message.
// Format: [version string only]
func parseTerrariaConnectRequest(payload []byte) (*TerrariaClassifyResult, error) {
	if len(payload) == 0 {
		return nil, errors.New("empty ConnectRequest payload")
	}

	br := bytes.NewReader(payload)

	// Read the version string (7-bit encoded length + UTF-8).
	version, err := readTerrariaString(br)
	if err != nil {
		return nil, fmt.Errorf("read version string: %w", err)
	}

	result := &TerrariaClassifyResult{
		Version: version,
	}

	return result, nil
}

// buildTerrariaDisconnect builds a Terraria Disconnect message.
// Format: [length uint16 LE][type byte 0x02][reason NetworkText]
func buildTerrariaDisconnect(reason string) ([]byte, error) {
	var payload bytes.Buffer

	// Encode the reason as a NetworkText (mode=0 for literal text).
	if err := writeTerrariaNetworkText(&payload, reason); err != nil {
		return nil, fmt.Errorf("encode disconnect reason: %w", err)
	}

	// Frame the message: [length][type][payload]
	var msg bytes.Buffer
	msg.WriteByte(terrariaDisconnect)
	msg.Write(payload.Bytes())

	frame := msg.Bytes()
	totalLength := len(frame) + 2 // +2 for the length field itself

	if totalLength > terrariaMaxPacketSize {
		return nil, fmt.Errorf("disconnect message too large: %d > %d", totalLength, terrariaMaxPacketSize)
	}

	// Write the framed message: [length LE][message].
	var out bytes.Buffer
	out.Write(binary.LittleEndian.AppendUint16(nil, uint16(totalLength)))
	out.Write(frame)

	return out.Bytes(), nil
}

// readTerrariaString reads a .NET BinaryWriter style 7-bit-encoded string.
func readTerrariaString(r *bytes.Reader) (string, error) {
	length, err := readTerrafia7BitEncodedInt(r)
	if err != nil {
		return "", err
	}

	if length < 0 {
		return "", fmt.Errorf("negative string length: %d", length)
	}

	rlen := r.Len()
	if rlen < 0 || int64(length) > int64(rlen) {
		return "", fmt.Errorf("string length %d exceeds remaining %d", length, rlen)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("read string data: %w", err)
	}

	return string(buf), nil
}

// writeTerrariaString writes a .NET BinaryWriter style 7-bit-encoded string.
func writeTerrariaString(w *bytes.Buffer, s string) error {
	slen := len(s)
	if slen > 0x7fffffff {
		return fmt.Errorf("string length %d out of int32 range", slen)
	}
	if err := writeTerrafia7BitEncodedInt(w, int32(slen)); err != nil {
		return err
	}
	w.WriteString(s)
	return nil
}

// readTerrafia7BitEncodedInt reads a 7-bit-encoded integer.
func readTerrafia7BitEncodedInt(r *bytes.Reader) (int32, error) {
	var result int32
	for shift := 0; shift < 32; shift += 7 {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, nil
		}
	}
	return 0, errors.New("7-bit encoded int too long")
}

// writeTerrafia7BitEncodedInt writes a 7-bit-encoded integer.
func writeTerrafia7BitEncodedInt(w *bytes.Buffer, v int32) error {
	if v < 0 {
		return fmt.Errorf("cannot encode negative value %d as 7-bit-encoded int", v)
	}
	uv := uint32(v)
	for {
		b := byte(uv & 0x7f)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		w.WriteByte(b)
		if uv == 0 {
			return nil
		}
	}
}

// writeTerrariaNetworkText writes a NetworkText (mode byte + string + optionally substitutions).
// For simplicity, we only support literal text (mode=0).
func writeTerrariaNetworkText(w *bytes.Buffer, text string) error {
	// Mode byte: 0 = literal text (no substitutions).
	w.WriteByte(0)

	// Write the text as a 7-bit-encoded string.
	return writeTerrariaString(w, text)
}

// TerrariaClassifier implements the Classifier interface for Terraria.
type TerrariaClassifier struct{}

// Classify reads and classifies a Terraria connection attempt.
// It calls the existing classifyTerrariaConnect to parse the wire format,
// then repackages the result into a ClassificationResult with a TerrariaDetail.
// Returns non-nil ClassificationResult even on Kind == Unknown, to enable
// stream replay. Errors are propagated unchanged.
func (tc *TerrariaClassifier) Classify(br *bufio.Reader) (*ClassificationResult, error) {
	kind, tcr, err := classifyTerrariaConnect(br)
	if err != nil {
		return nil, err
	}

	// classifyTerrariaConnect now returns a non-nil *TerrariaClassifyResult
	// in all non-error cases, including Unknown (non-ConnectRequest) messages,
	// with Consumed bytes always present. Detail is only populated on Join.
	var detail Detail
	if kind == Join {
		detail = &TerrariaDetail{
			Version: tcr.Version,
		}
	}

	result := &ClassificationResult{
		Kind:     kind,
		Consumed: tcr.Consumed,
		Detail:   detail,
	}

	return result, nil
}

// SupportsStatusPing returns false because Terraria does not support out-of-band
// status pings. Any attempt to call BuildStatusResponse is a contract violation.
func (tc *TerrariaClassifier) SupportsStatusPing() bool {
	return false
}

// BuildStatusResponse is not supported for Terraria and returns an error.
// The caller should have checked SupportsStatusPing() first; this method
// exists only as a defensive path and should never be called in correct usage.
func (tc *TerrariaClassifier) BuildStatusResponse(payload string) ([]byte, error) {
	return nil, ErrStatusPingUnsupported
}

// BuildDisconnect builds a Terraria Disconnect message containing the given reason.
// It calls the existing buildTerrariaDisconnect function.
func (tc *TerrariaClassifier) BuildDisconnect(reason string) ([]byte, error) {
	return buildTerrariaDisconnect(reason)
}
