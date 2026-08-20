package gameproto

import (
	"bufio"
	"bytes"
	"testing"
)

// TestDemoClassifier_Classify verifies that DemoClassifier.Classify always
// returns a non-nil ClassificationResult with Kind==Unknown, nil Consumed,
// nil Detail, and nil error, regardless of input.
func TestDemoClassifier_Classify(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty input", []byte{}},
		{"single byte", []byte{0x00}},
		{"arbitrary bytes", []byte{0x01, 0x02, 0x03, 0x04, 0x05}},
		{"long buffer", make([]byte, 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := bufio.NewReader(bytes.NewReader(tt.input))
			result, err := (&DemoClassifier{}).Classify(br)

			if err != nil {
				t.Fatalf("Classify() returned error %v, want nil", err)
			}
			if result == nil {
				t.Fatalf("Classify() returned nil result, want non-nil *ClassificationResult")
			}
			if result.Kind != Unknown {
				t.Fatalf("Classify() returned Kind=%v, want Kind=Unknown(%v)", result.Kind, Unknown)
			}
			if result.Consumed != nil {
				t.Fatalf("Classify() returned Consumed=%v, want nil", result.Consumed)
			}
			if result.Detail != nil {
				t.Fatalf("Classify() returned Detail=%v, want nil", result.Detail)
			}
		})
	}
}

// TestDemoClassifier_ConsumesNothing verifies that DemoClassifier.Classify
// does not consume any bytes from the bufio.Reader, enabling lossless stream
// replay: the original bytes remain in the reader after Classify returns.
func TestDemoClassifier_ConsumesNothing(t *testing.T) {
	original := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	br := bufio.NewReader(bytes.NewReader(original))

	result, err := (&DemoClassifier{}).Classify(br)
	if err != nil {
		t.Fatalf("Classify() returned error %v, want nil", err)
	}
	if result == nil {
		t.Fatalf("Classify() returned nil result, want non-nil")
	}

	// Read the remaining bytes from the reader.
	remaining, err := bytes.ReadAll(br)
	if err != nil {
		t.Fatalf("ReadAll() returned error %v", err)
	}

	// Verify the remaining bytes equal the original input.
	if !bytes.Equal(remaining, original) {
		t.Fatalf("remaining bytes do not match original: got %v, want %v", remaining, original)
	}
}

// TestDemoClassifier_SupportsStatusPing verifies that DemoClassifier.SupportsStatusPing
// returns false.
func TestDemoClassifier_SupportsStatusPing(t *testing.T) {
	classifier := &DemoClassifier{}
	if classifier.SupportsStatusPing() {
		t.Fatalf("SupportsStatusPing() returned true, want false")
	}
}

// TestDemoClassifier_BuildStatusResponse verifies that DemoClassifier.BuildStatusResponse
// returns a nil payload and ErrStatusPingUnsupported error, and does not panic.
func TestDemoClassifier_BuildStatusResponse(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{"empty payload", ""},
		{"json payload", "{}"},
		{"status json", `{"online":true,"players":5}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifier := &DemoClassifier{}
			payload, err := classifier.BuildStatusResponse(tt.payload)

			if payload != nil {
				t.Fatalf("BuildStatusResponse() returned payload %v, want nil", payload)
			}
			if err != ErrStatusPingUnsupported {
				t.Fatalf("BuildStatusResponse() returned error %v, want ErrStatusPingUnsupported", err)
			}
		})
	}
}

// TestDemoClassifier_BuildDisconnect verifies that DemoClassifier.BuildDisconnect
// returns nil payload and nil error.
func TestDemoClassifier_BuildDisconnect(t *testing.T) {
	tests := []struct {
		name   string
		reason string
	}{
		{"empty reason", ""},
		{"simple reason", "Server shutting down"},
		{"long reason", "The server has been shut down by an administrator for maintenance. Please try again later."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifier := &DemoClassifier{}
			payload, err := classifier.BuildDisconnect(tt.reason)

			if payload != nil {
				t.Fatalf("BuildDisconnect() returned payload %v, want nil", payload)
			}
			if err != nil {
				t.Fatalf("BuildDisconnect() returned error %v, want nil", err)
			}
		})
	}
}

// TestDemoClassifier_ImplementsClassifier is a compile-time assertion that
// *DemoClassifier satisfies the Classifier interface.
func TestDemoClassifier_ImplementsClassifier(t *testing.T) {
	var _ Classifier = (*DemoClassifier)(nil)
}
