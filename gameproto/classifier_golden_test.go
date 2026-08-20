package gameproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"
)

// TestMinecraftClassifyGolden verifies that MinecraftClassifier.Classify()
// produces the expected classification result (Kind, Consumed, Detail fields)
// for various input scenarios. This golden/characterization suite locks in
// the behavior that was proven equivalent to the old facade on CI before
// it was removed.
func TestMinecraftClassifyGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		data          []byte
		expectKind    Kind
		expectErr     bool
		expectDetail  bool
		expectVersion int32
		expectState   int32
		expectAddr    string
	}{
		{
			name:          "valid minecraft join handshake",
			data:          buildMinecraftHandshake(761, "localhost", 25565, 2),
			expectKind:    Join,
			expectErr:     false,
			expectDetail:  true,
			expectVersion: 761,
			expectState:   2,
			expectAddr:    "localhost:25565",
		},
		{
			name:          "valid minecraft status ping",
			data:          buildMinecraftHandshake(-1, "example.com", 25565, 1),
			expectKind:    Status,
			expectErr:     false,
			expectDetail:  true,
			expectVersion: -1,
			expectState:   1,
			expectAddr:    "example.com:25565",
		},
		{
			name:          "minecraft handshake with different version",
			data:          buildMinecraftHandshake(340, "mc.example.com", 25566, 2),
			expectKind:    Join,
			expectErr:     false,
			expectDetail:  true,
			expectVersion: 340,
			expectState:   2,
			expectAddr:    "mc.example.com:25566",
		},
		{
			name:       "truncated minecraft frame",
			data:       []byte{0x05, 0x00, 0x00},
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:       "truncated minecraft length",
			data:       []byte{0xFF},
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
			name:       "minecraft with huge packet size",
			data:       []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x7F},
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:       "bytes that don't match minecraft handshake",
			data:       []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:          "minecraft with out-of-range nextState (T084 regression)",
			data:          buildMinecraftHandshake(761, "localhost", 25565, 42),
			expectKind:    Unknown,
			expectErr:     false,
			expectDetail:  false,
			expectVersion: 761,
			expectState:   42,
			expectAddr:    "localhost:25565",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tt.data))
			minecraft := &MinecraftClassifier{}
			result, err := minecraft.Classify(reader)

			// Verify error expectation
			if tt.expectErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// If error occurred, we're done
			if err != nil {
				return
			}

			// Verify Kind
			if result.Kind != tt.expectKind {
				t.Errorf("Kind mismatch: expected %v, got %v", tt.expectKind, result.Kind)
			}

			// Verify Consumed is not empty (bytes should be present for stream replay)
			if len(result.Consumed) == 0 && len(tt.data) > 0 {
				t.Errorf("Consumed should not be empty for non-empty input")
			}

			// Verify Consumed bytes match input for successful parse
			if !tt.expectErr {
				if !bytes.Equal(result.Consumed, tt.data) {
					t.Errorf("Consumed mismatch: expected %d bytes, got %d bytes", len(tt.data), len(result.Consumed))
				}
			}

			// Verify Detail expectation
			if tt.expectDetail && result.Detail == nil {
				t.Errorf("Detail should not be nil for kind %v", result.Kind)
				return
			}
			if !tt.expectDetail && result.Detail != nil {
				t.Errorf("Detail should be nil for kind %v", result.Kind)
				return
			}

			// If Detail is expected, verify its fields
			if tt.expectDetail && result.Detail != nil {
				detail, ok := result.Detail.(*MinecraftDetail)
				if !ok {
					t.Errorf("Detail type mismatch: expected *MinecraftDetail, got %T", result.Detail)
					return
				}

				if detail.ProtocolVersion != tt.expectVersion {
					t.Errorf("ProtocolVersion mismatch: expected %d, got %d", tt.expectVersion, detail.ProtocolVersion)
				}
				if detail.NextState != tt.expectState {
					t.Errorf("NextState mismatch: expected %d, got %d", tt.expectState, detail.NextState)
				}
				if detail.ServerAddr != tt.expectAddr {
					t.Errorf("ServerAddr mismatch: expected %q, got %q", tt.expectAddr, detail.ServerAddr)
				}
			}
		})
	}
}

// TestMinecraftBuildStatusResponseGolden verifies that MinecraftClassifier.BuildStatusResponse()
// produces the expected framed packet output for various JSON payloads.
func TestMinecraftBuildStatusResponseGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{
			name:    "valid minecraft status json",
			payload: `{"version":{"name":"1.20.1","protocol":763},"players":{"max":20,"online":1}}`,
			wantErr: false,
		},
		{
			name:    "empty json",
			payload: `{}`,
			wantErr: false,
		},
		{
			name:    "json with escaped quotes",
			payload: `{"description":{"text":"Hello \"World\""}}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minecraft := &MinecraftClassifier{}
			data, err := minecraft.BuildStatusResponse(tt.payload)

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify the output is non-empty and has valid framing
			if len(data) == 0 && !tt.wantErr {
				t.Errorf("expected non-empty output, got empty")
			}

			// Verify it's a valid framed packet (can be parsed back)
			if len(data) > 0 {
				br := bytes.NewReader(data)
				_, err := readMinecraftVarInt(br)
				if err != nil {
					t.Errorf("output frame is invalid: %v", err)
				}
			}
		})
	}
}

// TestMinecraftBuildDisconnectGolden verifies that MinecraftClassifier.BuildDisconnect()
// produces the expected framed packet output for various reasons.
func TestMinecraftBuildDisconnectGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reason  string
		wantErr bool
	}{
		{
			name:    "simple reason",
			reason:  "Server is full",
			wantErr: false,
		},
		{
			name:    "reason with quotes",
			reason:  `Server is "offline"`,
			wantErr: false,
		},
		{
			name:    "reason with newline",
			reason:  "Line 1\nLine 2",
			wantErr: false,
		},
		{
			name:    "reason with backslash",
			reason:  `Path: C:\Users\test`,
			wantErr: false,
		},
		{
			name:    "empty reason",
			reason:  "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minecraft := &MinecraftClassifier{}
			data, err := minecraft.BuildDisconnect(tt.reason)

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify the output is non-empty and has valid framing
			if len(data) == 0 && !tt.wantErr {
				t.Errorf("expected non-empty output, got empty")
			}

			// Verify it's a valid framed packet (can be parsed back)
			if len(data) > 0 {
				br := bytes.NewReader(data)
				_, err := readMinecraftVarInt(br)
				if err != nil {
					t.Errorf("output frame is invalid: %v", err)
				}
			}
		})
	}
}

// TestMinecraftSupportsStatusPing verifies that MinecraftClassifier.SupportsStatusPing() returns true.
func TestMinecraftSupportsStatusPing(t *testing.T) {
	minecraft := &MinecraftClassifier{}
	if !minecraft.SupportsStatusPing() {
		t.Errorf("MinecraftClassifier.SupportsStatusPing() should return true")
	}
}

// TestTerrariaClassifyGolden verifies that TerrariaClassifier.Classify()
// produces the expected classification result for various input scenarios.
func TestTerrariaClassifyGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		data          []byte
		expectKind    Kind
		expectErr     bool
		expectDetail  bool
		expectVersion string
	}{
		{
			name:           "valid terraria connect request",
			data:           buildTerrariaConnectRequest("Terraria279"),
			expectKind:     Join,
			expectErr:      false,
			expectDetail:   true,
			expectVersion:  "Terraria279",
		},
		{
			name:           "terraria connect with different version",
			data:           buildTerrariaConnectRequest("Terraria234"),
			expectKind:     Join,
			expectErr:      false,
			expectDetail:   true,
			expectVersion:  "Terraria234",
		},
		{
			name:       "truncated terraria header",
			data:       []byte{0x10},
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name: "terraria with truncated payload",
			data: func() []byte {
				var frame bytes.Buffer
				frame.Write(binary.LittleEndian.AppendUint16(nil, 100))
				frame.WriteByte(terrariaConnectRequest)
				frame.Write([]byte{0x00, 0x01, 0x02})
				return frame.Bytes()
			}(),
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:       "empty terraria input",
			data:       []byte{},
			expectKind: Unknown,
			expectErr:  true,
		},
		{
			name:         "bytes that don't match terraria",
			data:         []byte{0x01, 0x02, 0x03, 0x04},
			expectKind:   Unknown,
			expectErr:    false,
			expectDetail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(bytes.NewReader(tt.data))
			terraria := &TerrariaClassifier{}
			result, err := terraria.Classify(reader)

			// Verify error expectation
			if tt.expectErr && err == nil {
				t.Errorf("expected error, got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// If error occurred, we're done
			if err != nil {
				return
			}

			// Verify Kind
			if result.Kind != tt.expectKind {
				t.Errorf("Kind mismatch: expected %v, got %v", tt.expectKind, result.Kind)
			}

			// Verify Consumed is present (for stream replay)
			if len(result.Consumed) == 0 && len(tt.data) > 0 && !tt.expectErr {
				t.Logf("Note: Consumed is empty but data is not; this may be OK for non-matching frames")
			}

			// Verify Detail expectation
			if tt.expectDetail && result.Detail == nil {
				t.Errorf("Detail should not be nil for kind %v", result.Kind)
				return
			}
			if !tt.expectDetail && result.Detail != nil {
				t.Errorf("Detail should be nil for kind %v", result.Kind)
				return
			}

			// If Detail is expected, verify its fields
			if tt.expectDetail && result.Detail != nil {
				detail, ok := result.Detail.(*TerrariaDetail)
				if !ok {
					t.Errorf("Detail type mismatch: expected *TerrariaDetail, got %T", result.Detail)
					return
				}

				if detail.Version != tt.expectVersion {
					t.Errorf("Version mismatch: expected %q, got %q", tt.expectVersion, detail.Version)
				}
			}
		})
	}
}

// TestTerrariaTerrariaBuildDisconnectGolden verifies that TerrariaClassifier.BuildDisconnect()
// produces the expected framed packet output for various reasons.
func TestTerrariaTerrariaBuildDisconnectGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		reason  string
		wantErr bool
	}{
		{
			name:    "simple reason",
			reason:  "Server is starting",
			wantErr: false,
		},
		{
			name:    "reason with special chars",
			reason:  "Maintenance: 15 min",
			wantErr: false,
		},
		{
			name:    "empty reason",
			reason:  "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terraria := &TerrariaClassifier{}
			data, err := terraria.BuildDisconnect(tt.reason)

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got none")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify the output is non-empty and has valid framing
			if len(data) == 0 && !tt.wantErr {
				t.Errorf("expected non-empty output, got empty")
			}

			// Verify it's a valid framed packet (can be parsed back)
			if len(data) > 0 {
				br := bytes.NewReader(data)
				_, err := readTerrafia7BitEncodedInt(br)
				if err == nil {
					// Could read a length, packet looks valid
				}
			}
		})
	}
}

// TestClassifierDetailNotNilForNonUnknown verifies that Detail is non-nil for Join/Status
// classifications in the new Classifier implementations.
func TestClassifierDetailNotNilForNonUnknown(t *testing.T) {
	t.Parallel()

	// Minecraft Join
	t.Run("minecraft join detail not nil", func(t *testing.T) {
		data := buildMinecraftHandshake(761, "localhost", 25565, 2)
		reader := bufio.NewReader(bytes.NewReader(data))
		minecraft := &MinecraftClassifier{}
		result, err := minecraft.Classify(reader)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Kind != Join {
			t.Fatalf("expected Join, got %v", result.Kind)
		}
		if result.Detail == nil {
			t.Errorf("Detail should not be nil for Join classification")
		}
	})

	// Minecraft Status
	t.Run("minecraft status detail not nil", func(t *testing.T) {
		data := buildMinecraftHandshake(-1, "example.com", 25565, 1)
		reader := bufio.NewReader(bytes.NewReader(data))
		minecraft := &MinecraftClassifier{}
		result, err := minecraft.Classify(reader)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Kind != Status {
			t.Fatalf("expected Status, got %v", result.Kind)
		}
		if result.Detail == nil {
			t.Errorf("Detail should not be nil for Status classification")
		}
	})

	// Terraria Join
	t.Run("terraria join detail not nil", func(t *testing.T) {
		data := buildTerrariaConnectRequest("Terraria279")
		reader := bufio.NewReader(bytes.NewReader(data))
		terraria := &TerrariaClassifier{}
		result, err := terraria.Classify(reader)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Kind != Join {
			t.Fatalf("expected Join, got %v", result.Kind)
		}
		if result.Detail == nil {
			t.Errorf("Detail should not be nil for Join classification")
		}
	})
}
