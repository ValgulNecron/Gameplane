package rcon

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"
)

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

func TestTelnetClient_Exec(t *testing.T) {
	cases := []struct {
		name       string
		serverPW   string
		clientPW   string
		cmd        string
		wantErr    bool
		wantErrSub string
		wantOutSub string
	}{
		{
			name:       "successful command returns the server's echo",
			serverPW:   "s3kret",
			clientPW:   "s3kret",
			cmd:        "help",
			wantOutSub: "help: ok",
		},
		{
			name:       "wrong password fails to authenticate",
			serverPW:   "s3kret",
			clientPW:   "nope",
			cmd:        "help",
			wantErr:    true,
			wantErrSub: "authentication failed",
		},
		{
			name:       "silent command returns empty output, not an error",
			serverPW:   "s3kret",
			clientPW:   "s3kret",
			cmd:        "quiet",
			wantOutSub: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr, cleanup := fakeTelnetServer(t, tc.serverPW, "Welcome\n")
			defer cleanup()

			host, port, _ := net.SplitHostPort(addr)
			c := NewTelnet(host, mustAtoi(port), func() (string, error) { return tc.clientPW, nil })
			// Shrink the deadlines so the no-output case doesn't cost the
			// full production response deadline in every test run.
			c.responseDeadline = 200 * time.Millisecond
			c.readQuiet = 50 * time.Millisecond
			defer c.Close()

			out, err := c.Exec(tc.cmd)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil (out=%q)", out)
				}
				if tc.wantErrSub != "" && !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("error %q missing substring %q", err.Error(), tc.wantErrSub)
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
	c.responseDeadline = 200 * time.Millisecond
	c.readQuiet = 50 * time.Millisecond
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
