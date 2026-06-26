package modsrc

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func moduleDirFS(entries map[string]map[string]string) fstest.MapFS {
	m := fstest.MapFS{}
	for dir, files := range entries {
		for name, content := range files {
			m[dir+"/"+name] = &fstest.MapFile{Data: []byte(content)}
		}
	}
	return m
}

func validModuleFiles(name, version string) map[string]string {
	return map[string]string{
		FileMetadata: "apiVersion: gameplane.local/module/v1\nname: " + name +
			"\ndisplayName: " + strings.ToUpper(name) + "\nversion: " + version +
			"\ngame: " + name + "\nsummary: test\n",
		FileTemplate: "spec:\n  game: " + name + "\n",
	}
}

func staticFS(fsys fs.FS, digest string) func(context.Context) (fs.FS, string, error) {
	return func(context.Context) (fs.FS, string, error) { return fsys, digest, nil }
}

func TestFSFetcher_IndexDiscoversModules(t *testing.T) {
	fsys := moduleDirFS(map[string]map[string]string{
		"modules/mc":      validModuleFiles("mc", "1.2.0"),
		"modules/valheim": validModuleFiles("valheim", "0.9.0"),
		"docs":            {"README.md": "not a module"},
	})
	f := newFSFetcher(staticFS(fsys, ""), "local:bundles", nil)
	entries, warnings, err := f.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	mc := entries[0]
	if mc.Name != "mc" || mc.LatestVersion != "1.2.0" {
		t.Errorf("mc = %+v", mc)
	}
	if mc.Reference != "local:bundles/modules/mc" {
		t.Errorf("mc reference = %q", mc.Reference)
	}
	if len(mc.Versions) != 1 || mc.Versions[0] != "1.2.0" {
		t.Errorf("mc versions = %v", mc.Versions)
	}
	if !strings.HasPrefix(mc.Digest, "sha256:") {
		t.Errorf("mc digest = %q", mc.Digest)
	}
	if mc.DisplayName != "MC" || mc.Game != "mc" {
		t.Errorf("mc metadata = %+v", mc)
	}
}

func TestFSFetcher_DigestOverrideAndStability(t *testing.T) {
	files := map[string]map[string]string{"mc": validModuleFiles("mc", "1.0.0")}

	withOverride := newFSFetcher(staticFS(moduleDirFS(files), "git:abc123"), "git:repo", nil)
	entries, _, err := withOverride.Index(context.Background())
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if entries[0].Digest != "git:abc123" {
		t.Errorf("digest = %q, want git:abc123", entries[0].Digest)
	}

	// Without an override the content hash must be stable across loads
	// and change when content changes.
	hashed := newFSFetcher(staticFS(moduleDirFS(files), ""), "local:x", nil)
	first, _, _ := hashed.Index(context.Background())
	second, _, _ := hashed.Index(context.Background())
	if first[0].Digest != second[0].Digest {
		t.Errorf("digest unstable: %q vs %q", first[0].Digest, second[0].Digest)
	}

	changed := map[string]map[string]string{"mc": validModuleFiles("mc", "1.0.0")}
	changed["mc"][FileTemplate] += "  image: other\n"
	mutated := newFSFetcher(staticFS(moduleDirFS(changed), ""), "local:x", nil)
	third, _, _ := mutated.Index(context.Background())
	if third[0].Digest == first[0].Digest {
		t.Error("digest did not change with content")
	}
}

func TestFSFetcher_InvalidModuleBecomesWarning(t *testing.T) {
	fsys := moduleDirFS(map[string]map[string]string{
		"good": validModuleFiles("good", "1.0.0"),
		"bad":  {FileMetadata: "name: bad\n"}, // no template.yaml
	})
	f := newFSFetcher(staticFS(fsys, ""), "local:x", nil)
	entries, warnings, err := f.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "good" {
		t.Fatalf("entries = %+v", entries)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "template.yaml") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestFSFetcher_DuplicateNameKeepsFirst(t *testing.T) {
	fsys := moduleDirFS(map[string]map[string]string{
		"a-first": validModuleFiles("mc", "1.0.0"),
		"b-dup":   validModuleFiles("mc", "2.0.0"),
	})
	f := newFSFetcher(staticFS(fsys, ""), "local:x", nil)
	entries, warnings, err := f.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(entries) != 1 || entries[0].LatestVersion != "1.0.0" {
		t.Fatalf("entries = %+v", entries)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "duplicate") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestFSFetcher_AllowFilter(t *testing.T) {
	fsys := moduleDirFS(map[string]map[string]string{
		"minecraft-java":    validModuleFiles("minecraft-java", "1.0.0"),
		"minecraft-bedrock": validModuleFiles("minecraft-bedrock", "1.0.0"),
		"valheim":           validModuleFiles("valheim", "1.0.0"),
	})
	f := newFSFetcher(staticFS(fsys, ""), "local:x", []string{"minecraft-*"})
	entries, _, err := f.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name, "minecraft-") {
			t.Errorf("unexpected entry %q", e.Name)
		}
	}
}

func TestFSFetcher_Pull(t *testing.T) {
	fsys := moduleDirFS(map[string]map[string]string{
		"mc": validModuleFiles("mc", "1.0.0"),
	})
	f := newFSFetcher(staticFS(fsys, ""), "local:x", nil)

	b, err := f.Pull(context.Background(), "mc", "1.0.0")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if b.Metadata.Name != "mc" || !strings.HasPrefix(b.Digest, "sha256:") {
		t.Errorf("bundle = %+v", b)
	}

	// Empty version means "whatever the source has".
	if _, err := f.Pull(context.Background(), "mc", ""); err != nil {
		t.Errorf("Pull with empty version: %v", err)
	}

	if _, err := f.Pull(context.Background(), "mc", "9.9.9"); err == nil ||
		!strings.Contains(err.Error(), "source moved on") {
		t.Errorf("stale version err = %v", err)
	}
	if _, err := f.Pull(context.Background(), "ghost", "1.0.0"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Errorf("missing module err = %v", err)
	}
}

func TestFSFetcher_LoadErrorIsTotalFailure(t *testing.T) {
	f := newFSFetcher(func(context.Context) (fs.FS, string, error) {
		return nil, "", fmt.Errorf("mount gone")
	}, "local:x", nil)
	_, _, err := f.Index(context.Background())
	if err == nil || !strings.Contains(err.Error(), "mount gone") {
		t.Fatalf("err = %v", err)
	}
	if _, err := f.Pull(context.Background(), "mc", "1.0.0"); err == nil {
		t.Fatal("Pull should fail when load fails")
	}
}
