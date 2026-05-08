package handlers

import (
	"sort"
	"testing"
)

func TestSemverDescending(t *testing.T) {
	in := []string{"1.2.0", "0.9.0", "2.0.0", "1.10.0", "1.2.1"}
	want := []string{"2.0.0", "1.10.0", "1.2.1", "1.2.0", "0.9.0"}
	sort.SliceStable(in, func(i, j int) bool { return semverDescending(in[i], in[j]) })
	for i, v := range want {
		if in[i] != v {
			t.Errorf("[%d] got %q want %q (full: %v)", i, in[i], v, in)
		}
	}
}

func TestSemverDescending_PrereleaseOrdering(t *testing.T) {
	if !semverDescending("1.0.0", "1.0.0-rc.1") {
		t.Fatal("release should sort before its rc")
	}
	if semverDescending("1.0.0-rc.1", "1.0.0") {
		t.Fatal("rc should not sort before release")
	}
}
