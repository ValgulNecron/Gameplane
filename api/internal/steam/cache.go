// Package steam resolves Steam IDs to display names via the public Steam Web API.
// Resolution is optional: if no API key is configured, callers fall back to raw
// Steam IDs. Results are cached in a bounded in-process LRU cache with positive
// and negative TTLs, and concurrent resolution requests are de-duplicated via
// singleflight to avoid redundant API calls.
package steam

import (
	"container/list"
	"sync"
	"time"
)

// Clock is an abstraction for time.Now, allowing tests to inject a fake clock.
type Clock interface {
	Now() time.Time
}

// realClock wraps time.Now.
type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

// entry holds a cached resolution (positive or negative) along with its expiration time.
// A positive entry has a non-empty value; a negative entry is when Steam failed to resolve the id.
type entry struct {
	id     string    // the Steam ID key
	value  string    // non-empty = positive, empty = negative
	expiry time.Time // when this entry expires
}

// Cache is a bounded, thread-safe, LRU cache of Steam ID resolutions.
// Positive entries (successful resolutions) live for opts.TTL.
// Negative entries (ids Steam didn't return) live for opts.NegativeTTL.
// Eviction is LRU when the cache reaches MaxEntries.
//
// The cache is in-process memory only: no persistence, no distribution across replicas.
// A cold start simply re-resolves; the handful of duplicate Steam calls across replicas is accepted.
type Cache struct {
	opts  *Options
	clock Clock
	mu    sync.Mutex
	m     map[string]*list.Element // id -> position in recency list
	list  *list.List               // LRU order
}

// NewCache returns a Cache ready to use.
// clock is optional; if nil, Cache uses time.Now().
func NewCache(opts *Options, clock Clock) *Cache {
	if opts == nil {
		opts = &Options{}
	}
	opts = opts.Normalize()
	if clock == nil {
		clock = realClock{}
	}
	return &Cache{
		opts:  opts,
		clock: clock,
		m:     make(map[string]*list.Element),
		list:  list.New(),
	}
}

// Get returns the cached resolution for id, or ("", false) if not cached, expired, or negative.
// A negative entry (empty value but cached) returns ("", true), signaling "this id was tried and failed".
// The returned bool indicates whether a result is cached (including negative).
func (c *Cache) Get(id string) (value string, cached bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.m[id]
	if !ok {
		return "", false
	}
	ent, _ := el.Value.(entry) // Safe: el.Value is always an entry since we control what gets stored

	if c.clock.Now().After(ent.expiry) {
		// Expired; clean it up.
		c.list.Remove(el)
		delete(c.m, id)
		return "", false
	}

	// Hit; move to head (most recently used).
	c.list.MoveToFront(el)
	return ent.value, true
}

// Set stores a resolution in the cache (positive if value != "", negative otherwise).
// It must be paired with the appropriate TTL: positive entries with opts.TTL, negative with opts.NegativeTTL.
func (c *Cache) Set(id, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.clock.Now()
	ent := entry{
		id:     id,
		value:  value,
		expiry: now.Add(ttl),
	}

	if el, ok := c.m[id]; ok {
		// Update existing entry.
		el.Value = ent
		c.list.MoveToFront(el)
		return
	}

	// New entry; may need to evict if at capacity.
	if len(c.m) >= c.opts.MaxEntries {
		// Evict the least recently used (tail).
		tail := c.list.Back()
		c.list.Remove(tail)
		tailEnt, _ := tail.Value.(entry) // Safe: tail.Value is always an entry since we control what gets stored
		delete(c.m, tailEnt.id)
	}

	el := c.list.PushFront(ent)
	c.m[id] = el
}

// Len returns the current number of cached entries.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}
