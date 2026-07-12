// telnet.go implements a minimal, line-based remote-console protocol used
// by games whose "RCON" is actually a raw TCP/telnet session rather than
// the Valve packet framing in rcon.go — 7 Days to Die's in-game telnet
// server is the reference implementation. There's no message framing or
// request/response correlation (this is plain text, not RFC 854 telnet
// with option negotiation): a command is one line, and a reply — when
// there is one at all — is whatever text arrives before the socket goes
// quiet. Replies are matched to commands purely by ordering on one
// connection.
package rcon

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// defaultTelnetConnectTimeout bounds dialing and the password exchange.
	defaultTelnetConnectTimeout = 5 * time.Second

	// defaultTelnetResponseDeadline bounds how long Exec waits for the
	// FIRST byte of a reply after sending a command. Many telnet commands
	// print nothing at all (an ack-less "save", for example), so this is
	// deliberately much shorter than Source RCON's execDeadline — a real
	// reply from these consoles arrives near-instantly, and there's no
	// protocol-level guarantee of a reply to wait out.
	defaultTelnetResponseDeadline = 5 * time.Second

	// defaultTelnetReadQuiet bounds how long Exec keeps reading once a
	// reply has started arriving: once this elapses with no further
	// bytes, the reply is considered complete. Mirrors rcon.go's
	// responseGrace, but tighter since these servers stream plain text
	// rather than framed packets with a length prefix.
	defaultTelnetReadQuiet = 200 * time.Millisecond
)

// TelnetClient is a lazy, auto-reconnecting client for line-based telnet
// RCON consoles. It's safe for use from multiple goroutines; all ops are
// serialized on a single connection.
type TelnetClient struct {
	addr     string
	password PassFn

	// connectTimeout/responseDeadline/readQuiet are struct fields (not
	// package consts) so tests can shrink them — otherwise a test
	// exercising the no-output-command path would block for the full
	// production deadline before observing an empty reply.
	connectTimeout   time.Duration
	responseDeadline time.Duration
	readQuiet        time.Duration

	mu   sync.Mutex
	conn net.Conn
}

// NewTelnet builds a telnet RCON client. The connection is dialed lazily
// on the first Exec.
func NewTelnet(host string, port int, pw PassFn) *TelnetClient {
	return &TelnetClient{
		addr:             net.JoinHostPort(host, fmt.Sprint(port)),
		password:         pw,
		connectTimeout:   defaultTelnetConnectTimeout,
		responseDeadline: defaultTelnetResponseDeadline,
		readQuiet:        defaultTelnetReadQuiet,
	}
}

// Exec sends one command as a line and returns whatever the server prints
// back before going quiet. A command with no output returns "", nil — the
// protocol has no way to distinguish "no output" from "slow output" other
// than the response deadline elapsing, so a silent command always costs
// that deadline.
func (c *TelnetClient) Exec(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureLocked(); err != nil {
		return "", err
	}

	conn := c.conn
	_ = conn.SetDeadline(time.Now().Add(c.responseDeadline))
	defer func() { _ = conn.SetDeadline(time.Time{}) }()

	if _, err := conn.Write([]byte(cmd + "\n")); err != nil {
		c.dropLocked()
		return "", fmt.Errorf("telnet rcon exec %q: %w", cmd, err)
	}

	out, err := c.readReplyLocked()
	if err != nil {
		c.dropLocked()
		return "", fmt.Errorf("telnet rcon exec %q: %w", cmd, err)
	}
	return out, nil
}

// Close shuts down the underlying connection.
func (c *TelnetClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *TelnetClient) ensureLocked() error {
	if c.conn != nil {
		return nil
	}
	pw, err := c.password()
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("tcp", c.addr, c.connectTimeout)
	if err != nil {
		return err
	}
	c.conn = conn
	_ = conn.SetDeadline(time.Now().Add(c.connectTimeout))

	// Drain whatever banner/prompt the server greets with before sending
	// the password — some servers print one, some don't; the quiet-period
	// read in readReplyLocked works either way.
	if _, err := c.readReplyLocked(); err != nil {
		c.dropLocked()
		return err
	}

	if _, err := conn.Write([]byte(pw + "\n")); err != nil {
		c.dropLocked()
		return err
	}
	resp, err := c.readReplyLocked()
	if err != nil {
		c.dropLocked()
		return err
	}
	if isTelnetAuthFailure(resp) {
		c.dropLocked()
		return errors.New("telnet rcon: authentication failed")
	}
	_ = conn.SetDeadline(time.Time{})
	return nil
}

func (c *TelnetClient) dropLocked() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// readReplyLocked reads from c.conn until readQuiet elapses with no new
// bytes, or responseDeadline elapses before any arrive. A timeout ending
// the read is the expected, successful outcome — a telnet console has no
// length prefix or terminator to signal "reply complete" — so only a
// non-timeout error (closed connection, reset) is returned as a failure.
func (c *TelnetClient) readReplyLocked() (string, error) {
	var out strings.Builder
	buf := make([]byte, 4096)
	deadline := time.Now().Add(c.responseDeadline)
	for {
		_ = c.conn.SetReadDeadline(deadline)
		n, err := c.conn.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			// Once a reply has started, a short quiet period is enough to
			// call it done — no need to keep waiting out the full
			// response deadline on every command.
			deadline = time.Now().Add(c.readQuiet)
		}
		if err != nil {
			if isTimeout(err) {
				return out.String(), nil
			}
			return out.String(), err
		}
	}
}

// isTelnetAuthFailure heuristically detects a rejected password from the
// server's response text. 7 Days to Die (the reference implementation)
// responds with a message containing "incorrect password"; other
// telnet-style consoles are expected to use similar wording. This is
// best-effort, not a protocol guarantee — a server that closes the
// connection outright on bad auth is caught separately, since the next
// write/read then returns a non-timeout error.
func isTelnetAuthFailure(resp string) bool {
	lower := strings.ToLower(resp)
	return strings.Contains(lower, "incorrect password") ||
		strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "password failed")
}
