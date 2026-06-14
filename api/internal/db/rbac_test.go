package db

import (
	"context"
	"testing"
)

func newRBACStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func insertUser(t *testing.T, s *Store, username, role string) int64 {
	t.Helper()
	res, err := s.DB.Exec(`INSERT INTO users(username, role) VALUES (?, ?)`, username, role)
	if err != nil {
		t.Fatalf("insert user %q: %v", username, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

func assertBinding(t *testing.T, s *Store, uid int64, ns, wantRole string) {
	t.Helper()
	var got string
	err := s.DB.QueryRow(
		`SELECT role_name FROM user_role_bindings WHERE user_id = ? AND namespace = ?`, uid, ns).Scan(&got)
	if err != nil {
		t.Fatalf("binding for ns=%q missing: %v", ns, err)
	}
	if got != wantRole {
		t.Fatalf("ns=%q role=%q, want %q", ns, got, wantRole)
	}
}

func TestSetClusterRoleBinding(t *testing.T) {
	s := newRBACStore(t)
	ctx := context.Background()
	uid := insertUser(t, s, "alice", "viewer")

	// A pre-existing per-namespace binding must survive cluster repointing.
	if _, err := s.DB.Exec(
		`INSERT INTO user_role_bindings(user_id, role_name, namespace) VALUES (?, 'operator', 'team-a')`, uid); err != nil {
		t.Fatalf("seed ns binding: %v", err)
	}

	if err := s.SetClusterRoleBinding(ctx, nil, uid, "operator"); err != nil {
		t.Fatalf("set: %v", err)
	}
	assertBinding(t, s, uid, "*", "operator")

	// Repointing replaces only the cluster ("*") binding.
	if err := s.SetClusterRoleBinding(ctx, nil, uid, "admin"); err != nil {
		t.Fatalf("repoint: %v", err)
	}
	assertBinding(t, s, uid, "*", "admin")

	var nStar int
	if err := s.DB.QueryRow(
		`SELECT COUNT(*) FROM user_role_bindings WHERE user_id = ? AND namespace = '*'`, uid).Scan(&nStar); err != nil {
		t.Fatalf("count: %v", err)
	}
	if nStar != 1 {
		t.Fatalf("want exactly 1 cluster binding, got %d", nStar)
	}
	// The team-a binding is untouched.
	assertBinding(t, s, uid, "team-a", "operator")
}

func TestSetClusterRoleBinding_InTx(t *testing.T) {
	s := newRBACStore(t)
	ctx := context.Background()
	uid := insertUser(t, s, "bob", "viewer")

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := s.SetClusterRoleBinding(ctx, tx, uid, "admin"); err != nil {
		t.Fatalf("set in tx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	assertBinding(t, s, uid, "*", "admin")
}

func TestDeleteUserBindings(t *testing.T) {
	s := newRBACStore(t)
	ctx := context.Background()
	uid := insertUser(t, s, "carol", "operator")
	for _, b := range []struct{ role, ns string }{{"operator", "*"}, {"viewer", "team-a"}} {
		if _, err := s.DB.Exec(
			`INSERT INTO user_role_bindings(user_id, role_name, namespace) VALUES (?, ?, ?)`,
			uid, b.role, b.ns); err != nil {
			t.Fatalf("seed binding: %v", err)
		}
	}
	if err := s.DeleteUserBindings(ctx, nil, uid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	if err := s.DB.QueryRow(
		`SELECT COUNT(*) FROM user_role_bindings WHERE user_id = ?`, uid).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 bindings after delete, got %d", n)
	}
}

func TestRoleExists(t *testing.T) {
	s := newRBACStore(t)
	ctx := context.Background()
	if ok, err := s.RoleExists(ctx, "admin"); err != nil || !ok {
		t.Fatalf("admin exists = %v, err = %v", ok, err)
	}
	if ok, err := s.RoleExists(ctx, "ghost"); err != nil || ok {
		t.Fatalf("ghost exists = %v, err = %v", ok, err)
	}
}

func TestRoleGrantsUserManagement(t *testing.T) {
	s := newRBACStore(t)
	ctx := context.Background()
	for _, tc := range []struct {
		role string
		want bool
	}{
		{"admin", true},     // holds the "*" wildcard
		{"operator", false}, // no users:manage
		{"viewer", false},
	} {
		got, err := s.RoleGrantsUserManagement(ctx, tc.role)
		if err != nil {
			t.Fatalf("%s: %v", tc.role, err)
		}
		if got != tc.want {
			t.Errorf("%s grants user management = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestUserManagesUsersAndManagerCount(t *testing.T) {
	s := newRBACStore(t)
	ctx := context.Background()
	adminID := insertUser(t, s, "root", "admin")
	viewerID := insertUser(t, s, "view", "viewer")

	if ok, err := s.UserManagesUsers(ctx, adminID); err != nil || !ok {
		t.Fatalf("admin manages users = %v, err = %v", ok, err)
	}
	if ok, err := s.UserManagesUsers(ctx, viewerID); err != nil || ok {
		t.Fatalf("viewer manages users = %v, err = %v", ok, err)
	}
	n, err := s.UserManagerCount(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("user-manager count = %d, want 1 (admin only)", n)
	}
}

func TestRBACReads_ClosedDB(t *testing.T) {
	s := newRBACStore(t)
	ctx := context.Background()
	_ = s.Close() // every query/exec below now fails

	if err := s.SetClusterRoleBinding(ctx, nil, 1, "admin"); err == nil {
		t.Error("SetClusterRoleBinding: want error on closed DB")
	}
	if err := s.DeleteUserBindings(ctx, nil, 1); err == nil {
		t.Error("DeleteUserBindings: want error on closed DB")
	}
	if _, err := s.RoleExists(ctx, "admin"); err == nil {
		t.Error("RoleExists: want error on closed DB")
	}
	if _, err := s.RoleGrantsUserManagement(ctx, "admin"); err == nil {
		t.Error("RoleGrantsUserManagement: want error on closed DB")
	}
	if _, err := s.UserManagesUsers(ctx, 1); err == nil {
		t.Error("UserManagesUsers: want error on closed DB")
	}
	if _, err := s.UserManagerCount(ctx); err == nil {
		t.Error("UserManagerCount: want error on closed DB")
	}
}
