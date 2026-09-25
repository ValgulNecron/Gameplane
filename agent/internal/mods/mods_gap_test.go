package mods

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// TestRedactURLErr_StripsQuery covers redactURLErr's main branch: a
// *url.Error's embedded URL has its query string stripped so injected
// mod-portal credentials (username/token query params) never reach a log
// line, while the errors.Is/As chain is preserved (the mutation is in
// place on the same *url.Error).
func TestRedactURLErr_StripsQuery(t *testing.T) {
	orig := &url.Error{
		Op:  "Get",
		URL: "https://mods.example.com/download?username=alice&token=secret",
		Err: errors.New("connection refused"),
	}
	got := redactURLErr(orig)

	var ue *url.Error
	if !errors.As(got, &ue) {
		t.Fatalf("redactURLErr should preserve the *url.Error in the chain, got %v", got)
	}
	if ue.URL != "https://mods.example.com/download" {
		t.Errorf("URL = %q, want query stripped", ue.URL)
	}
	// The same pointer identity is mutated in place.
	if got != error(orig) {
		t.Errorf("redactURLErr should return the same error value it mutated")
	}
}

// TestRedactURLErr_NonURLError covers the pass-through branch: an error
// that isn't a *url.Error (e.g. a plain wrapped error) is returned
// unchanged.
func TestRedactURLErr_NonURLError(t *testing.T) {
	plain := errors.New("boom")
	got := redactURLErr(plain)
	if got != plain {
		t.Errorf("redactURLErr should pass through a non-*url.Error unchanged, got %v", got)
	}
}

// TestRedactURLErr_UnparsableEmbeddedURL covers the perr != nil branch:
// when the *url.Error's own URL string fails to re-parse, redactURLErr
// leaves it untouched rather than panicking or corrupting it.
func TestRedactURLErr_UnparsableEmbeddedURL(t *testing.T) {
	// A URL containing a raw control byte fails url.Parse.
	badURL := "https://example.com/\x7f?token=secret"
	orig := &url.Error{Op: "Get", URL: badURL, Err: errors.New("bad")}
	got := redactURLErr(orig)

	var ue *url.Error
	if !errors.As(got, &ue) {
		t.Fatalf("expected *url.Error in chain, got %v", got)
	}
	if ue.URL != badURL {
		t.Errorf("URL = %q, want unchanged %q when re-parse fails", ue.URL, badURL)
	}
}

// TestDirSize sums regular file sizes recursively, skipping directories,
// and returns 0 for an empty or nonexistent directory (WalkDir's error is
// swallowed).
func TestDirSize(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("12345"), 0o600); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("1234567890"), 0o600); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}

	got := dirSize(root)
	if got != 15 {
		t.Errorf("dirSize = %d, want 15 (5 + 10, directories excluded)", got)
	}
}

func TestDirSize_Empty(t *testing.T) {
	if got := dirSize(t.TempDir()); got != 0 {
		t.Errorf("dirSize of empty dir = %d, want 0", got)
	}
}

func TestDirSize_Nonexistent(t *testing.T) {
	if got := dirSize(filepath.Join(t.TempDir(), "does-not-exist")); got != 0 {
		t.Errorf("dirSize of nonexistent dir = %d, want 0", got)
	}
}

// TestExtAllowed covers handler.extAllowed's two branches: no configured
// extensions permits everything, and a configured allowlist matches
// case-insensitively on suffix while rejecting anything else.
func TestExtAllowed(t *testing.T) {
	h := &handler{}
	if !h.extAllowed("anything.exe") {
		t.Errorf("no configured extensions should allow everything")
	}

	h = &handler{exts: []string{".jar", ".zip"}}
	cases := []struct {
		name string
		want bool
	}{
		{"plugin.jar", true},
		{"PLUGIN.JAR", true}, // case-insensitive
		{"archive.zip", true},
		{"readme.txt", false},
		{"noext", false},
	}
	for _, c := range cases {
		if got := h.extAllowed(c.name); got != c.want {
			t.Errorf("extAllowed(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
