package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// fakeIDP is a minimal OIDC issuer suitable for end-to-end exercising
// the HandleCallback flow. It issues RS256-signed ID tokens against a
// JWKS the relying party can verify.
type fakeIDP struct {
	t        *testing.T
	srv      *httptest.Server
	priv     *rsa.PrivateKey
	signer   jose.Signer
	clientID string
	subject  string
	email    string
	name     string
	nonce    string
	code     string
}

func newFakeIDP(t *testing.T, clientID string) *fakeIDP {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: priv},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key"),
	)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	idp := &fakeIDP{
		t:        t,
		priv:     priv,
		signer:   signer,
		clientID: clientID,
		subject:  "sub-1",
		email:    "alice@example.com",
		name:     "Alice",
		code:     "auth-code-1",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", idp.handleDiscovery)
	mux.HandleFunc("/jwks", idp.handleJWKS)
	mux.HandleFunc("/token", idp.handleToken)
	mux.HandleFunc("/authorize", idp.handleAuthorize)
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

func (i *fakeIDP) issuer() string { return i.srv.URL }

type discovery struct {
	Issuer        string   `json:"issuer"`
	AuthURL       string   `json:"authorization_endpoint"`
	TokenURL      string   `json:"token_endpoint"`
	JWKSURI       string   `json:"jwks_uri"`
	IDTokenAlg    []string `json:"id_token_signing_alg_values_supported"`
	ResponseTypes []string `json:"response_types_supported"`
	SubjectTypes  []string `json:"subject_types_supported"`
}

func (i *fakeIDP) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	d := discovery{
		Issuer:        i.srv.URL,
		AuthURL:       i.srv.URL + "/authorize",
		TokenURL:      i.srv.URL + "/token",
		JWKSURI:       i.srv.URL + "/jwks",
		IDTokenAlg:    []string{"RS256"},
		ResponseTypes: []string{"code"},
		SubjectTypes:  []string{"public"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}

func (i *fakeIDP) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &i.priv.PublicKey,
		KeyID:     "test-key",
		Algorithm: "RS256",
		Use:       "sig",
	}}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jwks)
}

func (i *fakeIDP) handleAuthorize(w http.ResponseWriter, _ *http.Request) {
	// HandleStart redirects here — we don't follow it in tests.
	w.WriteHeader(200)
}

type tokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	IDToken     string `json:"id_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func (i *fakeIDP) handleToken(w http.ResponseWriter, req *http.Request) {
	_ = req.ParseForm()
	if req.FormValue("code") != i.code {
		http.Error(w, "bad code", http.StatusBadRequest)
		return
	}
	now := time.Now()
	cl := struct {
		jwt.Claims
		Nonce string `json:"nonce,omitempty"`
		Email string `json:"email,omitempty"`
		Name  string `json:"name,omitempty"`
	}{
		Claims: jwt.Claims{
			Issuer:   i.srv.URL,
			Subject:  i.subject,
			Audience: jwt.Audience{i.clientID},
			Expiry:   jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt: jwt.NewNumericDate(now),
		},
		Nonce: i.nonce,
		Email: i.email,
		Name:  i.name,
	}
	idToken, err := jwt.Signed(i.signer).Claims(cl).Serialize()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokenResp{
		AccessToken: "tok",
		TokenType:   "Bearer",
		IDToken:     idToken,
		ExpiresIn:   3600,
	})
}

// TestHandleCallback_HappyPath exercises the full token-exchange + ID
// token verify + nonce check + user creation flow.
func TestHandleCallback_HappyPath(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.nonce = "nonce-abc"

	o, err := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app.example.com/cb")
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)
	sessions := NewSessionStore(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: "nonce-abc"})
	o.HandleCallback(sessions)(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("code=%d body=%q", rr.Code, rr.Body)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location=%q", loc)
	}
	// User row created with email-derived username.
	var u User
	err = store.DB.QueryRow(`SELECT id, username, email, role FROM users WHERE email = ?`, idp.email).
		Scan(&u.ID, &u.Username, &u.Email, &u.Role)
	if err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if u.Role != "viewer" {
		t.Fatalf("role=%q", u.Role)
	}
	// oidc_link row ties (issuer,subject) to the user.
	var linked int
	_ = store.DB.QueryRow(`SELECT COUNT(*) FROM oidc_links WHERE issuer = ? AND subject = ?`,
		idp.issuer(), idp.subject).Scan(&linked)
	if linked != 1 {
		t.Fatalf("oidc_link rows = %d", linked)
	}
}

// TestHandleCallback_NonceMismatch — IdP returns a token whose nonce
// claim doesn't match the cookie. The callback must reject with 400 and
// must not create a user.
func TestHandleCallback_NonceMismatch(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.nonce = "from-idp"

	o, err := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)
	sessions := NewSessionStore(store)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: "from-cookie"})
	o.HandleCallback(sessions)(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "nonce") {
		t.Fatalf("body=%q", rr.Body.String())
	}
}

// TestHandleCallback_ExchangeFailure — IdP returns 400 to the token
// endpoint (wrong code), which must surface as a callback 400.
func TestHandleCallback_ExchangeFailure(t *testing.T) {
	idp := newFakeIDP(t, "client-1")

	o, err := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	sessions := NewSessionStore(newAuthDB(t))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=wrong-code", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: "n"})
	o.HandleCallback(sessions)(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%q", rr.Code, rr.Body)
	}
}

// TestHandleCallback_NoNonceCookie — id_token verifies but the user has
// no nonce cookie at all. Reject with 400.
func TestHandleCallback_NoNonceCookie(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.nonce = "n"

	o, err := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	sessions := NewSessionStore(newAuthDB(t))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	o.HandleCallback(sessions)(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rr.Code)
	}
}
