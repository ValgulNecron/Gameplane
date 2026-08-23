// Command capture-sidecar is an optional ephemeral container that runs
// alongside a game server to capture network traffic to PCAPNG files.
// It listens on :9091 for mTLS-authenticated control commands from the
// API server: start a packet capture with a BPF filter, stop an active
// capture, poll capture status, or download a completed PCAPNG file.
//
// Captured packets are written to emptyDir volume at /tmp/captures.
// The sidecar is injected only when spec.capture.enabled = true on the
// GameServer; the capture emptyDir volume is pre-provisioned on all game
// pods (see contracts/capture-sidecar.md for full details).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Version is overridden at build time via -ldflags (see Dockerfile).
var Version = "dev"

func main() {
	listenAddr := flag.String("listen", "0.0.0.0:9091", "HTTP listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: mux,
	}

	// Start server in a goroutine
	errs := make(chan error, 1)
	go func() {
		slog.Info("capture-sidecar listening", "addr", server.Addr, "version", Version)
		errs <- server.ListenAndServe()
	}()

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		slog.Info("capture-sidecar shutting down")
	case err := <-errs:
		if err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
		os.Exit(1)
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "ok")
}
