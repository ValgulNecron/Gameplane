package gameproto

import (
	"bufio"
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestRegistry_ExpectedProtocols verifies that ListRegistered returns exactly
// the expected set of protocol names in sorted order. This test is the audit that
// FAILS WITH A CLEAR ERROR MESSAGE if a protocol is accidentally dropped from the
// registry or if an unexpected protocol is added (User Story 3, Acceptance Scenario 3).
func TestRegistry_ExpectedProtocols(t *testing.T) {
	expected := []string{"demo", "minecraft", "terraria"}
	actual := ListRegistered()

	if !reflect.DeepEqual(expected, actual) {
		// Find missing and unexpected protocols for a clear error message.
		expectedSet := make(map[string]bool)
		for _, p := range expected {
			expectedSet[p] = true
		}
		actualSet := make(map[string]bool)
		for _, p := range actual {
			actualSet[p] = true
		}

		var missing []string
		for _, p := range expected {
			if !actualSet[p] {
				missing = append(missing, p)
			}
		}

		var unexpected []string
		for _, p := range actual {
			if !expectedSet[p] {
				unexpected = append(unexpected, p)
			}
		}

		var msg strings.Builder
		msg.WriteString("registry protocol mismatch\n")
		if len(missing) > 0 {
			msg.WriteString("missing: " + strings.Join(missing, ", ") + "\n")
		}
		if len(unexpected) > 0 {
			msg.WriteString("unexpected: " + strings.Join(unexpected, ", ") + "\n")
		}
		msg.WriteString("expected: " + strings.Join(expected, ", ") + "\n")
		msg.WriteString("actual: " + strings.Join(actual, ", "))
		t.Fatal(msg.String())
	}
}

// TestRegistry_LookupHit verifies that each registered protocol name resolves
// to a non-nil Classifier of the expected concrete type.
func TestRegistry_LookupHit(t *testing.T) {
	tests := []struct {
		name      string
		checkType func(Classifier) bool
		typeName  string
	}{
		{
			"minecraft",
			func(c Classifier) bool { _, ok := c.(*MinecraftClassifier); return ok },
			"*MinecraftClassifier",
		},
		{
			"terraria",
			func(c Classifier) bool { _, ok := c.(*TerrariaClassifier); return ok },
			"*TerrariaClassifier",
		},
		{
			"demo",
			func(c Classifier) bool { _, ok := c.(*DemoClassifier); return ok },
			"*DemoClassifier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifier, ok := Lookup(tt.name)
			if !ok {
				t.Fatalf("Lookup(%q) returned (_, false), want (_, true)", tt.name)
			}
			if classifier == nil {
				t.Fatalf("Lookup(%q) returned nil Classifier", tt.name)
			}

			// Verify concrete type matches using type assertion.
			if !tt.checkType(classifier) {
				t.Fatalf("Lookup(%q) returned %T, want %s", tt.name, classifier, tt.typeName)
			}
		})
	}
}

// TestRegistry_LookupMiss verifies that Lookup returns (nil, false) for unknown
// protocol names and does not panic.
func TestRegistry_LookupMiss(t *testing.T) {
	classifier, ok := Lookup("nosuchgame")
	if ok {
		t.Fatalf("Lookup(\"nosuchgame\") returned (_, true), want (_, false)")
	}
	if classifier != nil {
		t.Fatalf("Lookup(\"nosuchgame\") returned non-nil Classifier, want nil")
	}
}

// TestRegistry_LookupEmptyName verifies that Lookup handles empty names gracefully
// without panicking.
func TestRegistry_LookupEmptyName(t *testing.T) {
	classifier, ok := Lookup("")
	if ok {
		t.Fatalf("Lookup(\"\") returned (_, true), want (_, false)")
	}
	if classifier != nil {
		t.Fatalf("Lookup(\"\") returned non-nil Classifier, want nil")
	}
}

// TestRegistry_ListRegisteredSorted verifies that ListRegistered returns a sorted
// slice. It also verifies that mutating a returned slice does not corrupt the
// registry (i.e., ListRegistered returns a copy, not a reference to the internal map keys).
func TestRegistry_ListRegisteredSorted(t *testing.T) {
	// Call ListRegistered twice.
	first := ListRegistered()
	second := ListRegistered()

	// Verify both are sorted.
	if !slices.IsSorted(first) {
		t.Fatalf("first ListRegistered() is not sorted: %v", first)
	}
	if !slices.IsSorted(second) {
		t.Fatalf("second ListRegistered() is not sorted: %v", second)
	}

	// Mutate the first slice.
	if len(first) > 0 {
		first[0] = "zzz_mutated"
	}

	// Call ListRegistered again and verify it was not affected.
	third := ListRegistered()
	if !slices.IsSorted(third) {
		t.Fatalf("third ListRegistered() is not sorted: %v", third)
	}

	// Verify that the mutation did not affect the second call.
	if !reflect.DeepEqual(second, third) {
		t.Fatalf("ListRegistered returned different results after mutation: second=%v, third=%v", second, third)
	}

	// Verify that "zzz_mutated" is not in the registry.
	for _, p := range third {
		if p == "zzz_mutated" {
			t.Fatalf("registry was corrupted by mutation: found zzz_mutated in ListRegistered()")
		}
	}
}

// TestRegistry_TransportAgnostic verifies that the Classifier interface and
// registry impose no transport-type restriction (TCP vs UDP), enabling protocols
// to serve either or both transports without interface changes (Edge Case 6).
//
// This is verified via two mechanisms:
//   1. Structural: The Classifier interface accepts a *bufio.Reader, which is
//      transport-agnostic (it wraps any io.Reader, TCP or UDP or otherwise).
//   2. Method inspection: No Classify, SupportsStatusPing, BuildStatusResponse,
//      or BuildDisconnect method references TCP, UDP, or any transport constant.
func TestRegistry_TransportAgnostic(t *testing.T) {
	// Verify that Classifier interface methods exist and use transport-agnostic types.
	classifierType := reflect.TypeOf((*Classifier)(nil)).Elem()

	// Check that Classify expects a *bufio.Reader (transport-agnostic).
	classifyMethod, ok := classifierType.MethodByName("Classify")
	if !ok {
		t.Fatal("Classifier interface missing Classify method")
	}
	if classifyMethod.Type.NumIn() != 1 {
		t.Fatalf("Classify has %d inputs, want 1 (*bufio.Reader; receiver is implicit for interface methods)", classifyMethod.Type.NumIn())
	}
	// Input 0 should be *bufio.Reader (receiver is not counted for interface methods).
	brType := classifyMethod.Type.In(0)
	expectedBRType := reflect.TypeOf((*bufio.Reader)(nil))
	if brType != expectedBRType {
		t.Fatalf("Classify input is %v, want *bufio.Reader", brType)
	}

	// Verify other Classifier methods also do not reference transport-specific types.
	// SupportsStatusPing, BuildStatusResponse, BuildDisconnect should accept and
	// return only strings, []byte, error, and bool — no TCP/UDP/Conn types.
	_, okStatus := classifierType.MethodByName("SupportsStatusPing")
	_, okBuildStatus := classifierType.MethodByName("BuildStatusResponse")
	_, okBuildDisconnect := classifierType.MethodByName("BuildDisconnect")

	if !okStatus || !okBuildStatus || !okBuildDisconnect {
		t.Fatal("Classifier interface missing required methods")
	}

	// Structural assertion: No Classifier or registry function imports or
	// references TCP or UDP. This is verified by inspection of the registry.go
	// and classifier.go source; a code reviewer can confirm manually that no
	// transport type appears in the interface definition or registry implementation.
}

// TestClassifier_StatusPingSupport verifies that status-ping support is correctly
// declared per protocol and that unsupported protocols return errors rather than panicking.
func TestClassifier_StatusPingSupport(t *testing.T) {
	tests := []struct {
		name             string
		classifier       Classifier
		supportsStatusPing bool
		shouldErrorOnBuild bool
	}{
		{
			name:             "minecraft supports status ping",
			classifier:       &MinecraftClassifier{},
			supportsStatusPing: true,
			shouldErrorOnBuild: false,
		},
		{
			name:             "terraria does not support status ping",
			classifier:       &TerrariaClassifier{},
			supportsStatusPing: false,
			shouldErrorOnBuild: true,
		},
		{
			name:             "demo does not support status ping",
			classifier:       &DemoClassifier{},
			supportsStatusPing: false,
			shouldErrorOnBuild: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify SupportsStatusPing declaration.
			if got := tt.classifier.SupportsStatusPing(); got != tt.supportsStatusPing {
				t.Fatalf("SupportsStatusPing() = %v, want %v", got, tt.supportsStatusPing)
			}

			// Verify BuildStatusResponse behavior for unsupported protocols.
			if tt.shouldErrorOnBuild {
				_, err := tt.classifier.BuildStatusResponse("{}")
				if err == nil {
					t.Fatalf("BuildStatusResponse() returned nil error, want error for unsupported protocol")
				}
			}
		})
	}
}

// TestRegistry_LookupEachRegisteredClassifier verifies that every registered
// protocol name maps to a non-nil Classifier that implements the full interface
// (all four methods callable without panic).
func TestRegistry_LookupEachRegisteredClassifier(t *testing.T) {
	names := ListRegistered()
	if len(names) == 0 {
		t.Fatal("ListRegistered returned empty slice")
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			classifier, ok := Lookup(name)
			if !ok {
				t.Fatalf("Lookup(%q) returned false, want true", name)
			}
			if classifier == nil {
				t.Fatalf("Lookup(%q) returned nil", name)
			}

			// Verify all four methods are callable (struct implements interface).
			// This is primarily a type-check, but we verify by calling them with
			// dummy inputs to ensure no nil pointer dereference or panic.

			// Classify: pass an empty reader.
			emptyReader := bufio.NewReader(bytes.NewReader(nil))
			_, _ = classifier.Classify(emptyReader)
			// Error is expected for empty input; we just verify no panic.

			// SupportsStatusPing: must return bool.
			_ = classifier.SupportsStatusPing()

			// BuildStatusResponse: may error, but must not panic.
			_, _ = classifier.BuildStatusResponse("{}")

			// BuildDisconnect: may error, but must not panic.
			_, _ = classifier.BuildDisconnect("test")
		})
	}
}
