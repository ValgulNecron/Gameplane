package rcon

import (
	"net"
	"strings"
	"testing"
	"time"
)

// TestExec_WriteFailsWhenServerGone forces writePacket to error by
// closing the server side mid-stream — exercises the dropLocked branch
// after a successful AUTH but failing EXEC.
func TestExec_WriteFailsWhenServerGone(t *testing.T) {
	// Custom server that authenticates then immediately closes the
	// connection so the real command write fails on the next packet.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		id, _, _, _ := readOne(conn)
		writeOne(conn, id, typeAuthResponse, "")
		_ = conn.Close()
	}()
	defer ln.Close()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := New(host, mustAtoi(port), func() (string, error) { return "x", nil })
	defer c.Close()

	if _, err := c.Exec("hi"); err == nil {
		t.Fatal("expected Exec to fail after server close")
	}
}

// TestExec_ResponseThenCloseReturnsOutput is the regression test for the
// Minecraft "EOF" bug: Minecraft answers the command and then closes the
// socket per command. The client (which no longer sends a sentinel) must read
// and return the command's output, not surface the close as an error.
func TestExec_ResponseThenCloseReturnsOutput(t *testing.T) {
	// Server authenticates, reads the (single) EXEC, answers it echoing the
	// request id, then closes — mirroring real Minecraft RCON behaviour.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		// AUTH
		id, _, _, _ := readOne(conn)
		writeOne(conn, id, typeAuthResponse, "")
		// EXEC -> answer -> close. The client sends no sentinel packet.
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		execID, _, body, _ := readOne(conn)
		writeOne(conn, execID, typeRespValue, body+": partial")
		_ = conn.Close()
	}()
	defer ln.Close()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := New(host, mustAtoi(port), func() (string, error) { return "x", nil })
	defer c.Close()
	out, err := c.Exec("hi")
	if err != nil {
		t.Fatalf("expected output despite the close, got err=%v", err)
	}
	if out != "hi: partial" {
		t.Fatalf("unexpected body %q", out)
	}
}

// TestExec_ErrorBeforeAnyResponse confirms that a connection that drops
// before the command's response arrives is still surfaced as a (wrapped)
// error — the graceful end-of-stream only applies once we have output.
func TestExec_ErrorBeforeAnyResponse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		// AUTH succeeds.
		id, _, _, _ := readOne(conn)
		writeOne(conn, id, typeAuthResponse, "")
		// Consume the EXEC but answer nothing, then close immediately.
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, _, _ = readOne(conn)
		_ = conn.Close()
	}()
	defer ln.Close()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := New(host, mustAtoi(port), func() (string, error) { return "x", nil })
	defer c.Close()
	_, err = c.Exec("hi")
	if err == nil {
		t.Fatal("expected an error when the server closes before responding")
	}
	if !strings.Contains(err.Error(), "rcon exec") {
		t.Fatalf("error should be wrapped with command context, got %v", err)
	}
}

// TestExec_DeadlineOnHungServer — a server that authenticates then never
// answers the command must not block Exec forever. With a short
// execDeadline, Exec returns a timeout error promptly instead of holding
// the client mutex (and every caller that shares it) indefinitely.
func TestExec_DeadlineOnHungServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Complete AUTH, then go silent — never answer the command.
		id, _, _, _ := readOne(conn)
		writeOne(conn, id, typeAuthResponse, "")
		time.Sleep(2 * time.Second)
		_ = conn.Close()
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := New(host, mustAtoi(port), func() (string, error) { return "x", nil })
	c.execDeadline = 100 * time.Millisecond
	defer c.Close()

	start := time.Now()
	if _, err := c.Exec("hi"); err == nil {
		t.Fatal("expected Exec to time out against a hung server")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Exec blocked %v, expected to return near the 100ms deadline", elapsed)
	}
}

// TestExec_ReuseConnectionAcrossCalls confirms the second Exec doesn't
// re-AUTH (covered branch in ensureLocked when c.conn != nil).
func TestExec_ReuseConnectionAcrossCalls(t *testing.T) {
	addr, cleanup := fakeServer(t, "x")
	defer cleanup()
	host, port, _ := net.SplitHostPort(addr)
	c := New(host, mustAtoi(port), func() (string, error) { return "x", nil })
	defer c.Close()

	if _, err := c.Exec("first"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := c.Exec("second"); err != nil {
		t.Fatalf("second: %v", err)
	}
}
