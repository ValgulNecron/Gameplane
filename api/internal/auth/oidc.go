package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/ValgulNecron/gameplane/api/internal/db"
)

const (
	oidcStateCookie = "gameplane_oidc_state"
	oidcNonceCookie = "gameplane_oidc_nonce"
)

type OIDC struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
	db       *db.Store
}

// NewOIDC returns (nil, nil) when no issuer is configured — the caller
// treats that as "OIDC disabled" rather than an error.
func NewOIDC(ctx context.Context, issuer, clientID, clientSecret, redirectURL string) (*OIDC, error) {
	if issuer == "" {
		return nil, nil
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &OIDC{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

func (o *OIDC) AttachStore(s *db.Store) { o.db = s }

func (o *OIDC) HandleStart() http.HandlerFunc {
	return o.HandleStartAt("/")
}

// HandleStartAt is HandleStart with an explicit cookie path. Dynamic
// providers scope their state/nonce cookies to /auth/oidc/{name} so two
// concurrent flows against different providers can't clobber each
// other's cookies; the legacy routes keep Path=/.
func (o *OIDC) HandleStartAt(cookiePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		state := randomToken()
		nonce := randomToken()
		ttl := 5 * time.Minute
		http.SetCookie(w, &http.Cookie{
			Name: oidcStateCookie, Value: state, Path: cookiePath, HttpOnly: true, Secure: true,
			SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(ttl),
		})
		// Nonce is bound to the ID token via OpenID Connect spec — the
		// IdP echoes it back in the `nonce` claim. Verifying the claim
		// matches the cookie prevents ID-token replay, complementing
		// the CSRF-style state check.
		http.SetCookie(w, &http.Cookie{
			Name: oidcNonceCookie, Value: nonce, Path: cookiePath, HttpOnly: true, Secure: true,
			SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(ttl),
		})
		http.Redirect(w, req, o.oauth.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
	}
}

func (o *OIDC) HandleCallback(sessions *SessionStore) http.HandlerFunc {
	return o.HandleCallbackAt(sessions, "/")
}

// HandleCallbackAt is HandleCallback with an explicit cookie path
// matching HandleStartAt's — clearing a cookie only works on the path it
// was set with.
func (o *OIDC) HandleCallbackAt(sessions *SessionStore, cookiePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		state, err := req.Cookie(oidcStateCookie)
		if err != nil || state.Value == "" || state.Value != req.URL.Query().Get("state") {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		clearCookieAt(w, oidcStateCookie, true, cookiePath)

		tok, err := o.oauth.Exchange(req.Context(), req.URL.Query().Get("code"))
		if err != nil {
			slog.Warn("oidc exchange", "err", err)
			http.Error(w, "oauth exchange failed", http.StatusBadRequest)
			return
		}
		rawID, _ := tok.Extra("id_token").(string)
		if rawID == "" {
			http.Error(w, "no id_token", http.StatusBadRequest)
			return
		}
		idt, err := o.verifier.Verify(req.Context(), rawID)
		if err != nil {
			http.Error(w, "invalid id_token", http.StatusUnauthorized)
			return
		}
		// Nonce check — the IdP is expected to echo the nonce we issued
		// at the start route into the ID token. Missing cookie or
		// mismatch means replay or a broken IdP; either way, don't
		// accept the login.
		nonceCookie, err := req.Cookie(oidcNonceCookie)
		if err != nil || nonceCookie.Value == "" || idt.Nonce != nonceCookie.Value {
			clearCookieAt(w, oidcNonceCookie, true, cookiePath)
			http.Error(w, "nonce mismatch", http.StatusBadRequest)
			return
		}
		clearCookieAt(w, oidcNonceCookie, true, cookiePath)
		var claims struct {
			Sub   string `json:"sub"`
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := idt.Claims(&claims); err != nil {
			slog.Warn("oidc claim parse", "err", err)
			http.Error(w, "invalid id_token claims", http.StatusBadRequest)
			return
		}

		user, err := o.resolveOrLinkUser(req.Context(), idt.Issuer, claims.Sub, claims.Email, claims.Name)
		if err != nil {
			slog.Error("oidc resolveOrLinkUser", "err", err)
			http.Error(w, "login failed", http.StatusInternalServerError)
			return
		}
		sess, csrf, err := sessions.Create(req.Context(), user.ID)
		if err != nil {
			slog.Error("oidc session create", "err", err)
			http.Error(w, "login failed", http.StatusInternalServerError)
			return
		}
		setSessionCookie(w, sess, sessionTTL)
		setCSRFCookie(w, csrf, sessionTTL)

		// Redirect back to the SPA root; the SPA reads the CSRF cookie
		// and starts making authenticated requests.
		http.Redirect(w, req, "/", http.StatusFound)
	}
}

func (o *OIDC) resolveOrLinkUser(
	ctx context.Context, issuer, sub, email, name string,
) (*User, error) {
	if o.db == nil {
		return nil, errors.New("oidc: no store attached")
	}
	var u User
	err := o.db.DB.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.display_name, u.email, u.role
		FROM users u JOIN oidc_links l ON l.user_id = u.id
		WHERE l.issuer = ? AND l.subject = ?`, issuer, sub,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role)
	if err == nil {
		return &u, nil
	}
	// First login — create user + link in a single tx.
	baseUsername := email
	if baseUsername == "" {
		baseUsername = sub
	}
	tx, err := o.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Find a username that doesn't collide with an existing local user.
	// Suffix with a short piece of the OIDC subject on conflict — keeps
	// the username recognizable while guaranteeing uniqueness.
	username, err := pickUniqueUsername(ctx, tx, baseUsername, sub)
	if err != nil {
		return nil, err
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users(username, email, display_name, role) VALUES (?, ?, ?, 'viewer')`,
		username, email, name,
	)
	if err != nil {
		return nil, err
	}
	uid, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO oidc_links(user_id, issuer, subject, email) VALUES (?, ?, ?, ?)`,
		uid, issuer, sub, email,
	); err != nil {
		return nil, err
	}
	// Mirror the default role into a cluster-wide role binding so RBAC
	// resolves the new SSO user's permissions.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_role_bindings(user_id, role_name, namespace) VALUES (?, 'viewer', '*')`,
		uid,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &User{
		ID: uid, Username: username, DisplayName: name, Email: email, Role: "viewer",
	}, nil
}

// pickUniqueUsername returns base if no existing user has that username,
// otherwise returns base with a short suffix derived from sub. Uses the
// transaction so the check and the caller's INSERT see a consistent view.
func pickUniqueUsername(ctx context.Context, tx *sql.Tx, base, sub string) (string, error) {
	var existing int64
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = ?`, base).Scan(&existing)
	if err != nil {
		return "", err
	}
	if existing == 0 {
		return base, nil
	}
	// Use the first 8 chars of sub as the tiebreaker. OIDC subs are
	// opaque but usually long; 8 chars is enough to disambiguate in
	// practice without exposing the whole subject identifier.
	suffix := sub
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return base + "+" + suffix, nil
}
