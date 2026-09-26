package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

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
	srv := newMetricsServer(":0")
	for _, path := range []string{"/", "/healthz", "/auth/providers", "/servers"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		srv.Handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("GET %s on the metrics listener code=%d, want 404", path, rr.Code)
		}
	}
}

func TestConfig_MetricsListenerSeparateFromAPIListener(t *testing.T) {
	// Clear any GAMEPLANE_METRICS_ADDR from the environment for this test;
	// t.Setenv restores the original value afterwards.
	t.Setenv("GAMEPLANE_METRICS_ADDR", "")
	if err := os.Unsetenv("GAMEPLANE_METRICS_ADDR"); err != nil {
		t.Fatal(err)
	}

	var c config
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	c.bindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.metricsAddr != ":9090" {
		t.Fatalf("default metricsAddr = %q, want %q", c.metricsAddr, ":9090")
	}
	if c.metricsAddr == c.addr {
		t.Fatalf("metrics listener shares the API listener address %q", c.addr)
	}

	var o config
	fs = flag.NewFlagSet("serve", flag.ContinueOnError)
	o.bindFlags(fs)
	if err := fs.Parse([]string{"--metrics-addr=:9191"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if o.metricsAddr != ":9191" {
		t.Fatalf("metricsAddr = %q, want %q", o.metricsAddr, ":9191")
	}
}
