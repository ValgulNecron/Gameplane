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

	// Call singleflight.Group.Do directly. Multiple callers arrive concurrently and
	// singleflight collapses them into a single upstream invocation. The closure always
	// returns nil as the error (error is wrapped in the result struct), so err will always
	// be nil; we check it explicitly to satisfy errcheck.
	v, err, _ := g.g.Do(key, func() (interface{}, error) {
		resolutions, err := f(sfCtx)
		return result{resolutions, err}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("unexpected error from singleflight: %w", err)
	}

	res := v.(result)
	return res.resolutions, res.err
}
