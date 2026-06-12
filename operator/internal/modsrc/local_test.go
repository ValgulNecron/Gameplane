package modsrc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeModuleDir(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func TestNewLocal_DisabledWithoutRoot(t *testing.T) {
	_, err := newLocal("", "bundles", nil)
	if err == nil || !strings.Contains(err.Error(), "module-local-root") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewLocal_RejectsEscape(t *testing.T) {
	// CRD validation rejects these earlier; the fetcher re-checks.
	for _, sub := range []string{"..", "../sibling", "a/../../b"} {
		if _, err := newLocal(t.TempDir(), sub, nil); err == nil ||
			!strings.Contains(err.Error(), "escapes") {
			t.Errorf("sub %q: err = %v", sub, err)
		}
	}
}

func TestNewLocal_IndexAndPull(t *testing.T) {
	root := t.TempDir()
	writeModuleDir(t, filepath.Join(root, "bundles", "mc"), validModuleFiles("mc", "1.0.0"))

	f, err := newLocal(root, "bundles", nil)
	if err != nil {
		t.Fatalf("newLocal: %v", err)
	}
	entries, warnings, err := f.Index(context.Background())
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Index: entries=%v warnings=%v err=%v", entries, warnings, err)
	}
	if len(entries) != 1 || entries[0].Name != "mc" || entries[0].Reference != "local:bundles/mc" {
		t.Fatalf("entries = %+v", entries)
	}

	b, err := f.Pull(context.Background(), "mc", "1.0.0")
	if err != nil || b.Metadata.Name != "mc" {
		t.Fatalf("Pull: %+v %v", b, err)
	}

	// Root itself (empty path) also works.
	atRoot, err := newLocal(filepath.Join(root, "bundles"), "", nil)
	if err != nil {
		t.Fatalf("newLocal at root: %v", err)
	}
	entries, _, err = atRoot.Index(context.Background())
	if err != nil || len(entries) != 1 {
		t.Fatalf("Index at root: %v %v", entries, err)
	}
}

func TestNewLocal_MissingDirFailsIndex(t *testing.T) {
	f, err := newLocal(t.TempDir(), "nope", nil)
	if err != nil {
		t.Fatalf("newLocal: %v", err)
	}
	if _, _, err := f.Index(context.Background()); err == nil {
		t.Fatal("expected error for missing directory")
	}
}
