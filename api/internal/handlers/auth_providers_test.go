package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthProviders_LocalOnly(t *testing.T) {
	rr := httptest.NewRecorder()
	AuthProvidersHandler(false, "")(rr, httptest.NewRequest(http.MethodGet, "/auth/providers", nil))
	var resp loginProvidersResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Providers) != 1 || resp.Providers[0].Kind != "local" {
		t.Fatalf("want only local provider, got %+v", resp.Providers)
	}
}

func TestAuthProviders_OIDCEnabledUsesLabel(t *testing.T) {
	rr := httptest.NewRecorder()
	AuthProvidersHandler(true, "Acme SSO")(rr, httptest.NewRequest(http.MethodGet, "/auth/providers", nil))
	var resp loginProvidersResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Providers) != 2 {
		t.Fatalf("want local + oidc, got %+v", resp.Providers)
	}
	var oidc *loginProvider
	for i := range resp.Providers {
		if resp.Providers[i].Kind == "oidc" {
			oidc = &resp.Providers[i]
		}
	}
	if oidc == nil || oidc.Label != "Acme SSO" {
		t.Fatalf("oidc provider label = %+v, want Acme SSO", oidc)
	}
}

func TestAuthProviders_OIDCDefaultLabel(t *testing.T) {
	rr := httptest.NewRecorder()
	AuthProvidersHandler(true, "")(rr, httptest.NewRequest(http.MethodGet, "/auth/providers", nil))
	body := rr.Body.String()
	if !strings.Contains(body, "Single sign-on") {
		t.Fatalf("empty label should default to generic, got %s", body)
	}
	// Login-privacy: never leak an issuer URL / hostname in the pre-auth
	// response. The handler has no access to the issuer, but guard anyway.
	if strings.Contains(body, "http") || strings.Contains(body, "://") {
		t.Fatalf("providers response must not contain a URL: %s", body)
	}
}
