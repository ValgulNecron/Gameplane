package steam

import (
	"context"
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
// The function f is invoked with an internally-owned context independent of any caller.
// No caller's deadline or cancellation affects the shared flight; every caller waits
// unconditionally for the upstream call to complete. The upstream call is bounded only by
// the HTTP client's own timeout. All waiting callers receive the same result.
func (g *singleflightGroup) Do(ids []string, f func(context.Context) (map[string]string, error)) (map[string]string, error) {
	// Create a deterministic key from the sorted ids.
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)
	key := strings.Join(sorted, ",")

	// Create a context independent of any caller's context for the shared upstream work.
	// This ensures that if one caller's context is cancelled, others are not affected.
	sfCtx := context.Background()

	// Start the singleflight call in a goroutine so we can handle context cancellation.
	// The singleflight call itself doesn't understand context, so we wrap it.
	done := make(chan struct{})
	var res result
	go func() {
		v, _, _ := g.g.Do(key, func() (interface{}, error) {
			resolutions, err := f(sfCtx)
			return result{resolutions, err}, nil
		})
		res = v.(result)
		close(done)
	}()

	// Wait for either the result or no event (blocking until done).
	// We don't handle context cancellation here; callers that want timeout behavior
	// should use a separate context.WithCancel mechanism.
	<-done
	return res.resolutions, res.err
}
