package rcon

import (
	"encoding/binary"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPasswordFromFile(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		_, err := PasswordFromFile("")()
		if err == nil || !strings.Contains(err.Error(), "no rcon password") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := PasswordFromFile(filepath.Join(t.TempDir(), "missing"))()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "p")
		if err := os.WriteFile(path, []byte("  hunter2\n\n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		got, err := PasswordFromFile(path)()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != "hunter2" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestAllocID_WrapsZeroToOne(t *testing.T) {
	c := &Client{nextID: math.MaxUint32 - 1}
	_ = c.allocID() // → MaxUint32
	got := c.allocID()
	if got != 1 {
		t.Fatalf("expected wrap to 1, got %d", got)
	}
}

func TestWritePacket_TooLarge(t *testing.T) {
	c := &Client{}
	body := strings.Repeat("x", 5000)
	if err := c.writePacket(1, typeExecCmd, body); err == nil ||
		!strings.Contains(err.Error(), "too large") {
		t.Fatalf("got %v", err)
	}
}

func TestReadPacket_MalformedSize(t *testing.T) {
	// Pipe a header with size=0 → out of range.
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()
	c := &Client{conn: cli}
	go func() {
		hdr := make([]byte, 4)
		binary.LittleEndian.PutUint32(hdr, 0) // size=0
		_, _ = srv.Write(hdr)
	}()
	_ = cli.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, _, err := c.readPacket(); err == nil ||
		!strings.Contains(err.Error(), "malformed") {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureLocked_PasswordResolverError(t *testing.T) {
	c := New("127.0.0.1", 1, func() (string, error) {
		return "", errors.New("vault locked")
	})
	defer c.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureLocked(); err == nil ||
		!strings.Contains(err.Error(), "vault locked") {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureLocked_DialError(t *testing.T) {
	// 127.0.0.1:1 — port unreachable on a normal Linux box.
	c := New("127.0.0.1", 1, func() (string, error) { return "x", nil })
	defer c.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureLocked(); err == nil {
		t.Fatal("expected dial error")
	}
}

// fakePreambleServer answers the AUTH with a leading typeRespValue
// (some servers do this) followed by typeAuthResponse, exercising the
// ensureLocked loop's "skip non-auth packet" branch.
func TestEnsureLocked_PreambleBeforeAuth(t *testing.T) {
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
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		id, _, _, err := readOne(conn)
		if err != nil {
			return
		}
		// First: an unrelated RESPONSE_VALUE.
		writeOne(conn, 0, typeRespValue, "")
		// Then: the real AUTH_RESPONSE.
		writeOne(conn, id, typeAuthResponse, "")
		// Now act as a normal echo server for one more EXEC.
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		id2, kind2, body2, err := readOne(conn)
		if err != nil {
			return
		}
		if kind2 == typeExecCmd {
			writeOne(conn, id2, typeRespValue, body2+": ok")
			// Sentinel.
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			id3, _, _, _ := readOne(conn)
			writeOne(conn, id3, typeRespValue, "")
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := New(host, mustAtoi(port), func() (string, error) { return "x", nil })
	defer c.Close()
	out, err := c.Exec("hi")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if out != "hi: ok" {
		t.Fatalf("got %q", out)
	}
}

func TestExec_WriteAfterClose(t *testing.T) {
	addr, cleanup := fakeServer(t, "x")
	defer cleanup()
	host, port, _ := net.SplitHostPort(addr)
	c := New(host, mustAtoi(port), func() (string, error) { return "x", nil })
	if _, err := c.Exec("hi"); err != nil {
		t.Fatalf("first exec: %v", err)
	}
	_ = c.Close()
	// Server has already disconnected; a second Exec must reconnect or
	// surface an error cleanly without panic.
	_, _ = c.Exec("hi-again")
}
