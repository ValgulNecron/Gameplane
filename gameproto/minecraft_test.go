package gameproto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

// TestClassifyMinecraftStatus tests that a status-ping handshake is classified correctly.
func TestClassifyMinecraftStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		data          []byte
		expectKind    Kind
		expectVersion int32
		expectState   int32
		expectErr     bool
	}{
		{
			name:          "valid status ping",
			data:          buildMinecraftHandshake(-1, "localhost", 25565, 1),
			expectKind:    Status,
			expectVersion: -1,
			expectState:   1,
			expectErr:     false,
		},
		{
			name:          "status ping with real protocol version",
			data:          buildMinecraftHandshake(761, "example.com", 25565, 1),
			expectKind:    Status,
			expectVersion: 761,
			expectState:   1,
			expectErr:     false,
		},
		{
			name:          "truncated length",
			data:          []byte{0xFF},
			expectKind:    Unknown,
			expectErr:     true,
		},
		{
			name:          "truncated frame",
			data:          []byte{0x05, 0x00, 0x00}, // length=5 but only 2 bytes of data
			expectKind:    Unknown,
			expectErr:     true,
		},
		{
			name:          "empty input",
			data:          []byte{},
			expectKind:    Unknown,
			expectErr:     true,
		},
		{
			name:          "huge packet size",
			data:          []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x7F}, // 2GB size
			expectKind:    Unknown,
			expectErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, result, err := ClassifyMinecraft(bytes.NewReader(tt.data))

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if kind != tt.expectKind {
				t.Errorf("expected kind %v, got %v", tt.expectKind, kind)
			}

			if !tt.expectErr && result != nil {
				if result.ProtocolVersion != tt.expectVersion {
					t.Errorf("expected version %d, got %d", tt.expectVersion, result.ProtocolVersion)
				}
				if result.NextState != tt.expectState {
					t.Errorf("expected state %d, got %d", tt.expectState, result.NextState)
				}
			}
		})
	}
}

// TestClassifyMinecraftLogin tests that a login handshake is classified correctly.
func TestClassifyMinecraftLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		data       []byte
		expectKind Kind
		expectErr  bool
	}{
		{
			name:       "valid login",
			data:       buildMinecraftHandshake(761, "localhost", 25565, 2),
			expectKind: Join,
			expectErr:  false,
		},
		{
			name:       "login with different server",
			data:       buildMinecraftHandshake(340, "mc.example.com", 25566, 2),
			expectKind: Join,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, _, err := ClassifyMinecraft(bytes.NewReader(tt.data))

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if kind != tt.expectKind {
				t.Errorf("expected kind %v, got %v", tt.expectKind, kind)
			}
		})
	}
}

// TestMinecraftConsumedBytes tests that consumed bytes can be replayed.
func TestMinecraftConsumedBytes(t *testing.T) {
	t.Parallel()

	// Build a valid handshake with known consumed bytes.
	data := buildMinecraftHandshake(761, "localhost", 25565, 2)

	kind, result, err := ClassifyMinecraft(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != Join {
		t.Fatalf("expected kind Join, got %v", kind)
	}
	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	// Consumed + remaining should equal original
	if len(result.Consumed) != len(data) {
		t.Errorf("consumed %d != len(data) %d", len(result.Consumed), len(data))
	}

	// Consumed must be non-empty
	if len(result.Consumed) == 0 {
		t.Errorf("consumed should be non-empty")
	}

	// Consumed bytes should match the original data
	if !bytes.Equal(result.Consumed, data) {
		t.Errorf("consumed bytes do not match original data")
	}
}

// TestBuildMinecraftStatusResponse tests response building.
func TestBuildMinecraftStatusResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		jsonData string
		expectOK bool
	}{
		{
			name:     "valid JSON",
			jsonData: `{"version":{"name":"1.20.1","protocol":763},"players":{"max":20,"online":1}}`,
			expectOK: true,
		},
		{
			name:     "simple JSON",
			jsonData: `{}`,
			expectOK: true,
		},
		{
			name:     "JSON with quotes",
			jsonData: `{"description":{"text":"Hello \"World\""}}`,
			expectOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := BuildMinecraftStatusResponse(tt.jsonData)

			if !tt.expectOK && err == nil {
				t.Errorf("expected error, got none")
			}
			if tt.expectOK && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectOK && len(data) > 0 {
				// Verify we can read it back as a valid packet.
				kind, result, err := ClassifyMinecraftStatusResponse(data)
				if err != nil {
					t.Errorf("failed to parse built response: %v", err)
				}
				if kind != "status_response" {
					t.Errorf("expected kind 'status_response', got %s", kind)
				}
				if result != tt.jsonData {
					t.Errorf("roundtrip failed: expected %q, got %q", tt.jsonData, result)
				}
			}
		})
	}
}

// TestBuildMinecraftLoginDisconnect tests disconnect packet building.
func TestBuildMinecraftLoginDisconnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason string
	}{
		{
			name:   "simple reason",
			reason: "Server is full",
		},
		{
			name:   "reason with quotes",
			reason: `Server is "offline"`,
		},
		{
			name:   "reason with newline",
			reason: "Line 1\nLine 2",
		},
		{
			name:   "reason with backslash",
			reason: `Path: C:\Users\test`,
		},
		{
			name:   "empty reason",
			reason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := BuildMinecraftLoginDisconnect(tt.reason)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(data) == 0 {
				t.Errorf("expected non-empty packet")
			}

			// Verify the packet is properly framed.
			br := bytes.NewReader(data)
			length, err := readMinecraftVarInt(br)
			if err != nil {
				t.Errorf("failed to read frame length: %v", err)
			}
			if length <= 0 {
				t.Errorf("invalid frame length: %d", length)
			}
		})
	}
}

// TestMinecraftVarIntRoundtrip tests VarInt encoding/decoding.
func TestMinecraftVarIntRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []int32{
		0,
		1,
		127,
		128,
		255,
		256,
		16383,
		16384,
		2097151,
		2097152,
		268435455,
		-1,
		-128,
		-129,
	}

	for _, v := range tests {
		t.Run(fmt.Sprintf("value=%d", v), func(t *testing.T) {
			var buf bytes.Buffer
			writeMinecraftVarInt(&buf, v)

			result, err := readMinecraftVarInt(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Errorf("read failed: %v", err)
			}

			if result != v {
				t.Errorf("expected %d, got %d", v, result)
			}
		})
	}
}

// TestMinecraftStringRoundtrip tests string encoding/decoding.
func TestMinecraftStringRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"hello",
		"localhost",
		"example.com",
		"192.168.1.1",
		"a very long string with many characters to test encoding",
		"special chars: !@#$%^&*()",
	}

	for _, s := range tests {
		t.Run(fmt.Sprintf("string=%s", s), func(t *testing.T) {
			var buf bytes.Buffer
			err := writeMinecraftString(&buf, s)
			if err != nil {
				t.Errorf("write failed: %v", err)
			}

			result, err := readMinecraftString(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Errorf("read failed: %v", err)
			}

			if result != s {
				t.Errorf("expected %q, got %q", s, result)
			}
		})
	}
}

// TestMinecraftStringTooLong tests that oversized strings are rejected.
func TestMinecraftStringTooLong(t *testing.T) {
	t.Parallel()

	// Attempt to encode a string larger than the limit.
	longString := make([]byte, 40000)
	for i := range longString {
		longString[i] = 'a'
	}

	var buf bytes.Buffer
	err := writeMinecraftString(&buf, string(longString))
	if err == nil {
		t.Errorf("expected error for oversized string, got none")
	}
}

// TestClassifyMinecraftInvalidState tests unknown next states.
func TestClassifyMinecraftInvalidState(t *testing.T) {
	t.Parallel()

	// Build a handshake with next state = 99 (invalid).
	data := buildMinecraftHandshake(761, "localhost", 25565, 99)
	kind, result, err := ClassifyMinecraft(bytes.NewReader(data))

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid states should return Unknown.
	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}

	// But the result should still be populated.
	if result == nil {
		t.Errorf("expected result to be populated")
	}
	if result.NextState != 99 {
		t.Errorf("expected next state 99, got %d", result.NextState)
	}
}

// TestClassifyMinecraftZeroLength tests zero packet length (invalid).
func TestClassifyMinecraftZeroLength(t *testing.T) {
	t.Parallel()

	// VarInt encoding of 0 is just [0x00].
	data := []byte{0x00}
	kind, _, err := ClassifyMinecraft(bytes.NewReader(data))

	if err == nil {
		t.Errorf("expected error for zero-length packet")
	}
	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}
}

// TestMinecraftHostileInputs tests hostile/malformed input.
func TestMinecraftHostileInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   []byte
		errStr string // substring expected in error message
	}{
		{
			name:   "length 0xFFFFFFFF (2GB claim)",
			data:   []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x7F},
			errStr: "out of range",
		},
		{
			name:   "negative length encoding",
			data:   []byte{0x80, 0x80, 0x80, 0x80, 0x08}, // -1 as 5-byte varint
			errStr: "out of range",
		},
		{
			name:   "truncated packet (claim 100 bytes, send 5)",
			data:   []byte{0x64, 0x00, 0x01, 0x02, 0x03}, // length=100, data...
			errStr: "read handshake frame",
		},
		{
			name:   "empty input",
			data:   []byte{},
			errStr: "EOF",
		},
		{
			name:   "partial varint",
			data:   []byte{0xFF, 0xFF, 0xFF}, // incomplete 5-byte varint
			errStr: "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, _, err := ClassifyMinecraft(bytes.NewReader(tt.data))

			if err == nil {
				t.Errorf("expected error, got none")
			}
			if kind != Unknown {
				t.Errorf("expected kind Unknown, got %v", kind)
			}
			if tt.errStr != "" && !bytes.Contains([]byte(err.Error()), []byte(tt.errStr)) {
				t.Errorf("expected error containing %q, got: %v", tt.errStr, err)
			}
		})
	}
}

// TestJSONEscaping tests the JSON string escaper.
func TestJSONEscaping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `hello`,
			expected: `hello`,
		},
		{
			input:    `"quoted"`,
			expected: `\"quoted\"`,
		},
		{
			input:    `back\slash`,
			expected: `back\\slash`,
		},
		{
			input:    "line1\nline2",
			expected: `line1\nline2`,
		},
		{
			input:    "tab\there",
			expected: `tab\there`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeJSONString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}

			// Verify it's valid JSON.
			jsonStr := `"` + result + `"`
			var out string
			if err := json.Unmarshal([]byte(jsonStr), &out); err != nil {
				t.Errorf("invalid JSON after escape: %v", err)
			}
			if out != tt.input {
				t.Errorf("roundtrip failed: expected %q, got %q", tt.input, out)
			}
		})
	}
}

// Helper function to build a valid Minecraft handshake packet.
func buildMinecraftHandshake(protocol int32, host string, port uint16, nextState int32) []byte {
	var packet bytes.Buffer

	// Packet ID (0x00).
	writeMinecraftVarInt(&packet, 0x00)

	// Protocol version.
	writeMinecraftVarInt(&packet, protocol)

	// Server address.
	_ = writeMinecraftString(&packet, host)

	// Server port.
	packet.Write([]byte{byte(port >> 8), byte(port)})

	// Next state.
	writeMinecraftVarInt(&packet, nextState)

	// Frame the packet.
	return frameMinecraftPacket(packet.Bytes())
}

// ClassifyMinecraftStatusResponse is a helper that reads back a status response
// (for round-trip testing).
func ClassifyMinecraftStatusResponse(data []byte) (string, string, error) {
	br := bytes.NewReader(data)

	// Read frame length.
	length, err := readMinecraftVarInt(br)
	if err != nil {
		return "", "", err
	}

	frame := make([]byte, length)
	if _, err := io.ReadFull(br, frame); err != nil {
		return "", "", err
	}

	fbr := bytes.NewReader(frame)

	// Read packet ID.
	id, err := readMinecraftVarInt(fbr)
	if err != nil {
		return "", "", err
	}

	if id != 0x00 {
		return "", "", fmt.Errorf("expected packet id 0x00, got %d", id)
	}

	// Read JSON string.
	jsonStr, err := readMinecraftString(fbr)
	if err != nil {
		return "", "", err
	}

	return "status_response", jsonStr, nil
}
