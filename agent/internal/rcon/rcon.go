// Package rcon implements the Valve/Source RCON protocol used by
// Minecraft, Source engine games, CS servers, and many others.
//
// Protocol reference: https://developer.valvesoftware.com/wiki/Source_RCON_Protocol
//
// Packet layout (little-endian):
//
//	int32 size    // bytes that follow (excluding itself)
//	int32 id      // request id, echoed in response
//	int32 type    // 3=AUTH, 2=EXEC_CMD or AUTH_RESPONSE, 0=RESPONSE_VALUE
//	string body   // null-terminated ASCII
//	byte  pad     // extra null
//
// An authentication failure responds with id=-1. Large command
// responses are split across multiple RESPONSE_VALUE packets; we
// assemble them with the Valve-documented "empty-cmd sentinel" trick.
package rcon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	typeAuth         = 3
	typeExecCmd      = 2
	typeAuthResponse = 2
	typeRespValue    = 0
)

type PassFn func() (string, error)

// PasswordFromFile returns a password resolver that reads from disk
// on each call (so rotated Secrets are picked up without a restart).
func PasswordFromFile(path string) PassFn {
	return func() (string, error) {
		if path == "" {
			return "", errors.New("no rcon password file configured")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
}

// Client is a lazy, auto-reconnecting RCON client. It's safe for use
// from multiple goroutines; all ops are serialized on a single conn.
type Client struct {
	addr     string
	password PassFn

	mu     sync.Mutex
	conn   net.Conn
	nextID uint32
}

func New(host string, port int, pw PassFn) *Client {
	return &Client{addr: net.JoinHostPort(host, fmt.Sprint(port)), password: pw}
}

// Exec runs one RCON command and returns the concatenated response.
func (c *Client) Exec(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureLocked(); err != nil {
		return "", err
	}

	reqID := c.allocID()
	if err := c.writePacket(reqID, typeExecCmd, cmd); err != nil {
		c.dropLocked()
		return "", err
	}

	// Sentinel trick: send an empty command right after the real one.
	// RESPONSE_VALUE packets for the real command arrive first; when
	// we see the sentinel's response we know the real one is complete.
	sentinelID := c.allocID()
	if err := c.writePacket(sentinelID, typeRespValue, ""); err != nil {
		c.dropLocked()
		return "", err
	}

	var out bytes.Buffer
	for {
		id, _, body, err := c.readPacket()
		if err != nil {
			c.dropLocked()
			return "", err
		}
		if id == sentinelID {
			return out.String(), nil
		}
		out.WriteString(body)
	}
}

// Close shuts down the underlying connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *Client) ensureLocked() error {
	if c.conn != nil {
		return nil
	}
	pw, err := c.password()
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return err
	}
	c.conn = conn
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	authID := c.allocID()
	if err := c.writePacket(authID, typeAuth, pw); err != nil {
		c.dropLocked()
		return err
	}
	// Server MAY send an empty RESPONSE_VALUE before AUTH_RESPONSE.
	for {
		id, kind, _, err := c.readPacket()
		if err != nil {
			c.dropLocked()
			return err
		}
		if kind == typeAuthResponse {
			if id != authID {
				c.dropLocked()
				return errors.New("rcon auth failed")
			}
			_ = conn.SetDeadline(time.Time{})
			return nil
		}
	}
}

func (c *Client) dropLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) allocID() uint32 {
	c.nextID++
	if c.nextID == 0 {
		c.nextID = 1
	}
	return c.nextID
}

func (c *Client) writePacket(id, kind uint32, body string) error {
	payload := make([]byte, 0, 14+len(body))
	payload = binary.LittleEndian.AppendUint32(payload, id)
	payload = binary.LittleEndian.AppendUint32(payload, kind)
	payload = append(payload, body...)
	payload = append(payload, 0, 0)

	n := len(payload)
	if n > 4096 {
		return fmt.Errorf("rcon: payload too large (%d bytes)", n)
	}
	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint32(hdr, uint32(n))

	if _, err := c.conn.Write(hdr); err != nil {
		return err
	}
	_, err := c.conn.Write(payload)
	return err
}

func (c *Client) readPacket() (id, kind uint32, body string, err error) {
	var hdr [4]byte
	if _, err = io.ReadFull(c.conn, hdr[:]); err != nil {
		return
	}
	size := binary.LittleEndian.Uint32(hdr[:])
	if size < 10 || size > 4096 {
		err = fmt.Errorf("rcon: malformed packet size %d", size)
		return
	}
	buf := make([]byte, size)
	if _, err = io.ReadFull(c.conn, buf); err != nil {
		return
	}
	id = binary.LittleEndian.Uint32(buf[0:4])
	kind = binary.LittleEndian.Uint32(buf[4:8])
	// body is null-terminated; strip the two trailing nulls
	end := len(buf) - 2
	if end < 8 {
		end = 8
	}
	body = string(buf[8:end])
	return
}

// ErrDisabled marks RCON as deliberately not configured for this game
// (GameTemplate declares no RCON protocol). Callers degrade gracefully
// — players reports an unknown count, moderation reports unsupported —
// instead of treating it as an upstream outage.
var ErrDisabled = errors.New("rcon disabled for this game")

// Disabled returns an Exec-er whose every call fails with ErrDisabled.
type Disabled struct{}

func (Disabled) Exec(string) (string, error) { return "", ErrDisabled }
