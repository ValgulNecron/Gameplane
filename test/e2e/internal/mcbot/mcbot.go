// Package mcbot is a tiny, dependency-free Minecraft: Java Edition protocol
// client. The e2e suite (and the mcprobe CLI under cmd/) use it to prove that
// a Kestrel-managed Minecraft server is not merely "Running" in Kubernetes but
// genuinely playable: it answers a Server List Ping and accepts a login.
//
// It speaks just enough of the post-Netty protocol (Minecraft 1.7+):
//
//   - Ping  — Server List Ping (status): version name, protocol number, and
//     player counts, as JSON. Works regardless of online-mode.
//   - Login — completes the (offline-mode) login handshake and reports whether
//     the server answered with Login Success, demanded authentication
//     (Encryption Request => online-mode), or disconnected us.
//
// Only the Handshaking/Status/Login states are implemented (no Play state),
// which is enough to confirm a bot can join and keeps the code stable across
// Minecraft versions — login-state packet IDs have not changed in years.
package mcbot

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// Status is the subset of the Server List Ping JSON we care about.
type Status struct {
	Version struct {
		Name     string `json:"name"`
		Protocol int    `json:"protocol"`
	} `json:"version"`
	Players struct {
		Max    int `json:"max"`
		Online int `json:"online"`
	} `json:"players"`
}

// Outcome is the result of a login attempt.
type Outcome int

const (
	// Success: the server sent Login Success — a bot can join.
	Success Outcome = iota
	// NeedsAuth: the server sent an Encryption Request, i.e. it runs in
	// online-mode and requires a real (Mojang-authenticated) account.
	NeedsAuth
	// Disconnected: the server refused the login (Disconnect packet).
	Disconnected
)

// LoginResult carries the outcome plus a human-readable detail (the accepted
// username on success, or the server's reason otherwise).
type LoginResult struct {
	Outcome Outcome
	Detail  string
}

// Ping performs a Server List Ping against addr ("host:port") and returns the
// server's reported version/protocol/player counts.
func Ping(ctx context.Context, addr string) (*Status, error) {
	host, port, err := splitHostPort(addr)
	if err != nil {
		return nil, err
	}
	conn, err := dial(ctx, addr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Handshake with next state = 1 (status). Protocol version is ignored by
	// servers for status, so -1 is conventional.
	if _, err := conn.Write(handshake(-1, host, port, 1)); err != nil {
		return nil, fmt.Errorf("mcbot: write handshake: %w", err)
	}
	if _, err := conn.Write(buildPacket(0x00, nil)); err != nil { // Status Request
		return nil, fmt.Errorf("mcbot: write status request: %w", err)
	}

	rd := &reader{br: bufio.NewReader(conn)}
	id, payload, err := rd.readPacket()
	if err != nil {
		return nil, fmt.Errorf("mcbot: read status response: %w", err)
	}
	if id != 0x00 {
		return nil, fmt.Errorf("mcbot: unexpected status packet id 0x%02x", id)
	}
	raw, err := readString(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("mcbot: read status json: %w", err)
	}
	var s Status
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("mcbot: parse status json: %w", err)
	}
	return &s, nil
}

// Login completes a login handshake against addr using the given protocol
// number (get it from Ping) and username, in offline mode (no encryption).
func Login(ctx context.Context, addr string, protocol int, username string) (*LoginResult, error) {
	host, port, err := splitHostPort(addr)
	if err != nil {
		return nil, err
	}
	conn, err := dial(ctx, addr, 25*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if _, err := conn.Write(handshake(int32(protocol), host, port, 2)); err != nil {
		return nil, fmt.Errorf("mcbot: write handshake: %w", err)
	}

	// Login Start: name + player UUID (UUID has been mandatory since 1.20.2).
	var ls bytes.Buffer
	writeString(&ls, username)
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		return nil, fmt.Errorf("mcbot: uuid: %w", err)
	}
	ls.Write(uuid)
	if _, err := conn.Write(buildPacket(0x00, ls.Bytes())); err != nil {
		return nil, fmt.Errorf("mcbot: write login start: %w", err)
	}

	rd := &reader{br: bufio.NewReader(conn)}
	for {
		id, payload, err := rd.readPacket()
		if err != nil {
			return nil, fmt.Errorf("mcbot: read login response: %w", err)
		}
		switch id {
		case 0x03: // Set Compression — subsequent frames are compressed.
			threshold, _ := readVarInt(bytes.NewReader(payload))
			rd.compressed = threshold >= 0
		case 0x02: // Login Success — the server accepted our (offline) login.
			return &LoginResult{Outcome: Success, Detail: username}, nil
		case 0x01: // Encryption Request — server is in online-mode.
			return &LoginResult{Outcome: NeedsAuth, Detail: "server requires authentication (online-mode)"}, nil
		case 0x00: // Disconnect — server refused us.
			reason, _ := readString(bytes.NewReader(payload))
			return &LoginResult{Outcome: Disconnected, Detail: reason}, nil
		default:
			// Ignore anything else (e.g. Login Plugin Request 0x04 from modded
			// servers) and keep reading until a terminal packet or the deadline.
		}
	}
}

// Connect pings addr and then attempts a login using the protocol the ping
// reported, returning both. It's the convenience the e2e test uses.
func Connect(ctx context.Context, addr, username string) (*Status, *LoginResult, error) {
	st, err := Ping(ctx, addr)
	if err != nil {
		return nil, nil, err
	}
	res, err := Login(ctx, addr, st.Version.Protocol, username)
	if err != nil {
		return st, nil, err
	}
	return st, res, nil
}

// ---- wire helpers ----

func dial(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mcbot: dial %s: %w", addr, err)
	}
	deadline := time.Now().Add(timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	return conn, nil
}

func splitHostPort(addr string) (string, uint16, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("mcbot: split %q: %w", addr, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, fmt.Errorf("mcbot: port %q: %w", portStr, err)
	}
	return host, uint16(port), nil
}

func handshake(protocol int32, host string, port uint16, nextState int32) []byte {
	var p bytes.Buffer
	writeVarInt(&p, protocol)
	writeString(&p, host)
	_ = binary.Write(&p, binary.BigEndian, port)
	writeVarInt(&p, nextState)
	return buildPacket(0x00, p.Bytes())
}

// buildPacket frames an uncompressed packet: VarInt(length) + VarInt(id) + data.
func buildPacket(id int32, data []byte) []byte {
	var inner bytes.Buffer
	writeVarInt(&inner, id)
	inner.Write(data)
	var out bytes.Buffer
	writeVarInt(&out, int32(inner.Len()))
	out.Write(inner.Bytes())
	return out.Bytes()
}

type reader struct {
	br         *bufio.Reader
	compressed bool
}

// readPacket reads one framed packet, transparently inflating it once
// compression has been negotiated, and returns its id + remaining payload.
func (rd *reader) readPacket() (int32, []byte, error) {
	length, err := readVarInt(rd.br)
	if err != nil {
		return 0, nil, err
	}
	if length <= 0 || length > 8<<20 {
		return 0, nil, fmt.Errorf("mcbot: bad packet length %d", length)
	}
	frame := make([]byte, length)
	if _, err := io.ReadFull(rd.br, frame); err != nil {
		return 0, nil, err
	}
	fr := bytes.NewReader(frame)
	if rd.compressed {
		dataLen, err := readVarInt(fr)
		if err != nil {
			return 0, nil, err
		}
		if dataLen > 0 { // dataLen == 0 means the rest is stored uncompressed
			zr, err := zlib.NewReader(fr)
			if err != nil {
				return 0, nil, fmt.Errorf("mcbot: zlib: %w", err)
			}
			out := make([]byte, dataLen)
			if _, err := io.ReadFull(zr, out); err != nil {
				return 0, nil, fmt.Errorf("mcbot: inflate: %w", err)
			}
			_ = zr.Close()
			fr = bytes.NewReader(out)
		}
	}
	id, err := readVarInt(fr)
	if err != nil {
		return 0, nil, err
	}
	rest, err := io.ReadAll(fr)
	if err != nil {
		return 0, nil, err
	}
	return id, rest, nil
}

func writeVarInt(buf *bytes.Buffer, v int32) {
	uv := uint32(v)
	for {
		b := byte(uv & 0x7f)
		uv >>= 7
		if uv != 0 {
			b |= 0x80
		}
		buf.WriteByte(b)
		if uv == 0 {
			return
		}
	}
}

func readVarInt(r io.ByteReader) (int32, error) {
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
	return 0, errors.New("mcbot: VarInt too long")
}

func writeString(buf *bytes.Buffer, s string) {
	writeVarInt(buf, int32(len(s)))
	buf.WriteString(s)
}

func readString(r *bytes.Reader) (string, error) {
	n, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	if n < 0 || int(n) > r.Len() {
		return "", errors.New("mcbot: bad string length")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return "", err
	}
	return string(b), nil
}
