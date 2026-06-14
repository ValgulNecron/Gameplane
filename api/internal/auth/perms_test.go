package auth

import (
	"context"
	"testing"
)

func TestLoadPerms_GroupsByNamespace(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "viewer") // first user inserted → id 1
	// The migration backfill only binds users that exist at migrate time;
	// alice is inserted afterward, so create her bindings explicitly: a
	// cluster-wide viewer binding plus an operator binding scoped to one ns.
	for _, b := range []struct{ role, ns string }{
		{"viewer", "*"},
		{"operator", "team-a"},
	} {
		if _, err := s.DB.Exec(
			`INSERT INTO user_role_bindings(user_id, role_name, namespace) VALUES (1, ?, ?)`,
			b.role, b.ns); err != nil {
			t.Fatalf("insert binding %v: %v", b, err)
		}
	}

	ss := NewSessionStore(s)
	perms, err := ss.LoadPerms(context.Background(), 1)
	if err != nil {
		t.Fatalf("LoadPerms: %v", err)
	}

	// Cluster-wide viewer perms land under "*".
	if _, ok := perms["*"]["servers:read"]; !ok {
		t.Errorf("cluster-wide servers:read missing: %+v", perms["*"])
	}
	if _, ok := perms["*"]["servers:write"]; ok {
		t.Errorf("viewer must not hold servers:write cluster-wide")
	}
	// Operator perms land under the bound namespace only.
	if _, ok := perms["team-a"]["servers:write"]; !ok {
		t.Errorf("team-a servers:write missing: %+v", perms["team-a"])
	}
	if _, ok := perms["team-a"]; ok {
		if _, leaked := perms["team-a"]["users:manage"]; leaked {
			t.Errorf("operator unexpectedly holds users:manage")
		}
	}
}

func TestLoadPerms_EmptyForUnboundUser(t *testing.T) {
	s := newAuthDB(t)
	ss := NewSessionStore(s)
	perms, err := ss.LoadPerms(context.Background(), 42)
	if err != nil {
		t.Fatalf("LoadPerms: %v", err)
	}
	if len(perms) != 0 {
		t.Fatalf("expected no perms for unbound user, got %+v", perms)
	}
}

func TestLoadPerms_QueryError(t *testing.T) {
	s := newAuthDB(t)
	ss := NewSessionStore(s)
	_ = s.Close() // query against a closed DB fails
	if _, err := ss.LoadPerms(context.Background(), 1); err == nil {
		t.Fatal("expected error querying a closed DB")
	}
}

func TestUserCan(t *testing.T) {
	admin := &User{Perms: map[string]map[string]struct{}{"*": {"*": {}}}}
	clusterReader := &User{Perms: map[string]map[string]struct{}{"*": {"users:read": {}}}}
	nsOp := &User{Perms: map[string]map[string]struct{}{"team-a": {"servers:write": {}}}}
	nsWild := &User{Perms: map[string]map[string]struct{}{"team-a": {"*": {}}}}

	cases := []struct {
		name       string
		u          *User
		perm       string
		namespaced bool
		ns         string
		want       bool
	}{
		{"nil user denied", nil, "servers:read", true, "team-a", false},
		{"admin wildcard grants namespaced anywhere", admin, "servers:write", true, "team-a", true},
		{"admin wildcard grants cluster perm", admin, "users:manage", false, "", true},
		{"cluster perm granted anywhere", clusterReader, "users:read", false, "", true},
		{"cluster perm not held is denied", clusterReader, "users:manage", false, "", false},
		{"ns binding grants in its ns", nsOp, "servers:write", true, "team-a", true},
		{"ns binding denied in other ns", nsOp, "servers:write", true, "team-b", false},
		{"ns binding never grants cluster-scoped", nsOp, "users:manage", false, "", false},
		{"ns wildcard grants namespaced in its ns", nsWild, "servers:read", true, "team-a", true},
		{"ns wildcard denied cluster-scoped", nsWild, "users:manage", false, "", false},
		{"namespaced with empty ns denied", nsOp, "servers:write", true, "", false},
		{"namespaced with star ns denied", nsOp, "servers:write", true, "*", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.u.Can(tc.perm, tc.namespaced, tc.ns); got != tc.want {
				t.Errorf("Can(%q, %v, %q) = %v, want %v", tc.perm, tc.namespaced, tc.ns, got, tc.want)
			}
		})
	}
}
