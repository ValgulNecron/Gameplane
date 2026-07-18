package rcon

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// shrinkDeadlines shrinks every one of TelnetClient's tunable deadlines
// to test-friendly sizes, preserving the PRODUCTION ratio between
// connectTimeout and responseDeadline (both scaled down by the same
// factor, staying equal). Shrinking only responseDeadline while leaving
// connectTimeout at its 5s default previously decoupled the banner
// drain from the password-write deadline in tests, masking a bug where
// the two shared a budget in production (both defaulted to 5s) but
// never did in the test suite.
func shrinkDeadlines(c *TelnetClient) {
	c.connectTimeout = 200 * time.Millisecond
	c.responseDeadline = 200 * time.Millisecond
	c.bannerDrainBudget = 50 * time.Millisecond
	c.readQuiet = 50 * time.Millisecond
}

// fakeTelnetServer speaks just enough of a line-based telnet console to
// exercise TelnetClient end-to-end: sends a banner, expects a password
// line, replies with success/failure text, then echoes each subsequent
// command line back as "<cmd>: ok" — except "quiet", which gets no reply
// at all, exercising the no-output/response-deadline path.
func fakeTelnetServer(t *testing.T, password, banner string) (addr string, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		if banner != "" {
			_, _ = conn.Write([]byte(banner))
		}

		scan := bufio.NewScanner(conn)
		if !scan.Scan() {
			return
		}
		if scan.Text() != password {
			_, _ = conn.Write([]byte("Incorrect password.\n"))
			return
		}
		_, _ = conn.Write([]byte("Logon successful.\n"))

		for scan.Scan() {
			cmd := scan.Text()
			if cmd == "quiet" {
				continue
			}
			_, _ = conn.Write([]byte(cmd + ": ok\n"))
		}
	}()
	return ln.Addr().String(), func() { ln.Close(); <-done }
}

// fakeTelnetServerSilentReject speaks like fakeTelnetServer but, on a wrong
// password, closes the connection immediately with no rejection text at
// all — covering consoles that hang up on bad auth without printing
// anything, as opposed to fakeTelnetServer's "Incorrect password." line.
func fakeTelnetServerSilentReject(t *testing.T, password string) (addr string, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scan := bufio.NewScanner(conn)
		if !scan.Scan() {
			return
		}
		if scan.Text() != password {
			return // hang up with no text, unlike fakeTelnetServer
		}
		_, _ = conn.Write([]byte("Logon successful.\n"))
	}()
	return ln.Addr().String(), func() { ln.Close(); <-done }
}

func TestTelnetClient_Exec(t *testing.T) {
	cases := []struct {
		name       string
		banner     string
		serverPW   string
		clientPW   string
		cmd        string
		silent     bool // use fakeTelnetServerSilentReject instead of fakeTelnetServer
		wantErr    bool
		wantErrSub string
		wantErrIs  error // when set, err must also satisfy errors.Is(err, wantErrIs)
		wantOutSub string
	}{
		{
			name:       "successful command returns the server's echo",
			banner:     "Welcome\n",
			serverPW:   "s3kret",
			clientPW:   "s3kret",
			cmd:        "help",
			wantOutSub: "help: ok",
		},
		{
			// The fake server prints "Incorrect password." then hangs up,
			// so the auth-response read ends in EOF with that text already
			// captured — isTelnetAuthFailure must match it before the EOF
			// is allowed to fall through as a bare transport error.
			name:       "wrong password fails to authenticate",
			banner:     "Welcome\n",
			serverPW:   "s3kret",
			clientPW:   "nope",
			cmd:        "help",
			wantErr:    true,
			wantErrSub: "authentication failed",
			wantErrIs:  io.EOF,
		},
		{
			// The server hangs up on a bad password with NO text at all —
			// the auth-response read ends in EOF with an empty reply. This
			// must still surface as an auth failure (a healthy console has
			// no reason to close right after a correct password), not a
			// bare "read auth response: EOF".
			name:       "wrong password with no rejection text still fails to authenticate",
			serverPW:   "s3kret",
			clientPW:   "nope",
			cmd:        "help",
			silent:     true,
			wantErr:    true,
			wantErrSub: "authentication failed",
			wantErrIs:  io.EOF,
		},
		{
			name:       "silent command returns empty output, not an error",
			banner:     "Welcome\n",
			serverPW:   "s3kret",
			clientPW:   "s3kret",
			cmd:        "quiet",
			wantOutSub: "",
		},
		{
			// Covers BLOCKER 2: a server that prints no greeting at all
			// must not make the banner drain burn a deadline shared with
			// the password write that follows. Uses shrinkDeadlines,
			// which keeps connectTimeout == responseDeadline — the exact
			// production ratio that let this bug hide before.
			name:       "auth succeeds against a server with no banner",
			banner:     "",
			serverPW:   "s3kret",
			clientPW:   "s3kret",
			cmd:        "help",
			wantOutSub: "help: ok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var addr string
			var cleanup func()
			if tc.silent {
				addr, cleanup = fakeTelnetServerSilentReject(t, tc.serverPW)
			} else {
				addr, cleanup = fakeTelnetServer(t, tc.serverPW, tc.banner)
			}
			defer cleanup()

			host, port, _ := net.SplitHostPort(addr)
			c := NewTelnet(host, mustAtoi(port), func() (string, error) { return tc.clientPW, nil })
			// Shrink the deadlines so the no-output case doesn't cost the
			// full production response deadline in every test run.
			shrinkDeadlines(c)
			defer c.Close()

			out, err := c.Exec(tc.cmd)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil (out=%q)", out)
				}
				if tc.wantErrSub != "" && !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("error %q missing substring %q", err.Error(), tc.wantErrSub)
				}
				if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
					t.Fatalf("error %q does not wrap %v (the underlying transport cause must survive via %%w)", err.Error(), tc.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(out, tc.wantOutSub) {
				t.Fatalf("output %q missing substring %q", out, tc.wantOutSub)
			}
		})
	}
}

// TestTelnetClient_ReusesConnectionAcrossCalls verifies that a second Exec
// reuses the already-authenticated connection instead of redialing and
// re-authenticating for every command.
func TestTelnetClient_ReusesConnectionAcrossCalls(t *testing.T) {
	addr, cleanup := fakeTelnetServer(t, "s3kret", "")
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	calls := 0
	c := NewTelnet(host, mustAtoi(port), func() (string, error) {
		calls++
		return "s3kret", nil
	})
	shrinkDeadlines(c)
	defer c.Close()

	for _, cmd := range []string{"ping", "pong"} {
		out, err := c.Exec(cmd)
		if err != nil {
			t.Fatalf("exec %q: unexpected error: %v", cmd, err)
		}
		if !strings.Contains(out, cmd+": ok") {
			t.Fatalf("exec %q: output %q missing expected echo", cmd, out)
		}
	}
	if calls != 1 {
		t.Fatalf("expected exactly one password resolution across both calls, got %d", calls)
	}
}

// fakeChatterServer simulates a live-log-mirror telnet console — 7 Days to
// Die's reference implementation is exactly this: once connected (no auth
// needed for this test), it streams a line every 10ms indefinitely, faster
// than TelnetClient's readQuiet gap, until the test tears the listener
// down. The ONLY thing that can end a read against a server like this is
// an absolute deadline — the quiet-gap check alone never fires.
func fakeChatterServer(t *testing.T, password string) (addr string, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scan := bufio.NewScanner(conn)
		if !scan.Scan() || scan.Text() != password {
			return
		}
		_, _ = conn.Write([]byte("Logon successful.\n"))

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if _, err := conn.Write([]byte("log line\n")); err != nil {
					return
				}
			}
		}
	}()
	return ln.Addr().String(), func() { close(stop); ln.Close(); <-done }
}

// TestTelnetClient_ExecReturnsOnContinuousStream covers BLOCKER 1: a telnet
// console that never goes quiet (a live log mirror) must not hang Exec
// forever. readQuiet is deliberately set LONGER than the server's write
// interval so the quiet-gap check can never be what ends the read — only
// the absolute (budget-clamped) deadline in readReplyLocked can.
func TestTelnetClient_ExecReturnsOnContinuousStream(t *testing.T) {
	addr, cleanup := fakeChatterServer(t, "s3kret")
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	c := NewTelnet(host, mustAtoi(port), func() (string, error) { return "s3kret", nil })
	c.connectTimeout = 200 * time.Millisecond
	c.responseDeadline = 200 * time.Millisecond
	c.bannerDrainBudget = 50 * time.Millisecond
	// Longer than the server's 10ms write interval, so the quiet-gap exit
	// can never trigger — only the hard, budget-clamped deadline can.
	c.readQuiet = 2 * time.Second
	defer c.Close()

	execDone := make(chan error, 1)
	go func() {
		_, err := c.Exec("status")
		execDone <- err
	}()

	select {
	case err := <-execDone:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Exec did not return against a continuously-streaming console — the read loop hung past its deadline")
	}
}

// TestTelnetClient_ReadReplyBoundsBufferSize covers the buffer half of
// BLOCKER 1: a console dumping far more than maxTelnetReply in one go,
// faster than readQuiet apart, must not grow the reply buffer without
// bound. readQuiet is set longer than the whole burst so only the buffer
// cap — not the quiet gap — can be what ends the read early.
func TestTelnetClient_ReadReplyBoundsBufferSize(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan struct{})
	stop := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scan := bufio.NewScanner(conn)
		if !scan.Scan() {
			return
		}
		_, _ = conn.Write([]byte("Logon successful.\n"))

		// Drain (and discard) anything the client sends for the rest of
		// this connection's life. The client's Exec below writes "dump\n",
		// which this fake server otherwise never reads back; left unread,
		// those bytes would still be sitting in the server's receive
		// buffer whenever the server closes — and closing a socket with
		// unread inbound data makes the kernel send a RST instead of a
		// clean FIN/EOF. That RST is indistinguishable from a genuine
		// transport error to whatever the client happens to be reading at
		// the time, which was part of what made this test flaky.
		go func() { _, _ = io.Copy(io.Discard, conn) }()

		// Forcibly close the connection once the test signals stop, to
		// unblock a Write that's currently blocked (or about to block) —
		// closing a net.Conn from another goroutine is the standard way
		// to cancel an in-flight blocking call on it.
		go func() {
			<-stop
			_ = conn.Close()
		}()

		// Keep writing until told to stop, rather than a fixed number of
		// chunks. The client, by design, stops draining once it hits
		// maxTelnetReply and returns from Exec well before anywhere near
		// 4 MiB could be sent — so whether a *fixed*-size write loop ever
		// finishes silently on its own (instead of blocking on a full send
		// buffer) depends on kernel socket-buffer sizes. That
		// nondeterminism was the actual source of the flake: it raced the
		// server closing (whether cleanly, on loop completion, or via an
		// unread-data RST) against the client's own in-flight read, which
		// could observe "connection reset by peer" instead of returning
		// cleanly at the buffer cap. Looping until stop is signaled means
		// this server can only close AFTER the test does so explicitly —
		// which it does only once Exec has already returned.
		chunk := bytes.Repeat([]byte("x"), 65536)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := conn.Write(chunk); err != nil {
				return
			}
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := NewTelnet(host, mustAtoi(port), func() (string, error) { return "s3kret", nil })
	c.connectTimeout = 2 * time.Second
	c.responseDeadline = 2 * time.Second
	c.bannerDrainBudget = 200 * time.Millisecond
	// Longer than the whole burst can possibly take on loopback, so only
	// the buffer cap — not the quiet gap or the hard deadline — ends the
	// read before it would otherwise grow past maxTelnetReply.
	c.readQuiet = 2 * time.Second

	out, err := c.Exec("dump")
	// Only now signal the fake server to stop, and close the client side.
	// Both of TelnetClient's reads above (the auth response, then this
	// Exec's reply) have already returned via the buffer cap under test by
	// the time Exec returns, so neither can be affected by the server
	// closing — that race is exactly what closing stop only here (after
	// Exec has already returned) rules out.
	close(stop)
	_ = c.Close()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("fake server goroutine never returned after being signaled to stop")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Allow one read-chunk of slack past the cap (the bound is checked
	// after each 4KiB read, not mid-chunk).
	if len(out) > maxTelnetReply+4096 {
		t.Fatalf("reply buffer grew to %d bytes, want capped near %d", len(out), maxTelnetReply)
	}
}

// TestTelnetClient_AuthFailureCooldown mirrors websocket.go's
// TestWebSocketAuthFailureCooldown / battleye.go's
// TestBattlEyeAuthFailureCooldown: a rejected password must arm a cooldown
// so a caller hammering Exec on a fixed timer (heartbeat, players) doesn't
// re-dial and resend the same known-bad password every tick.
func TestTelnetClient_AuthFailureCooldown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	var connCount int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&connCount, 1)
			go func(conn net.Conn) {
				defer conn.Close()
				scan := bufio.NewScanner(conn)
				if !scan.Scan() {
					return
				}
				// Always reject, regardless of what's sent, then hang up —
				// mirrors fakeTelnetServer's rejection behavior.
				_, _ = conn.Write([]byte("Incorrect password.\n"))
			}(conn)
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := NewTelnet(host, mustAtoi(port), func() (string, error) { return "wrong", nil })
	shrinkDeadlines(c)
	c.authFailureCooldown = 100 * time.Millisecond
	defer c.Close()

	_, err1 := c.Exec("cmd1")
	if !errors.Is(err1, ErrAuth) {
		t.Fatalf("first Exec expected ErrAuth, got %v", err1)
	}
	if got := atomic.LoadInt32(&connCount); got != 1 {
		t.Fatalf("expected 1 connection after first Exec, got %d", got)
	}

	// Within the cooldown window: must NOT re-dial with the same
	// known-bad password.
	_, err2 := c.Exec("cmd2")
	if !errors.Is(err2, ErrAuth) {
		t.Fatalf("second Exec (within cooldown) expected ErrAuth, got %v", err2)
	}
	if got := atomic.LoadInt32(&connCount); got != 1 {
		t.Fatalf("expected still 1 connection during the cooldown (no re-dial), got %d", got)
	}

	time.Sleep(150 * time.Millisecond)

	// After the cooldown expires: must attempt to re-dial (and fail again,
	// since the password is still wrong).
	_, err3 := c.Exec("cmd3")
	if !errors.Is(err3, ErrAuth) {
		t.Fatalf("third Exec (after cooldown) expected ErrAuth, got %v", err3)
	}
	if got := atomic.LoadInt32(&connCount); got != 2 {
		t.Fatalf("expected 2 connections after the cooldown expired, got %d", got)
	}
}

// TestTelnetClient_AuthFailureCooldownClearsOnSuccess pins the other half
// of the auth cooldown: a rejection arms it, but a later successful auth
// must CLEAR it — otherwise a password that gets fixed would keep being
// throttled by a stale failure long after the cause is gone.
func TestTelnetClient_AuthFailureCooldownClearsOnSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	var reject atomic.Bool
	reject.Store(true)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				scan := bufio.NewScanner(conn)
				if !scan.Scan() {
					return
				}
				if reject.Load() {
					_, _ = conn.Write([]byte("Incorrect password.\n"))
					return
				}
				_, _ = conn.Write([]byte("Logon successful.\n"))
				for scan.Scan() {
					_, _ = conn.Write([]byte(scan.Text() + ": ok\n"))
				}
			}(conn)
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := NewTelnet(host, mustAtoi(port), func() (string, error) { return "pw", nil })
	shrinkDeadlines(c)
	c.authFailureCooldown = 10 * time.Millisecond
	defer c.Close()

	// A rejection arms the cooldown.
	if _, err := c.Exec("cmd"); !errors.Is(err, ErrAuth) {
		t.Fatalf("first Exec err = %v, want ErrAuth", err)
	}
	c.mu.Lock()
	armed := !c.lastAuthFailure.IsZero()
	c.mu.Unlock()
	if !armed {
		t.Fatal("a rejected auth must arm the cooldown")
	}

	// Let it lapse, then let the server accept: the success must clear it.
	time.Sleep(15 * time.Millisecond)
	reject.Store(false)
	out, err := c.Exec("cmd")
	if err != nil {
		t.Fatalf("Exec after the cooldown lapsed should succeed: %v", err)
	}
	if !strings.Contains(out, "cmd: ok") {
		t.Fatalf("unexpected output %q", out)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.lastAuthFailure.IsZero() {
		t.Error("a successful auth must clear the auth cooldown, not leave it armed")
	}
}
