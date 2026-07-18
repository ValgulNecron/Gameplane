package controller

import (
	"strconv"
	"strings"
	"testing"
)

// TestIndexedMessage_CorrectUnderBothWarningConventions proves the Synced
// condition message stays sane regardless of how a fetcher relates entries
// to warnings: the OCI fetcher's stub-per-failure convention (entries
// includes failures) and the fsFetcher/uploadFetcher convention (warnings
// are disjoint from entries, so entries is successes only). The old
// "indexed %d of %d" computation (len(entries)-len(warnings) of
// len(entries)) was only correct under the OCI convention and could
// undercount successes, or go negative, under the disjoint one.
func TestIndexedMessage_CorrectUnderBothWarningConventions(t *testing.T) {
	cases := []struct {
		name     string
		entries  int
		warnings int
	}{
		{"disjoint convention: 2 valid + 1 invalid", 2, 1},
		{"disjoint convention: more failures than successes", 1, 3},
		{"OCI convention: all succeeded", 3, 0},
		{"OCI convention: one stub failure among entries", 3, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := indexedMessage(tc.entries, tc.warnings)
			if strings.Contains(got, "-") {
				t.Errorf("message contains a negative-looking count: %q", got)
			}
			wantEntries := "indexed " + strconv.Itoa(tc.entries) + " module(s)"
			wantWarnings := strconv.Itoa(tc.warnings) + " warning(s)"
			if !strings.Contains(got, wantEntries) {
				t.Errorf("message = %q, want it to contain %q", got, wantEntries)
			}
			if !strings.Contains(got, wantWarnings) {
				t.Errorf("message = %q, want it to contain %q", got, wantWarnings)
			}
		})
	}
}
