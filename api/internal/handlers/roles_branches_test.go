package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A JSON array body fails to decode into the handler's request struct,
// hitting the "bad request" branch through doReq.

func TestRoles_Create_BadJSON(t *testing.T) {
	srv, _ := newRolesServer(t)
	status, _ := doReq(t, "POST", srv.URL+"/roles", []int{1})
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}
}

func TestRoles_Create_InvalidName(t *testing.T) {
	srv, _ := newRolesServer(t)
	status, _ := doReq(t, "POST", srv.URL+"/roles", map[string]any{
		"name":        "not a valid name!",
		"permissions": []string{"servers:read"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}
}

func TestRoles_Update_BadJSON(t *testing.T) {
	srv, _ := newRolesServer(t)
	// operator is a built-in but editable role, so we get past the
	// not-found / admin-immutable guards into the JSON decode.
	status, _ := doReq(t, "PATCH", srv.URL+"/roles/operator", []int{1})
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", status)
	}
}

// Creating with duplicate permission keys exercises dedupSorted's
// already-seen branch; the stored set must be de-duplicated.
func TestRoles_Create_DeduplicatesPermissions(t *testing.T) {
	srv, store := newRolesServer(t)
	status, body := doReq(t, "POST", srv.URL+"/roles", map[string]any{
		"name":        "dup",
		"permissions": []string{"servers:read", "servers:read", "backups:read"},
	})
	if status != http.StatusCreated {
		t.Fatalf("status=%d body=%s want 201", status, body)
	}
	var created roleDTO
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(created.Permissions) != 2 {
		t.Fatalf("permissions = %v, want 2 unique", created.Permissions)
	}
	var n int
	if err := store.DB.QueryRow(
		`SELECT COUNT(*) FROM role_permissions WHERE role_name='dup'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("stored %d permission rows, want 2 (deduped)", n)
	}
}
