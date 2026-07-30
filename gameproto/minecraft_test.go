package gameproto

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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
			name:       "truncated length",
			data:       []byte{0xFF},
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:       "truncated frame",
			data:       []byte{0x05, 0x00, 0x00}, // length=5 but only 2 bytes of data
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:       "empty input",
			data:       []byte{},
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:       "huge packet size",
			data:       []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x7F}, // 2GB size
			expectKind: Unknown,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, result, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(tt.data)))

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
			kind, _, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(tt.data)))

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

	kind, result, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(data)))
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
	kind, result, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(data)))

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
		return
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
	kind, _, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(data)))

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
			kind, _, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(tt.data)))

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
		{
			input:    "back\bspace",
			expected: `back\bspace`,
		},
		{
			input:    "form\ffeed",
			expected: `form\ffeed`,
		},
		{
			input:    "carriage\rreturn",
			expected: `carriage\rreturn`,
		},
		{
			input:    "control\x01char",
			expected: `control\u0001char`,
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

// minecraftStepReader hands back one queued chunk per Read call, simulating
// data arriving across separate reads (e.g. distinct TCP segments) instead
// of being available synchronously in a single buffer. This matches how a
// live net.Conn behaves: a Read call returns only what has actually arrived
// on the wire, never more than that.
type minecraftStepReader struct {
	chunks [][]byte
}

func (s *minecraftStepReader) Read(p []byte) (int, error) {
	if len(s.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.chunks[0])
	s.chunks[0] = s.chunks[0][n:]
	if len(s.chunks[0]) == 0 {
		s.chunks = s.chunks[1:]
	}
	return n, nil
}

// TestClassifyMinecraftReplayContract proves the replay contract from the
// package doc comment: Consumed plus whatever remains unread in the source
// reconstructs the original stream byte-for-byte. The handshake and a
// trailing "next packet" arrive as two separate reads (as they would from a
// live connection), so the internal bufio.Reader never buffers ahead past
// the handshake.
func TestClassifyMinecraftReplayContract(t *testing.T) {
	t.Parallel()

	handshake := buildMinecraftHandshake(761, "localhost", 25565, 2)
	extra := []byte("subsequent packet bytes belonging to a different message")
	full := append(append([]byte{}, handshake...), extra...)

	sr := &minecraftStepReader{chunks: [][]byte{
		append([]byte{}, handshake...),
		append([]byte{}, extra...),
	}}

	kind, result, err := ClassifyMinecraft(bufio.NewReader(sr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != Join {
		t.Fatalf("expected kind Join, got %v", kind)
	}
	if result == nil {
		t.Fatalf("expected result, got nil")
	}
	if !bytes.Equal(result.Consumed, handshake) {
		t.Fatalf("consumed %d bytes does not match handshake %d bytes", len(result.Consumed), len(handshake))
	}

	var remaining bytes.Buffer
	buf := make([]byte, 4096)
	for {
		n, rerr := sr.Read(buf)
		if n > 0 {
			remaining.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}

	if !bytes.Equal(remaining.Bytes(), extra) {
		t.Errorf("remaining %q does not match expected extra %q", remaining.Bytes(), extra)
	}

	reconstructed := append(append([]byte{}, result.Consumed...), remaining.Bytes()...)
	if !bytes.Equal(reconstructed, full) {
		t.Errorf("consumed+remaining does not reconstruct original: got %d bytes, want %d", len(reconstructed), len(full))
	}
}

// TestClassifyMinecraftPipelinedPackets proves the bug fix: when a client
// sends pipelined packets (Handshake immediately followed by Login Start),
// all bytes must be recoverable. The fix ensures the caller owns the
// *bufio.Reader and can continue reading from it after classification.
// This test feeds all data in a single Read call, which would previously
// cause the extra bytes to be lost in bufio's internal buffer.
func TestClassifyMinecraftPipelinedPackets(t *testing.T) {
	t.Parallel()

	handshake := buildMinecraftHandshake(761, "localhost", 25565, 2)
	// Simulate a Login Start packet (frame length + packet ID + username)
	loginStart := []byte{0x06, 0x00, 0x04, 't', 'e', 's', 't'} // length=6, id=0, username="test"
	full := append(append([]byte{}, handshake...), loginStart...)

	// Create a reader that returns ALL data in a single Read call.
	// This simulates a real network scenario where pipelined packets arrive
	// together from the socket buffer.
	br := bufio.NewReader(bytes.NewReader(full))

	kind, result, err := ClassifyMinecraft(br)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kind != Join {
		t.Fatalf("expected kind Join, got %v", kind)
	}
	if result == nil {
		t.Fatalf("expected result, got nil")
	}

	// Verify consumed bytes match the handshake exactly
	if !bytes.Equal(result.Consumed, handshake) {
		t.Fatalf("consumed bytes do not match handshake: got %d, want %d", len(result.Consumed), len(handshake))
	}

	// Verify the pipelined bytes are still in the caller's bufio.Reader
	// (this is the core of the bug fix: they must not be lost)
	remaining := make([]byte, len(loginStart))
	n, err := io.ReadFull(br, remaining)
	if err != nil {
		t.Fatalf("failed to read remaining bytes from bufio.Reader: %v", err)
	}
	if n != len(loginStart) {
		t.Fatalf("expected to read %d remaining bytes, got %d", len(loginStart), n)
	}
	if !bytes.Equal(remaining, loginStart) {
		t.Fatalf("remaining bytes do not match pipelined packet: got %q, want %q", remaining, loginStart)
	}

	// Verify that Consumed + remaining equals the original input
	reconstructed := append(append([]byte{}, result.Consumed...), remaining...)
	if !bytes.Equal(reconstructed, full) {
		t.Fatalf("consumed+remaining does not reconstruct original: got %d bytes, want %d", len(reconstructed), len(full))
	}
}

// TestClassifyMinecraftLengthVarIntTooLong tests that a length-prefix VarInt
// with five continuation bytes (never terminating) is rejected with a
// specific, non-EOF error rather than treated as a truncated read.
func TestClassifyMinecraftLengthVarIntTooLong(t *testing.T) {
	t.Parallel()

	data := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	kind, result, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(data)))

	if err == nil {
		t.Fatal("expected error for an unterminated length VarInt")
	}
	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("varint too long")) {
		t.Errorf("expected error containing %q, got: %v", "varint too long", err)
	}
	if bytes.Contains([]byte(err.Error()), []byte("EOF")) {
		t.Errorf("expected a non-EOF error, got: %v", err)
	}
}

// TestClassifyMinecraftFrameReadNonEOFError tests that a non-EOF read error
// while reading the frame body is wrapped and surfaced distinctly from the
// EOF/truncation case (which returns Unknown with an EOF-flavored error).
func TestClassifyMinecraftFrameReadNonEOFError(t *testing.T) {
	t.Parallel()

	r := &minecraftFlakyReader{}
	kind, result, err := ClassifyMinecraft(bufio.NewReader(r))

	if err == nil {
		t.Fatal("expected error")
	}
	if kind != Unknown {
		t.Errorf("expected kind Unknown, got %v", kind)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("read handshake frame")) {
		t.Errorf("expected error containing %q, got: %v", "read handshake frame", err)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("simulated read failure")) {
		t.Errorf("expected wrapped cause %q, got: %v", "simulated read failure", err)
	}
}

// minecraftFlakyReader supplies a valid one-byte length prefix (5) on the
// first Read call, then fails with a distinct, non-EOF error on every
// subsequent call, forcing the frame-body read to hit a genuine I/O error
// rather than EOF/ErrUnexpectedEOF.
type minecraftFlakyReader struct {
	calls int
}

func (f *minecraftFlakyReader) Read(p []byte) (int, error) {
	f.calls++
	if f.calls == 1 {
		p[0] = 0x05
		return 1, nil
	}
	return 0, errors.New("simulated read failure")
}

// TestClassifyMinecraftMalformedFrames drives classifyMinecraftHandshake
// through every distinct error branch inside the frame body: a bad packet
// ID, a truncated field at each stage, an oversized embedded string length,
// and an inner VarInt that never terminates.
func TestClassifyMinecraftMalformedFrames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		inner  []byte
		errStr string
	}{
		{
			name:   "unexpected packet id",
			inner:  []byte{0x01},
			errStr: "expected handshake packet id 0x00",
		},
		{
			name:   "truncated packet id varint",
			inner:  []byte{0x80},
			errStr: "read packet id",
		},
		{
			name:   "missing protocol version",
			inner:  []byte{0x00},
			errStr: "read protocol version",
		},
		{
			name:   "protocol version varint too long",
			inner:  append([]byte{0x00}, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF),
			errStr: "read protocol version",
		},
		{
			name:   "missing server address",
			inner:  []byte{0x00, 0x00},
			errStr: "read server address",
		},
		{
			name:   "server address string length out of range",
			inner:  append([]byte{0x00, 0x00}, mustMinecraftVarInt(40000)...),
			errStr: "read server address",
		},
		{
			name:   "server address string data truncated",
			inner:  []byte{0x00, 0x00, 0x0A}, // claims a 10-byte address, sends none
			errStr: "read server address",
		},
		{
			name:   "missing server port",
			inner:  []byte{0x00, 0x00, 0x00}, // packet id, protocol version, empty address
			errStr: "read server port",
		},
		{
			name:   "missing next state",
			inner:  []byte{0x00, 0x00, 0x00, 0x00, 0x00}, // ...+ empty address + 2-byte port
			errStr: "read next state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := frameMinecraftPacket(tt.inner)
			kind, result, err := ClassifyMinecraft(bufio.NewReader(bytes.NewReader(data)))

			if err == nil {
				t.Fatalf("expected error, got none (kind=%v)", kind)
			}
			if kind != Unknown {
				t.Errorf("expected kind Unknown, got %v", kind)
			}
			if result != nil {
				t.Errorf("expected nil result, got %+v", result)
			}
			if !bytes.Contains([]byte(err.Error()), []byte(tt.errStr)) {
				t.Errorf("expected error containing %q, got: %v", tt.errStr, err)
			}
		})
	}
}

// mustMinecraftVarInt encodes v as a Minecraft VarInt for use in hand-built
// test frames.
func mustMinecraftVarInt(v int32) []byte {
	var buf bytes.Buffer
	writeMinecraftVarInt(&buf, v)
	return buf.Bytes()
}

// TestBuildMinecraftStatusResponseTooLong tests that an oversized JSON
// payload is rejected rather than silently truncated or panicking.
func TestBuildMinecraftStatusResponseTooLong(t *testing.T) {
	t.Parallel()

	huge := bigString(40000)
	_, err := BuildMinecraftStatusResponse(huge)
	if err == nil {
		t.Fatal("expected error for oversized JSON payload")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("encode status response")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestBuildMinecraftLoginDisconnectTooLong tests that an oversized
// disconnect reason is rejected rather than silently truncated.
func TestBuildMinecraftLoginDisconnectTooLong(t *testing.T) {
	t.Parallel()

	huge := bigString(40000)
	_, err := BuildMinecraftLoginDisconnect(huge)
	if err == nil {
		t.Fatal("expected error for oversized disconnect reason")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("encode disconnect reason")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// bigString builds an n-byte ASCII string for oversized-input tests.
func bigString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// minecraftByteOnlyReader implements io.ByteReader but deliberately not
// io.Reader, to exercise readMinecraftString's defensive type assertion.
type minecraftByteOnlyReader struct {
	data []byte
	pos  int
}

func (b *minecraftByteOnlyReader) ReadByte() (byte, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	c := b.data[b.pos]
	b.pos++
	return c, nil
}

// TestReadMinecraftStringNonIOReader tests the defensive branch that
// rejects a ByteReader which does not also implement io.Reader.
func TestReadMinecraftStringNonIOReader(t *testing.T) {
	t.Parallel()

	r := &minecraftByteOnlyReader{data: []byte{0x03, 'a', 'b', 'c'}}
	_, err := readMinecraftString(r)
	if err == nil {
		t.Fatal("expected error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("does not implement io.Reader")) {
		t.Errorf("unexpected error: %v", err)
	}
}
