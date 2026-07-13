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
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// defaultTelnetConnectTimeout bounds dialing and each individual step
	// of the password exchange (banner drain gets its own, shorter budget
	// — see defaultTelnetBannerDrain).
	defaultTelnetConnectTimeout = 5 * time.Second

	// defaultTelnetResponseDeadline bounds how long Exec waits for the
	// FIRST byte of a reply after sending a command. Many telnet commands
	// print nothing at all (an ack-less "save", for example), so this is
	// deliberately much shorter than Source RCON's execDeadline — a real
	// reply from these consoles arrives near-instantly, and there's no
	// protocol-level guarantee of a reply to wait out. It also doubles as
	// the ABSOLUTE wall-clock cap on one read (see readReplyLocked) — a
	// console that streams continuously (7 Days to Die's telnet is a live
	// log mirror once connected) must not be able to keep a read alive
	// past this by never going quiet.
	defaultTelnetResponseDeadline = 5 * time.Second

	// defaultTelnetReadQuiet bounds how long Exec keeps reading once a
	// reply has started arriving: once this elapses with no further
	// bytes, the reply is considered complete. Mirrors rcon.go's
	// responseGrace, but tighter since these servers stream plain text
	// rather than framed packets with a length prefix.
	defaultTelnetReadQuiet = 200 * time.Millisecond

	// defaultTelnetBannerDrain bounds the banner/prompt drain in
	// ensureLocked. Deliberately short and separate from
	// connectTimeout/responseDeadline: a server that prints no banner at
	// all (common) must not burn a long shared budget just finding that
	// out, leaving nothing for the password write/read that follows.
	// We're draining unsolicited output here, not awaiting a specific
	// reply, so a short wait is enough to catch a real banner while
	// costing almost nothing when there isn't one.
	defaultTelnetBannerDrain = 500 * time.Millisecond

	// maxTelnetReply bounds how much of a reply readReplyLocked will
	// buffer. Without this, a console that streams continuously (see
	// defaultTelnetResponseDeadline above) grows the buffer without limit
	// for as long as the read stays alive, which — combined with an
	// unbounded read — is how a chatty telnet console OOMs the sidecar.
	maxTelnetReply = 1 << 20 // 1 MiB
)

// TelnetClient is a lazy, auto-reconnecting client for line-based telnet
// RCON consoles. It's safe for use from multiple goroutines; all ops are
// serialized on a single connection.
type TelnetClient struct {
	addr     string
	password PassFn

	// connectTimeout/responseDeadline/readQuiet/bannerDrainBudget are
	// struct fields (not package consts) so tests can shrink them —
	// otherwise a test exercising the no-output-command path would block
	// for the full production deadline before observing an empty reply.
	connectTimeout    time.Duration
	responseDeadline  time.Duration
	readQuiet         time.Duration
	bannerDrainBudget time.Duration

	mu   sync.Mutex
	conn net.Conn
}

// NewTelnet builds a telnet RCON client. The connection is dialed lazily
// on the first Exec.
func NewTelnet(host string, port int, pw PassFn) *TelnetClient {
	return &TelnetClient{
		addr:              net.JoinHostPort(host, fmt.Sprint(port)),
		password:          pw,
		connectTimeout:    defaultTelnetConnectTimeout,
		responseDeadline:  defaultTelnetResponseDeadline,
		readQuiet:         defaultTelnetReadQuiet,
		bannerDrainBudget: defaultTelnetBannerDrain,
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

	out, err := c.readReplyLocked(c.responseDeadline)
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
		return fmt.Errorf("telnet rcon: resolve password: %w", err)
	}
	conn, err := net.DialTimeout("tcp", c.addr, c.connectTimeout)
	if err != nil {
		return fmt.Errorf("telnet rcon: dial %s: %w", c.addr, err)
	}
	c.conn = conn

	// Drain whatever banner/prompt the server greets with before sending
	// the password — some servers print one, some don't. This gets its
	// own short budget (bannerDrainBudget), not connectTimeout: we're
	// draining unsolicited output, not awaiting a reply, and a server
	// with no banner must not burn a budget shared with the password
	// write/read that follows — doing so left nothing for the write
	// deadline below on a real server that greets with silence.
	if _, err := c.readReplyLocked(c.bannerDrainBudget); err != nil {
		c.dropLocked()
		return fmt.Errorf("telnet rcon: drain banner: %w", err)
	}

	// Each auth step gets its own fresh deadline rather than one shared
	// deadline pinned at dial time — otherwise a slow (or merely
	// nonexistent) earlier step silently starves a later one.
	_ = conn.SetWriteDeadline(time.Now().Add(c.connectTimeout))
	if _, err := conn.Write([]byte(pw + "\n")); err != nil {
		c.dropLocked()
		return fmt.Errorf("telnet rcon: write password: %w", err)
	}
	resp, err := c.readReplyLocked(c.connectTimeout)
	if err != nil {
		c.dropLocked()
		if !errors.Is(err, io.EOF) {
			// A genuine transport failure (reset, broken pipe, timeout
			// already handled inside readReplyLocked) — not a signal about
			// the password, so don't reinterpret it as one.
			return fmt.Errorf("telnet rcon: read auth response: %w", err)
		}
		// The server closed the connection while (or instead of) replying
		// to the password. Real telnet consoles do this on a rejected
		// password — 7 Days to Die prints a rejection line and hangs up;
		// others hang up with no line at all — and a healthy console has
		// no reason to close right after a correct one. So treat the close
		// itself as the auth-failure signal even with no matching text, but
		// keep the EOF visible via %w rather than discarding it, so this
		// remains distinguishable from a real "authentication failed" line
		// for anyone inspecting the cause.
		if isTelnetAuthFailure(resp) {
			return fmt.Errorf("telnet rcon: authentication failed: %w", err)
		}
		return fmt.Errorf("telnet rcon: authentication failed (connection closed after password write): %w", err)
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
// bytes, or budget elapses since this call started — whichever comes
// first. budget is an ABSOLUTE wall-clock cap on the whole call: some
// telnet consoles (7 Days to Die's reference implementation is one) are
// a live log mirror that streams output continuously to every connected
// client, so bytes can keep arriving faster than readQuiet apart
// indefinitely — the quiet gap alone would never fire and the read would
// never return. Re-arming the per-read deadline on every chunk is fine
// as long as it's clamped to budget. The read is also bounded to
// maxTelnetReply bytes so a chatty console can't grow the buffer without
// limit while budget is still running.
//
// A timeout ending the read is the expected, successful outcome — a
// telnet console has no length prefix or terminator to signal "reply
// complete" — so only a non-timeout error (closed connection, reset) is
// returned as a failure.
func (c *TelnetClient) readReplyLocked(budget time.Duration) (string, error) {
	var out strings.Builder
	buf := make([]byte, 4096)
	hard := time.Now().Add(budget)
	deadline := hard
	for {
		d := deadline
		if d.After(hard) {
			d = hard
		}
		_ = c.conn.SetReadDeadline(d)
		n, err := c.conn.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			if out.Len() > maxTelnetReply {
				return out.String(), nil
			}
			// Once a reply has started, a short quiet period is enough to
			// call it done — no need to keep waiting out the full budget
			// on every command. Clamped to hard above, so a continuously
			// chatty console still can't keep this loop alive past budget.
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
