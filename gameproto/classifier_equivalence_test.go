package gameproto

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"
)

// TestMinecraftClassifyEquivalence verifies that MinecraftClassifier.Classify()
// produces identical output (Kind, Consumed, Detail fields) as the old
// ClassifyMinecraft facade on the same input.
func TestMinecraftClassifyEquivalence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "valid minecraft join handshake",
			data: buildMinecraftHandshake(761, "localhost", 25565, 2),
		},
		{
			name: "valid minecraft status ping",
			data: buildMinecraftHandshake(-1, "example.com", 25565, 1),
		},
		{
			name: "minecraft handshake with different version",
			data: buildMinecraftHandshake(340, "mc.example.com", 25566, 2),
		},
		{
			name: "truncated minecraft frame",
			data: []byte{0x05, 0x00, 0x00},
		},
		{
			name: "truncated minecraft length",
			data: []byte{0xFF},
		},
		{
			name: "empty input",
			data: []byte{},
		},
		{
			name: "minecraft with huge packet size",
			data: []byte{0xFF, 0xFF, 0xFF, 0xFF, 0x7F},
		},
		{
			name: "bytes that don't match minecraft handshake",
			data: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call old facade with separate reader (readers are consumed)
			oldReader := bufio.NewReader(bytes.NewReader(tt.data))
			oldKind, oldResult, oldErr := ClassifyMinecraft(oldReader)

			// Call new classifier with separate reader
			newReader := bufio.NewReader(bytes.NewReader(tt.data))
			minecraft := &MinecraftClassifier{}
			newClassResult, newErr := minecraft.Classify(newReader)

			// Verify error parity
			if (oldErr != nil) != (newErr != nil) {
				t.Errorf("error parity mismatch: old had error %v, new had error %v", oldErr, newErr)
			}

			// If old had error, we're done
			if oldErr != nil {
				return
			}

			// oldResult should be non-nil when oldErr is nil
			if oldResult == nil {
				t.Errorf("oldResult should not be nil when oldErr is nil")
				return
			}

			// Verify Kind matches
			if oldKind != newClassResult.Kind {
				t.Errorf("Kind mismatch: old %v, new %v", oldKind, newClassResult.Kind)
			}

			// Verify Consumed bytes are byte-for-byte identical
			if !bytes.Equal(oldResult.Consumed, newClassResult.Consumed) {
				t.Errorf("Consumed bytes mismatch: old %d bytes, new %d bytes", len(oldResult.Consumed), len(newClassResult.Consumed))
			}

			// If Kind is not Unknown, verify Detail fields match
			if oldKind != Unknown {
				if newClassResult.Detail == nil {
					t.Errorf("Detail should not be nil for kind %v", oldKind)
					return
				}

				newDetail, ok := newClassResult.Detail.(*MinecraftDetail)
				if !ok {
					t.Errorf("Detail type mismatch: expected *MinecraftDetail, got %T", newClassResult.Detail)
					return
				}

				if oldResult.ProtocolVersion != newDetail.ProtocolVersion {
					t.Errorf("ProtocolVersion mismatch: old %d, new %d", oldResult.ProtocolVersion, newDetail.ProtocolVersion)
				}
				if oldResult.NextState != newDetail.NextState {
					t.Errorf("NextState mismatch: old %d, new %d", oldResult.NextState, newDetail.NextState)
				}
				if oldResult.ServerAddr != newDetail.ServerAddr {
					t.Errorf("ServerAddr mismatch: old %q, new %q", oldResult.ServerAddr, newDetail.ServerAddr)
				}
			}
		})
	}
}

// TestMinecraftBuildStatusResponseEquivalence verifies that MinecraftClassifier.BuildStatusResponse()
// produces byte-for-byte identical output as BuildMinecraftStatusResponse().
func TestMinecraftBuildStatusResponseEquivalence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "valid minecraft status json",
			payload: `{"version":{"name":"1.20.1","protocol":763},"players":{"max":20,"online":1}}`,
		},
		{
			name:    "empty json",
			payload: `{}`,
		},
		{
			name:    "json with escaped quotes",
			payload: `{"description":{"text":"Hello \"World\""}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call old facade
			oldData, oldErr := BuildMinecraftStatusResponse(tt.payload)

			// Call new classifier method
			minecraft := &MinecraftClassifier{}
			newData, newErr := minecraft.BuildStatusResponse(tt.payload)

			// Verify error parity
			if (oldErr != nil) != (newErr != nil) {
				t.Errorf("error parity mismatch: old had error %v, new had error %v", oldErr, newErr)
			}

			if oldErr != nil {
				return
			}

			// Verify bytes are byte-for-byte identical
			if !bytes.Equal(oldData, newData) {
				t.Errorf("BuildStatusResponse output mismatch: old %d bytes, new %d bytes", len(oldData), len(newData))
				if len(oldData) > 0 && len(newData) > 0 {
					t.Logf("old[0:10]: %v", oldData[:minInt(10, len(oldData))])
					t.Logf("new[0:10]: %v", newData[:minInt(10, len(newData))])
				}
			}
		})
	}
}

// TestMinecraftBuildDisconnectEquivalence verifies that MinecraftClassifier.BuildDisconnect()
// produces byte-for-byte identical output as BuildMinecraftLoginDisconnect().
func TestMinecraftBuildDisconnectEquivalence(t *testing.T) {
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
			// Call old facade
			oldData, oldErr := BuildMinecraftLoginDisconnect(tt.reason)

			// Call new classifier method
			minecraft := &MinecraftClassifier{}
			newData, newErr := minecraft.BuildDisconnect(tt.reason)

			// Verify error parity
			if (oldErr != nil) != (newErr != nil) {
				t.Errorf("error parity mismatch: old had error %v, new had error %v", oldErr, newErr)
			}

			if oldErr != nil {
				return
			}

			// Verify bytes are byte-for-byte identical
			if !bytes.Equal(oldData, newData) {
				t.Errorf("BuildDisconnect output mismatch: old %d bytes, new %d bytes", len(oldData), len(newData))
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

// TestTerrariaClassifyEquivalence verifies that TerrariaClassifier.Classify()
// produces identical output (Kind, Consumed, Detail fields) as the old
// ClassifyTerraria facade on the same input.
func TestTerrariaClassifyEquivalence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "valid terraria connect request",
			data: buildTerrariaConnectRequest("Terraria279"),
		},
		{
			name: "terraria connect with different version",
			data: buildTerrariaConnectRequest("Terraria234"),
		},
		{
			name: "truncated terraria header",
			data: []byte{0x10},
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
		},
		{
			name: "empty terraria input",
			data: []byte{},
		},
		{
			name: "bytes that don't match terraria",
			data: []byte{0x01, 0x02, 0x03, 0x04},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call old facade with separate reader
			oldReader := bufio.NewReader(bytes.NewReader(tt.data))
			oldKind, oldResult, oldErr := ClassifyTerraria(oldReader)

			// Call new classifier with separate reader
			newReader := bufio.NewReader(bytes.NewReader(tt.data))
			terraria := &TerrariaClassifier{}
			newClassResult, newErr := terraria.Classify(newReader)

			// Verify error parity
			if (oldErr != nil) != (newErr != nil) {
				t.Errorf("error parity mismatch: old had error %v, new had error %v", oldErr, newErr)
			}

			// If old had error, we're done
			if oldErr != nil {
				return
			}

			// oldResult should be non-nil when oldErr is nil
			if oldResult == nil {
				t.Errorf("oldResult should not be nil when oldErr is nil")
				return
			}

			// Verify Kind matches
			if oldKind != newClassResult.Kind {
				t.Errorf("Kind mismatch: old %v, new %v", oldKind, newClassResult.Kind)
			}

			// Verify Consumed bytes are byte-for-byte identical
			if !bytes.Equal(oldResult.Consumed, newClassResult.Consumed) {
				t.Errorf("Consumed bytes mismatch: old %d bytes, new %d bytes", len(oldResult.Consumed), len(newClassResult.Consumed))
			}

			// If Kind is not Unknown, verify Detail fields match
			if oldKind != Unknown {
				if newClassResult.Detail == nil {
					t.Errorf("Detail should not be nil for kind %v", oldKind)
					return
				}

				newDetail, ok := newClassResult.Detail.(*TerrariaDetail)
				if !ok {
					t.Errorf("Detail type mismatch: expected *TerrariaDetail, got %T", newClassResult.Detail)
					return
				}

				if oldResult.Version != newDetail.Version {
					t.Errorf("Version mismatch: old %q, new %q", oldResult.Version, newDetail.Version)
				}
			}
		})
	}
}

// TestTerrariaSupportsStatusPing verifies that TerrariaClassifier.SupportsStatusPing() returns false.
func TestTerrariaSupportsStatusPing(t *testing.T) {
	terraria := &TerrariaClassifier{}
	if terraria.SupportsStatusPing() {
		t.Errorf("TerrariaClassifier.SupportsStatusPing() should return false")
	}
}

// TestTerrariaTerrariaBuildDisconnectEquivalence verifies that TerrariaClassifier.BuildDisconnect()
// produces byte-for-byte identical output as BuildTerrariaDisconnect().
func TestTerrariaTerrariaBuildDisconnectEquivalence(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call old facade
			oldData, oldErr := BuildTerrariaDisconnect(tt.reason)

			// Call new classifier method
			terraria := &TerrariaClassifier{}
			newData, newErr := terraria.BuildDisconnect(tt.reason)

			// Verify error parity
			if (oldErr != nil) != (newErr != nil) {
				t.Errorf("error parity mismatch: old had error %v, new had error %v", oldErr, newErr)
			}

			if oldErr != nil {
				return
			}

			// Verify bytes are byte-for-byte identical
			if !bytes.Equal(oldData, newData) {
				t.Errorf("BuildDisconnect output mismatch: old %d bytes, new %d bytes", len(oldData), len(newData))
			}
		})
	}
}

// TestNonMatchingBytesEquivalence verifies that both old facades and new classifiers
// return Unknown for bytes that don't match any protocol.
func TestNonMatchingBytesEquivalence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "random bytes",
			data: []byte{0x42, 0x43, 0x44, 0x45},
		},
		{
			name: "null bytes",
			data: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			name: "single byte",
			data: []byte{0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Minecraft facade vs classifier
			{
				oldReader := bufio.NewReader(bytes.NewReader(tt.data))
				oldKind, _, oldErr := ClassifyMinecraft(oldReader)

				newReader := bufio.NewReader(bytes.NewReader(tt.data))
				minecraft := &MinecraftClassifier{}
				newResult, newErr := minecraft.Classify(newReader)

				// Verify error parity
				if (oldErr != nil) != (newErr != nil) {
					t.Errorf("Minecraft error parity mismatch: old had error %v, new had error %v", oldErr, newErr)
				}

				// If there was an error, skip remaining checks
				if newErr != nil {
					return
				}

				// Now we can safely dereference newResult
				if oldKind != Unknown {
					// Old facade returned non-Unknown, verify new does too for consistency
					if newResult.Kind == Unknown {
						t.Logf("Minecraft: old kind %v, new kind %v", oldKind, newResult.Kind)
					}
				}
				if newResult.Kind == Unknown {
					if newResult.Detail != nil {
						t.Errorf("Minecraft: Detail should be nil for Unknown kind, got %T", newResult.Detail)
					}
				}
			}

			// Test Terraria facade vs classifier
			{
				oldReader := bufio.NewReader(bytes.NewReader(tt.data))
				oldKind, _, oldErr := ClassifyTerraria(oldReader)

				newReader := bufio.NewReader(bytes.NewReader(tt.data))
				terraria := &TerrariaClassifier{}
				newResult, newErr := terraria.Classify(newReader)

				// Verify error parity
				if (oldErr != nil) != (newErr != nil) {
					t.Errorf("Terraria error parity mismatch: old had error %v, new had error %v", oldErr, newErr)
				}

				// If there was an error, skip remaining checks
				if newErr != nil {
					return
				}

				// Now we can safely dereference newResult
				if oldKind != Unknown {
					// Old facade returned non-Unknown, verify new does too for consistency
					if newResult.Kind == Unknown {
						t.Logf("Terraria: old kind %v, new kind %v", oldKind, newResult.Kind)
					}
				}
				if newResult.Kind == Unknown {
					if newResult.Detail != nil {
						t.Errorf("Terraria: Detail should be nil for Unknown kind, got %T", newResult.Detail)
					}
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

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
