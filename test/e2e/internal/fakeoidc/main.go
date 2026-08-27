// Command fakeoidc is a minimal RS256 OpenID Connect issuer used only by
// the e2e suite to exercise the Helm-seeded OIDC role-mapping path
// (spec 006-install-time-config: SC-003/SC-004, FR-007..FR-011). It runs
// as a long-lived in-cluster Deployment (see deploy/kind/e2e.sh) that the
// api Deployment's --oidc-issuer flag points at, so the real "helm"
// provider — built once at API startup from Helm-rendered flags — is
// genuinely live for the whole e2e cluster's lifetime.
//
// It deliberately reimplements ID-token signing with the standard
// library's crypto/rsa instead of pulling in a JOSE library: this binary
// is built with GOWORK=off against test/e2e's own go.mod (see
// ../../Dockerfile.fakeoidc), and this repo's tooling constraints don't
// allow running `go mod tidy` to add and hash a new dependency here.
//
// Identity for a login is chosen by the TEST, not by a real user: the
// /authorize request additionally accepts `sub`, `email`, `name`, and
// `groups` (comma-separated) query parameters, which are embedded
// verbatim into the ID token minted at the following /token call. A real
// IdP decides identity from whoever is logged into its own session; here
// the caller — the e2e test, dialing /authorize directly through a
// port-forward, never a browser — IS that decision.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const keyID = "fakeoidc-key"

func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

type discoveryDoc struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	IDTokenSigningAlg     []string `json:"id_token_signing_alg_values_supported"`
	ResponseTypes         []string `json:"response_types_supported"`
	SubjectTypes          []string `json:"subject_types_supported"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksDoc struct {
	Keys []jwk `json:"keys"`
}

// pendingCode is what /authorize stashed for one issued authorization
// code, read back (and consumed) by /token to mint the ID token.
type pendingCode struct {
	sub    string
	email  string
	name   string
	groups []string
	nonce  string
}

type server struct {
	issuer      string
	clientID    string
	redirectURI string
	priv        *rsa.PrivateKey

	mu      sync.Mutex
	pending map[string]pendingCode
}

func main() {
	issuer := os.Getenv("ISSUER")
	if issuer == "" {
		log.Fatal("ISSUER env var is required")
	}
	clientID := os.Getenv("CLIENT_ID")
	if clientID == "" {
		log.Fatal("CLIENT_ID env var is required")
	}
	redirectURI := os.Getenv("REDIRECT_URI")
	if redirectURI == "" {
		log.Fatal("REDIRECT_URI env var is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generate key: %v", err)
	}

	s := &server{
		issuer:      issuer,
		clientID:    clientID,
		redirectURI: redirectURI,
		priv:        priv,
		pending:     map[string]pendingCode{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", s.handleDiscovery)
	mux.HandleFunc("/jwks", s.handleJWKS)
	mux.HandleFunc("/authorize", s.handleAuthorize)
	mux.HandleFunc("/token", s.handleToken)

	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("fake OIDC issuer listening on :%s (issuer=%q)", port, issuer)
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func (s *server) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	d := discoveryDoc{
		Issuer:                s.issuer,
		AuthorizationEndpoint: s.issuer + "/authorize",
		TokenEndpoint:         s.issuer + "/token",
		JWKSURI:               s.issuer + "/jwks",
		IDTokenSigningAlg:     []string{"RS256"},
		ResponseTypes:         []string{"code"},
		SubjectTypes:          []string{"public"},
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(d); err != nil {
		log.Printf("encode discovery doc: %v", err)
	}
}

func (s *server) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	pub := s.priv.PublicKey
	set := jwksDoc{Keys: []jwk{{
		Kty: "RSA",
		Kid: keyID,
		Use: "sig",
		Alg: "RS256",
		N:   b64url(pub.N.Bytes()),
		E:   b64url(big.NewInt(int64(pub.E)).Bytes()),
	}}}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(set); err != nil {
		log.Printf("encode jwks: %v", err)
	}
}

// handleAuthorize stands in for a real IdP's login screen. See the
// package doc comment: the caller supplies the identity directly.
func (s *server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" {
		http.Error(w, "missing redirect_uri", http.StatusBadRequest)
		return
	}
	u, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	if redirectURI != s.redirectURI {
		http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
		return
	}

	sub := q.Get("sub")
	if sub == "" {
		sub = "e2e-user"
	}
	var groups []string
	if raw := q.Get("groups"); raw != "" {
		groups = strings.Split(raw, ",")
	}

	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		http.Error(w, "code generation failed", http.StatusInternalServerError)
		return
	}
	code := hex.EncodeToString(codeBytes)

	s.mu.Lock()
	s.pending[code] = pendingCode{
		sub:    sub,
		email:  q.Get("email"),
		name:   q.Get("name"),
		groups: groups,
		nonce:  q.Get("nonce"),
	}
	s.mu.Unlock()

	qs := u.Query()
	qs.Set("state", q.Get("state"))
	qs.Set("code", code)
	u.RawQuery = qs.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")

	s.mu.Lock()
	entry, ok := s.pending[code]
	if ok {
		delete(s.pending, code)
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "unknown or expired code", http.StatusBadRequest)
		return
	}

	idToken, err := s.signIDToken(entry)
	if err != nil {
		http.Error(w, "sign id_token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{
		"access_token": "fakeoidc-access-token",
		"token_type":   "Bearer",
		"id_token":     idToken,
		"expires_in":   3600,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode token response: %v", err)
	}
}

// signIDToken hand-rolls a compact RS256 JWS: base64url(header) + "." +
// base64url(payload) + "." + base64url(signature). See the package doc
// comment for why this doesn't use a JOSE library.
func (s *server) signIDToken(p pendingCode) (string, error) {
	now := time.Now()
	header := map[string]any{"alg": "RS256", "typ": "JWT", "kid": keyID}
	claims := map[string]any{
		"iss": s.issuer,
		"sub": p.sub,
		"aud": s.clientID,
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
	}
	if p.nonce != "" {
		claims["nonce"] = p.nonce
	}
	if p.email != "" {
		claims["email"] = p.email
	}
	if p.name != "" {
		claims["name"] = p.name
	}
	if p.groups != nil {
		claims["groups"] = p.groups
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	signingInput := b64url(headerJSON) + "." + b64url(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.priv, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign id_token: %w", err)
	}
	return signingInput + "." + b64url(sig), nil
}
