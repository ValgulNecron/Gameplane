package archetypes

import (
	"bytes"
	"image/png"
	"testing"
)

func TestArchetypes(t *testing.T) {
	for _, id := range []string{"steamcmd", "java", "generic"} {
		arch, err := GetArchetype(id)
		if err != nil {
			t.Fatalf("GetArchetype(%q) failed: %v", id, err)
		}
		if arch.ID != id {
			t.Errorf("expected ID %q, got %q", id, arch.ID)
		}
		if len(arch.DefaultPorts) == 0 {
			t.Errorf("expected archetype %q to have default ports", id)
		}
		if arch.DefaultStorage.Size == "" || arch.DefaultStorage.MountPath == "" {
			t.Errorf("expected archetype %q to have valid storage", id)
		}
	}

	_, err := GetArchetype("nonexistent")
	if err == nil {
		t.Errorf("expected error for nonexistent archetype")
	}
}

func TestPlaceholderIconBytes(t *testing.T) {
	icon := PlaceholderIconBytes()
	if len(icon) == 0 {
		t.Fatalf("expected non-empty icon bytes")
	}
	// Verify it decodes as a valid 256x256 PNG
	img, err := png.Decode(bytes.NewReader(icon))
	if err != nil {
		t.Fatalf("failed to decode icon as PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 256 || bounds.Dy() != 256 {
		t.Errorf("expected 256x256 image, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// The steamcmd preset carries a non-root default for the game container.
func TestSteamcmdArchetypeDefaultsToNonRoot(t *testing.T) {
	arch, err := GetArchetype("steamcmd")
	if err != nil {
		t.Fatalf("GetArchetype(steamcmd) failed: %v", err)
	}
	sec := arch.DefaultSecurity
	if sec == nil {
		t.Fatal("steamcmd archetype has no DefaultSecurity")
	}
	if sec.RunAsUser <= 0 || sec.RunAsGroup <= 0 {
		t.Errorf("steamcmd DefaultSecurity runs as uid %d gid %d, want a non-root uid and gid", sec.RunAsUser, sec.RunAsGroup)
	}
	if sec.FSGroup != sec.RunAsGroup {
		t.Errorf("steamcmd DefaultSecurity fsGroup = %d, want the run group %d", sec.FSGroup, sec.RunAsGroup)
	}
}
