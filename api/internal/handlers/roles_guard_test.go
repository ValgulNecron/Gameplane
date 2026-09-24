package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
)

// newRolesServerAs mounts the role routes behind a stub that authenticates
// every request as caller.
func newRolesServerAs(t *testing.T, caller *auth.User) (*httptest.Server, *db.Store) {
	t.Helper()
	store := newTestStore(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), caller)))
		})
	})
	MountRoles(r, store)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, store
}

// seedRole inserts a custom role holding perms.
func seedRole(t *testing.T, store *db.Store, name string, perms ...string) {
	t.Helper()
	if _, err := store.DB.ExecContext(t.Context(),
		`INSERT INTO roles(name, builtin, description) VALUES (?, 0, '')`, name); err != nil {
		t.Fatalf("seed role %s: %v", name, err)
	}
	for _, p := range perms {
		if _, err := store.DB.ExecContext(t.Context(),
			`INSERT INTO role_permissions(role_name, permission) VALUES (?, ?)`, name, p); err != nil {
			t.Fatalf("seed permission %s on %s: %v", p, name, err)
		}
	}
}

func roleHasPermission(t *testing.T, store *db.Store, role, perm string) bool {
	t.Helper()
	var n int
	if err := store.DB.QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM role_permissions WHERE role_name = ? AND permission = ?`, role, perm).Scan(&n); err != nil {
		t.Fatalf("query permission %s on %s: %v", perm, role, err)
	}
	return n > 0
}

// A role edit may not remove users:manage from the caller's own primary role.
func TestRoles_UpdateKeepsCallersOwnUserManagement(t *testing.T) {
	caller := &auth.User{Username: "um-self-user", Role: "um-self"}
	srv, store := newRolesServerAs(t, caller)
	seedRole(t, store, "um-self", "users:manage", "roles:manage")
	caller.ID = seedUser(t, store, "um-self-user", "um-self", "")
	// Another user can still manage users, so only the self-edit rule applies.
	seedUser(t, store, "um-self-keeper", "admin", "")

	status, body := doReq(t, "PATCH", srv.URL+"/roles/um-self", map[string]any{
		"permissions": []string{"servers:read", "roles:manage"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("self edit dropping users:manage: want 400 got %d body=%s", status, body)
	}
	if !roleHasPermission(t, store, "um-self", "users:manage") {
		t.Fatal("role lost users:manage although the edit was refused")
	}

	// Edits that keep users:manage still go through.
	status, body = doReq(t, "PATCH", srv.URL+"/roles/um-self", map[string]any{
		"permissions": []string{"servers:read", "users:manage", "roles:manage"},
	})
	if status != http.StatusOK {
		t.Fatalf("self edit keeping users:manage: want 200 got %d body=%s", status, body)
	}
}

// A role edit may not leave the install with no user who can manage users.
func TestRoles_UpdateKeepsAtLeastOneUserManager(t *testing.T) {
	srv, store := newRolesServerAs(t, &auth.User{ID: 9999, Username: "roles-editor", Role: "roles-editor"})
	seedRole(t, store, "um-last", "users:manage")
	seedUser(t, store, "um-last-user", "um-last", "")

	status, body := doReq(t, "PATCH", srv.URL+"/roles/um-last", map[string]any{
		"permissions": []string{"servers:read"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("edit removing the last user manager: want 400 got %d body=%s", status, body)
	}
	if !roleHasPermission(t, store, "um-last", "users:manage") {
		t.Fatal("role lost users:manage although the edit was refused")
	}
}

// Removing users:manage from a role is allowed while another user can
// still manage users.
func TestRoles_UpdateRemovesUserManagementWhenAnotherManagerRemains(t *testing.T) {
	srv, store := newRolesServerAs(t, &auth.User{ID: 9999, Username: "roles-editor", Role: "roles-editor"})
	seedRole(t, store, "um-shared", "users:manage")
	seedUser(t, store, "um-shared-user", "um-shared", "")
	seedUser(t, store, "um-shared-keeper", "admin", "")

	status, body := doReq(t, "PATCH", srv.URL+"/roles/um-shared", map[string]any{
		"permissions": []string{"servers:read"},
	})
	if status != http.StatusOK {
		t.Fatalf("edit with another user manager left: want 200 got %d body=%s", status, body)
	}
	if roleHasPermission(t, store, "um-shared", "users:manage") {
		t.Fatal("users:manage still on the role after an accepted edit")
	}
}
