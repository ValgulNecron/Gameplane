package registry

import "testing"

func TestSetFor(t *testing.T) {
	s := NewSet("test")

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
	if _, ok := s.For(Config{Provider: "curseforge"}); ok {
		t.Error("unknown provider should not be selectable")
	}
	if _, ok := s.For(Config{}); ok {
		t.Error("empty provider should not be selectable")
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
