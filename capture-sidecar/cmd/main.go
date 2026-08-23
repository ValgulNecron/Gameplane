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
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ValgulNecron/gameplane/capture-sidecar/internal/auth"
	"github.com/ValgulNecron/gameplane/capture-sidecar/internal/httpserver"
)

// Version is overridden at build time via -ldflags (see Dockerfile).
var Version = "dev"

// shutdownTimeout bounds the graceful drain on SIGTERM. An in-flight capture
// download may legitimately still be streaming when the pod is torn down.
const shutdownTimeout = 30 * time.Second

func main() {
	listenAddr := flag.String("listen", "0.0.0.0:9091", "HTTP listen address")
	certFile := flag.String("tls-cert", "/etc/tls/tls.crt", "Server certificate file")
	keyFile := flag.String("tls-key", "/etc/tls/tls.key", "Server key file")
	caFile := flag.String("tls-client-ca", "/etc/tls/ca.crt", "Client CA certificate file")
	captureDataDir := flag.String("capture-dir", "/tmp/captures", "Directory for capture files")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create capture server. ctx (cancelled on SIGTERM/SIGINT) is the parent
	// for every capture goroutine, so shutdown cancels in-flight captures
	// instead of leaking them past server exit.
	captureServer := httpserver.NewServer(ctx, *captureDataDir)

	// Set up mTLS. The certificates come from the same per-GameServer
	// `agent-tls` Secret the agent uses, mounted at /etc/tls.
	tlsConfig, err := auth.ServerTLS(*certFile, *keyFile, *caFile)
	if err != nil {
		slog.Error("failed to setup TLS", "err", err)
		os.Exit(1)
	}

	// Every capture endpoint is wrapped in the mTLS middleware; /healthz is
	// the only unauthenticated route and reports nothing but "ok".
	mux := captureServer.Routes(auth.Middleware)

	server := &http.Server{
		Addr:      *listenAddr,
		Handler:   mux,
		TLSConfig: tlsConfig,
		// ReadHeaderTimeout guards against slow-header connections holding the
		// server open indefinitely (gosec G112).
		ReadHeaderTimeout: 10 * time.Second,
		// ReadTimeout bounds how long reading a full request (headers + body)
		// may take. Every request body handled here is a small JSON control
		// message, so this is safe to bound tightly.
		ReadTimeout: 30 * time.Second,
		// WriteTimeout is deliberately left unset (zero == no limit): capture
		// file downloads can be multi-GiB and legitimately take far longer to
		// stream than any fixed response deadline would allow.
	}

	slog.Info("capture-sidecar starting", "addr", server.Addr, "version", Version)

	// Serve TLS with client-certificate verification (RequireAndVerifyClientCert
	// and TLS 1.2 minimum, from auth.ServerTLS). This is the sidecar's only
	// authentication boundary: on a plaintext listener anything that can reach
	// the pod network could start captures and download another server's
	// packet data, so svcutil.RunHTTP - which is HTTP-only - must not be used
	// here. Empty file arguments make ListenAndServeTLS use the certificates
	// already in server.TLSConfig, matching agent/cmd/main.go.
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}

	slog.Info("capture-sidecar stopped")
}
