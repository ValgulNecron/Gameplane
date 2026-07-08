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
			`INSERT INTO user_role_bindings(user_id, role_name, cluster, namespace) VALUES (1, ?, 'local', ?)`,
			b.role, b.ns); err != nil {
			t.Fatalf("insert binding %v: %v", b, err)
		}
	}

	ss := NewSessionStore(s)
	perms, err := ss.LoadPerms(context.Background(), 1)
	if err != nil {
		t.Fatalf("LoadPerms: %v", err)
	}

	// Cluster-wide viewer perms land under local/"*".
	if _, ok := perms["local"]["*"]["servers:read"]; !ok {
		t.Errorf("cluster-wide servers:read missing: %+v", perms["local"])
	}
	if _, ok := perms["local"]["*"]["servers:write"]; ok {
		t.Errorf("viewer must not hold servers:write cluster-wide")
	}
	// Operator perms land under local/team-a only.
	if _, ok := perms["local"]["team-a"]["servers:write"]; !ok {
		t.Errorf("team-a servers:write missing: %+v", perms["local"]["team-a"])
	}
	if _, ok := perms["local"]["team-a"]; ok {
		if _, leaked := perms["local"]["team-a"]["users:manage"]; leaked {
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

func TestLoadPerms_MultipleClusterBuildsThreeLevelMap(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "viewer")
	// Insert bindings on two clusters: local and prod.
	for _, b := range []struct{ cluster, role, ns string }{
		{"local", "viewer", "*"},
		{"local", "operator", "team-a"},
		{"prod", "operator", "*"},
	} {
		if _, err := s.DB.Exec(
			`INSERT INTO user_role_bindings(user_id, role_name, cluster, namespace) VALUES (1, ?, ?, ?)`,
			b.role, b.cluster, b.ns); err != nil {
			t.Fatalf("insert binding %v: %v", b, err)
		}
	}

	ss := NewSessionStore(s)
	perms, err := ss.LoadPerms(context.Background(), 1)
	if err != nil {
		t.Fatalf("LoadPerms: %v", err)
	}

	// Verify the three-level structure.
	if _, ok := perms["local"]["*"]["servers:read"]; !ok {
		t.Errorf("local cluster-wide servers:read missing")
	}
	if _, ok := perms["local"]["team-a"]["servers:write"]; !ok {
		t.Errorf("local/team-a servers:write missing")
	}
	if _, ok := perms["prod"]["*"]["servers:write"]; !ok {
		t.Errorf("prod cluster-wide servers:write missing (operator)")
	}
	// Cross-cluster shouldn't exist.
	if _, ok := perms["prod"]["team-a"]; ok {
		t.Errorf("prod/team-a should not exist, got %+v", perms["prod"]["team-a"])
	}
}

func TestUserCan_NamespacedGatedByCluster(t *testing.T) {
	// An admin bound only to local should not grant access on prod.
	localAdmin := &User{Perms: map[string]map[string]map[string]struct{}{"local": {"*": {"*": {}}}}}

	if got := localAdmin.Can("servers:write", true, "local", "team-a"); !got {
		t.Errorf("servers:write on local/team-a: want true, got %v", got)
	}
	if got := localAdmin.Can("servers:write", true, "prod", "team-a"); got {
		t.Errorf("servers:write on prod/team-a: want false, got %v", got)
	}
}

func TestUserCan_ClusterScopedNotGatedByCluster(t *testing.T) {
	// A viewer bound only to local should still grant users:read (cluster-scoped)
	// even when querying a different cluster.
	localReader := &User{Perms: map[string]map[string]map[string]struct{}{"local": {"*": {"users:read": {}}}}}

	if got := localReader.Can("users:read", false, "prod", ""); !got {
		t.Errorf("users:read on prod (cluster-scoped): want true, got %v", got)
	}
}

func TestUserCan_WildcardClusterGrantsAll(t *testing.T) {
	// A binding on the "*" wildcard cluster should grant access on any cluster.
	wildcardBinding := &User{Perms: map[string]map[string]map[string]struct{}{"*": {"*": {"servers:write": {}}}}}

	if got := wildcardBinding.Can("servers:write", true, "prod", "team-a"); !got {
		t.Errorf("servers:write on prod/team-a with wildcard cluster: want true, got %v", got)
	}
	if got := wildcardBinding.Can("servers:write", true, "local", "team-b"); !got {
		t.Errorf("servers:write on local/team-b with wildcard cluster: want true, got %v", got)
	}
}

func TestUserCan_NamespacedBindingClusterRestricted(t *testing.T) {
	// A namespace-specific binding on local should not grant on prod.
	localNsOp := &User{Perms: map[string]map[string]map[string]struct{}{"local": {"team-a": {"servers:write": {}}}}}

	if got := localNsOp.Can("servers:write", true, "local", "team-a"); !got {
		t.Errorf("servers:write on local/team-a: want true, got %v", got)
	}
	if got := localNsOp.Can("servers:write", true, "prod", "team-a"); got {
		t.Errorf("servers:write on prod/team-a: want false, got %v", got)
	}
}

func TestUserCan(t *testing.T) {
	admin := &User{Perms: map[string]map[string]map[string]struct{}{"local": {"*": {"*": {}}}}}
	clusterReader := &User{Perms: map[string]map[string]map[string]struct{}{"local": {"*": {"users:read": {}}}}}
	nsOp := &User{Perms: map[string]map[string]map[string]struct{}{"local": {"team-a": {"servers:write": {}}}}}
	nsWild := &User{Perms: map[string]map[string]map[string]struct{}{"local": {"team-a": {"*": {}}}}}
	prodAdmin := &User{Perms: map[string]map[string]map[string]struct{}{"prod": {"*": {"*": {}}}}}

	cases := []struct {
		name       string
		u          *User
		perm       string
		namespaced bool
		cluster    string
		ns         string
		want       bool
	}{
		{"nil user denied", nil, "servers:read", true, "local", "team-a", false},
		{"admin wildcard grants namespaced on same cluster", admin, "servers:write", true, "local", "team-a", true},
		{"admin wildcard grants cluster perm", admin, "users:manage", false, "prod", "", true},
		{"admin on local denied on prod", admin, "servers:write", true, "prod", "team-a", false},
		{"prod admin grants on prod", prodAdmin, "servers:write", true, "prod", "team-a", true},
		{"prod admin denied on local", prodAdmin, "servers:write", true, "local", "team-a", false},
		{"wildcard cluster grants namespaced on any cluster", &User{Perms: map[string]map[string]map[string]struct{}{"*": {"*": {"servers:write": {}}}}}, "servers:write", true, "prod", "team-a", true},
		{"cluster perm granted on any binding", clusterReader, "users:read", false, "prod", "", true},
		{"cluster perm not held is denied", clusterReader, "users:manage", false, "local", "", false},
		{"ns binding grants in its ns", nsOp, "servers:write", true, "local", "team-a", true},
		{"ns binding denied in other ns", nsOp, "servers:write", true, "local", "team-b", false},
		{"ns binding never grants cluster-scoped", nsOp, "users:manage", false, "local", "", false},
		{"ns wildcard grants namespaced in its ns", nsWild, "servers:read", true, "local", "team-a", true},
		{"ns wildcard denied cluster-scoped", nsWild, "users:manage", false, "local", "", false},
		{"namespaced with empty ns denied", nsOp, "servers:write", true, "local", "", false},
		{"namespaced with star ns denied", nsOp, "servers:write", true, "local", "*", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.u.Can(tc.perm, tc.namespaced, tc.cluster, tc.ns); got != tc.want {
				t.Errorf("Can(%q, %v, %q, %q) = %v, want %v", tc.perm, tc.namespaced, tc.cluster, tc.ns, got, tc.want)
			}
		})
	}
}
