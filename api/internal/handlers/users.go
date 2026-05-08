package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
	"github.com/kestrel-gg/kestrel/api/internal/db"
	"github.com/kestrel-gg/kestrel/api/internal/httperr"
)

func MountUsers(r chi.Router, store *db.Store, sessions *auth.SessionStore) {
	h := &userHandler{db: store, sessions: sessions}
	r.Route("/users", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/me", h.me)
		r.Delete("/{id}", h.del)
		r.Patch("/{id}", h.update)
		r.Post("/{id}/reset-password", h.resetPassword)
	})
}

type userHandler struct {
	db       *db.Store
	sessions *auth.SessionStore
}

type userDTO struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Provider    string `json:"provider"`
	CreatedAt   string `json:"createdAt"`
}

// validRoles is the closed set the RBAC middleware understands. Any
// other string silently evaluates to "less than viewer" and breaks.
var validRoles = map[string]bool{"admin": true, "operator": true, "viewer": true}

// usernameRE constrains usernames to a conservative DNS-label-ish set.
// This also stops homoglyph tricks and keeps URLs clean since usernames
// can show up in audit logs and (eventually) API paths.
var usernameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)


func (h *userHandler) list(w http.ResponseWriter, req *http.Request) {
	rows, err := h.db.DB.QueryContext(req.Context(), `
		SELECT u.id, u.username, u.display_name, u.email, u.role, u.created_at,
		       CASE
		         WHEN EXISTS (SELECT 1 FROM oidc_links l WHERE l.user_id = u.id) THEN 'oidc'
		         WHEN u.pw_hash IS NOT NULL AND u.pw_hash <> ''               THEN 'local'
		         ELSE 'pending'
		       END AS provider
		FROM users u
		ORDER BY u.id`)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	defer rows.Close()
	out := []userDTO{}
	for rows.Next() {
		var u userDTO
		if err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role, &u.CreatedAt, &u.Provider); err != nil {
			httperr.Write(w, req, err)
			return
		}
		out = append(out, u)
	}
	writeJSON(w, out)
}

type createUserReq struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

func (h *userHandler) create(w http.ResponseWriter, req *http.Request) {
	var body createUserReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !usernameRE.MatchString(body.Username) {
		http.Error(w, "invalid username", http.StatusBadRequest)
		return
	}
	if body.Email != "" {
		if _, err := mail.ParseAddress(body.Email); err != nil {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}
	}
	if body.Role == "" {
		body.Role = "viewer"
	}
	if !validRoles[body.Role] {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	var hash string
	if body.Password != "" {
		if len(body.Password) < auth.MinPasswordLen {
			http.Error(w, "password too short", http.StatusBadRequest)
			return
		}
		h2, err := auth.HashPassword(body.Password)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		hash = h2
	}
	res, err := h.db.DB.ExecContext(req.Context(),
		`INSERT INTO users(username, display_name, email, role, pw_hash) VALUES (?, ?, ?, ?, ?)`,
		body.Username, body.DisplayName, body.Email, body.Role, nullable(hash),
	)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	id, _ := res.LastInsertId()
	provider := "pending"
	if hash != "" {
		provider = "local"
	}
	writeJSON(w, userDTO{
		ID: id, Username: body.Username, DisplayName: body.DisplayName,
		Email: body.Email, Role: body.Role, Provider: provider,
	})
}

func (h *userHandler) me(w http.ResponseWriter, req *http.Request) {
	u := auth.UserFromContext(req.Context())
	if u == nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	writeJSON(w, userDTO{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName,
		Email: u.Email, Role: u.Role,
	})
}

func (h *userHandler) del(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	u := auth.UserFromContext(req.Context())
	if u != nil && u.ID == id {
		http.Error(w, "cannot delete self", http.StatusBadRequest)
		return
	}
	if _, err := h.db.DB.ExecContext(req.Context(), `DELETE FROM users WHERE id = ?`, id); err != nil {
		httperr.Write(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateUserReq carries optional updates. nil means "leave unchanged"
// — pointer-vs-empty-string lets a caller clear an email by sending
// "" without ambiguity.
type updateUserReq struct {
	DisplayName *string `json:"displayName,omitempty"`
	Email       *string `json:"email,omitempty"`
	Role        *string `json:"role,omitempty"`
}

func (h *userHandler) update(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body updateUserReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Email != nil && *body.Email != "" {
		if _, err := mail.ParseAddress(*body.Email); err != nil {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}
	}
	if body.Role != nil && !validRoles[*body.Role] {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	caller := auth.UserFromContext(req.Context())
	roleChanging := body.Role != nil && caller != nil && caller.ID == id && *body.Role != caller.Role
	// Safety net: don't let an admin accidentally demote themselves and
	// lock the cluster out of admin operations.
	if roleChanging && *body.Role != "admin" {
		http.Error(w, "cannot demote self", http.StatusBadRequest)
		return
	}

	tx, err := h.db.DB.BeginTx(req.Context(), nil)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	if body.DisplayName != nil {
		if _, err := tx.ExecContext(req.Context(),
			`UPDATE users SET display_name = ?, updated_at = datetime('now') WHERE id = ?`,
			*body.DisplayName, id); err != nil {
			httperr.Write(w, req, err)
			return
		}
	}
	if body.Email != nil {
		if _, err := tx.ExecContext(req.Context(),
			`UPDATE users SET email = ?, updated_at = datetime('now') WHERE id = ?`,
			*body.Email, id); err != nil {
			httperr.Write(w, req, err)
			return
		}
	}
	if body.Role != nil {
		res, err := tx.ExecContext(req.Context(),
			`UPDATE users SET role = ?, updated_at = datetime('now') WHERE id = ?`,
			*body.Role, id)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			httperr.Write(w, req, sql.ErrNoRows)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Invalidate active sessions for the target user when their role
	// changes, so the new trust boundary takes effect immediately.
	// Without this a demoted admin keeps acting as admin until session
	// TTL expires.
	if body.Role != nil && h.sessions != nil {
		if err := h.sessions.DeleteForUser(req.Context(), id); err != nil {
			slog.Warn("invalidate sessions on role change", "err", err, "user", id)
		}
	}

	out, err := h.fetchByID(req.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, out)
}

type resetPasswordReq struct {
	Password string `json:"password"`
}

func (h *userHandler) resetPassword(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body resetPasswordReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body.Password) < auth.MinPasswordLen {
		http.Error(w, "password too short", http.StatusBadRequest)
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	res, err := h.db.DB.ExecContext(req.Context(),
		`UPDATE users SET pw_hash = ?, updated_at = datetime('now') WHERE id = ?`,
		hash, id)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	// Force the target user to re-authenticate under the new credential.
	// A leaked old password and an active session are otherwise equivalent
	// from the attacker's perspective; we want reset to actually evict.
	if h.sessions != nil {
		if err := h.sessions.DeleteForUser(req.Context(), id); err != nil {
			slog.Warn("invalidate sessions on password reset", "err", err, "user", id)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *userHandler) fetchByID(ctx context.Context, id int64) (userDTO, error) {
	var u userDTO
	err := h.db.DB.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.display_name, u.email, u.role, u.created_at,
		       CASE
		         WHEN EXISTS (SELECT 1 FROM oidc_links l WHERE l.user_id = u.id) THEN 'oidc'
		         WHEN u.pw_hash IS NOT NULL AND u.pw_hash <> ''               THEN 'local'
		         ELSE 'pending'
		       END
		FROM users u WHERE u.id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Role, &u.CreatedAt, &u.Provider)
	return u, err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
