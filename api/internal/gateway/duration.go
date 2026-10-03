package gateway

import (
	"context"
	"time"
)

const operationTimeout = 30 * time.Second

// requestContext keeps ordinary operations bounded without imposing their
// deadline on healthy streams or transfers. A positive MaxRequestDuration is
// an optional total lifetime for every operation. Peer certificate expiry and
// parent cancellation always apply, including to upgraded WebSockets.
func (h *handler) requestContext(parent context.Context, peerExpiry time.Time, longRunning bool) (context.Context, context.CancelFunc) {
	deadline := peerExpiry
	limit := h.cfg.MaxRequestDuration
	if !longRunning && (limit == 0 || limit > operationTimeout) {
		limit = operationTimeout
	}
	if limit > 0 {
		if maximum := time.Now().Add(limit); maximum.Before(deadline) {
			deadline = maximum
		}
	}
	return context.WithDeadline(parent, deadline)
}
