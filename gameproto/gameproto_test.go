package gameproto

import "testing"

// TestKindString tests the String method for every declared Kind value plus
// an out-of-range value to exercise the default branch.
func TestKindString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind     Kind
		expected string
	}{
		{Join, "Join"},
		{Status, "Status"},
		{Unknown, "Unknown"},
		{Kind(42), "Kind(42)"},
		{Kind(-1), "Kind(-1)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
