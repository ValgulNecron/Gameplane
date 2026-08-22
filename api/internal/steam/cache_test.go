package steam

import (
	"testing"
	"time"
)

func TestCachePositiveHit(t *testing.T) {
	// Test that a positive cache entry is returned.
	clock := &fakeClock{now: time.Unix(0, 0)}
	cache := NewCache(&Options{TTL: 1 * time.Hour}, clock)

	cache.Set("id1", "Player One", 1*time.Hour)

	val, cached := cache.Get("id1")
	if !cached || val != "Player One" {
		t.Errorf("expected cached=true, val=Player One; got cached=%t, val=%s", cached, val)
	}
}

func TestCachePositiveExpiry(t *testing.T) {
	// Test that an expired positive entry is treated as uncached.
	clock := &fakeClock{now: time.Unix(0, 0)}
	cache := NewCache(&Options{TTL: 1 * time.Hour}, clock)

	cache.Set("id1", "Player One", 1*time.Hour)

	// Advance clock past the TTL.
	clock.now = clock.now.Add(2 * time.Hour)

	val, cached := cache.Get("id1")
	if cached {
		t.Errorf("expected uncached after expiry; got cached=true, val=%s", val)
	}
}

func TestCacheNegativeHit(t *testing.T) {
	// Test that a negative cache entry is distinguished from a miss.
	clock := &fakeClock{now: time.Unix(0, 0)}
	cache := NewCache(&Options{NegativeTTL: 15 * time.Minute}, clock)

	// Store a negative entry (empty value).
	cache.Set("unresolvable", "", 15*time.Minute)

	val, cached := cache.Get("unresolvable")
	if !cached || val != "" {
		t.Errorf("expected cached=true, val=empty; got cached=%t, val=%s", cached, val)
	}
}

func TestCacheNegativeExpiry(t *testing.T) {
	// Test that a negative entry expires on its own shorter TTL.
	clock := &fakeClock{now: time.Unix(0, 0)}
	cache := NewCache(&Options{NegativeTTL: 15 * time.Minute}, clock)

	cache.Set("unresolvable", "", 15*time.Minute)

	// Advance clock past the negative TTL.
	clock.now = clock.now.Add(20 * time.Minute)

	val, cached := cache.Get("unresolvable")
	if cached {
		t.Errorf("expected uncached after negative TTL expiry; got cached=true, val=%s", val)
	}
}

func TestCacheMiss(t *testing.T) {
	// Test that a missing entry returns (empty, false).
	cache := NewCache(&Options{}, nil)

	val, cached := cache.Get("nonexistent")
	if cached {
		t.Errorf("expected uncached=true for missing key; got cached=false, val=%s", val)
	}
}

func TestCacheLRUEviction(t *testing.T) {
	// Test that the cache evicts the least recently used entry when at capacity.
	clock := &fakeClock{now: time.Unix(0, 0)}
	opts := &Options{MaxEntries: 3, TTL: 1 * time.Hour}
	cache := NewCache(opts, clock)

	// Fill the cache.
	cache.Set("id1", "Player1", 1*time.Hour)
	cache.Set("id2", "Player2", 1*time.Hour)
	cache.Set("id3", "Player3", 1*time.Hour)

	if cache.Len() != 3 {
		t.Errorf("expected Len()=3, got %d", cache.Len())
	}

	// Adding a fourth entry should evict the LRU (id1).
	cache.Set("id4", "Player4", 1*time.Hour)

	if cache.Len() != 3 {
		t.Errorf("expected Len()=3 after LRU eviction, got %d", cache.Len())
	}

	// Verify id1 was evicted.
	_, cached := cache.Get("id1")
	if cached {
		t.Error("expected id1 to be evicted")
	}

	// Verify the others are still there.
	for _, id := range []string{"id2", "id3", "id4"} {
		_, cached := cache.Get(id)
		if !cached {
			t.Errorf("expected %s to still be cached", id)
		}
	}
}

func TestCacheRecencyUpdate(t *testing.T) {
	// Test that a read (Get) updates the recency, so the correct entry is evicted.
	clock := &fakeClock{now: time.Unix(0, 0)}
	opts := &Options{MaxEntries: 3, TTL: 1 * time.Hour}
	cache := NewCache(opts, clock)

	cache.Set("id1", "Player1", 1*time.Hour)
	cache.Set("id2", "Player2", 1*time.Hour)
	cache.Set("id3", "Player3", 1*time.Hour)

	// Touch id1 to move it to the head (most recently used).
	cache.Get("id1")

	// Now add id4; id2 (not id1) should be evicted.
	cache.Set("id4", "Player4", 1*time.Hour)

	_, cached := cache.Get("id2")
	if cached {
		t.Error("expected id2 to be evicted (id1 was more recently used)")
	}

	_, cached = cache.Get("id1")
	if !cached {
		t.Error("expected id1 to still be cached")
	}
}

func TestCacheZeroMaxEntriesFallback(t *testing.T) {
	// Test that zero or negative MaxEntries falls back to the default.
	opts := &Options{MaxEntries: -1, TTL: 1 * time.Hour}
	cache := NewCache(opts, nil)

	// Fill beyond the negative value (which would mean no limit if not normalized).
	for i := 0; i < 100; i++ {
		cache.Set(string(rune('a'+i)), "Player", 1*time.Hour)
	}

	// Len should be pinned at the default MaxEntries (10000), not growing unbounded.
	if cache.Len() > DefaultMaxEntries {
		t.Errorf("expected Len() <= DefaultMaxEntries (%d), got %d", DefaultMaxEntries, cache.Len())
	}
}

func TestCacheConcurrentReadWrite(t *testing.T) {
	// Test that concurrent readers and writers don't cause a race.
	// This is best verified with `go test -race`.
	cache := NewCache(&Options{MaxEntries: 100}, nil)

	done := make(chan bool, 10)

	// 5 writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				cache.Set("id-"+string(rune('a'+id))+"-"+string(rune('0'+j%10)), "value", 1*time.Hour)
			}
			done <- true
		}(i)
	}

	// 5 readers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				cache.Get("id-" + string(rune('a'+id)) + "-" + string(rune('0'+j%10)))
			}
			done <- true
		}(i)
	}

	// Wait for all to finish.
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestCacheNegativeEntryDoesNotMaskLaterPositive(t *testing.T) {
	// Test that a negative entry, once expired, can be replaced with a positive one.
	clock := &fakeClock{now: time.Unix(0, 0)}
	opts := &Options{TTL: 1 * time.Hour, NegativeTTL: 15 * time.Minute}
	cache := NewCache(opts, clock)

	// Store a negative entry.
	cache.Set("id1", "", 15*time.Minute)

	// Verify it's negative.
	val, cached := cache.Get("id1")
	if !cached || val != "" {
		t.Error("expected negative entry cached")
	}

	// Expire the negative entry.
	clock.now = clock.now.Add(20 * time.Minute)

	// Verify it's gone.
	_, cached = cache.Get("id1")
	if cached {
		t.Error("expected negative entry expired")
	}

	// Store a positive entry.
	cache.Set("id1", "Player One", 1*time.Hour)

	// Verify the positive entry is now cached.
	val, cached = cache.Get("id1")
	if !cached || val != "Player One" {
		t.Errorf("expected positive entry cached; got cached=%t, val=%s", cached, val)
	}
}

func TestCacheUpdateExistingEntry(t *testing.T) {
	// Test that updating an existing entry keeps it at the head (most recently used).
	clock := &fakeClock{now: time.Unix(0, 0)}
	opts := &Options{MaxEntries: 3, TTL: 1 * time.Hour}
	cache := NewCache(opts, clock)

	cache.Set("id1", "Player1", 1*time.Hour)
	cache.Set("id2", "Player2", 1*time.Hour)

	// Update id1.
	cache.Set("id1", "Player1-Updated", 1*time.Hour)

	// Verify the update.
	val, cached := cache.Get("id1")
	if !cached || val != "Player1-Updated" {
		t.Errorf("expected updated value; got %s", val)
	}

	// Add id3.
	cache.Set("id3", "Player3", 1*time.Hour)

	// Add id4; id2 should be evicted (not id1, which was updated).
	cache.Set("id4", "Player4", 1*time.Hour)

	_, cached = cache.Get("id2")
	if cached {
		t.Error("expected id2 to be evicted (id1 was more recently updated)")
	}

	_, cached = cache.Get("id1")
	if !cached {
		t.Error("expected id1 to still be cached")
	}
}
