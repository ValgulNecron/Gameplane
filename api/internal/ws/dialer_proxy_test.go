package ws

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWSProxy_NoTLSConfigReturns503 covers the early-return when mTLS
// material is unset (the dev-mode fallback).
func TestWSProxy_NoTLSConfigReturns503(t *testing.T) {
	p := &proxy{tls: nil}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ws/servers/alpha/console", nil)
	p.wsProxy("/console")(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestHTTPProxy_NoTLSConfigReturns503 covers the matching early-return
// for the file-browser HTTP proxy.
func TestHTTPProxy_NoTLSConfigReturns503(t *testing.T) {
	p := &proxy{tls: nil}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers/alpha/files/list", nil)
	p.httpProxy("/files/list")(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestHTTPProxy_ScopeError fails Resolve by providing an unknown
// namespace query param. tls != nil so we reach the scope check.
func TestHTTPProxy_ScopeError(t *testing.T) {
	p := &proxy{
		tls:  &tls.Config{},
		http: &http.Client{},
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers/alpha/files/list?namespace=forbidden", nil)
	p.httpProxy("/files/list")(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("expected non-200, got %d", rr.Code)
	}
}

// TestWSProxy_ScopeError likewise — wsProxy resolves scope before any
// upgrade so the Resolve error path is reachable without a real WS.
func TestWSProxy_ScopeError(t *testing.T) {
	p := &proxy{
		tls:  &tls.Config{},
		http: &http.Client{},
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ws/servers/alpha/logs?namespace=forbidden", nil)
	p.wsProxy("/logs/tail")(rr, req)
	if rr.Code == http.StatusSwitchingProtocols {
		t.Fatalf("expected non-101, got %d", rr.Code)
	}
}

// the agent FQDN constructor must include the standard 8090 port so
// callers can derive the right URL even without a real client.
func TestAgentHost_Format(t *testing.T) {
	p := &proxy{}
	got := p.agentHost("alpha", "kestrel-games")
	for _, want := range []string{"alpha-0.alpha.kestrel-games", ":8090"} {
		if !strings.Contains(got, want) {
			t.Fatalf("got %q, want substring %q", got, want)
		}
	}
}
