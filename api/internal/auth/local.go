package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ValgulNecron/gameplane/api/internal/db"
)

type Local struct {
	db *db.Store
}

func NewLocal(store *db.Store) *Local { return &Local{db: store} }

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	User User   `json:"user"`
	CSRF string `json:"csrf"`
}

// HandleLogin serves POST /auth/login. reg gates the whole method: when
// the admin has disabled the local provider, every request gets the same
// neutral 403 before any credential is read — the branch is
// provider-level, not per-user, so it can't become an enumeration oracle.
func (l *Local) HandleLogin(sessions *SessionStore, reg *Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if reg != nil && !reg.LocalEnabled(req.Context()) {
			http.Error(w, "login method disabled", http.StatusForbidden)
			return
		}
		// Enforce JSON content-type — a cross-site form POST can carry
		// Cookie but not application/json (without CORS preflight), so
		// rejecting non-JSON here is cheap defense-in-depth on top of
		// the CSRF token check on the mutation path.
		ct := req.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
			return
		}
		var body loginReq
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Per-username throttle layered on top of the per-IP LoginLimiter:
		// a distributed attacker with a botnet still burns through
		// usernames at full speed when limiting is IP-only. Checked for
		// every submitted username (valid or not) so the decision doesn't
		// leak existence.
		if body.Username != "" && !LoginUserLimiter.AllowUser(body.Username) {
			w.Header().Set("Retry-After", "30")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		// Every branch below that returns 401 must first spend the argon2
		// cost so that response time can't distinguish "user not found",
		// "OIDC-only account", and "wrong password" — that differential
		// is a username-enumeration oracle.
		u, hash, err := l.fetchUser(req.Context(), body.Username)
		if err != nil || hash == "" {
			VerifyDummy(body.Password)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		ok, err := VerifyPassword(body.Password, hash)
		if err != nil || !ok {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		token, csrf, err := sessions.Create(req.Context(), u.ID)
		if err != nil {
			http.Error(w, "session error", http.StatusInternalServerError)
			return
		}
		setSessionCookie(w, token, sessionTTL)
		setCSRFCookie(w, csrf, sessionTTL)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(loginResp{User: *u, CSRF: csrf})
	}
}

func (l *Local) fetchUser(ctx context.Context, username string) (*User, string, error) {
	var (
		u    User
		hash sql.NullString
	)
	err := l.db.DB.QueryRowContext(ctx,
		`SELECT id, username, display_name, email, role, COALESCE(pw_hash, '')
		 FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", errors.New("no such user")
	}
	if err != nil {
		return nil, "", err
	}
	return &u, hash.String, nil
}
