package oci

import (
	"context"
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

func TestClient_Pull(t *testing.T) {
	reg := newFakeRegistry(t)
	repo := "kestrel/minecraft-java"
	digest := reg.pushBundle(repo, "1.0.0", map[string][]byte{
		LayerNameMetadata: []byte(fixtureModuleYAML),
		LayerNameTemplate: []byte(fixtureTemplateYAML),
		LayerNameReadme:   []byte("# Minecraft\n"),
	})

	c := New(nil, true)
	bundle, err := c.Pull(context.Background(), reg.host()+"/"+repo, "1.0.0")
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if bundle.Digest != digest {
		t.Errorf("Digest = %q, want %q", bundle.Digest, digest)
	}
	if bundle.Metadata.Name != "minecraft-java" {
		t.Errorf("metadata.name = %q, want minecraft-java", bundle.Metadata.Name)
	}
	if bundle.Metadata.Version != "1.0.0" {
		t.Errorf("metadata.version = %q, want 1.0.0", bundle.Metadata.Version)
	}
	if !strings.Contains(string(bundle.TemplateYAML), "itzg/minecraft-server") {
		t.Errorf("template missing image line; got %q", bundle.TemplateYAML)
	}
	if !strings.Contains(string(bundle.Readme), "Minecraft") {
		t.Errorf("readme missing; got %q", bundle.Readme)
	}
}

func TestClient_Pull_RejectsForeignArtifact(t *testing.T) {
	reg := newFakeRegistry(t)
	repo := "kestrel/bogus"
	// Push a manifest with a different artifactType to simulate a
	// non-Kestrel bundle pushed to the same registry.
	manifest := []byte(`{"mediaType":"application/vnd.oci.image.manifest.v1+json",` +
		`"artifactType":"application/vnd.something.else","config":{"mediaType":"application/json","digest":"sha256:` +
		emptyDigestHex + `","size":2},"layers":[]}`)
	reg.pushManifest(repo, "1.0.0", manifest)
	_ = reg.putBlob([]byte(`{}`))

	_, err := New(nil, true).Pull(context.Background(), reg.host()+"/"+repo, "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "artifactType") {
		t.Fatalf("want artifactType rejection, got %v", err)
	}
}

func TestClient_ListTags_DropsNonSemverAndSorts(t *testing.T) {
	reg := newFakeRegistry(t)
	repo := "kestrel/multi"
	for _, tag := range []string{"1.0.0", "1.2.0", "0.9.0", "latest", "main"} {
		reg.pushBundle(repo, tag, map[string][]byte{
			LayerNameMetadata: []byte(fixtureModuleYAML),
			LayerNameTemplate: []byte(fixtureTemplateYAML),
		})
	}
	tags, err := New(nil, true).ListTags(context.Background(), reg.host()+"/"+repo)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	wantOrdered := []string{"1.2.0", "1.0.0", "0.9.0"}
	if len(tags) != len(wantOrdered) {
		t.Fatalf("ListTags = %v, want %v", tags, wantOrdered)
	}
	for i, w := range wantOrdered {
		if tags[i] != w {
			t.Errorf("tags[%d] = %q, want %q (full=%v)", i, tags[i], w, tags)
		}
	}
}

const emptyDigestHex = "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"
