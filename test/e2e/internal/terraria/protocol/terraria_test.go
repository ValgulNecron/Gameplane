package protocol

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// TestWriteReadString covers the 7-bit-encoded-int length prefix round-trip.
func TestWriteReadString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
	}{
		{"empty", ""},
		{"ascii", "test"},
		{"short", "a"},
		{"long", "this is a much longer string that tests multi-byte encoding"},
		{"unicode", "hello世界"},
		{"max_single_byte_length", string(make([]byte, 127))},
		{"min_double_byte_length", string(make([]byte, 128))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeString(&buf, tt.s)

			r := bytes.NewReader(buf.Bytes())
			got, err := readString(r)
			if err != nil {
				t.Fatalf("readString: %v", err)
			}
			if got != tt.s {
				t.Fatalf("round-trip failed: got %q, want %q", got, tt.s)
			}
		})
	}
}

// TestReadStringErrors covers truncated and garbage input.
func TestReadStringErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"truncated_length", []byte{0x80}}, // High bit set but no continuation
		{"truncated_data", []byte{0x05, 0x68, 0x69}}, // Says 5 bytes but only has 2
		{"overflow_shift", []byte{0x80, 0x80, 0x80, 0x80, 0x01}}, // Shift > 21
		{"length_exceeds_buffer", []byte{0x0a, 0x61}}, // Says 10 bytes but buffer is short
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.input)
			_, err := readString(r)
			if err == nil {
				t.Fatalf("readString should have failed but didn't")
			}
		})
	}
}

// TestMessageFraming covers writeMessage and readMessage round-trip.
func TestMessageFraming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msgType byte
		payload []byte
	}{
		{"empty", 0x01, []byte{}},
		{"simple", 0x06, []byte{0x00}},
		{"multi_byte", 0x03, []byte{0x01, 0x02, 0x03}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeMessage(&buf, tt.msgType, tt.payload); err != nil {
				t.Fatalf("writeMessage: %v", err)
			}

			msgType, payload, err := readMessage(&buf)
			if err != nil {
				t.Fatalf("readMessage: %v", err)
			}
			if msgType != tt.msgType {
				t.Fatalf("type mismatch: got %d, want %d", msgType, tt.msgType)
			}
			if !bytes.Equal(payload, tt.payload) {
				t.Fatalf("payload mismatch: got %v, want %v", payload, tt.payload)
			}
		})
	}
}

// TestReadMessageErrors covers truncated and malformed frames.
func TestReadMessageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"incomplete_header", []byte{0x01, 0x00}},
		{"bad_frame_length_too_small", []byte{0x01, 0x00, 0x01}}, // Length 1 < 3
		{"truncated_body", []byte{0x05, 0x00, 0x01, 0x02}}, // Says 5 bytes but only has 3
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.input)
			_, _, err := readMessage(r)
			if err == nil {
				t.Fatalf("readMessage should have failed but didn't")
			}
		})
	}
}

// TestConnectHandshakeSuccess covers a successful Connect against a fake server.
func TestConnectHandshakeSuccess(t *testing.T) {
	t.Parallel()

	// Create a listener that simulates a Terraria server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	// Run server in background.
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read the ConnectRequest.
		_, body, err := readMessage(conn)
		if err != nil {
			return
		}

		// Verify it contains the version string.
		r := bytes.NewReader(body)
		version, _ := readString(r)
		if version != DefaultVersion {
			return
		}

		// Send ContinueConnecting with slot 5.
		_ = writeMessage(conn, msgContinueConnecting, []byte{5})
	}()

	// Client connects.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, res, err := Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if res == nil {
		t.Fatalf("Connect returned nil ConnectResult")
	}
	if res.Slot != 5 {
		t.Fatalf("slot mismatch: got %d, want 5", res.Slot)
	}
	if res.Version != DefaultVersion {
		t.Fatalf("version mismatch: got %s, want %s", res.Version, DefaultVersion)
	}
}

// TestConnectPasswordRequired covers the password-prompt path.
func TestConnectPasswordRequired(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn == nil {
			return
		}
		defer conn.Close()

		// Read and discard the ConnectRequest.
		readMessage(conn)

		// Send PasswordRequired.
		_ = writeMessage(conn, msgPasswordRequired, nil)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err = Connect(ctx, listener.Addr().String())
	if !errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("Connect should return ErrPasswordRequired, got %v", err)
	}
}

// TestConnectVersionMismatch covers the version-mismatch kick with self-correction.
func TestConnectVersionMismatch(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	attempt := 0
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// Read the ConnectRequest.
			_, body, err := readMessage(conn)
			if err != nil {
				return
			}
			r := bytes.NewReader(body)
			version, _ := readString(r)

			if attempt == 0 && version == DefaultVersion {
				// First attempt: kick with the new version.
				attempt++
				var kickPayload bytes.Buffer
				kickPayload.WriteByte(1) // mode: has substitutions
				writeString(&kickPayload, "server wants {0}")
				kickPayload.WriteByte(1) // 1 substitution
				kickPayload.WriteByte(0) // sub mode: literal
				writeString(&kickPayload, "Terraria280")
				_ = writeMessage(conn, msgDisconnect, kickPayload.Bytes())
			} else if attempt == 1 && version == "Terraria280" {
				// Second attempt with corrected version: accept it.
				_ = writeMessage(conn, msgContinueConnecting, []byte{3})
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, res, err := Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer conn.Close()

	if res == nil {
		t.Fatalf("Connect returned nil ConnectResult")
	}
	if res.Version != "Terraria280" {
		t.Fatalf("version should be Terraria280, got %s", res.Version)
	}
}

// TestConnectVersionMismatchNoVersion covers a kick that does NOT name a version.
func TestConnectVersionMismatchNoVersion(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, _ := listener.Accept()
		if conn == nil {
			return
		}
		defer conn.Close()

		// Read and discard the ConnectRequest.
		readMessage(conn)

		// Send a disconnect without a version substitution (like LegacyMultiplayer.4).
		var kickPayload bytes.Buffer
		kickPayload.WriteByte(0) // mode: literal
		writeString(&kickPayload, "server rejected")
		_ = writeMessage(conn, msgDisconnect, kickPayload.Bytes())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err = Connect(ctx, listener.Addr().String())
	if err == nil {
		t.Fatalf("Connect should fail when version kick names no version")
	}
	if errors.Is(err, ErrPasswordRequired) {
		t.Fatalf("Connect should not return ErrPasswordRequired")
	}
}

// TestRequestWorldData covers a successful RequestWorldData call.
func TestRequestWorldData(t *testing.T) {
	t.Parallel()

	// Create a mock connection (pipe).
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Server goroutine: read RequestWorldData, then send WorldData.
	go func() {
		defer server.Close()
		msgType, _, err := readMessage(server)
		if err != nil {
			return
		}
		if msgType != msgRequestWorldData {
			return
		}
		// Send WorldData with non-empty payload.
		_ = writeMessage(server, msgWorldData, []byte{0x01, 0x02})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := &Conn{c: client}
	err := conn.RequestWorldData(ctx)
	if err != nil {
		t.Fatalf("RequestWorldData: %v", err)
	}
}

// TestRequestWorldDataEmpty covers the empty WorldData payload error.
func TestRequestWorldDataEmpty(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		defer server.Close()
		readMessage(server)
		// Send empty WorldData.
		_ = writeMessage(server, msgWorldData, []byte{})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := &Conn{c: client}
	err := conn.RequestWorldData(ctx)
	if err == nil || err.Error() != "empty WorldData payload" {
		t.Fatalf("RequestWorldData should reject empty payload, got %v", err)
	}
}

// TestRequestWorldDataDisconnect covers a disconnect before WorldData.
func TestRequestWorldDataDisconnect(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		defer server.Close()
		readMessage(server)
		// Send Disconnect instead.
		var payload bytes.Buffer
		payload.WriteByte(0)
		writeString(&payload, "server full")
		_ = writeMessage(server, msgDisconnect, payload.Bytes())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn := &Conn{c: client}
	err := conn.RequestWorldData(ctx)
	if err == nil {
		t.Fatalf("RequestWorldData should fail on disconnect")
	}
}

// TestParseNetworkText covers the NetworkText parser with substitutions.
func TestParseNetworkText(t *testing.T) {
	t.Parallel()

	// Build a NetworkText with one substitution.
	var payload bytes.Buffer
	payload.WriteByte(1) // mode: has substitutions
	writeString(&payload, "main text")
	payload.WriteByte(1) // 1 substitution
	payload.WriteByte(0) // sub mode: literal
	writeString(&payload, "Terraria279")

	text, subs := parseNetworkText(payload.Bytes())
	if text != "main text" {
		t.Fatalf("text mismatch: got %s", text)
	}
	if len(subs) != 1 || subs[0] != "Terraria279" {
		t.Fatalf("subs mismatch: got %v", subs)
	}
}

// TestParseNetworkTextLiteral covers a literal NetworkText (no subs).
func TestParseNetworkTextLiteral(t *testing.T) {
	t.Parallel()

	var payload bytes.Buffer
	payload.WriteByte(0) // mode: literal
	writeString(&payload, "literal text")

	text, subs := parseNetworkText(payload.Bytes())
	if text != "literal text" {
		t.Fatalf("text mismatch: got %s", text)
	}
	if len(subs) != 0 {
		t.Fatalf("subs should be empty, got %v", subs)
	}
}

// TestParseNetworkTextTruncated covers truncated/garbage NetworkText.
func TestParseNetworkTextTruncated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"mode_only", []byte{0x01}},
		{"no_sub_count", []byte{0x01, 0x04, 0x74, 0x65, 0x78, 0x74}}, // mode + short text, no count
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Best-effort: these should not panic.
			text, subs := parseNetworkText(tt.input)
			_ = text
			_ = subs
		})
	}
}

// TestConnClose verifies Close() delegates to the underlying connection.
func TestConnClose(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer server.Close()

	conn := &Conn{c: client}
	err := conn.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Verify the connection is closed by writing to it.
	_, err = client.Write([]byte{0x01})
	if err == nil {
		t.Fatalf("Write to closed connection should fail")
	}
}

// BenchmarkWriteString benchmarks the 7-bit encoding.
func BenchmarkWriteString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		writeString(&buf, "Hello, World!")
	}
}

// BenchmarkReadString benchmarks the 7-bit decoding.
func BenchmarkReadString(b *testing.B) {
	var buf bytes.Buffer
	writeString(&buf, "Hello, World!")
	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := bytes.NewReader(data)
		_, _ = readString(r)
	}
}
