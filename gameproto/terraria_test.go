package gameproto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

// TestClassifyTerrariaConnectRequest tests that a ConnectRequest is classified as Join.
func TestClassifyTerrariaConnectRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		data       []byte
		expectKind Kind
		expectErr  bool
	}{
		{
			name:       "valid connect request",
			data:       buildTerrariaConnectRequest("Terraria279"),
			expectKind: Join,
			expectErr:  false,
		},
		{
			name:       "connect with different version",
			data:       buildTerrariaConnectRequest("Terraria234"),
			expectKind: Join,
			expectErr:  false,
		},
		{
			name:       "connect minimal",
			data:       buildTerrariaConnectRequest("Terraria279"),
			expectKind: Join,
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, result, err := ClassifyTerraria(bytes.NewReader(tt.data))

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if kind != tt.expectKind {
				t.Errorf("expected kind %v, got %v", tt.expectKind, kind)
			}

			if !tt.expectErr && kind == Join && result == nil {
				t.Errorf("expected result to be populated")
			}
		})
	}
}

// TestClassifyTerrariaDisconnectMessage tests that a Disconnect message is classified as Unknown.
func TestClassifyTerrariaDisconnectMessage(t *testing.T) {
	t.Parallel()

	// Build a Disconnect message (type 2).
	data := buildTerrariaMessage(terrariaDisconnect, []byte{0x00})

	kind, _, err := ClassifyTerraria(bytes.NewReader(data))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if kind != Unknown {
		t.Errorf("expected kind Unknown for Disconnect, got %v", kind)
	}
}

// TestClassifyTerrariaPasswordRequired tests that PasswordRequired is classified as Unknown.
func TestClassifyTerrariaPasswordRequired(t *testing.T) {
	t.Parallel()

	// Build a PasswordRequired message (type 37).
	data := buildTerrariaMessage(terrariaPasswordRequired, []byte{})

	kind, _, err := ClassifyTerraria(bytes.NewReader(data))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if kind != Unknown {
		t.Errorf("expected kind Unknown for PasswordRequired, got %v", kind)
	}
}

// TestClassifyTerrariaTruncatedHeader tests handling of truncated frame header.
func TestClassifyTerrariaTruncatedHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   []byte
		isEOF  bool
	}{
		{
			name:  "no bytes",
			data:  []byte{},
			isEOF: true,
		},
		{
			name:  "one byte only",
			data:  []byte{0x10},
			isEOF: true,
		},
		{
			name:  "two bytes only (missing type)",
			data:  []byte{0x10, 0x00},
			isEOF: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, _, err := ClassifyTerraria(bytes.NewReader(tt.data))

			if err == nil {
				t.Errorf("expected error for truncated frame")
			}
			if kind != Unknown {
				t.Errorf("expected kind Unknown, got %v", kind)
			}
		})
	}
}

// TestClassifyTerrariaTruncatedPayload tests handling of truncated frame payload.
func TestClassifyTerrariaTruncatedPayload(t *testing.T) {
	t.Parallel()

	// Build a frame that claims to be 100 bytes but only contains 10.
	var frame bytes.Buffer
	frame.Write(binary.LittleEndian.AppendUint16(nil, 100))
	frame.WriteByte(terrariaConnectRequest)
	frame.Write([]byte{0x00, 0x01, 0x02, 0x03, 0x04})

	kind, _, err := ClassifyTerraria(bytes.NewReader(frame.Bytes()))

	if err == nil {
		t.Errorf("expected error for truncated payload")
	}
	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}
}

// TestClassifyTerrariaFrameTooSmall tests frames with invalid sizes.
func TestClassifyTerrariaFrameTooSmall(t *testing.T) {
	t.Parallel()

	// Length of 2 is too small (minimum is 3: 2 bytes length + 1 byte type).
	var frame bytes.Buffer
	frame.Write(binary.LittleEndian.AppendUint16(nil, 2))
	frame.WriteByte(terrariaConnectRequest)

	kind, _, err := ClassifyTerraria(bytes.NewReader(frame.Bytes()))

	if err == nil {
		t.Errorf("expected error for frame too small")
	}
	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}
}

// TestClassifyTerrariaFrameTooLarge tests frames with truncated data due to claimed large size.
func TestClassifyTerrariaFrameTooLarge(t *testing.T) {
	t.Parallel()

	// Claim a frame size of 50000 (valid uint16 but we don't provide that much data).
	var frame bytes.Buffer
	frame.Write(binary.LittleEndian.AppendUint16(nil, 50000))
	frame.WriteByte(terrariaConnectRequest)
	frame.Write([]byte{0x01, 0x02, 0x03})

	kind, _, err := ClassifyTerraria(bytes.NewReader(frame.Bytes()))

	if err == nil {
		t.Errorf("expected error for frame with insufficient data")
	}
	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}
}

// TestBuildTerrariaDisconnect tests disconnect message building.
func TestBuildTerrariaDisconnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason string
	}{
		{
			name:   "simple reason",
			reason: "Server is offline",
		},
		{
			name:   "reason with quotes",
			reason: `Server is "restarting"`,
		},
		{
			name:   "reason with newline",
			reason: "Line 1\nLine 2",
		},
		{
			name:   "empty reason",
			reason: "",
		},
		{
			name:   "long reason",
			reason: "This is a much longer disconnect reason that contains more information about why the player was disconnected from the server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := BuildTerrariaDisconnect(tt.reason)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(data) < 4 {
				t.Errorf("packet too small: %d bytes", len(data))
			}

			// Verify frame structure: [length LE][type][payload...]
			length := binary.LittleEndian.Uint16(data[:2])
			messageType := data[2]

			if messageType != terrariaDisconnect {
				t.Errorf("expected message type %d, got %d", terrariaDisconnect, messageType)
			}

			// The length field includes itself, so actual data is length bytes after the length field.
			if len(data) != int(length)+2 {
				t.Errorf("frame length mismatch: claimed %d, actual %d", length, len(data)-2)
			}
		})
	}
}

// TestTerrafia7BitEncodedIntRoundtrip tests 7-bit integer encoding/decoding.
func TestTerrafia7BitEncodedIntRoundtrip(t *testing.T) {
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
	}

	for _, v := range tests {
		t.Run(fmt.Sprintf("value=%d", v), func(t *testing.T) {
			var buf bytes.Buffer
			err := writeTerrafia7BitEncodedInt(&buf, v)
			if err != nil {
				t.Errorf("write failed: %v", err)
			}

			result, err := readTerrafia7BitEncodedInt(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Errorf("read failed: %v", err)
			}

			if result != v {
				t.Errorf("expected %d, got %d", v, result)
			}
		})
	}
}

// TestTerrariaStringRoundtrip tests string encoding/decoding.
func TestTerrariaStringRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"hello",
		"Terraria279",
		"a much longer string with various characters!@#$%",
	}

	for _, s := range tests {
		t.Run(fmt.Sprintf("string=%s", s), func(t *testing.T) {
			var buf bytes.Buffer
			err := writeTerrariaString(&buf, s)
			if err != nil {
				t.Errorf("write failed: %v", err)
			}

			result, err := readTerrariaString(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Errorf("read failed: %v", err)
			}

			if result != s {
				t.Errorf("expected %q, got %q", s, result)
			}
		})
	}
}

// TestTerrariaConnectRequestParsing tests parsing of ConnectRequest payload.
func TestTerrariaConnectRequestParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		version       string
		expectVersion string
		expectErr     bool
	}{
		{
			name:          "standard connect",
			version:       "Terraria279",
			expectVersion: "Terraria279",
			expectErr:     false,
		},
		{
			name:          "different version",
			version:       "Terraria238",
			expectVersion: "Terraria238",
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build payload.
			var payload bytes.Buffer
			_ = writeTerrariaString(&payload, tt.version)

			result, err := parseTerrariaConnectRequest(payload.Bytes())

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectErr && result != nil {
				if result.Version != tt.expectVersion {
					t.Errorf("expected version %q, got %q", tt.expectVersion, result.Version)
				}
			}
		})
	}
}

// TestTerrariaConnectRequestEmpty tests parsing an empty payload.
func TestTerrariaConnectRequestEmpty(t *testing.T) {
	t.Parallel()

	_, err := parseTerrariaConnectRequest([]byte{})
	if err == nil {
		t.Errorf("expected error for empty payload")
	}
}

// TestTerrariaNetworkTextRoundtrip tests NetworkText encoding.
func TestTerrariaNetworkTextRoundtrip(t *testing.T) {
	t.Parallel()

	reasons := []string{
		"Server offline",
		"Version mismatch",
		"Server is full",
		`Check: C:\server\logs`,
	}

	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			var buf bytes.Buffer
			err := writeTerrariaNetworkText(&buf, reason)
			if err != nil {
				t.Errorf("write failed: %v", err)
			}

			// Verify it can be read back (at least the mode and string).
			br := bytes.NewReader(buf.Bytes())
			mode, err := br.ReadByte()
			if err != nil {
				t.Errorf("read mode failed: %v", err)
			}

			if mode != 0 {
				t.Errorf("expected literal mode (0), got %d", mode)
			}

			text, err := readTerrariaString(br)
			if err != nil {
				t.Errorf("read text failed: %v", err)
			}

			if text != reason {
				t.Errorf("expected %q, got %q", reason, text)
			}
		})
	}
}

// TestClassifyTerrariaRoundtrip tests building and parsing a disconnect message.
func TestClassifyTerrariaRoundtrip(t *testing.T) {
	t.Parallel()

	reason := "Server maintenance"

	// Build a disconnect message.
	data, err := BuildTerrariaDisconnect(reason)
	if err != nil {
		t.Errorf("build failed: %v", err)
	}

	// Parse it back (as Unknown since it's not a ConnectRequest).
	kind, _, err := ClassifyTerraria(bytes.NewReader(data))
	if err != nil {
		t.Errorf("parse failed: %v", err)
	}

	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}
}

// TestClassifyTerrariaEOFHandling tests EOF vs truncation handling.
func TestClassifyTerrariaEOFHandling(t *testing.T) {
	t.Parallel()

	// A proper ConnectRequest followed by EOF should parse successfully.
	data := buildTerrariaConnectRequest("Terraria279")
	kind, result, err := ClassifyTerraria(bytes.NewReader(data))

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if kind != Join {
		t.Errorf("expected kind Join, got %v", kind)
	}
	if result == nil {
		t.Errorf("expected result to be populated")
	}
}

// TestTerrariaConsumedBytes tests that consumed bytes can be replayed.
func TestTerrariaConsumedBytes(t *testing.T) {
	t.Parallel()

	data := buildTerrariaConnectRequest("Terraria279")
	kind, result, err := ClassifyTerraria(bytes.NewReader(data))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != Join {
		t.Fatalf("expected kind Join, got %v", kind)
	}
	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	// Consumed should equal the total data length for a complete read
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

// TestTerrariaHostileInputs tests hostile/malformed input.
func TestTerrariaHostileInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		data   []byte
		errStr string // substring expected in error message
	}{
		{
			name:   "length 0xFFFFFFFF (claim max uint16)",
			data:   buildTerrariaMessage(terrariaConnectRequest, make([]byte, 65530)),
			errStr: "", // This should succeed as it's within bounds
		},
		{
			name:   "zero length",
			data:   []byte{0x00, 0x00, 0x01},
			errStr: "too short",
		},
		{
			name:   "truncated header",
			data:   []byte{0x00},
			errStr: "EOF",
		},
		{
			name:   "truncated payload",
			data:   []byte{0x10, 0x00, 0x01, 0x02}, // length=16 but only 4 bytes total
			errStr: "read terraria frame payload",
		},
		{
			name:   "empty input",
			data:   []byte{},
			errStr: "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, _, err := ClassifyTerraria(bytes.NewReader(tt.data))

			if tt.errStr != "" {
				if err == nil {
					t.Errorf("expected error, got none")
				}
				if kind != Unknown {
					t.Errorf("expected kind Unknown, got %v", kind)
				}
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errStr)) {
					t.Errorf("expected error containing %q, got: %v", tt.errStr, err)
				}
			} else {
				// No error expected
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Helper function to build a ConnectRequest frame.
func buildTerrariaConnectRequest(version string) []byte {
	var payload bytes.Buffer

	// Write version string only.
	_ = writeTerrariaString(&payload, version)

	return buildTerrariaMessage(terrariaConnectRequest, payload.Bytes())
}

// Helper function to build a Terraria message frame.
func buildTerrariaMessage(msgType byte, payload []byte) []byte {
	var msg bytes.Buffer
	msg.WriteByte(msgType)
	msg.Write(payload)

	frame := msg.Bytes()
	totalLength := len(frame) + 2 // +2 for the length field itself

	var out bytes.Buffer
	out.Write(binary.LittleEndian.AppendUint16(nil, uint16(totalLength)))
	out.Write(frame)

	return out.Bytes()
}

// TestTerrariaStringTruncation tests string reading with truncated data.
func TestTerrariaStringTruncation(t *testing.T) {
	t.Parallel()

	// Claim a string of length 100 but provide only 10 bytes.
	var buf bytes.Buffer
	_ = writeTerrafia7BitEncodedInt(&buf, 100)
	buf.Write([]byte{0x01, 0x02, 0x03, 0x04, 0x05})

	_, err := readTerrariaString(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Errorf("expected error for truncated string")
	}
}

// TestTerrafia7BitEncodedIntTooLong tests overflow of 7-bit encoded integers.
func TestTerrafia7BitEncodedIntTooLong(t *testing.T) {
	t.Parallel()

	// Six continuation bytes would overflow the 32-bit limit.
	// Craft a malicious sequence: all high bits set, making it read 6 bytes.
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	_, err := readTerrafia7BitEncodedInt(bytes.NewReader(data))
	if err == nil {
		t.Errorf("expected error for oversized 7-bit int")
	}
}
