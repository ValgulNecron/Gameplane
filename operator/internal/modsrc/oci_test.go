package modsrc

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// fakeOCIClient implements OCIClient with canned tags and bundle files.
type fakeOCIClient struct {
	tags  map[string][]string          // ref → tags (descending, as the real client returns)
	files map[string]map[string][]byte // ref:version → files
	errOn map[string]error             // "tags:<ref>" or "pull:<ref>:<version>"
}

func (f *fakeOCIClient) ListTags(_ context.Context, ref string) ([]string, error) {
	if err, ok := f.errOn["tags:"+ref]; ok {
		return nil, err
	}
	return f.tags[ref], nil
}

func (f *fakeOCIClient) Pull(_ context.Context, ref, version string) (string, map[string][]byte, error) {
	if err, ok := f.errOn["pull:"+ref+":"+version]; ok {
		return "", nil, err
	}
	files, ok := f.files[ref+":"+version]
	if !ok {
		return "", nil, fmt.Errorf("no artifact at %s:%s", ref, version)
	}
	return "sha256:" + version, files, nil
}

func moduleFiles(name, version string) map[string][]byte {
	return map[string][]byte{
		FileMetadata: []byte("apiVersion: gameplane.local/module/v1\nname: " + name +
			"\ndisplayName: " + strings.ToUpper(name) + "\nversion: " + version +
			"\ngame: " + name + "\nsummary: test\n"),
		FileTemplate: []byte("spec:\n  game: " + name + "\n"),
	}
}

func TestOCIFetcher_Index(t *testing.T) {
	cli := &fakeOCIClient{
		tags: map[string][]string{
			"reg/mods/mc":      {"1.1.0", "1.0.0"},
			"reg/mods/valheim": {"0.9.0"},
		},
		files: map[string]map[string][]byte{
			"reg/mods/mc:1.1.0":      moduleFiles("mc", "1.1.0"),
			"reg/mods/valheim:0.9.0": moduleFiles("valheim", "0.9.0"),
		},
		errOn: map[string]error{},
	}
	f := NewOCI(cli, "reg/mods", []string{"mc", "valheim"})
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
	if mc.Name != "mc" || mc.LatestVersion != "1.1.0" || mc.Reference != "reg/mods/mc" {
		t.Errorf("mc entry = %+v", mc)
	}
	if len(mc.Versions) != 2 || mc.Versions[0] != "1.1.0" {
		t.Errorf("mc versions = %v", mc.Versions)
	}
	if mc.DisplayName != "MC" || mc.Game != "mc" {
		t.Errorf("mc metadata = %+v", mc)
	}
	// The latest bundle's digest must be recorded so the module controller
	// can detect same-version content drift (fake Pull returns sha256:<ver>).
	if mc.Digest != "sha256:1.1.0" {
		t.Errorf("mc digest = %q, want sha256:1.1.0", mc.Digest)
	}
}

func TestOCIFetcher_Index_PartialFailureKeepsStub(t *testing.T) {
	cli := &fakeOCIClient{
		tags: map[string][]string{
			"reg/mods/good": {"1.0.0"},
		},
		files: map[string]map[string][]byte{
			"reg/mods/good:1.0.0": moduleFiles("good", "1.0.0"),
		},
		errOn: map[string]error{"tags:reg/mods/broken": fmt.Errorf("registry error")},
	}
	f := NewOCI(cli, "reg/mods", []string{"good", "broken"})
	entries, warnings, err := f.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "broken") {
		t.Fatalf("warnings = %v", warnings)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	stub := entries[1]
	if stub.Name != "broken" || len(stub.Versions) != 0 || stub.Reference != "reg/mods/broken" {
		t.Errorf("stub = %+v", stub)
	}
}

func TestOCIFetcher_Index_AllFailedIsTotalFailure(t *testing.T) {
	cli := &fakeOCIClient{
		tags:  map[string][]string{},
		files: map[string]map[string][]byte{},
		errOn: map[string]error{"tags:reg/mods/ghost": fmt.Errorf("dial tcp: no such host")},
	}
	f := NewOCI(cli, "reg/mods", []string{"ghost"})
	entries, _, err := f.Index(context.Background())
	if err == nil || !strings.Contains(err.Error(), "all 1 module(s) failed") {
		t.Fatalf("err = %v", err)
	}
	if entries != nil {
		t.Fatalf("entries should be nil on total failure, got %+v", entries)
	}
}

func TestOCIFetcher_Index_NoTagsIsModuleFailure(t *testing.T) {
	cli := &fakeOCIClient{
		tags: map[string][]string{
			"reg/mods/good":  {"1.0.0"},
			"reg/mods/empty": {},
		},
		files: map[string]map[string][]byte{
			"reg/mods/good:1.0.0": moduleFiles("good", "1.0.0"),
		},
		errOn: map[string]error{},
	}
	f := NewOCI(cli, "reg/mods", []string{"good", "empty"})
	_, warnings, err := f.Index(context.Background())
	if err != nil {
		t.Fatalf("Index: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "no semver tags") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestOCIFetcher_Index_MetadataNameMismatch(t *testing.T) {
	cli := &fakeOCIClient{
		tags: map[string][]string{"reg/mods/alias": {"1.0.0"}},
		files: map[string]map[string][]byte{
			// Bundle claims to be "other", indexed as "alias".
			"reg/mods/alias:1.0.0": moduleFiles("other", "1.0.0"),
		},
		errOn: map[string]error{},
	}
	f := NewOCI(cli, "reg/mods", []string{"alias"})
	_, _, err := f.Index(context.Background())
	// Single module + mismatch → total failure with the mismatch message.
	if err == nil || !strings.Contains(err.Error(), "!= source ref name") {
		t.Fatalf("err = %v", err)
	}
}

func TestOCIFetcher_Pull(t *testing.T) {
	cli := &fakeOCIClient{
		tags: map[string][]string{},
		files: map[string]map[string][]byte{
			"reg/mods/mc:1.0.0": moduleFiles("mc", "1.0.0"),
		},
		errOn: map[string]error{},
	}
	f := NewOCI(cli, "reg/mods", []string{"mc"})
	b, err := f.Pull(context.Background(), "mc", "1.0.0")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if b.Digest != "sha256:1.0.0" || b.Metadata.Name != "mc" {
		t.Errorf("bundle = %+v", b)
	}

	if _, err := f.Pull(context.Background(), "mc", "9.9.9"); err == nil {
		t.Fatal("expected error for missing version")
	}
}
