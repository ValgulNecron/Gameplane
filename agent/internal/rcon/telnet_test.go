package rcon

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
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

		chunk := bytes.Repeat([]byte("x"), 65536)
		for i := 0; i < 64; i++ { // 4 MiB total, well past the 1 MiB cap
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
	defer c.Close()

	out, err := c.Exec("dump")
	<-done
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Allow one read-chunk of slack past the cap (the bound is checked
	// after each 4KiB read, not mid-chunk).
	if len(out) > maxTelnetReply+4096 {
		t.Fatalf("reply buffer grew to %d bytes, want capped near %d", len(out), maxTelnetReply)
	}
}
