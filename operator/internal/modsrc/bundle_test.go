package modsrc

import (
	"strings"
	"testing"
)

const fixtureModuleYAML = `apiVersion: kestrel.gg/module/v1
name: minecraft-java
displayName: Minecraft (Java Edition)
version: 1.0.0
game: minecraft-java
summary: Vanilla / Paper / Forge / Fabric
`

const fixtureTemplateYAML = `apiVersion: kestrel.gg/v1alpha1
kind: GameTemplate
spec:
  displayName: Minecraft (Java Edition)
  game: minecraft-java
  version: 1.0.0
  image: itzg/minecraft-server:2025.1.0
`

func TestFromFiles(t *testing.T) {
	b, err := FromFiles("sha256:abc", map[string][]byte{
		FileMetadata: []byte(fixtureModuleYAML),
		FileTemplate: []byte(fixtureTemplateYAML),
		FileReadme:   []byte("# Minecraft\n"),
		FileIcon:     {0x89, 'P', 'N', 'G'},
	})
	if err != nil {
		t.Fatalf("FromFiles: %v", err)
	}
	if b.Digest != "sha256:abc" {
		t.Errorf("digest = %q", b.Digest)
	}
	if b.Metadata.Name != "minecraft-java" || b.Metadata.Version != "1.0.0" {
		t.Errorf("metadata = %+v", b.Metadata)
	}
	if b.Metadata.DisplayName != "Minecraft (Java Edition)" {
		t.Errorf("displayName = %q", b.Metadata.DisplayName)
	}
	if !strings.Contains(string(b.TemplateYAML), "itzg/minecraft-server") {
		t.Errorf("template = %q", b.TemplateYAML)
	}
	if string(b.Readme) != "# Minecraft\n" {
		t.Errorf("readme = %q", b.Readme)
	}
	if len(b.Icon) != 4 {
		t.Errorf("icon = %v", b.Icon)
	}
}

func TestFromFiles_OptionalFilesAbsent(t *testing.T) {
	b, err := FromFiles("sha256:abc", map[string][]byte{
		FileMetadata: []byte(fixtureModuleYAML),
		FileTemplate: []byte(fixtureTemplateYAML),
	})
	if err != nil {
		t.Fatalf("FromFiles: %v", err)
	}
	if b.Readme != nil || b.Icon != nil {
		t.Errorf("readme/icon should be nil, got %q / %v", b.Readme, b.Icon)
	}
}

func TestFromFiles_Errors(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string][]byte
		wantErr string
	}{
		{
			name:    "missing module.yaml",
			files:   map[string][]byte{FileTemplate: []byte(fixtureTemplateYAML)},
			wantErr: "module.yaml",
		},
		{
			name: "bad module.yaml",
			files: map[string][]byte{
				FileMetadata: []byte("not: : valid : yaml: ::"),
				FileTemplate: []byte(fixtureTemplateYAML),
			},
			wantErr: "parse module.yaml",
		},
		{
			name: "empty name",
			files: map[string][]byte{
				FileMetadata: []byte("apiVersion: kestrel.gg/module/v1\nversion: 1.0.0\n"),
				FileTemplate: []byte(fixtureTemplateYAML),
			},
			wantErr: "name",
		},
		{
			name:    "missing template.yaml",
			files:   map[string][]byte{FileMetadata: []byte(fixtureModuleYAML)},
			wantErr: "template.yaml",
		},
		{
			name: "empty template.yaml",
			files: map[string][]byte{
				FileMetadata: []byte(fixtureModuleYAML),
				FileTemplate: nil,
			},
			wantErr: "template.yaml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromFiles("sha256:x", tc.files)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}
