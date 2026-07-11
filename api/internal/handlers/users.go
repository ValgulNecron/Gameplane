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

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

func MountUsers(r chi.Router, store *db.Store, sessions *auth.SessionStore, clusters scope.ClusterLister) {
	h := &userHandler{db: store, sessions: sessions, clusters: clusters}
	r.Route("/users", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Get("/me", h.me)
		r.Delete("/{id}", h.del)
		r.Patch("/{id}", h.update)
		r.Post("/{id}/reset-password", h.resetPassword)
		// Per-namespace role grants (the cluster-wide role is the primary
		// role, set via PATCH above).
		r.Get("/{id}/bindings", h.listBindings)
		r.Post("/{id}/bindings", h.addBinding)
		r.Delete("/{id}/bindings/{role}/{namespace}", h.deleteBinding)
	})
}

type userHandler struct {
	db       *db.Store
	sessions *auth.SessionStore
	clusters scope.ClusterLister
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
	if err := h.db.SetClusterRoleBinding(req.Context(), nil, id, scope.DefaultCluster, body.Role); err != nil {
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
		Email: u.Email, Role: u.Role, Permissions: auth.PermsToJSON(u.Perms),
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
		if err := h.db.SetClusterRoleBinding(req.Context(), tx, id, scope.DefaultCluster, *body.Role); err != nil {
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

type bindingDTO struct {
	RoleName  string `json:"roleName"`
	Namespace string `json:"namespace"`
	Cluster   string `json:"cluster"`
}

func (h *userHandler) listBindings(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	rows, err := h.db.DB.QueryContext(req.Context(),
		`SELECT role_name, namespace, cluster FROM user_role_bindings WHERE user_id = ? ORDER BY cluster, namespace, role_name`, id)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	defer rows.Close()
	out := []bindingDTO{}
	for rows.Next() {
		var b bindingDTO
		if err := rows.Scan(&b.RoleName, &b.Namespace, &b.Cluster); err != nil {
			httperr.Write(w, req, err)
			return
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, out)
}

type addBindingReq struct {
	RoleName  string `json:"roleName"`
	Namespace string `json:"namespace"`
	Cluster   string `json:"cluster"`
}

func (h *userHandler) addBinding(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body addBindingReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Default the cluster to the default cluster.
	if body.Cluster == "" {
		body.Cluster = scope.DefaultCluster
	}
	// The cluster-wide role is the user's primary role (set via PATCH); this
	// endpoint only grants additional per-namespace roles.
	if body.Namespace == "*" || !scope.Allowed(body.Namespace) {
		http.Error(w, "namespace not permitted", http.StatusBadRequest)
		return
	}
	// Validate the cluster.
	if body.Cluster == "*" {
		http.Error(w, "cluster not permitted", http.StatusBadRequest)
		return
	}
	known := false
	for _, id := range h.clusters.IDs() {
		if id == body.Cluster {
			known = true
			break
		}
	}
	if !known {
		http.Error(w, "cluster not permitted", http.StatusBadRequest)
		return
	}
	if ok, err := h.db.RoleExists(req.Context(), body.RoleName); err != nil {
		httperr.Write(w, req, err)
		return
	} else if !ok {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	if ok, err := h.userExists(req.Context(), id); err != nil {
		httperr.Write(w, req, err)
		return
	} else if !ok {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	var dup int
	if err := h.db.DB.QueryRowContext(req.Context(),
		`SELECT COUNT(*) FROM user_role_bindings WHERE user_id = ? AND role_name = ? AND cluster = ? AND namespace = ?`,
		id, body.RoleName, body.Cluster, body.Namespace).Scan(&dup); err != nil {
		httperr.Write(w, req, err)
		return
	}
	if dup > 0 {
		http.Error(w, "binding already exists", http.StatusConflict)
		return
	}
	if _, err := h.db.DB.ExecContext(req.Context(),
		`INSERT INTO user_role_bindings(user_id, role_name, cluster, namespace) VALUES (?, ?, ?, ?)`,
		id, body.RoleName, body.Cluster, body.Namespace); err != nil {
		httperr.Write(w, req, err)
		return
	}
	h.invalidateSessions(req, id, "role binding added")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, bindingDTO{RoleName: body.RoleName, Namespace: body.Namespace, Cluster: body.Cluster})
}

func (h *userHandler) deleteBinding(w http.ResponseWriter, req *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	role := chi.URLParam(req, "role")
	namespace := chi.URLParam(req, "namespace")
	cluster := req.URL.Query().Get("cluster")
	if cluster == "" {
		cluster = scope.DefaultCluster
	}
	if namespace == "*" {
		http.Error(w, "the cluster-wide role is managed via the primary role", http.StatusBadRequest)
		return
	}
	if cluster == "*" {
		http.Error(w, "cluster not permitted", http.StatusBadRequest)
		return
	}
	res, err := h.db.DB.ExecContext(req.Context(),
		`DELETE FROM user_role_bindings WHERE user_id = ? AND role_name = ? AND cluster = ? AND namespace = ?`,
		id, role, cluster, namespace)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		http.Error(w, "binding not found", http.StatusNotFound)
		return
	}
	h.invalidateSessions(req, id, "role binding removed")
	w.WriteHeader(http.StatusNoContent)
}

func (h *userHandler) userExists(ctx context.Context, id int64) (bool, error) {
	var x int
	err := h.db.DB.QueryRowContext(ctx, `SELECT 1 FROM users WHERE id = ?`, id).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// invalidateSessions clears the target user's sessions so a binding change
// takes effect immediately. Permissions are resolved per request, so this
// is belt-and-suspenders — it forces a clean re-auth under the new grants.
func (h *userHandler) invalidateSessions(req *http.Request, id int64, reason string) {
	if h.sessions == nil {
		return
	}
	if err := h.sessions.DeleteForUser(req.Context(), id); err != nil {
		slog.Warn("invalidate sessions", "err", err, "user", id, "reason", reason)
	}
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
