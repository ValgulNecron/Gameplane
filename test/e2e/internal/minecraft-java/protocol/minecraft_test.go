package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// TestVarIntRoundTrip tests encoding and decoding of VarInt across the valid range.
func TestVarIntRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []int32{
		0, 1, 127,                          // single byte
		128, 255, 16383,                    // two bytes
		16384, 1<<20 - 1, 1<<21 - 1,        // three bytes
		1 << 21, 1<<27 - 1, 1<<28 - 1,      // four bytes
		1 << 28, (1 << 31) - 1, -1, -128,   // five bytes and negatives
	}
	for _, v := range tests {
		t.Run(fmt.Sprintf("%d", v), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			writeVarInt(&buf, v)
			// Check that encoded length never exceeds 5 bytes.
			if buf.Len() > 5 {
				t.Fatalf("VarInt %d encoded to %d bytes; want <= 5", v, buf.Len())
			}
			// Decode and verify round-trip.
			decoded, err := readVarInt(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("readVarInt failed: %v", err)
			}
			if decoded != v {
				t.Fatalf("round-trip mismatch: got %d, want %d", decoded, v)
			}
		})
	}
}

// TestVarIntTooLong tests that reading more than 5 bytes returns an error.
func TestVarIntTooLong(t *testing.T) {
	t.Parallel()
	// Construct a 6-byte VarInt (all continuation bits set, then one more).
	tooLong := []byte{0x80, 0x80, 0x80, 0x80, 0x80, 0x00}
	_, err := readVarInt(bytes.NewReader(tooLong))
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("readVarInt with 6 bytes should error; got %v", err)
	}
}

// TestStringRoundTrip tests encoding and decoding of length-prefixed strings.
func TestStringRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []string{
		"", "a", "hello", "Minecraft", "a" + strings.Repeat("b", 1000),
	}
	for _, s := range tests {
		t.Run(fmt.Sprintf("%d-chars", len(s)), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			writeString(&buf, s)
			decoded, err := readString(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("readString failed: %v", err)
			}
			if decoded != s {
				t.Fatalf("round-trip mismatch: got %q, want %q", decoded, s)
			}
		})
	}
}

// TestPingWithFakeServer tests Ping against a fake local server.
func TestPingWithFakeServer(t *testing.T) {
	t.Parallel()
	// Start a fake server that echoes a well-formed status response.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Spawn a goroutine to accept one connection and send a status response.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return // server closed
		}
		defer conn.Close()

		// Read the handshake + status request (don't validate, just drain).
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = io.ReadAll(conn)

		// Send a valid status response packet.
		status := Status{
			Version: struct {
				Name     string `json:"name"`
				Protocol int    `json:"protocol"`
			}{Name: "1.21.4", Protocol: 767},
			Players: struct {
				Max    int `json:"max"`
				Online int `json:"online"`
			}{Max: 20, Online: 1},
		}
		jsonBytes, _ := json.Marshal(status)

		// Build the response: packet ID 0x00 + JSON as a string.
		var payload bytes.Buffer
		writeString(&payload, string(jsonBytes))
		response := buildPacket(0x00, payload.Bytes())

		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_, _ = conn.Write(response)
	}()

	// Give the server goroutine time to accept.
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	st, err := Ping(ctx, addr)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if st == nil {
		t.Fatal("Ping returned nil Status")
	}
	if st.Version.Name != "1.21.4" {
		t.Fatalf("got version %q, want 1.21.4", st.Version.Name)
	}
	if st.Version.Protocol != 767 {
		t.Fatalf("got protocol %d, want 767", st.Version.Protocol)
	}
	if st.Players.Online != 1 {
		t.Fatalf("got online %d, want 1", st.Players.Online)
	}
}

// TestLoginLongUsernameRejected tests that a username > 16 characters is
// rejected before any bytes are sent to the server.
func TestLoginLongUsernameRejected(t *testing.T) {
	t.Parallel()
	longName := "gameplane-e2e-bot-toolong"
	if len(longName) <= 16 {
		t.Fatalf("test setup error: username is only %d chars", len(longName))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Call Login with a too-long username. Should fail immediately without
	// dialing the server.
	_, err := Login(ctx, "127.0.0.1:12345", 767, longName)
	if err == nil {
		t.Fatal("Login with 16+ char username should error")
	}
	if !strings.Contains(err.Error(), "at most 16") {
		t.Fatalf("error should mention 16-char limit; got: %v", err)
	}
}

// TestLoginConnectFailure tests that a dial failure is propagated.
func TestLoginConnectFailure(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Use a port that's almost certainly not listening.
	_, err := Login(ctx, "127.0.0.1:54321", 767, "bot")
	if err == nil {
		t.Fatal("Login to closed port should error")
	}
}

// TestTruncatedPacketError tests that a truncated response produces an error.
func TestTruncatedPacketError(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Server that sends incomplete data then closes.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Drain the request (handshake + status request).
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _ = io.ReadAll(conn)
		// Send only 3 bytes (an incomplete VarInt length field).
		_ = conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
		conn.Write([]byte{0x80, 0x80, 0x80})
		// Close, forcing a read error in the client.
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = Ping(ctx, addr)
	if err == nil {
		t.Fatal("Ping with truncated response should error")
	}
}

// TestGarbageResponseError tests that garbage data produces an error.
func TestGarbageResponseError(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Server that sends garbage.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, _ = io.ReadAll(conn)
		_ = conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
		// Send a packet with ID 0xFF (not 0x00) to trigger an error.
		conn.Write(buildPacket(0xFF, []byte("garbage")))
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = Ping(ctx, addr)
	if err == nil {
		t.Fatal("Ping with wrong packet ID should error")
	}
	if !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("error should mention unexpected; got: %v", err)
	}
}

// TestContextDeadlineEnforced tests that the context deadline is respected.
func TestContextDeadlineEnforced(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// Server that accepts but never sends a response.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = io.ReadAll(conn)
		// Block forever without sending anything.
		select {}
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = Ping(ctx, addr)
	if err == nil {
		t.Fatal("Ping with tight context deadline should timeout")
	}
}

// TestStringLengthValidation tests that readString rejects invalid lengths.
func TestStringLengthValidation(t *testing.T) {
	t.Parallel()
	// Construct a VarInt indicating 100 bytes of string data, but provide only 10.
	var buf bytes.Buffer
	writeVarInt(&buf, 100)
	buf.Write([]byte("short"))

	_, err := readString(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("readString should error on truncated data")
	}
}
