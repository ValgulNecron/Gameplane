package svcutil

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// RunHTTP runs an HTTP server until the context is cancelled, then performs a
// graceful shutdown. It returns nil on clean shutdown and the listen error
// otherwise.
//
// The function starts the server's ListenAndServe in a goroutine and selects
// on either a listen error (returned immediately) or context cancellation.
// On context cancellation, it calls srv.Shutdown with a deadline-bounded
// context derived from the provided shutdownTimeout. The http.ErrServerClosed
// error (which occurs after successful shutdown) is mapped to nil.
func RunHTTP(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, shutCancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer shutCancel()
		return srv.Shutdown(shutCtx)
	}
}
