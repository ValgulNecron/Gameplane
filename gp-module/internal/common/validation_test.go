package common

import (
	"strings"
	"testing"
)

func TestValidateModuleName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid single char", "a", false},
		{"valid with digits and hyphens", "my-game-123", false},
		{"valid max 63 chars", strings.Repeat("a", 63), false},
		{"invalid empty", "", true},
		{"invalid uppercase", "MyGame", true},
		{"invalid underscore", "my_game", true},
		{"invalid leading hyphen", "-game", true},
		{"invalid trailing hyphen", "game-", true},
		{"invalid too long", strings.Repeat("a", 64), true},
		{"invalid special characters", "game!test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModuleName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateModuleName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCanonicalCategories(t *testing.T) {
	cats := CanonicalCategories()
	if len(cats) != 11 {
		t.Fatalf("expected 11 canonical categories, got %d", len(cats))
	}

	for _, cat := range []string{"Survival", "Sandbox", "Shooter", "Simulation", "Building", "Adventure", "Horror", "Co-op", "PvP", "Modded", "Creative"} {
		if !IsCanonicalCategory(cat) {
			t.Errorf("expected %q to be canonical", cat)
		}
		if !IsCanonicalCategory(strings.ToLower(cat)) {
			t.Errorf("expected lower-case %q to be canonical", cat)
		}
	}

	if IsCanonicalCategory("NonExistentCategory") {
		t.Errorf("expected NonExistentCategory to not be canonical")
	}

	if got := NormalizeCategory("survival"); got != "Survival" {
		t.Errorf("expected NormalizeCategory('survival') = 'Survival', got %q", got)
	}
}
