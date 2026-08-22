package steam

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/sync/singleflight"
)

// singleflightGroup wraps singleflight.Group to handle concurrent resolution requests.
// Multiple concurrent requests for overlapping sets of ids are collapsed into a single upstream call
// whose result is fanned out to all waiters. The key is the sorted, comma-separated list of ids.
type singleflightGroup struct {
	g singleflight.Group
}

// result is the result type passed through singleflight.
type result struct {
	resolutions map[string]string
	err         error
}

// Do executes the function f once for the sorted set of ids, even if multiple callers invoke Do
// concurrently with the same set of ids. All callers receive the same result.
//
// If the context of any caller is cancelled, only that caller's wait is abandoned; the shared
// flight continues for the others, and all receive the same result once it completes.
func (g *singleflightGroup) Do(ctx context.Context, ids []string, f func() (map[string]string, error)) (map[string]string, error) {
	// Create a deterministic key from the sorted ids.
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)
	key := strings.Join(sorted, ",")

	// Start the singleflight call in a goroutine so we can handle context cancellation.
	// The singleflight call itself doesn't understand context, so we wrap it.
	done := make(chan struct{})
	var res result
	go func() {
		v, err, _ := g.g.Do(key, func() (interface{}, error) {
			resolutions, err := f()
			return result{resolutions, err}, nil
		})
		res = v.(result)
		close(done)
	}()

	// Wait for either the result or context cancellation.
	select {
	case <-done:
		return res.resolutions, res.err
	case <-ctx.Done():
		// Caller's context was cancelled, but the shared flight continues.
		// Return the context error; the shared flight's result, once ready, is available
		// only to other waiters, not to this one.
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	}
}
