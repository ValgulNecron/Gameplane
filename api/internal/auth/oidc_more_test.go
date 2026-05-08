package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-jose/go-jose/v4"
)

// noIDTokenIDP returns a /token response without an id_token field —
// triggers the "no id_token" branch.
func newNoIDTokenIDP(t *testing.T) *fakeIDP {
	idp := newFakeIDP(t, "client-1")
	idp.srv.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", idp.handleDiscovery)
	mux.HandleFunc("/jwks", idp.handleJWKS)
	mux.HandleFunc("/authorize", idp.handleAuthorize)
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResp{
			AccessToken: "tok", TokenType: "Bearer", ExpiresIn: 3600,
			// IDToken intentionally empty.
		})
	})
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

func TestHandleCallback_NoIDToken(t *testing.T) {
	idp := newNoIDTokenIDP(t)
	o, _ := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	o.AttachStore(newAuthDB(t))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	o.HandleCallback(NewSessionStore(newAuthDB(t)))(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("got %d body=%q", rr.Code, rr.Body.String())
	}
}

// invalidIDTokenIDP signs the id_token with a different key than the
// one published in JWKS — verifier.Verify rejects it as invalid.
func newInvalidIDTokenIDP(t *testing.T) *fakeIDP {
	idp := newFakeIDP(t, "client-1")
	wrong, _ := rsa.GenerateKey(rand.Reader, 2048)
	wrongSigner, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: wrong},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
	)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	// Override the signer used by the existing handleToken — the IDP's
	// JWKS still publishes idp.priv's pub key, so verification fails.
	idp.signer = wrongSigner
	return idp
}

func TestHandleCallback_InvalidIDToken(t *testing.T) {
	idp := newInvalidIDTokenIDP(t)
	o, _ := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	o.AttachStore(newAuthDB(t))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	o.HandleCallback(NewSessionStore(newAuthDB(t)))(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestHandleCallback_ResolveStoreError drops the users table out from
// under the resolveOrLinkUser tx so the call fails. Surfaces as 500.
func TestHandleCallback_ResolveStoreError(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.nonce = "n"
	store := newAuthDB(t)
	if _, err := store.DB.Exec(`DROP TABLE users`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	o, _ := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	o.AttachStore(store)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: "n"})
	o.HandleCallback(NewSessionStore(store))(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestHandleCallback_SessionCreateError drops sessions table so
// sessions.Create fails after the user is resolved.
func TestHandleCallback_SessionCreateError(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.nonce = "n"
	store := newAuthDB(t)
	o, _ := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	o.AttachStore(store)
	if _, err := store.DB.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: "n"})
	o.HandleCallback(NewSessionStore(store))(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
}
