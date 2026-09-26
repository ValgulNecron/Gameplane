package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"WARN":  slog.LevelWarn,
		"error": slog.LevelError,
		// Unknown values degrade to info rather than crashing the sidecar.
		"":        slog.LevelInfo,
		"verbose": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLogLevel(in); got != want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMetricsServer_ServesPrometheusMetrics(t *testing.T) {
	srv := newMetricsServer(":0")
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	srv.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics code=%d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "go_goroutines") {
		t.Fatalf("GET /metrics body is not Prometheus text:\n%s", rr.Body.String())
	}
}

func TestMetricsServer_ServesOnlyMetrics(t *testing.T) {
	// The metrics listener must never carry the control mux's routes: it
	// has no auth, so anything else served here would be reachable to any
	// in-cluster pod without a client cert or token.
	srv := newMetricsServer(":0")
	for _, path := range []string{"/", "/healthz", "/console", "/files", "/players"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		srv.Handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("GET %s on the metrics listener code=%d, want 404", path, rr.Code)
		}
	}
}
