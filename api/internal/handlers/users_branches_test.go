package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
	"github.com/kestrel-gg/kestrel/api/internal/scope"
)

// These tests sweep the validation / error branches of the user handlers
// that the happy-path tests in users_test.go don't reach. A JSON array
// body ([]int) decodes-fails against the handler's request struct, which
// is the cleanest way to hit the "bad request" branch through doReq.

func TestUsers_Create_ValidationBranches(t *testing.T) {
	srv, _, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	cases := []struct {
		name string
		body any
		want int
	}{
		{"bad json", []int{1}, http.StatusBadRequest},
		{"invalid username", map[string]any{"username": "bad name!", "password": "longenoughpw1"}, http.StatusBadRequest},
		{"invalid email", map[string]any{"username": "okuser", "email": "not-an-email", "password": "longenoughpw1"}, http.StatusBadRequest},
		{"invalid role", map[string]any{"username": "okuser2", "role": "superhero", "password": "longenoughpw1"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := doReq(t, "POST", srv.URL+"/users", tc.body)
			if status != tc.want {
				t.Fatalf("status=%d want %d body=%s", status, tc.want, body)
			}
		})
	}
}

func TestUsers_Create_NoPasswordDefaultsToPendingViewer(t *testing.T) {
	srv, _, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	status, body := doReq(t, "POST", srv.URL+"/users", map[string]any{"username": "pendinguser"})
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var u userDTO
	if err := json.Unmarshal(body, &u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.Provider != "pending" {
		t.Fatalf("provider=%q want pending", u.Provider)
	}
	if u.Role != "viewer" {
		t.Fatalf("role=%q want viewer (default)", u.Role)
	}
}

func TestUsers_Me(t *testing.T) {
	t.Run("unauthenticated returns 401", func(t *testing.T) {
		srv, _, _ := newUsersServer(t, nil) // no caller middleware
		status, _ := doReq(t, "GET", srv.URL+"/users/me", nil)
		if status != http.StatusUnauthorized {
			t.Fatalf("status=%d want 401", status)
		}
	})
	t.Run("returns the caller with its permission set", func(t *testing.T) {
		caller := &auth.User{
			ID: 7, Username: "self", Role: "operator",
			Perms: map[string]map[string]struct{}{"*": {"servers:read": {}, "servers:write": {}}},
		}
		srv, _, _ := newUsersServer(t, caller)
		status, body := doReq(t, "GET", srv.URL+"/users/me", nil)
		if status != http.StatusOK {
			t.Fatalf("status=%d body=%s", status, body)
		}
		var u userDTO
		if err := json.Unmarshal(body, &u); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if u.Username != "self" || len(u.Permissions["*"]) != 2 {
			t.Fatalf("unexpected me payload: %+v", u)
		}
	})
}

func TestUsers_Delete_SuccessCleansBindings(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 999, Role: "admin"})
	// Two managers so deleting one doesn't trip the last-manager guard.
	_ = seedUser(t, store, "keepadmin", "admin", "longenoughpw1")
	victim := seedUser(t, store, "goner", "admin", "longenoughpw1")
	if _, err := store.DB.Exec(
		`INSERT INTO user_role_bindings(user_id, role_name, namespace) VALUES (?, 'viewer', 'team-a')`, victim); err != nil {
		t.Fatalf("seed binding: %v", err)
	}

	status, body := doReq(t, "DELETE", srv.URL+"/users/"+strconv.FormatInt(victim, 10), nil)
	if status != http.StatusNoContent {
		t.Fatalf("status=%d body=%s want 204", status, body)
	}
	var n int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM user_role_bindings WHERE user_id=?`, victim).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("bindings not cleaned up: %d", n)
	}
}

func TestUsers_Delete_BadID(t *testing.T) {
	srv, _, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	status, _ := doReq(t, "DELETE", srv.URL+"/users/not-a-number", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}
}

func TestUsers_Update_Branches(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 999, Role: "admin"})
	id := seedUser(t, store, "ulla", "viewer", "longenoughpw1")
	sid := strconv.FormatInt(id, 10)

	t.Run("bad id", func(t *testing.T) {
		status, _ := doReq(t, "PATCH", srv.URL+"/users/xyz", map[string]any{"displayName": "x"})
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		status, _ := doReq(t, "PATCH", srv.URL+"/users/"+sid, []int{1})
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("invalid email", func(t *testing.T) {
		status, _ := doReq(t, "PATCH", srv.URL+"/users/"+sid, map[string]any{"email": "nope"})
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("displayName update on a missing user is 404 (fetchByID path)", func(t *testing.T) {
		status, _ := doReq(t, "PATCH", srv.URL+"/users/424242", map[string]any{"displayName": "ghost"})
		if status != http.StatusNotFound {
			t.Fatalf("status=%d want 404", status)
		}
	})
	t.Run("role update on a missing user surfaces an error (RowsAffected path)", func(t *testing.T) {
		status, _ := doReq(t, "PATCH", srv.URL+"/users/535353", map[string]any{"role": "operator"})
		if status != http.StatusInternalServerError {
			t.Fatalf("status=%d want 500", status)
		}
	})
}

// Demoting the only user who can manage users is refused even when the
// caller is someone else (the system-wide last-manager guard).
func TestUsers_Update_DemoteLastManagerRejected(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 999, Role: "admin"})
	id := seedUser(t, store, "soleadmin", "admin", "longenoughpw1")
	status, body := doReq(t, "PATCH", srv.URL+"/users/"+strconv.FormatInt(id, 10), map[string]any{"role": "viewer"})
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s want 400", status, body)
	}
}

func TestUsers_ResetPassword_Branches(t *testing.T) {
	srv, store, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	id := seedUser(t, store, "rosa", "viewer", "")
	sid := strconv.FormatInt(id, 10)

	t.Run("bad id", func(t *testing.T) {
		status, _ := doReq(t, "POST", srv.URL+"/users/zzz/reset-password", map[string]any{"password": "longenoughpw1"})
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		status, _ := doReq(t, "POST", srv.URL+"/users/"+sid+"/reset-password", []int{1})
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("missing user is 404", func(t *testing.T) {
		status, _ := doReq(t, "POST", srv.URL+"/users/787878/reset-password", map[string]any{"password": "longenoughpw1"})
		if status != http.StatusNotFound {
			t.Fatalf("status=%d want 404", status)
		}
	})
}

func TestUsers_Bindings_Branches(t *testing.T) {
	saved := scope.AllowedNamespaces
	t.Cleanup(func() { scope.AllowedNamespaces = saved })
	scope.AllowedNamespaces = []string{scope.DefaultNamespace, "team-a"}

	srv, store, _ := newUsersServer(t, &auth.User{ID: 1, Role: "admin"})
	id := seedUser(t, store, "bindme", "viewer", "longenoughpw1")
	sid := strconv.FormatInt(id, 10)

	t.Run("list bad id", func(t *testing.T) {
		status, _ := doReq(t, "GET", srv.URL+"/users/nan/bindings", nil)
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("add bad id", func(t *testing.T) {
		status, _ := doReq(t, "POST", srv.URL+"/users/nan/bindings", map[string]any{"roleName": "operator", "namespace": "team-a"})
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("add bad json", func(t *testing.T) {
		status, _ := doReq(t, "POST", srv.URL+"/users/"+sid+"/bindings", []int{1})
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("add to a missing user is 404", func(t *testing.T) {
		status, _ := doReq(t, "POST", srv.URL+"/users/9988/bindings", map[string]any{"roleName": "operator", "namespace": "team-a"})
		if status != http.StatusNotFound {
			t.Fatalf("status=%d want 404", status)
		}
	})
	t.Run("delete bad id", func(t *testing.T) {
		status, _ := doReq(t, "DELETE", srv.URL+"/users/nan/bindings/operator/team-a", nil)
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
	t.Run("delete cluster-wide namespace is rejected", func(t *testing.T) {
		status, _ := doReq(t, "DELETE", srv.URL+"/users/"+sid+"/bindings/operator/*", nil)
		if status != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", status)
		}
	})
}
