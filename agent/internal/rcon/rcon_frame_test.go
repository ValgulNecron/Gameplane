package rcon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// frameWriteConn observes the writes submitted to the socket, independently
// of TCP coalescing. Negative writeN means accept the whole write.
type frameWriteConn struct {
	net.Conn
	writes   [][]byte
	writeN   int
	writeErr error
	reads    int
	closed   bool
}

func (c *frameWriteConn) Write(p []byte) (int, error) {
	c.writes = append(c.writes, bytes.Clone(p))
	n := len(p)
	if c.writeN >= 0 {
		n = c.writeN
	}
	return n, c.writeErr
}

func (c *frameWriteConn) Read([]byte) (int, error) {
	c.reads++
	return 0, errors.New("unexpected response read after failed write")
}

func (c *frameWriteConn) Close() error {
	c.closed = true
	return nil
}

func (*frameWriteConn) SetDeadline(time.Time) error { return nil }

func TestWritePacketSubmitsOneCompleteFrame(t *testing.T) {
	for _, test := range []struct {
		name string
		kind uint32
		body string
	}{
		{"auth", typeAuth, "test-password"},
		{"empty command", typeExecCmd, ""},
		{"command", typeExecCmd, "list"},
		{"UTF8 command", typeExecCmd, "say café 世界"},
		{"maximum payload", typeExecCmd, strings.Repeat("x", 4096-minPacketSize)},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn := &frameWriteConn{writeN: -1}
			client := &Client{conn: conn}
			const id = uint32(0x01020304)
			if err := client.writePacket(id, test.kind, test.body); err != nil {
				t.Fatal(err)
			}
			if len(conn.writes) != 1 {
				t.Fatalf("frame submitted as %d writes, want one", len(conn.writes))
			}
			frame := conn.writes[0]
			if len(frame) != 14+len(test.body) || binary.LittleEndian.Uint32(frame[:4]) != uint32(len(frame)-4) {
				t.Fatalf("invalid wire size: %d bytes", len(frame))
			}
			if binary.LittleEndian.Uint32(frame[4:8]) != id || binary.LittleEndian.Uint32(frame[8:12]) != test.kind {
				t.Fatal("request identity or type changed")
			}
			if string(frame[12:len(frame)-2]) != test.body || !bytes.Equal(frame[len(frame)-2:], []byte{0, 0}) {
				t.Fatal("body or terminators changed")
			}
		})
	}
}

func TestWritePacketRejectsOversizeBeforeWrite(t *testing.T) {
	conn := &frameWriteConn{writeN: -1}
	client := &Client{conn: conn}
	if err := client.writePacket(1, typeExecCmd, strings.Repeat("x", 4097-minPacketSize)); err == nil {
		t.Fatal("oversized payload accepted")
	}
	if len(conn.writes) != 0 {
		t.Fatal("oversized payload reached the connection")
	}
}

func TestExecFailedFrameWriteDropsConnectionWithoutReplay(t *testing.T) {
	writeErr := errors.New("injected socket write failure")
	for _, test := range []struct {
		name string
		n    int
		err  error
		want error
	}{
		{"short successful write", 4, nil, io.ErrShortWrite},
		{"write failure", 0, writeErr, writeErr},
		{"partial write failure", 4, writeErr, writeErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			conn := &frameWriteConn{writeN: test.n, writeErr: test.err}
			// An existing connection represents a completed authentication.
			client := &Client{conn: conn}
			out, err := client.Exec("whitelist add fixture")
			if !errors.Is(err, test.want) || out != "" {
				t.Fatalf("lost write failure: output=%q err=%v", out, err)
			}
			if len(conn.writes) != 1 || conn.reads != 0 || !conn.closed || client.conn != nil {
				t.Fatalf("failed write was replayed, read, or reused: writes=%d reads=%d closed=%v", len(conn.writes), conn.reads, conn.closed)
			}
		})
	}
}

func TestExecStrictSingleReadServerSupportsSequentialCommands(t *testing.T) {
	server, socket := net.Pipe()
	// Use an already-authenticated pipe so each Write maps deterministically
	// to one server Read. A header-only Write must fail, not pass by chance
	// because TCP happened to coalesce it with the following payload Write.
	client := &Client{conn: socket, execDeadline: 2 * time.Second}
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	done := make(chan error, 1)
	go func() {
		defer server.Close()
		for i, command := range []string{"list", "save-all flush"} {
			_ = server.SetDeadline(time.Now().Add(2 * time.Second))
			buf := make([]byte, 1460)
			n, err := server.Read(buf)
			if err != nil {
				done <- err
				return
			}
			if n < 14 || int(binary.LittleEndian.Uint32(buf[:4])) != n-4 {
				done <- fmt.Errorf("strict server rejected incomplete frame of %d bytes", n)
				return
			}
			id := binary.LittleEndian.Uint32(buf[4:8])
			if id != uint32(i+1) || binary.LittleEndian.Uint32(buf[8:12]) != typeExecCmd ||
				string(buf[12:n-2]) != command || !bytes.Equal(buf[n-2:n], []byte{0, 0}) {
				done <- errors.New("strict server received an unexpected command frame")
				return
			}
			writeOne(server, int32(id), typeRespValue, command+": ok")
		}
		done <- nil
	}()
	for i, command := range []string{"list", "save-all flush"} {
		out, err := client.Exec(command)
		if err != nil || out != command+": ok" {
			t.Fatalf("command %d: output=%q err=%v", i+1, out, err)
		}
		if i == 0 && client.conn != socket {
			t.Fatal("healthy persistent connection was discarded after first command")
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
