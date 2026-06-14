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
	"sort"
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
	// Permissions is the caller's effective permission set, keyed by
	// namespace ("*" = cluster-wide). Populated only on /users/me; it
	// drives the dashboard's can()-based UI gating.
	Permissions map[string][]string `json:"permissions,omitempty"`
}

// permsToJSON flattens the in-memory permission set into a
// JSON-friendly namespace→permissions map.
func permsToJSON(perms map[string]map[string]struct{}) map[string][]string {
	if len(perms) == 0 {
		return nil
	}
	out := make(map[string][]string, len(perms))
	for ns, set := range perms {
		keys := make([]string, 0, len(set))
		for p := range set {
			keys = append(keys, p)
		}
		sort.Strings(keys)
		out[ns] = keys
	}
	return out
}

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
	if ok, err := h.db.RoleExists(req.Context(), body.Role); err != nil {
		httperr.Write(w, req, err)
		return
	} else if !ok {
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
	// Mirror the primary role into a cluster-wide ("*") role binding so
	// RBAC resolves the new user's permissions. Without this the user has
	// no effective permissions at all.
	if err := h.db.SetClusterRoleBinding(req.Context(), nil, id, body.Role); err != nil {
		httperr.Write(w, req, err)
		return
	}
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
		Email: u.Email, Role: u.Role, Permissions: permsToJSON(u.Perms),
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
	// Don't delete the last user who can manage users.
	if managesNow, err := h.db.UserManagesUsers(req.Context(), id); err != nil {
		httperr.Write(w, req, err)
		return
	} else if managesNow {
		count, err := h.db.UserManagerCount(req.Context())
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		if count <= 1 {
			http.Error(w, "cannot delete the last user who can manage users", http.StatusBadRequest)
			return
		}
	}
	if _, err := h.db.DB.ExecContext(req.Context(), `DELETE FROM users WHERE id = ?`, id); err != nil {
		httperr.Write(w, req, err)
		return
	}
	// sqlite runs without FK cascade, so clear the user's bindings too.
	if err := h.db.DeleteUserBindings(req.Context(), nil, id); err != nil {
		slog.Warn("delete user bindings", "err", err, "user", id)
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
	if body.Role != nil {
		if ok, err := h.db.RoleExists(req.Context(), *body.Role); err != nil {
			httperr.Write(w, req, err)
			return
		} else if !ok {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
	}
	caller := auth.UserFromContext(req.Context())
	if body.Role != nil {
		newGrantsManage, err := h.db.RoleGrantsUserManagement(req.Context(), *body.Role)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		selfChanging := caller != nil && caller.ID == id && *body.Role != caller.Role
		// Safety net: don't let a user strip their own ability to manage
		// users (the generalization of the old "don't demote yourself out
		// of admin" guard) — someone else must do it.
		if selfChanging && !newGrantsManage {
			http.Error(w, "cannot remove your own user-management access", http.StatusBadRequest)
			return
		}
		// System-wide safety net: never demote the last user who can manage
		// users, or the cluster loses all user administration.
		if !newGrantsManage {
			if managesNow, err := h.db.UserManagesUsers(req.Context(), id); err != nil {
				httperr.Write(w, req, err)
				return
			} else if managesNow {
				count, err := h.db.UserManagerCount(req.Context())
				if err != nil {
					httperr.Write(w, req, err)
					return
				}
				if count <= 1 {
					http.Error(w, "cannot demote the last user who can manage users", http.StatusBadRequest)
					return
				}
			}
		}
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
		// Keep the cluster-wide ("*") role binding in sync with the primary
		// role; per-namespace bindings are left untouched.
		if err := h.db.SetClusterRoleBinding(req.Context(), tx, id, *body.Role); err != nil {
			httperr.Write(w, req, err)
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
