package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
)

// providersRegistry builds a registry over a fresh store, optionally
// seeded with an auth config row. The helm/legacy provider paths are
// covered in the auth package's registry tests (they need a fake IdP).
func providersRegistry(t *testing.T, authJSON string) *auth.Registry {
	t.Helper()
	store := newTestStore(t)
	if authJSON != "" {
		seedAuthConfig(t, store, authJSON)
	}
	noSecrets := func(context.Context, string) (map[string][]byte, error) {
		t.Fatal("providers listing must not read Secrets")
		return nil, nil
	}
	return auth.NewRegistry(store, noSecrets, nil, "")
}

func seedAuthConfig(t *testing.T, store *db.Store, blob string) {
	t.Helper()
	if _, err := store.DB.ExecContext(context.Background(),
		`INSERT INTO config(key, value, updated_at) VALUES ('auth', ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, blob); err != nil {
		t.Fatalf("seed auth config: %v", err)
	}
}

func getProviders(t *testing.T, reg *auth.Registry) (loginProvidersResp, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	AuthProvidersHandler(reg)(rr, httptest.NewRequest(http.MethodGet, "/auth/providers", nil))
	var resp loginProvidersResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp, rr.Body.String()
}

func TestAuthProviders_DefaultIsLocalOnly(t *testing.T) {
	resp, _ := getProviders(t, providersRegistry(t, ""))
	if len(resp.Providers) != 1 || resp.Providers[0].Kind != "local" {
		t.Fatalf("want only local provider, got %+v", resp.Providers)
	}
	if resp.Providers[0].Label != "Local account" {
		t.Fatalf("local label = %q", resp.Providers[0].Label)
	}
}

func TestAuthProviders_ListsEnabledDBProviders(t *testing.T) {
	reg := providersRegistry(t, `{"providers":[
		{"name":"local","kind":"local","enabled":true},
		{"name":"corp","kind":"oidc","displayName":"Acme SSO","enabled":true,"issuer":"https://idp.internal.example","clientID":"gameplane"},
		{"name":"old","kind":"oidc","enabled":false,"issuer":"https://old.example","clientID":"x"}]}`)
	resp, body := getProviders(t, reg)
	if len(resp.Providers) != 2 {
		t.Fatalf("want local + corp (old is disabled), got %+v", resp.Providers)
	}
	var corp *loginProvider
	for i := range resp.Providers {
		if resp.Providers[i].Name == "corp" {
			corp = &resp.Providers[i]
		}
	}
	if corp == nil || corp.Label != "Acme SSO" || corp.Kind != "oidc" {
		t.Fatalf("corp provider = %+v, want Acme SSO/oidc", corp)
	}
	// Login-privacy: the pre-auth response must not leak the issuer URL,
	// the client id, or anything else from the config row.
	if strings.Contains(body, "issuer") || strings.Contains(body, "idp.internal.example") ||
		strings.Contains(body, "clientID") || strings.Contains(body, "http") {
		t.Fatalf("providers response leaks config internals: %s", body)
	}
}

func TestAuthProviders_LocalDisabledOmitted(t *testing.T) {
	reg := providersRegistry(t, `{"providers":[
		{"name":"local","kind":"local","enabled":false},
		{"name":"corp","kind":"oidc","displayName":"Acme SSO","enabled":true,"issuer":"https://idp.example","clientID":"g"}]}`)
	resp, _ := getProviders(t, reg)
	if len(resp.Providers) != 1 || resp.Providers[0].Name != "corp" {
		t.Fatalf("want only corp, got %+v", resp.Providers)
	}
}

// A provider without a displayName falls back to its name — never to
// anything derived from the issuer.
func TestAuthProviders_LabelFallsBackToName(t *testing.T) {
	reg := providersRegistry(t, `{"providers":[
		{"name":"local","kind":"local","enabled":true},
		{"name":"corp-sso","kind":"oidc","enabled":true,"issuer":"https://idp.example","clientID":"g"}]}`)
	resp, _ := getProviders(t, reg)
	for _, p := range resp.Providers {
		if p.Name == "corp-sso" && p.Label != "corp-sso" {
			t.Fatalf("label = %q, want the provider name", p.Label)
		}
	}
}
