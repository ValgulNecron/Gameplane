package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// newUsersServer wires MountUsers behind a test middleware that injects
// `caller` as the authenticated user. Mirrors what sessions.Authenticate
// does in production, without paying for the cookie round-trip.
func newUsersServer(t *testing.T, caller *auth.User) (*httptest.Server, *db.Store, *auth.SessionStore) {
	t.Helper()
	store := newTestStore(t)
	sessions := auth.NewSessionStore(store)
	r := chi.NewRouter()
	if caller != nil {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), caller)))
			})
		})
	}
	MountUsers(r, store, sessions)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, store, sessions
}

func seedUser(t *testing.T, store *db.Store, username, role, password string) int64 {
	t.Helper()
	auth.SetFastHashParams(t)
	var hash any
	if password != "" {
		h, err := auth.HashPassword(password)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		hash = h
	}
	res, err := store.DB.ExecContext(context.Background(),
		`INSERT INTO users(username, display_name, email, role, pw_hash) VALUES (?,?,?,?,?)`,
		username, username, username+"@example.com", role, hash)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func sessionCount(t *testing.T, store *db.Store, userID int64) int {
	t.Helper()
	var n int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatalf("session count: %v", err)
	}
	return n
}

func TestUsers_CreateValidatesPassword(t *testing.T) {
	srv, _, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	status, body := doReq(t, "POST", srv.URL+"/users", map[string]any{
		"username": "alice",
		"password": "short",
		"role":     "operator",
	})
	if status != 400 {
		t.Fatalf("want 400, got %d body=%s", status, body)
	}
}

func TestUsers_CreatePersistsAndLists(t *testing.T) {
	srv, _, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	in := map[string]any{
		"username":    "alice",
		"displayName": "Alice",
		"email":       "alice@example.com",
		"password":    "longenoughpw1",
		"role":        "operator",
	}
	status, body := doReq(t, "POST", srv.URL+"/users", in)
	if status != 200 {
		t.Fatalf("create want 200 got %d body=%s", status, body)
	}
	var created userDTO
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Provider != "local" {
		t.Fatalf("provider = %q want local", created.Provider)
	}

	status, body = doReq(t, "GET", srv.URL+"/users", nil)
	if status != 200 {
		t.Fatalf("list want 200 got %d", status)
	}
	var listed []userDTO
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 || listed[0].Username != "alice" {
		t.Fatalf("unexpected list: %+v", listed)
	}
}

// Creating a user mirrors their primary role into a cluster-wide ("*")
// role binding — without it the user would resolve to no permissions.
func TestUsers_CreateBindsClusterRole(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	status, body := doReq(t, "POST", srv.URL+"/users", map[string]any{
		"username": "ivy",
		"password": "longenoughpw1",
		"role":     "operator",
	})
	if status != 200 {
		t.Fatalf("create want 200 got %d body=%s", status, body)
	}
	var created userDTO
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var role string
	err := store.DB.QueryRow(
		`SELECT role_name FROM user_role_bindings WHERE user_id = ? AND namespace = '*'`,
		created.ID).Scan(&role)
	if err != nil || role != "operator" {
		t.Fatalf("cluster binding = %q err=%v, want operator", role, err)
	}
}

// A custom role can be assigned to a user once it exists in the roles table.
func TestUsers_CreateAcceptsCustomRole(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	if _, err := store.DB.Exec(`INSERT INTO roles(name, builtin) VALUES ('support', 0)`); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	status, _ := doReq(t, "POST", srv.URL+"/users", map[string]any{
		"username": "jo",
		"password": "longenoughpw1",
		"role":     "support",
	})
	if status != 200 {
		t.Fatalf("create with custom role want 200 got %d", status)
	}
}

// Deleting the only user who can manage users is refused.
func TestUsers_DeleteLastManagerRejected(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 999, Role: "admin"})
	id := seedUser(t, store, "onlyadmin", "admin", "longenoughpw1")
	status, body := doReq(t, "DELETE", srv.URL+"/users/"+strconv.FormatInt(id, 10), nil)
	if status != 400 {
		t.Fatalf("want 400 deleting last manager got %d body=%s", status, body)
	}
}

// Per-namespace role bindings: add, list, reject bad namespace, delete.
func TestUsers_RoleBindings(t *testing.T) {
	saved := scope.AllowedNamespaces
	t.Cleanup(func() { scope.AllowedNamespaces = saved })
	scope.AllowedNamespaces = []string{scope.DefaultNamespace, "team-a"}

	srv, store, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	id := seedUser(t, store, "ned", "viewer", "longenoughpw1")
	base := srv.URL + "/users/" + strconv.FormatInt(id, 10) + "/bindings"

	// Cluster-wide ("*") is reserved for the primary role.
	if status, _ := doReq(t, "POST", base, map[string]any{"roleName": "operator", "namespace": "*"}); status != 400 {
		t.Fatalf("'*' namespace want 400 got %d", status)
	}
	// Unknown namespace rejected.
	if status, _ := doReq(t, "POST", base, map[string]any{"roleName": "operator", "namespace": "nope"}); status != 400 {
		t.Fatalf("unknown ns want 400 got %d", status)
	}
	// Unknown role rejected.
	if status, _ := doReq(t, "POST", base, map[string]any{"roleName": "ghost", "namespace": "team-a"}); status != 400 {
		t.Fatalf("unknown role want 400 got %d", status)
	}
	// Valid grant.
	if status, body := doReq(t, "POST", base, map[string]any{"roleName": "operator", "namespace": "team-a"}); status != 201 {
		t.Fatalf("add binding want 201 got %d body=%s", status, body)
	}
	// Duplicate rejected.
	if status, _ := doReq(t, "POST", base, map[string]any{"roleName": "operator", "namespace": "team-a"}); status != 409 {
		t.Fatalf("duplicate binding want 409 got %d", status)
	}
	// List shows it.
	status, body := doReq(t, "GET", base, nil)
	if status != 200 {
		t.Fatalf("list want 200 got %d", status)
	}
	var bindings []bindingDTO
	if err := json.Unmarshal(body, &bindings); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(bindings) != 1 || bindings[0].RoleName != "operator" || bindings[0].Namespace != "team-a" {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}
	// Delete it.
	if status, _ := doReq(t, "DELETE", base+"/operator/team-a", nil); status != 204 {
		t.Fatalf("delete binding want 204 got %d", status)
	}
	if status, _ := doReq(t, "DELETE", base+"/operator/team-a", nil); status != 404 {
		t.Fatalf("delete missing binding want 404 got %d", status)
	}
}

func TestUsers_PatchEditsAllFields(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 999, Role: "admin"})
	id := seedUser(t, store, "bob", "viewer", "longenoughpw1")

	status, body := doReq(t, "PATCH", srv.URL+"/users/"+strconv.FormatInt(id, 10), map[string]any{
		"displayName": "Robert",
		"email":       "robert@example.com",
		"role":        "operator",
	})
	if status != 200 {
		t.Fatalf("patch want 200 got %d body=%s", status, body)
	}
	var got userDTO
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.DisplayName != "Robert" || got.Email != "robert@example.com" || got.Role != "operator" {
		t.Fatalf("update did not stick: %+v", got)
	}
}

func TestUsers_PatchRejectsInvalidRole(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 999, Role: "admin"})
	id := seedUser(t, store, "carol", "viewer", "longenoughpw1")
	status, _ := doReq(t, "PATCH", srv.URL+"/users/"+strconv.FormatInt(id, 10), map[string]any{
		"role": "superadmin",
	})
	if status != 400 {
		t.Fatalf("want 400 got %d", status)
	}
}

// PATCH /users/{id} where {id} is the caller's own id and the role
// drops below admin must 400 — otherwise a slip locks the cluster out
// of admin operations.
func TestUsers_PatchSelfDemoteRejected(t *testing.T) {
	store := newTestStore(t)
	id := seedUser(t, store, "evan", "admin", "longenoughpw1")
	sessions := auth.NewSessionStore(store)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: id, Role: "admin"})))
		})
	})
	MountUsers(r, store, sessions)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	status, body := doReq(t, "PATCH", srv.URL+"/users/"+strconv.FormatInt(id, 10), map[string]any{
		"role": "viewer",
	})
	if status != 400 {
		t.Fatalf("want 400 self-demote got %d body=%s", status, body)
	}
}

// Role change on someone else clears that user's active sessions so the
// new trust boundary takes effect immediately.
func TestUsers_PatchRoleInvalidatesSessions(t *testing.T) {
	store := newTestStore(t)
	target := seedUser(t, store, "trish", "operator", "longenoughpw1")
	sessions := auth.NewSessionStore(store)
	if _, _, err := sessions.Create(context.Background(), target); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if got := sessionCount(t, store, target); got != 1 {
		t.Fatalf("seed sanity: session count = %d, want 1", got)
	}

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 999, Role: "admin"})))
		})
	})
	MountUsers(r, store, sessions)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	status, _ := doReq(t, "PATCH", srv.URL+"/users/"+strconv.FormatInt(target, 10), map[string]any{
		"role": "viewer",
	})
	if status != 200 {
		t.Fatalf("want 200 got %d", status)
	}
	if got := sessionCount(t, store, target); got != 0 {
		t.Fatalf("expected sessions cleared, got %d", got)
	}
}

func TestUsers_DeleteSelfRejected(t *testing.T) {
	store := newTestStore(t)
	id := seedUser(t, store, "frank", "admin", "longenoughpw1")
	sessions := auth.NewSessionStore(store)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: id, Role: "admin"})))
		})
	})
	MountUsers(r, store, sessions)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	status, _ := doReq(t, "DELETE", srv.URL+"/users/"+strconv.FormatInt(id, 10), nil)
	if status != 400 {
		t.Fatalf("want 400 got %d", status)
	}
}

func TestUsers_ResetPasswordRejectsShort(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	id := seedUser(t, store, "gina", "viewer", "")
	status, _ := doReq(t, "POST", srv.URL+"/users/"+strconv.FormatInt(id, 10)+"/reset-password",
		map[string]any{"password": "short"})
	if status != 400 {
		t.Fatalf("want 400 got %d", status)
	}
}

// Reset writes a hash that verifies the new password and not the old,
// and clears every session for the target user.
func TestUsers_ResetPasswordHashesAndInvalidatesSessions(t *testing.T) {
	store := newTestStore(t)
	id := seedUser(t, store, "hank", "viewer", "originalpassword")
	sessions := auth.NewSessionStore(store)

	if _, _, err := sessions.Create(context.Background(), id); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if got := sessionCount(t, store, id); got != 1 {
		t.Fatalf("seed sanity: session count = %d, want 1", got)
	}

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 999, Role: "admin"})))
		})
	})
	MountUsers(r, store, sessions)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	status, _ := doReq(t, "POST", srv.URL+"/users/"+strconv.FormatInt(id, 10)+"/reset-password",
		map[string]any{"password": "brand-new-password-1"})
	if status != 204 {
		t.Fatalf("want 204 got %d", status)
	}

	var newHash string
	if err := store.DB.QueryRow(`SELECT pw_hash FROM users WHERE id=?`, id).Scan(&newHash); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if ok, _ := auth.VerifyPassword("brand-new-password-1", newHash); !ok {
		t.Fatalf("new password does not verify against stored hash")
	}
	if ok, _ := auth.VerifyPassword("originalpassword", newHash); ok {
		t.Fatalf("old password unexpectedly still verifies")
	}
	if got := sessionCount(t, store, id); got != 0 {
		t.Fatalf("expected sessions cleared, got %d", got)
	}
}
