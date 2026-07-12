package registry

import (
	"context"
	"testing"
	"time"
)

func TestSetFor(t *testing.T) {
	ctx := context.Background()
	s := NewSet("test", StaticKeys(map[string]string{}))

	if p, ok := s.For(ctx, Config{Provider: "modrinth"}); !ok || p == nil {
		t.Errorf("modrinth: ok=%v p=%v", ok, p)
	}
	if p, ok := s.For(ctx, Config{Provider: "thunderstore", Community: "valheim"}); !ok || p == nil {
		t.Errorf("thunderstore: ok=%v p=%v", ok, p)
	}
	// Thunderstore without a community is unusable → not selectable.
	if _, ok := s.For(ctx, Config{Provider: "thunderstore"}); ok {
		t.Error("thunderstore without community should not be selectable")
	}
	if p, ok := s.For(ctx, Config{Provider: "hangar"}); !ok || p == nil {
		t.Errorf("hangar: ok=%v p=%v", ok, p)
	}
	if p, ok := s.For(ctx, Config{Provider: "factorio"}); !ok || p == nil {
		t.Errorf("factorio: ok=%v p=%v", ok, p)
	}
	// CurseForge is key-gated: not selectable without a key.
	if _, ok := s.For(ctx, Config{Provider: "curseforge"}); ok {
		t.Error("curseforge without a key should not be selectable")
	}
	if _, ok := s.For(ctx, Config{Provider: "nope"}); ok {
		t.Error("unknown provider should not be selectable")
	}
	if _, ok := s.For(ctx, Config{}); ok {
		t.Error("empty provider should not be selectable")
	}
}

func TestSetAvailable(t *testing.T) {
	ctx := context.Background()
	noKey := NewSet("test", StaticKeys(map[string]string{}))
	for _, p := range []string{"modrinth", "thunderstore", "hangar", "factorio"} {
		if !noKey.Available(ctx, p) {
			t.Errorf("%s should be available", p)
		}
	}
	if noKey.Available(ctx, "curseforge") {
		t.Error("curseforge should be unavailable without a key")
	}
	if noKey.Available(ctx, "nope") {
		t.Error("unknown provider should be unavailable")
	}

	withKey := NewSet("test", StaticKeys(map[string]string{"curseforge": "cf-key"}))
	if !withKey.Available(ctx, "curseforge") {
		t.Error("curseforge should be available with a key")
	}
	if p, ok := withKey.For(ctx, Config{Provider: "curseforge"}); !ok || p == nil {
		t.Errorf("curseforge with key: ok=%v p=%v", ok, p)
	}
}

func TestCurseforgeLazyBuilding(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	// Test 1: engine cached when key doesn't change
	s := NewSet("test", StaticKeys(map[string]string{"curseforge": "key1"}))
	s.now = func() time.Time { return now }

	cf1, err1 := s.curseforgeLazy(ctx)
	if err1 != nil || cf1 == nil {
		t.Fatalf("first build: err=%v cf=%v", err1, cf1)
	}

	cf2, err2 := s.curseforgeLazy(ctx)
	if err2 != nil || cf2 == nil {
		t.Fatalf("cached call: err=%v cf=%v", err2, cf2)
	}

	if cf1 != cf2 {
		t.Error("same key should return cached engine")
	}

	// Test 2: engine rebuilt when key changes
	s = NewSet("test", StaticKeys(map[string]string{"curseforge": "key1"}))
	s.now = func() time.Time { return now }

	cf1, _ = s.curseforgeLazy(ctx)

	// Change the key
	s.keyFunc = StaticKeys(map[string]string{"curseforge": "key2"})
	cf3, err3 := s.curseforgeLazy(ctx)
	if err3 != nil || cf3 == nil {
		t.Fatalf("rebuild with new key: err=%v cf=%v", err3, cf3)
	}

	if cf1 == cf3 {
		t.Error("different key should rebuild engine")
	}

	// Test 3: unavailable when key is empty
	s = NewSet("test", StaticKeys(map[string]string{}))
	cf, err := s.curseforgeLazy(ctx)
	if err != nil || cf != nil {
		t.Errorf("empty key: err=%v cf=%v (expected nil, nil)", err, cf)
	}

	// Test 4: cached within TTL
	s = NewSet("test", StaticKeys(map[string]string{"curseforge": "key"}))
	mockTime := now
	s.now = func() time.Time { return mockTime }

	cf1, _ = s.curseforgeLazy(ctx)

	// Advance time within TTL
	mockTime = now.Add(5 * time.Minute)
	cf2, _ = s.curseforgeLazy(ctx)

	if cf1 != cf2 {
		t.Error("should use cached entry within TTL")
	}

	// Test 5: rebuilt after TTL expires
	mockTime = now.Add(11 * time.Minute)
	cf3, _ = s.curseforgeLazy(ctx)

	if cf1 == cf3 {
		t.Error("should rebuild after TTL expires")
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
