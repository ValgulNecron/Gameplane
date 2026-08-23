package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestClampRetention exercises the pure retention-bound arithmetic in
// isolation, ahead of the reconciler tests that build on it.
func TestClampRetention(t *testing.T) {
	tests := []struct {
		name                          string
		requested, clusterMax         int32
		clusterDefault, wantRetention int32
	}{
		{
			name:           "under max passes through unchanged",
			requested:      3600,
			clusterMax:     604800,
			clusterDefault: 86400,
			wantRetention:  3600,
		},
		{
			name:           "over max clamps to max",
			requested:      1000000,
			clusterMax:     604800,
			clusterDefault: 86400,
			wantRetention:  604800,
		},
		{
			name:           "exactly at max passes through unchanged",
			requested:      604800,
			clusterMax:     604800,
			clusterDefault: 86400,
			wantRetention:  604800,
		},
		{
			name:           "zero requested falls back to cluster default",
			requested:      0,
			clusterMax:     604800,
			clusterDefault: 86400,
			wantRetention:  86400,
		},
		{
			name:           "negative requested falls back to cluster default",
			requested:      -1,
			clusterMax:     604800,
			clusterDefault: 86400,
			wantRetention:  86400,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := clampRetention(tc.requested, tc.clusterMax, tc.clusterDefault)
			if got != tc.wantRetention {
				t.Errorf("clampRetention(%d, %d, %d) = %d, want %d",
					tc.requested, tc.clusterMax, tc.clusterDefault, got, tc.wantRetention)
			}
		})
	}
}

// TestEffectiveRetentionSeconds covers the spec-value-vs-cluster-default
// resolution, including the zero-configured-reconciler fallback to this
// file's package-level default/max constants.
func TestEffectiveRetentionSeconds(t *testing.T) {
	tests := []struct {
		name                       string
		specTTL                    *int32
		clusterDefault, clusterMax int32
		want                       int32
	}{
		{
			name:           "nil spec TTL falls back to cluster default",
			specTTL:        nil,
			clusterDefault: 43200,
			clusterMax:     604800,
			want:           43200,
		},
		{
			name:           "under-max spec TTL passes through unchanged",
			specTTL:        ptrTo(int32(7200)),
			clusterDefault: 86400,
			clusterMax:     604800,
			want:           7200,
		},
		{
			name:           "over-max spec TTL clamps to cluster max",
			specTTL:        ptrTo(int32(2592000)),
			clusterDefault: 86400,
			clusterMax:     604800,
			want:           604800,
		},
		{
			name:           "zero/unconfigured reconciler falls back to package defaults",
			specTTL:        nil,
			clusterDefault: 0,
			clusterMax:     0,
			want:           defaultCaptureRetentionSeconds,
		},
		{
			name:           "zero spec TTL falls back to cluster default, not zero",
			specTTL:        ptrTo(int32(0)),
			clusterDefault: 86400,
			clusterMax:     604800,
			want:           86400,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveRetentionSeconds(tc.specTTL, tc.clusterDefault, tc.clusterMax)
			if got != tc.want {
				t.Errorf("effectiveRetentionSeconds(%v, %d, %d) = %d, want %d",
					tc.specTTL, tc.clusterDefault, tc.clusterMax, got, tc.want)
			}
		})
	}
}

// TestCaptureExpiresAt covers the anchor-plus-TTL arithmetic, including the
// nil-anchor (no completionTime) case.
func TestCaptureExpiresAt(t *testing.T) {
	anchor := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	got := captureExpiresAt(&anchor, 3600)
	want := anchor.Add(3600 * time.Second)
	if !got.Equal(want) {
		t.Errorf("captureExpiresAt = %v, want %v", got, want)
	}

	if got := captureExpiresAt(nil, 3600); !got.IsZero() {
		t.Errorf("captureExpiresAt(nil, ...) = %v, want zero Time", got)
	}
}

// TestCaptureIsExpired is the boundary-condition test the task calls out
// explicitly: expiry is defined as a strict "now is after expiresAt" — an
// exact match at the boundary (completionTime + ttl == now) is NOT yet
// expired, matching data-model.md's documented transition condition
// "completionTime + ttl < now()".
func TestCaptureIsExpired(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		now       time.Time
		want      bool
	}{
		{
			name:      "not yet expired",
			expiresAt: base.Add(time.Hour),
			now:       base,
			want:      false,
		},
		{
			name:      "exactly at the boundary is not yet expired (strict less-than)",
			expiresAt: base,
			now:       base,
			want:      false,
		},
		{
			name:      "one nanosecond past the boundary is expired",
			expiresAt: base,
			now:       base.Add(time.Nanosecond),
			want:      true,
		},
		{
			name:      "long past expiry",
			expiresAt: base.Add(-30 * 24 * time.Hour),
			now:       base,
			want:      true,
		},
		{
			name:      "zero expiresAt (no completion time) is never expired",
			expiresAt: time.Time{},
			now:       base.Add(1000 * time.Hour),
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := captureIsExpired(tc.expiresAt, tc.now); got != tc.want {
				t.Errorf("captureIsExpired(%v, %v) = %v, want %v", tc.expiresAt, tc.now, got, tc.want)
			}
		})
	}
}

// TestCaptureIsExpired_ZeroTTLAtAnchor covers a zero/absent-TTL capture
// combined with a real anchor: expiresAt collapses to the anchor itself, so
// anything after the anchor instant is immediately expired, and the anchor
// instant itself is not (same strict boundary as TestCaptureIsExpired).
func TestCaptureIsExpired_ZeroTTLAtAnchor(t *testing.T) {
	anchor := metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	expiresAt := captureExpiresAt(&anchor, 0)

	if got := captureIsExpired(expiresAt, anchor.Time); got {
		t.Errorf("captureIsExpired at the zero-TTL anchor instant = %v, want false (boundary is exclusive)", got)
	}
	if got := captureIsExpired(expiresAt, anchor.Add(time.Second)); !got {
		t.Errorf("captureIsExpired one second past a zero-TTL anchor = %v, want true", got)
	}
}
