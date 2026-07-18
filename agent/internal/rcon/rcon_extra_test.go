package rcon

import (
	"encoding/binary"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// TestClient_AuthFailureCooldown mirrors websocket.go's
// TestWebSocketAuthFailureCooldown / battleye.go's
// TestBattlEyeAuthFailureCooldown: an explicit AUTH_RESPONSE rejection
// (id=-1) must arm a cooldown so a caller hammering Exec on a fixed timer
// (heartbeat, players) doesn't re-dial and resend the same known-bad
// password every tick — which on Source-engine games risks tripping
// sv_rcon_maxfailures.
func TestClient_AuthFailureCooldown(t *testing.T) {
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
				_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, kind, _, err := readOne(conn)
				if err != nil || kind != typeAuth {
					return
				}
				// Always reject: id=-1 never matches the client's auth id.
				writeOne(conn, -1, typeAuthResponse, "")
			}(conn)
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := New(host, mustAtoi(port), func() (string, error) { return "wrong", nil })
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

// TestClient_AuthFailureCooldownClearsOnSuccess pins the other half of the
// auth cooldown: a rejection arms it, but a later successful auth must
// CLEAR it — otherwise a password that gets fixed would keep being
// throttled by a stale failure long after the cause is gone.
func TestClient_AuthFailureCooldownClearsOnSuccess(t *testing.T) {
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
				for {
					_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					id, kind, body, err := readOne(conn)
					if err != nil {
						return
					}
					switch kind {
					case typeAuth:
						if reject.Load() {
							writeOne(conn, -1, typeAuthResponse, "")
							return
						}
						writeOne(conn, id, typeAuthResponse, "")
					case typeExecCmd:
						writeOne(conn, id, typeRespValue, body+": ok")
					}
				}
			}(conn)
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	c := New(host, mustAtoi(port), func() (string, error) { return "pw", nil })
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
