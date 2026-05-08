package rcon

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// fakeServer speaks just enough RCON to exercise the client end-to-end:
// accepts an AUTH packet with the configured password, replies with
// AUTH_RESPONSE and an empty RESPONSE_VALUE, then echoes any EXEC_CMD
// back as a RESPONSE_VALUE containing "<cmd>: ok".
func fakeServer(t *testing.T, password string) (addr string, cleanup func()) {
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
		for {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			id, kind, body, err := readOne(conn)
			if err != nil {
				return
			}
			switch kind {
			case typeAuth:
				respID := id
				if body != password {
					respID = -1
				}
				writeOne(conn, respID, typeAuthResponse, "")
			case typeExecCmd:
				writeOne(conn, id, typeRespValue, body+": ok")
			case typeRespValue:
				// sentinel — echo an empty packet with same id
				writeOne(conn, id, typeRespValue, "")
			}
		}
	}()
	return ln.Addr().String(), func() { ln.Close(); <-done }
}

func TestExec(t *testing.T) {
	addr, cleanup := fakeServer(t, "s3kret")
	defer cleanup()

	host, port, _ := net.SplitHostPort(addr)
	c := New(host, mustAtoi(port), func() (string, error) { return "s3kret", nil })
	defer c.Close()

	out, err := c.Exec("list")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if out != "list: ok" {
		t.Fatalf("unexpected body %q", out)
	}
}

func TestAuthFailure(t *testing.T) {
	addr, cleanup := fakeServer(t, "correct")
	defer cleanup()
	host, port, _ := net.SplitHostPort(addr)
	c := New(host, mustAtoi(port), func() (string, error) { return "wrong", nil })
	defer c.Close()

	if _, err := c.Exec("list"); err == nil {
		t.Fatal("expected auth failure")
	}
}

// ---- helpers mirroring the client's wire format ----

func readOne(conn net.Conn) (id, kind int32, body string, err error) {
	var hdr [4]byte
	if _, err = io.ReadFull(conn, hdr[:]); err != nil {
		return
	}
	size := binary.LittleEndian.Uint32(hdr[:])
	buf := make([]byte, size)
	if _, err = io.ReadFull(conn, buf); err != nil {
		return
	}
	id = int32(binary.LittleEndian.Uint32(buf[0:4]))
	kind = int32(binary.LittleEndian.Uint32(buf[4:8]))
	body = string(buf[8 : len(buf)-2])
	return
}

func writeOne(conn net.Conn, id, kind int32, body string) {
	payload := make([]byte, 0, 14+len(body))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(id))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(kind))
	payload = append(payload, body...)
	payload = append(payload, 0, 0)

	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint32(hdr, uint32(len(payload)))
	_, _ = conn.Write(append(hdr, payload...))
}

func mustAtoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
