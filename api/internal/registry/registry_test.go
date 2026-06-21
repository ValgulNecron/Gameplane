package registry

import "testing"

func TestSetFor(t *testing.T) {
	s := NewSet("test", "")

	if p, ok := s.For(Config{Provider: "modrinth"}); !ok || p == nil {
		t.Errorf("modrinth: ok=%v p=%v", ok, p)
	}
	if p, ok := s.For(Config{Provider: "thunderstore", Community: "valheim"}); !ok || p == nil {
		t.Errorf("thunderstore: ok=%v p=%v", ok, p)
	}
	// Thunderstore without a community is unusable → not selectable.
	if _, ok := s.For(Config{Provider: "thunderstore"}); ok {
		t.Error("thunderstore without community should not be selectable")
	}
	if p, ok := s.For(Config{Provider: "hangar"}); !ok || p == nil {
		t.Errorf("hangar: ok=%v p=%v", ok, p)
	}
	// CurseForge is key-gated: not selectable without a key.
	if _, ok := s.For(Config{Provider: "curseforge"}); ok {
		t.Error("curseforge without a key should not be selectable")
	}
	if _, ok := s.For(Config{Provider: "nope"}); ok {
		t.Error("unknown provider should not be selectable")
	}
	if _, ok := s.For(Config{}); ok {
		t.Error("empty provider should not be selectable")
	}
}

func TestSetAvailable(t *testing.T) {
	noKey := NewSet("test", "")
	for _, p := range []string{"modrinth", "thunderstore", "hangar"} {
		if !noKey.Available(p) {
			t.Errorf("%s should be available", p)
		}
	}
	if noKey.Available("curseforge") {
		t.Error("curseforge should be unavailable without a key")
	}
	if noKey.Available("nope") {
		t.Error("unknown provider should be unavailable")
	}

	withKey := NewSet("test", "cf-key")
	if !withKey.Available("curseforge") {
		t.Error("curseforge should be available with a key")
	}
	if p, ok := withKey.For(Config{Provider: "curseforge"}); !ok || p == nil {
		t.Errorf("curseforge with key: ok=%v p=%v", ok, p)
	}
}

func TestClampLimit(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{0, 20}, {-5, 20}, {1000, 20}, {1, 1}, {50, 50}, {100, 100},
	} {
		if got := clampLimit(tc.in); got != tc.want {
			t.Errorf("clampLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
