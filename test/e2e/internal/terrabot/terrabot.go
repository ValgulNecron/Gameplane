// Package terrabot is a minimal headless Terraria protocol client, just
// deep enough to prove a server is playable: it completes the connection
// handshake (ConnectRequest → ContinueConnecting) and requests world data.
//
// Wire format: every message is [length uint16 LE][type byte][payload],
// where length counts the whole message including the 2 length bytes.
// Strings are .NET BinaryWriter style: 7-bit-encoded length + UTF-8.
package terrabot

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"time"
)

// Message types used by the handshake.
const (
	msgConnectRequest     = 1
	msgDisconnect         = 2
	msgContinueConnecting = 3
	msgRequestWorldData   = 6
	msgWorldData          = 7
	msgPasswordRequired   = 37
)

// DefaultVersion is the protocol string for Terraria 1.4.4.x. Connect
// attempts to self-correct on version-mismatch kicks, but only when the
// server's disconnect message names a version (some servers' kicks, like
// Terraria's LegacyMultiplayer.4, do not); see e2e's terraria template.
const DefaultVersion = "Terraria279"

// ErrPasswordRequired reports a handshake that reached the server's
// password prompt — the protocol works; the server is just gated.
var ErrPasswordRequired = errors.New("server requires a password")

var versionTokenRE = regexp.MustCompile(`^Terraria\d+$`)

// ConnectResult describes a successful handshake.
type ConnectResult struct {
	Slot    byte   // player slot the server assigned
	Version string // protocol string that was accepted
}

// Conn is a live, handshaken Terraria connection.
type Conn struct {
	c net.Conn
}

func (c *Conn) Close() error { return c.c.Close() }

// Connect dials addr and completes the ConnectRequest handshake. On a
// version-mismatch kick, it retries once with the version string the
// server's disconnect message names — but only if the kick actually names one.
// Terraria's LegacyMultiplayer.4 kick does not, so e2e pins the image to
// prevent protocol drift; see terraria_bot_e2e_test.go.
func Connect(ctx context.Context, addr string) (*Conn, *ConnectResult, error) {
	version := DefaultVersion
	for attempt := 0; attempt < 2; attempt++ {
		conn, res, err := connectOnce(ctx, addr, version)
		if err == nil {
			return conn, res, nil
		}
		var mismatch *versionMismatchError
		if errors.As(err, &mismatch) && mismatch.want != version {
			version = mismatch.want
			continue
		}
		return nil, nil, err
	}
	return nil, nil, fmt.Errorf("server rejected both %s and its own advertised version", DefaultVersion)
}

type versionMismatchError struct{ want string }

func (e *versionMismatchError) Error() string {
	return "server wants protocol version " + e.want
}

func connectOnce(ctx context.Context, addr, version string) (*Conn, *ConnectResult, error) {
	d := net.Dialer{}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	} else {
		_ = c.SetDeadline(time.Now().Add(20 * time.Second))
	}

	var payload bytes.Buffer
	writeString(&payload, version)
	if err := writeMessage(c, msgConnectRequest, payload.Bytes()); err != nil {
		_ = c.Close()
		return nil, nil, fmt.Errorf("send ConnectRequest: %w", err)
	}

	typ, body, err := readMessage(c)
	if err != nil {
		_ = c.Close()
		return nil, nil, fmt.Errorf("read handshake reply: %w", err)
	}
	switch typ {
	case msgContinueConnecting:
		slot := byte(0)
		if len(body) > 0 {
			slot = body[0]
		}
		return &Conn{c: c}, &ConnectResult{Slot: slot, Version: version}, nil
	case msgPasswordRequired:
		_ = c.Close()
		return nil, nil, ErrPasswordRequired
	case msgDisconnect:
		_ = c.Close()
		reason, subs := parseNetworkText(body)
		for _, s := range subs {
			if versionTokenRE.MatchString(s) {
				return nil, nil, &versionMismatchError{want: s}
			}
		}
		return nil, nil, fmt.Errorf("server disconnected: %s %v", reason, subs)
	default:
		_ = c.Close()
		return nil, nil, fmt.Errorf("unexpected handshake reply: message type %d", typ)
	}
}

// RequestWorldData asks for the world header (message 6) and waits until
// the server answers with WorldData (message 7), skipping any unrelated
// traffic in between. Receiving world data proves the server considers
// the session a real joining client, well past a bare TCP accept.
func (c *Conn) RequestWorldData(ctx context.Context) error {
	if dl, ok := ctx.Deadline(); ok {
		_ = c.c.SetDeadline(dl)
	} else {
		_ = c.c.SetDeadline(time.Now().Add(20 * time.Second))
	}
	if err := writeMessage(c.c, msgRequestWorldData, nil); err != nil {
		return fmt.Errorf("send RequestWorldData: %w", err)
	}
	for {
		typ, body, err := readMessage(c.c)
		if err != nil {
			return fmt.Errorf("await WorldData: %w", err)
		}
		switch typ {
		case msgWorldData:
			if len(body) == 0 {
				return errors.New("empty WorldData payload")
			}
			return nil
		case msgDisconnect:
			reason, subs := parseNetworkText(body)
			return fmt.Errorf("server disconnected before WorldData: %s %v", reason, subs)
		default:
			// Servers may interleave other state (player slots, settings);
			// keep reading until the world header shows up.
		}
	}
}

// writeMessage frames and sends one message.
func writeMessage(w io.Writer, typ byte, payload []byte) error {
	total := 2 + 1 + len(payload)
	buf := make([]byte, 0, total)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(total))
	buf = append(buf, typ)
	buf = append(buf, payload...)
	_, err := w.Write(buf)
	return err
}

// readMessage reads one framed message.
func readMessage(r io.Reader) (byte, []byte, error) {
	var head [3]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return 0, nil, err
	}
	total := binary.LittleEndian.Uint16(head[:2])
	if total < 3 {
		return 0, nil, fmt.Errorf("bad frame length %d", total)
	}
	body := make([]byte, total-3)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return head[2], body, nil
}

// writeString appends a .NET BinaryWriter string (7-bit-encoded length +
// UTF-8 bytes).
func writeString(buf *bytes.Buffer, s string) {
	n := len(s)
	for n >= 0x80 {
		buf.WriteByte(byte(n) | 0x80)
		n >>= 7
	}
	buf.WriteByte(byte(n))
	buf.WriteString(s)
}

// readString consumes a .NET BinaryWriter string from r.
func readString(r *bytes.Reader) (string, error) {
	length, shift := 0, 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		length |= int(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift > 21 {
			return "", errors.New("string length prefix too long")
		}
	}
	if length < 0 || length > r.Len() {
		return "", fmt.Errorf("string length %d exceeds remaining %d", length, r.Len())
	}
	out := make([]byte, length)
	if _, err := io.ReadFull(r, out); err != nil {
		return "", err
	}
	return string(out), nil
}

// parseNetworkText decodes Terraria's NetworkText: a mode byte, the text,
// and (for non-literal modes) a substitution list of nested NetworkTexts.
// Version-mismatch kicks carry the wanted "TerrariaNNN" as a substitution.
// Best-effort: on any parse error it returns what it has.
func parseNetworkText(body []byte) (text string, substitutions []string) {
	r := bytes.NewReader(body)
	mode, err := r.ReadByte()
	if err != nil {
		return "", nil
	}
	text, err = readString(r)
	if err != nil {
		return "", nil
	}
	if mode == 0 {
		return text, nil
	}
	count, err := r.ReadByte()
	if err != nil {
		return text, nil
	}
	for i := 0; i < int(count); i++ {
		subText, _ := parseNetworkTextReader(r)
		substitutions = append(substitutions, subText)
	}
	return text, substitutions
}

func parseNetworkTextReader(r *bytes.Reader) (string, error) {
	mode, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	text, err := readString(r)
	if err != nil {
		return "", err
	}
	if mode == 0 {
		return text, nil
	}
	count, err := r.ReadByte()
	if err != nil {
		return text, nil
	}
	for i := 0; i < int(count); i++ {
		if _, err := parseNetworkTextReader(r); err != nil {
			return text, nil
		}
	}
	return text, nil
}
