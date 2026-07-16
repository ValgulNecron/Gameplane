package registry

import (
	"context"
	"sync"
	"sync/atomic"
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
	if p, ok := s.For(ctx, Config{Provider: "spigot"}); !ok || p == nil {
		t.Errorf("spigot: ok=%v p=%v", ok, p)
	}
	if p, ok := s.For(ctx, Config{Provider: "umod"}); !ok || p == nil {
		t.Errorf("umod: ok=%v p=%v", ok, p)
	}
	if p, ok := s.For(ctx, Config{Provider: "github", GitHubOwner: "someorg", GitHubRepo: "somemod"}); !ok || p == nil {
		t.Errorf("github: ok=%v p=%v", ok, p)
	}
	// GitHub without both owner and repo is unusable → not selectable.
	if _, ok := s.For(ctx, Config{Provider: "github"}); ok {
		t.Error("github without owner/repo should not be selectable")
	}
	if _, ok := s.For(ctx, Config{Provider: "github", GitHubOwner: "someorg"}); ok {
		t.Error("github without repo should not be selectable")
	}
	if _, ok := s.For(ctx, Config{Provider: "github", GitHubRepo: "somemod"}); ok {
		t.Error("github without owner should not be selectable")
	}
	// CurseForge is key-gated: not selectable without a key.
	if _, ok := s.For(ctx, Config{Provider: "curseforge", CurseForgeGameID: 432}); ok {
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
	for _, p := range []string{"modrinth", "thunderstore", "hangar", "factorio", "spigot", "github", "umod"} {
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
	if p, ok := withKey.For(ctx, Config{Provider: "curseforge", CurseForgeGameID: 432}); !ok || p == nil {
		t.Errorf("curseforge with key and gameID: ok=%v p=%v", ok, p)
	}
	// CurseForge is also gameID-gated: a key alone isn't enough, the same
	// way Steam needs both a key and an appID.
	if _, ok := withKey.For(ctx, Config{Provider: "curseforge"}); ok {
		t.Error("curseforge without a curseforgeGameID should not be selectable")
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

// TestKeyedLazyCoalescesConcurrentKeyResolution is the B3 regression test:
// keyedLazy must not call keyFunc once per caller — DBKeyFunc does a DB
// read plus a live apiserver Secret GET, and a hot caller resolving the
// same provider many times concurrently (e.g. mod_updates.go fans out one
// goroutine per distinct installed-mod project) would otherwise hit the
// DB/Secret once per goroutine. resolveKey's singleflight coalescing must
// collapse at least some of a batch of concurrent overlapping calls into
// shared keyFunc invocations, while still fully re-resolving on any call
// that does NOT overlap an in-flight one — so key/Secret rotation still
// takes effect immediately rather than waiting out a cache window (see
// TestSet_AvailableTogglesWithSecret in keys_test.go for the invariant this
// preserves).
//
// The synchronization here (and the "over 0 and less than n" bound, not
// exactly 1) mirrors singleflight's own TestDoDupSuppress: reaching the
// line just before Do doesn't guarantee every goroutine's Do call has
// actually landed while the first is still in flight, so an exact count
// isn't a reliable assertion even with careful synchronization.
func TestKeyedLazyCoalescesConcurrentKeyResolution(t *testing.T) {
	ctx := context.Background()
	var calls atomic.Int64
	const n = 10
	var wg1, wg2 sync.WaitGroup
	c := make(chan string, 1)
	keyFunc := func(context.Context, string) string {
		if calls.Add(1) == 1 {
			wg1.Done() // first invocation is now blocked below
		}
		v := <-c
		c <- v // pump; make available to any other invocation that still forms
		// Give any goroutine that has reached the line just before
		// curseforgeLazy, but not yet actually called Do, time to land as
		// a follower of this still-in-flight call rather than a new
		// leader — the same trick singleflight's own TestDoDupSuppress
		// uses.
		time.Sleep(10 * time.Millisecond)
		return v
	}
	s := NewSet("test", keyFunc)

	wg1.Add(1)
	for range n {
		wg1.Add(1)
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			wg1.Done() // reached the line just before curseforgeLazy
			if _, err := s.curseforgeLazy(ctx); err != nil {
				t.Errorf("curseforgeLazy: %v", err)
			}
		}()
	}
	wg1.Wait() // at least one goroutine is in keyFunc; all have reached the call site
	c <- "cf-key"
	wg2.Wait()

	if got := calls.Load(); got <= 0 || got >= n {
		t.Errorf("keyFunc calls = %d, want over 0 and less than %d (some coalescing happened)", got, n)
	}

	// A later, non-overlapping call still fully re-resolves — no staleness.
	before := calls.Load()
	if _, err := s.curseforgeLazy(ctx); err != nil {
		t.Fatalf("curseforgeLazy: %v", err)
	}
	if got := calls.Load(); got != before+1 {
		t.Errorf("keyFunc calls = %d, want %d for the later, non-concurrent call", got, before+1)
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
