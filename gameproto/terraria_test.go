package gameproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
			terraria := &TerrariaClassifier{}
			result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(tt.data)))

			if tt.expectErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil {
				if result.Kind != tt.expectKind {
					t.Errorf("expected kind %v, got %v", tt.expectKind, result.Kind)
				}

				if !tt.expectErr && result.Kind == Join && result.Detail == nil {
					t.Errorf("expected detail to be populated for Join")
				}
			}
		})
	}
}

// TestClassifyTerrariaDisconnectMessage tests that a Disconnect message is classified as Unknown.
func TestClassifyTerrariaDisconnectMessage(t *testing.T) {
	t.Parallel()

	// Build a Disconnect message (type 2).
	data := buildTerrariaMessage(terrariaDisconnect, []byte{0x00})

	terraria := &TerrariaClassifier{}
	result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Kind != Unknown {
		t.Errorf("expected kind Unknown for Disconnect, got %v", result.Kind)
	}
}

// TestClassifyTerrariaPasswordRequired tests that PasswordRequired is classified as Unknown.
func TestClassifyTerrariaPasswordRequired(t *testing.T) {
	t.Parallel()

	// Build a PasswordRequired message (type 37).
	data := buildTerrariaMessage(terrariaPasswordRequired, []byte{})

	terraria := &TerrariaClassifier{}
	result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Kind != Unknown {
		t.Errorf("expected kind Unknown for PasswordRequired, got %v", result.Kind)
	}
}

// TestClassifyTerrariaTruncatedHeader tests handling of truncated frame header.
func TestClassifyTerrariaTruncatedHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "single byte only",
			data: []byte{0x01},
		},
		{
			name: "two bytes only",
			data: []byte{0x01, 0x02},
		},
		{
			name: "empty",
			data: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terraria := &TerrariaClassifier{}
			_, err := terraria.Classify(bufio.NewReader(bytes.NewReader(tt.data)))

			if err == nil {
				t.Errorf("expected error for truncated header: %s", tt.name)
			}
		})
	}
}

// TestClassifyTerrariaTruncatedPayload tests handling of truncated frame payload.
func TestClassifyTerrariaTruncatedPayload(t *testing.T) {
	t.Parallel()

	// Build a frame header that says 100 bytes, but don't provide the payload.
	var frame bytes.Buffer
	frame.Write(binary.LittleEndian.AppendUint16(nil, 100))
	frame.WriteByte(terrariaConnectRequest)
	frame.Write([]byte{0x00, 0x01, 0x02})

	terraria := &TerrariaClassifier{}
	_, err := terraria.Classify(bufio.NewReader(bytes.NewReader(frame.Bytes())))

	if err == nil {
		t.Errorf("expected error for truncated payload")
	}
}

// TestClassifyTerrariaFrameTooSmall tests frames with invalid sizes.
func TestClassifyTerrariaFrameTooSmall(t *testing.T) {
	t.Parallel()

	// Frame size < 3 (too small even for header).
	var frame bytes.Buffer
	frame.Write(binary.LittleEndian.AppendUint16(nil, 2))
	frame.WriteByte(0xFF)

	terraria := &TerrariaClassifier{}
	_, err := terraria.Classify(bufio.NewReader(bytes.NewReader(frame.Bytes())))

	if err == nil {
		t.Errorf("expected error for frame too small")
	}
}

// TestClassifyTerrariaFrameTooLarge tests frames with truncated data due to claimed large size.
func TestClassifyTerrariaFrameTooLarge(t *testing.T) {
	t.Parallel()

	// Frame size = max int + 1 (way too large).
	var frame bytes.Buffer
	frame.Write(binary.LittleEndian.AppendUint16(nil, 0xFFFF))
	frame.WriteByte(terrariaConnectRequest)
	frame.Write([]byte{0x00, 0x01})

	terraria := &TerrariaClassifier{}
	_, err := terraria.Classify(bufio.NewReader(bytes.NewReader(frame.Bytes())))

	if err == nil {
		t.Errorf("expected error for frame too large")
	}
}

// TestClassifyTerrariaValidConnectRequest tests parsing a valid ConnectRequest.
func TestClassifyTerrariaValidConnectRequest(t *testing.T) {
	t.Parallel()

	data := buildTerrariaConnectRequest("Terraria279")

	terraria := &TerrariaClassifier{}
	result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Kind != Join {
		t.Fatalf("expected Join, got %v", result.Kind)
	}

	detail, ok := result.Detail.(*TerrariaDetail)
	if !ok {
		t.Fatalf("expected *TerrariaDetail, got %T", result.Detail)
	}

	if detail.Version != "Terraria279" {
		t.Errorf("expected version Terraria279, got %q", detail.Version)
	}
}

// TestClassifyTerrariaConsumedBytes tests that consumed bytes can be replayed.
func TestClassifyTerrariaConsumedBytes(t *testing.T) {
	t.Parallel()

	data := buildTerrariaConnectRequest("Terraria279")

	terraria := &TerrariaClassifier{}
	result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Kind != Join {
		t.Fatalf("expected Join, got %v", result.Kind)
	}

	// Consumed bytes should match the full input
	if !bytes.Equal(result.Consumed, data) {
		t.Errorf("consumed bytes do not match input")
	}

	// Consumed should not be empty
	if len(result.Consumed) == 0 {
		t.Errorf("consumed should be non-empty")
	}
}

// TestClassifyTerrariaEmptyConnectRequestPayload tests a ConnectRequest with empty version.
func TestClassifyTerrariaEmptyConnectRequestPayload(t *testing.T) {
	t.Parallel()

	data := buildTerrariaConnectRequest("")

	terraria := &TerrariaClassifier{}
	result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Kind != Join {
		t.Fatalf("expected Join, got %v", result.Kind)
	}

	detail, ok := result.Detail.(*TerrariaDetail)
	if !ok {
		t.Fatalf("expected *TerrariaDetail, got %T", result.Detail)
	}

	if detail.Version != "" {
		t.Errorf("expected empty version, got %q", detail.Version)
	}
}

// TestBuildTerrariaDisconnect tests building disconnect packets.
func TestBuildTerrariaDisconnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason string
	}{
		{
			name:   "simple reason",
			reason: "Server is starting",
		},
		{
			name:   "reason with special chars",
			reason: "Maintenance: 15 min",
		},
		{
			name:   "empty reason",
			reason: "",
		},
		{
			name:   "reason with unicode",
			reason: "Server closed: 你好",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terraria := &TerrariaClassifier{}
			data, err := terraria.BuildDisconnect(tt.reason)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify the output is non-empty and has valid framing
			if len(data) == 0 {
				t.Errorf("expected non-empty output")
			}

			// Verify it's a valid Terraria message (can extract length and it matches actual data length)
			if len(data) >= 2 {
				frameLen := binary.LittleEndian.Uint16(data[:2])
				if int(frameLen) != len(data) {
					t.Errorf("frame length mismatch: declared %d, actual %d", frameLen, len(data))
				}
			}
		})
	}
}

// TestTerrariaConsumedBytesWithPipelinedData tests replay with pipelined data.
func TestTerrariaConsumedBytesWithPipelinedData(t *testing.T) {
	t.Parallel()

	// Build a ConnectRequest followed by some pipelined data.
	connect := buildTerrariaConnectRequest("Terraria279")
	pipelined := []byte{0xFF, 0xEE, 0xDD, 0xCC}

	var full bytes.Buffer
	full.Write(connect)
	full.Write(pipelined)

	terraria := &TerrariaClassifier{}
	result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(full.Bytes())))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Kind != Join {
		t.Fatalf("expected Join, got %v", result.Kind)
	}

	// Verify Consumed bytes match the handshake only, not the pipelined data
	if !bytes.Equal(result.Consumed, connect) {
		t.Errorf("consumed bytes do not match connect request")
	}
}

// TestTerrariaMessageRoundtrip tests building and parsing a disconnect message.
func TestTerrariaMessageRoundtrip(t *testing.T) {
	t.Parallel()

	reasons := []string{
		"Server shutting down",
		"Kicked for inactivity",
		"",
		"Special \"quotes\" and \\slashes",
	}

	for _, reason := range reasons {
		t.Run(fmt.Sprintf("reason=%q", reason), func(t *testing.T) {
			terraria := &TerrariaClassifier{}
			data, err := terraria.BuildDisconnect(reason)
			if err != nil {
				t.Fatalf("BuildDisconnect error: %v", err)
			}

			// Verify it's a valid message by parsing it
			br := bufio.NewReader(bytes.NewReader(data))
			result, err := terraria.Classify(br)
			if err != nil {
				t.Fatalf("Classify error: %v", err)
			}

			// Disconnect message should be Unknown (not a connect request)
			if result.Kind != Unknown {
				t.Errorf("expected Unknown for disconnect message, got %v", result.Kind)
			}
		})
	}
}

// TestTerrariaStringRoundtrip tests Terraria string encoding/decoding.
func TestTerrariaStringRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"hello",
		"Terraria279",
		"Unicode: 你好世界",
		"Special chars: !@#$%^&*()",
	}

	for _, s := range tests {
		t.Run(fmt.Sprintf("%q", s), func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeTerrariaString(&buf, s); err != nil {
				t.Fatalf("write error: %v", err)
			}

			br := bytes.NewReader(buf.Bytes())
			decoded, err := readTerrariaString(br)
			if err != nil {
				t.Fatalf("read error: %v", err)
			}

			if decoded != s {
				t.Errorf("roundtrip failed: expected %q, got %q", s, decoded)
			}
		})
	}
}

// TestTerrariaStringTooLong tests that oversized strings are rejected.
func TestTerrariaStringTooLong(t *testing.T) {
	t.Parallel()

	// Write an extremely large string length.
	var buf bytes.Buffer
	if err := writeTerrafia7BitEncodedInt(&buf, 0x7fffffff); err != nil {
		t.Fatalf("write error: %v", err)
	}

	br := bytes.NewReader(buf.Bytes())
	_, err := readTerrariaString(br)
	if err == nil {
		t.Errorf("expected error for oversized string")
	}
}

// TestTerrafia7BitEncodedIntRoundtrip tests 7-bit encoding/decoding.
func TestTerrafia7BitEncodedIntRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []int32{
		0,
		1,
		127,
		128,
		16383,
		16384,
		2097151,
		2097152,
		268435455,
		0x7fffffff,
	}

	for _, v := range tests {
		t.Run(fmt.Sprintf("%d", v), func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeTerrafia7BitEncodedInt(&buf, v); err != nil {
				t.Fatalf("write error: %v", err)
			}

			br := bytes.NewReader(buf.Bytes())
			decoded, err := readTerrafia7BitEncodedInt(br)
			if err != nil {
				t.Fatalf("read error: %v", err)
			}

			if decoded != v {
				t.Errorf("roundtrip failed: expected %d, got %d", v, decoded)
			}
		})
	}
}

// TestTerrafia7BitEncodedNegative tests that negative values are rejected.
func TestTerrafia7BitEncodedNegative(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeTerrafia7BitEncodedInt(&buf, -1)
	if err == nil {
		t.Errorf("expected error for negative value")
	}
}

// TestTerrariaHostileInputs tests hostile/malformed input.
func TestTerrariaHostileInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{"all zeros", []byte{0x00, 0x00, 0x00}},
		{"single byte", []byte{0x01}},
		{"frame with wrong type", []byte{0x05, 0x00, 0xFF, 0xAA, 0xBB}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terraria := &TerrariaClassifier{}
			result, err := terraria.Classify(bufio.NewReader(bytes.NewReader(tt.data)))

			// Should either error or return Unknown
			if err == nil && result != nil && result.Kind != Unknown {
				t.Errorf("expected error or Unknown for hostile input, got %v", result.Kind)
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

// TestTerrariaSupportsStatusPing verifies that TerrariaClassifier.SupportsStatusPing() returns false.
func TestTerrariaSupportsStatusPing(t *testing.T) {
	terraria := &TerrariaClassifier{}
	if terraria.SupportsStatusPing() {
		t.Errorf("TerrariaClassifier.SupportsStatusPing() should return false")
	}
}

// TestTerrariaStatusPingNotSupported verifies that BuildStatusResponse returns an error.
func TestTerrariaStatusPingNotSupported(t *testing.T) {
	terraria := &TerrariaClassifier{}
	_, err := terraria.BuildStatusResponse(`{}`)
	if err == nil {
		t.Errorf("expected error for BuildStatusResponse on Terraria")
	}
	if !errors.Is(err, ErrStatusPingUnsupported) {
		t.Errorf("expected ErrStatusPingUnsupported, got %v", err)
	}
}

// TestBuildTerrariaDisconnectTooLarge tests that oversized disconnect messages are rejected.
func TestBuildTerrariaDisconnectTooLarge(t *testing.T) {
	t.Parallel()

	terraria := &TerrariaClassifier{}

	// Create an oversized reason string (larger than terrariaMaxPacketSize).
	// String of 70000 'a' characters will exceed the 65535 byte frame limit.
	largeReasonBytes := make([]byte, 70000)
	for i := range largeReasonBytes {
		largeReasonBytes[i] = 'a'
	}
	largeReason := string(largeReasonBytes)

	_, err := terraria.BuildDisconnect(largeReason)
	if err == nil {
		t.Errorf("expected error for oversized disconnect message")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("too large")) {
		t.Errorf("expected 'too large' in error message, got: %v", err)
	}
}

// TestReadTerrariaStringNegativeLength tests that negative string lengths are rejected.
func TestReadTerrariaStringNegativeLength(t *testing.T) {
	t.Parallel()

	// Create a 7-bit-encoded value that decodes to a negative int32.
	// The 7-bit encoding for -1 with sign bit set: {0x80, 0x80, 0x80, 0x80, 0x08}
	// This encodes to -1 when interpreted as a signed int32.
	negativeEncodedBytes := []byte{0x80, 0x80, 0x80, 0x80, 0x08}

	br := bytes.NewReader(negativeEncodedBytes)
	_, err := readTerrariaString(br)
	if err == nil {
		t.Errorf("expected error for negative string length")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("negative")) {
		t.Errorf("expected 'negative' in error message, got: %v", err)
	}
}

// mockReader is a custom io.Reader that fails with a specific non-EOF error.
type mockReader struct {
	failAfter int
	bytesRead int
	data      []byte
}

func (mr *mockReader) Read(p []byte) (int, error) {
	if mr.bytesRead >= mr.failAfter {
		return 0, fmt.Errorf("mock read error")
	}
	if mr.bytesRead+len(p) > mr.failAfter {
		// Read up to the failure point
		n := mr.failAfter - mr.bytesRead
		copy(p, mr.data[mr.bytesRead:mr.bytesRead+n])
		mr.bytesRead += n
		return n, fmt.Errorf("mock read error")
	}
	n := copy(p, mr.data[mr.bytesRead:])
	mr.bytesRead += n
	if mr.bytesRead >= len(mr.data) {
		return n, io.EOF
	}
	return n, nil
}

// TestClassifyTerrariaHeaderReadNonEOFError tests handling of non-EOF errors during header read.
func TestClassifyTerrariaHeaderReadNonEOFError(t *testing.T) {
	t.Parallel()

	// Create mock reader that fails after 2 bytes (before the full 3-byte header is read)
	mr := &mockReader{
		failAfter: 2,
		data:      []byte{0x01, 0x02, 0x03, 0x04, 0x05},
	}
	br := bufio.NewReader(mr)

	terraria := &TerrariaClassifier{}
	_, err := terraria.Classify(br)

	if err == nil {
		t.Errorf("expected error for header read failure")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("read terraria frame header")) {
		t.Errorf("expected 'read terraria frame header' in error message, got: %v", err)
	}
}

// TestClassifyTerrariaPayloadReadNonEOFError tests handling of non-EOF errors during payload read.
func TestClassifyTerrariaPayloadReadNonEOFError(t *testing.T) {
	t.Parallel()

	// Create header for a message with 100 bytes of payload, but only provide partial payload.
	// Header: 2-byte length (103) + 1-byte type (ConnectRequest) = 3 bytes.
	// We'll provide 10 bytes of payload instead of the required 100.
	headerBytes := binary.LittleEndian.AppendUint16(nil, 103) // 3-byte header + 100 bytes payload
	headerWithPayload := append(headerBytes, terrariaConnectRequest)
	headerWithPayload = append(headerWithPayload, make([]byte, 10)...) // Only 10 bytes instead of 100

	// failAfter = 13 means fail after reading 3 (header) + 10 (partial payload) bytes,
	// simulating a read error mid-payload when trying to read more bytes.
	mr := &mockReader{
		failAfter: 13,
		data:      headerWithPayload,
	}
	br := bufio.NewReader(mr)

	terraria := &TerrariaClassifier{}
	_, err := terraria.Classify(br)

	if err == nil {
		t.Errorf("expected error for payload read failure")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("read terraria frame payload")) {
		t.Errorf("expected 'read terraria frame payload' in error message, got: %v", err)
	}
}
