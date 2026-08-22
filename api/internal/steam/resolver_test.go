package steam

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResolverNoKeyConfigured(t *testing.T) {
	// With no key, NewResolver returns nil and no outbound calls are made.
	r := NewResolver("", nil, nil)
	if r != nil {
		t.Error("NewResolver with empty key should return nil")
	}

	// Resolve on nil resolver returns empty map.
	result := r.Resolve(context.Background(), []string{"12345"})
	if len(result) != 0 {
		t.Error("Resolve on nil resolver should return empty map")
	}
}

func TestResolverSuccessfulBatch(t *testing.T) {
	// httptest server returning a valid Steam API response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ISteamUser/GetPlayerSummaries/v2/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Parse query and verify the key and steamids.
		q := r.URL.Query()
		if q.Get("key") != "test-key" {
			t.Errorf("expected key=test-key, got %s", q.Get("key"))
		}

		ids := q["steamids"]
		if len(ids) != 3 {
			t.Errorf("expected 3 steamids, got %d", len(ids))
		}

		// Return a sample Steam API response.
		response := `{
  "response": {
    "players": [
      {"steamid": "76561198000000001", "personaname": "Player1"},
      {"steamid": "76561198000000002", "personaname": "Player2"},
      {"steamid": "76561198000000003", "personaname": "Player3"}
    ]
  }
}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	// Create a resolver with a custom HTTP client pointing to the test server.
	r := &Resolver{
		apiKey:  "test-key",
		baseURL: server.URL + "/ISteamUser/GetPlayerSummaries/v2/",
		client:  &http.Client{Timeout: 2 * time.Second},
		cache:   NewCache(&Options{}, nil),
		opts:    (&Options{}).Normalize(),
		sf:      &singleflightGroup{},
	}

	// Test getPlayerSummaries directly with the test server.
	result, err := r.getPlayerSummaries(context.Background(), []string{"76561198000000001", "76561198000000002", "76561198000000003"})
	if err != nil {
		t.Fatalf("getPlayerSummaries failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 resolved ids, got %d", len(result))
	}

	for _, id := range []string{"76561198000000001", "76561198000000002", "76561198000000003"} {
		if _, ok := result[id]; !ok {
			t.Errorf("id %s not in result", id)
		}
	}
}

func TestResolverCacheHit(t *testing.T) {
	// Test that cached entries are returned without upstream calls.
	clock := &fakeClock{now: time.Unix(0, 0)}
	r := &Resolver{
		apiKey: "test-key",
		client: &http.Client{},
		cache:  NewCache(&Options{TTL: 1 * time.Hour}, clock),
		opts:   (&Options{TTL: 1 * time.Hour}).Normalize(),
		sf:     &singleflightGroup{},
	}

	// Pre-populate the cache.
	r.cache.Set("id1", "Player1", 1*time.Hour)
	r.cache.Set("id2", "Player2", 1*time.Hour)

	// Resolve should return cached values without errors.
	result := r.Resolve(context.Background(), []string{"id1", "id2"})

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}

	if result["id1"] != "Player1" || result["id2"] != "Player2" {
		t.Errorf("expected cached values, got %v", result)
	}
}

func TestResolverNegativeCachingPreventsRetry(t *testing.T) {
	// Test that a negative cache entry prevents retries.
	clock := &fakeClock{now: time.Unix(0, 0)}
	r := &Resolver{
		apiKey: "test-key",
		client: &http.Client{},
		cache:  NewCache(&Options{NegativeTTL: 15 * time.Minute}, clock),
		opts:   (&Options{NegativeTTL: 15 * time.Minute}).Normalize(),
		sf:     &singleflightGroup{},
	}

	// Manually set a negative cache entry.
	r.cache.Set("unresolvable-id", "", 15*time.Minute)

	// Resolve should return empty map without calling upstream.
	result := r.Resolve(context.Background(), []string{"unresolvable-id"})
	if len(result) != 0 {
		t.Error("expected empty result for negative cache hit")
	}
}

func TestResolverNegativeCacheExpiry(t *testing.T) {
	// Test that a negative entry expires and allows retries.
	clock := &fakeClock{now: time.Unix(0, 0)}
	r := &Resolver{
		apiKey: "test-key",
		client: &http.Client{},
		cache:  NewCache(&Options{NegativeTTL: 15 * time.Minute}, clock),
		opts:   (&Options{NegativeTTL: 15 * time.Minute}).Normalize(),
		sf:     &singleflightGroup{},
	}

	r.cache.Set("unresolvable-id", "", 15*time.Minute)

	// Advance the clock past the negative TTL.
	clock.now = clock.now.Add(20 * time.Minute)

	// Now the entry should be expired.
	val, cached := r.cache.Get("unresolvable-id")
	if cached {
		t.Errorf("expected uncached after negative TTL expiry, got cached=%t, val=%q", cached, val)
	}
}

func TestResolverPartialResponse(t *testing.T) {
	// Test that a response with fewer players than requested ids is handled gracefully.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only return one of the three requested ids.
		response := `{
  "response": {
    "players": [
      {"steamid": "76561198000000001", "personaname": "Player1"}
    ]
  }
}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	r := &Resolver{
		apiKey:  "test-key",
		baseURL: server.URL + "/ISteamUser/GetPlayerSummaries/v2/",
		client:  &http.Client{Timeout: 2 * time.Second},
		cache:   NewCache(&Options{}, nil),
		opts:    (&Options{}).Normalize(),
		sf:      &singleflightGroup{},
	}

	// Test with three ids but only one returned.
	result, err := r.getPlayerSummaries(context.Background(), []string{"1", "2", "3"})
	if err != nil {
		t.Fatalf("getPlayerSummaries failed: %v", err)
	}

	// Only the returned id should be in the result.
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}

	if result["76561198000000001"] != "Player1" {
		t.Error("expected Player1 in result")
	}
}

func TestResolverNon200Status(t *testing.T) {
	// Test that non-200 responses are treated as errors.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"response": {}}`)
	}))
	defer server.Close()

	resolver := &Resolver{
		apiKey:  "bad-key",
		baseURL: server.URL + "/ISteamUser/GetPlayerSummaries/v2/",
		client:  &http.Client{Timeout: 2 * time.Second},
		cache:   NewCache(&Options{}, nil),
		opts:    (&Options{}).Normalize(),
		sf:      &singleflightGroup{},
	}

	// Call getPlayerSummaries against the test server returning a non-200 status.
	result, err := resolver.getPlayerSummaries(context.Background(), []string{"76561198000000001"})

	// A non-200 response should return an error.
	if err == nil {
		t.Error("expected error for non-200 status, got nil")
	}

	// The result should be nil on error.
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}

	// Verify the error message does not leak the URL or key.
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestResolverMalformedJSON(t *testing.T) {
	// Test that malformed JSON responses are handled gracefully.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{invalid json}`)
	}))
	defer server.Close()

	resolver := &Resolver{
		apiKey:  "test-key",
		baseURL: server.URL + "/ISteamUser/GetPlayerSummaries/v2/",
		client:  &http.Client{Timeout: 2 * time.Second},
		cache:   NewCache(&Options{}, nil),
		opts:    (&Options{}).Normalize(),
		sf:      &singleflightGroup{},
	}

	// Call getPlayerSummaries against the test server returning malformed JSON.
	result, err := resolver.getPlayerSummaries(context.Background(), []string{"76561198000000001"})

	// Malformed JSON should return an error, not crash.
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}

	// The result should be nil on error.
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}

	// Verify the error message does not leak the URL or key.
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestResolverNetworkFailure(t *testing.T) {
	// Test that network failures are handled gracefully and return an error without leaking the key.
	resolver := &Resolver{
		apiKey:  "sensitive-api-key",
		baseURL: "https://127.0.0.1:1/ISteamUser/GetPlayerSummaries/v2/", // Unreachable address
		client:  &http.Client{Timeout: 100 * time.Millisecond},            // Short timeout
		cache:   NewCache(&Options{}, nil),
		opts:    (&Options{}).Normalize(),
		sf:      &singleflightGroup{},
	}

	// Call getPlayerSummaries against an unreachable address.
	result, err := resolver.getPlayerSummaries(context.Background(), []string{"76561198000000001"})

	// A network failure should return an error.
	if err == nil {
		t.Error("expected error for network failure, got nil")
	}

	// The result should be nil on error.
	if result != nil {
		t.Errorf("expected nil result on error, got %v", result)
	}

	// Verify the error message does not leak the API key.
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error message")
	}

	// The error message should never contain the sensitive API key.
	if strings.Contains(errStr, "sensitive-api-key") {
		t.Errorf("error message leaked API key: %s", errStr)
	}
}

func TestResolverLargeBatchSplit(t *testing.T) {
	// Test that ids are correctly split into batches of at most 100.
	callCount := 0
	var mu sync.Mutex
	receivedBatches := []int{} // Track batch sizes for each call

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		ids := r.URL.Query()["steamids"]
		receivedBatches = append(receivedBatches, len(ids))
		mu.Unlock()

		// Return a valid response for all requested ids.
		var players []string
		for _, id := range ids {
			players = append(players, fmt.Sprintf(`{"steamid": "%s", "personaname": "Player%s"}`, id, id))
		}
		response := fmt.Sprintf(`{
  "response": {
    "players": [%s]
  }
}`, strings.Join(players, ","))

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, response)
	}))
	defer server.Close()

	resolver := &Resolver{
		apiKey:  "test-key",
		baseURL: server.URL + "/ISteamUser/GetPlayerSummaries/v2/",
		client:  &http.Client{Timeout: 2 * time.Second},
		cache:   NewCache(&Options{}, nil),
		opts:    (&Options{}).Normalize(),
		sf:      &singleflightGroup{},
	}

	// Create 250 ids (will be split into 3 batches: 100 + 100 + 50).
	var ids []string
	for i := 0; i < 250; i++ {
		ids = append(ids, fmt.Sprintf("7656119800000%04d", i))
	}

	// Resolve all ids.
	result, err := resolver.resolveBatch(context.Background(), ids)
	if err != nil {
		t.Fatalf("resolveBatch failed: %v", err)
	}

	// Verify all ids were resolved.
	if len(result) != 250 {
		t.Errorf("expected 250 results, got %d", len(result))
	}

	// Verify the batching was correct (3 batches: 100 + 100 + 50).
	if callCount != 3 {
		t.Errorf("expected 3 upstream calls, got %d", callCount)
	}

	mu.Lock()
	if len(receivedBatches) != 3 || receivedBatches[0] != 100 || receivedBatches[1] != 100 || receivedBatches[2] != 50 {
		t.Errorf("expected batch sizes [100, 100, 50], got %v", receivedBatches)
	}
	mu.Unlock()
}

// fakeClock is a test clock that can be advanced manually.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.now
}
