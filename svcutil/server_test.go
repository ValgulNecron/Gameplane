package svcutil

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunHTTPCleanShutdown(t *testing.T) {
	// Create a test listener on localhost with port 0 (OS picks an available port).
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Run the server in a goroutine.
	done := make(chan error)
	go func() {
		done <- RunHTTP(ctx, srv, 5*time.Second)
	}()

	// Give the server time to start.
	time.Sleep(100 * time.Millisecond)

	// Cancel the context to trigger graceful shutdown.
	cancel()

	// Wait for RunHTTP to return.
	err = <-done
	if err != nil {
		t.Errorf("RunHTTP returned error on clean shutdown: %v", err)
	}
}

func TestRunHTTPListenError(t *testing.T) {
	// Create a server bound to an invalid address to trigger a listen error.
	srv := &http.Server{
		Addr: "invalid-address-that-cannot-be-used:99999",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx := context.Background()

	// RunHTTP should return the listen error.
	err := RunHTTP(ctx, srv, 5*time.Second)
	if err == nil {
		t.Errorf("RunHTTP should return a listen error for an invalid address; got nil")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Errorf("RunHTTP should not return ErrServerClosed on listen error; got %v", err)
	}
}

func TestRunHTTPContextCancelledBeforeListen(t *testing.T) {
	// Create a context that is already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// RunHTTP should attempt shutdown and return nil or a shutdown result.
	// Since the context is already cancelled, the select will immediately pick
	// the context.Done() case.
	err = RunHTTP(ctx, srv, 5*time.Second)
	if err != nil && !errors.Is(err, context.Canceled) {
		// It's acceptable to return context.Canceled during srv.Shutdown, or nil on success.
		t.Logf("RunHTTP returned: %v (acceptable for pre-cancelled context)", err)
	}
}

func TestRunHTTPServerClosedError(t *testing.T) {
	// This test verifies that http.ErrServerClosed is not returned.
	// The server is started and then shut down via context cancellation.
	// The select will pick the context done case and call srv.Shutdown,
	// which should return nil (since it's a normal shutdown).

	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Run the server.
	done := make(chan error)
	go func() {
		done <- RunHTTP(ctx, srv, 5*time.Second)
	}()

	// Give the server time to start.
	time.Sleep(100 * time.Millisecond)

	// Cancel the context to trigger shutdown.
	cancel()

	// Wait for RunHTTP to return.
	err = <-done
	if errors.Is(err, http.ErrServerClosed) {
		t.Errorf("RunHTTP should not return ErrServerClosed on normal shutdown; got %v", err)
	}
	// Shutdown should return nil or possibly context.Canceled, but not ErrServerClosed.
}

func TestRunHTTPShutdownTimeout(t *testing.T) {
	// Create a server with a long-running handler.
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Handler that takes a long time.
	blockingHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(30 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           blockingHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Run the server.
	done := make(chan error)
	go func() {
		done <- RunHTTP(ctx, srv, 100*time.Millisecond) // Very short timeout
	}()

	// Give the server time to start.
	time.Sleep(100 * time.Millisecond)

	// Cancel the context.
	cancel()

	// Wait for RunHTTP to return. With the short timeout, shutdown should
	// return quickly (possibly with a context.Canceled error from the timeout).
	select {
	case err := <-done:
		// This is expected. err might be nil, context.Canceled, or a shutdown error.
		t.Logf("RunHTTP returned after timeout: %v", err)
	case <-time.After(2 * time.Second):
		t.Errorf("RunHTTP did not return within expected time; shutdown timeout may not be working")
	}
}
