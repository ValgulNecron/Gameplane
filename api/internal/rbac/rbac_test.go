package rbac

import (
	"context"
	"testing"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// userForRole builds an authenticated User whose cluster-wide ("*")
// binding carries exactly the seeded permissions of the named built-in
// role. Loading from a freshly-migrated DB makes the test a proof that
// the SQL seed reproduces the historical matrix below — not a restatement
// of it.
func userForRole(t *testing.T, store *db.Store, role string) *auth.User {
	t.Helper()
	return &auth.User{Role: role, Perms: map[string]map[string]map[string]struct{}{
		scope.DefaultCluster: {
			"*": loadRolePerms(t, store, role),
		},
	}}
}

func loadRolePerms(t *testing.T, store *db.Store, role string) map[string]struct{} {
	t.Helper()
	rows, err := store.DB.Query(`SELECT permission FROM role_permissions WHERE role_name = ?`, role)
	if err != nil {
		t.Fatalf("load perms for %q: %v", role, err)
	}
	defer rows.Close()
	set := map[string]struct{}{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("scan: %v", err)
		}
		set[p] = struct{}{}
	}
	return set
}

func migratedStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func TestAllow(t *testing.T) {
	store := migratedStore(t)
	users := map[string]*auth.User{
		RoleViewer:   userForRole(t, store, RoleViewer),
		RoleOperator: userForRole(t, store, RoleOperator),
		RoleAdmin:    userForRole(t, store, RoleAdmin),
	}
	// Cluster-wide bindings ignore ns; use the default for every case.
	const ns = scope.DefaultNamespace

	cases := []struct {
		role, method, path, cluster string
		want                        bool
	}{
		{RoleViewer, "GET", "/servers", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/servers", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/servers", scope.DefaultCluster, true},
		{RoleOperator, "DELETE", "/users/1", scope.DefaultCluster, false},
		{RoleAdmin, "DELETE", "/users/1", scope.DefaultCluster, true},
		// Own profile: every authenticated role reads /users/me; the rest
		// of /users stays admin-only, and /users/me stays read-only.
		{RoleViewer, "GET", "/users/me", scope.DefaultCluster, true},
		{RoleOperator, "GET", "/users/me", scope.DefaultCluster, true},
		{RoleAdmin, "GET", "/users/me", scope.DefaultCluster, true},
		{RoleViewer, "GET", "/users", scope.DefaultCluster, false},
		{RoleOperator, "GET", "/users", scope.DefaultCluster, false},
		{RoleAdmin, "GET", "/users", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/users/me", scope.DefaultCluster, false},
		{RoleViewer, "PUT", "/users/me", scope.DefaultCluster, false},
		{RoleOperator, "GET", "/admin/audit", scope.DefaultCluster, false},
		{RoleAdmin, "GET", "/admin/audit", scope.DefaultCluster, true},
		{RoleViewer, "PATCH", "/servers/foo", scope.DefaultCluster, false},
		// Prefix-confusion regression: /serverz must not inherit /servers.
		{RoleAdmin, "POST", "/serverz", scope.DefaultCluster, false},
		// Verb-suffix paths: /servers/foo:start resolves to segment "servers".
		{RoleOperator, "POST", "/servers/foo:start", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/servers/foo:start", scope.DefaultCluster, false},
		// WebSocket console forwards to RCON.Exec — viewers denied even
		// though the upgrade handshake is a GET.
		{RoleViewer, "GET", "/ws/servers/foo/console", scope.DefaultCluster, false},
		{RoleOperator, "GET", "/ws/servers/foo/console", scope.DefaultCluster, true},
		{RoleAdmin, "GET", "/ws/servers/foo/console", scope.DefaultCluster, true},
		// WS logs tail is read-only → viewer ok.
		{RoleViewer, "GET", "/ws/servers/foo/logs", scope.DefaultCluster, true},
		{RoleOperator, "GET", "/ws/servers/foo/logs", scope.DefaultCluster, true},
		// PTY console attaches stdin → operator+, like RCON console.
		{RoleViewer, "GET", "/ws/servers/foo/console-pty", scope.DefaultCluster, false},
		{RoleOperator, "GET", "/ws/servers/foo/console-pty", scope.DefaultCluster, true},
		{RoleAdmin, "GET", "/ws/servers/foo/console-pty", scope.DefaultCluster, true},
		// Guard against the console-suffix rule triggering on a spoofed
		// resource path "/servers/foo/console" (segment "servers", not "ws").
		{RoleViewer, "GET", "/servers/foo/console", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/servers/foo/console", scope.DefaultCluster, false},
		// Player moderation are POSTs under /servers → operator+; banlist read.
		{RoleViewer, "POST", "/servers/foo/players/kick", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/servers/foo/players/kick", scope.DefaultCluster, true},
		{RoleAdmin, "POST", "/servers/foo/players/kick", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/servers/foo/players/ban", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/servers/foo/players/ban", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/servers/foo/players/unban", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/servers/foo/players/unban", scope.DefaultCluster, true},
		{RoleViewer, "GET", "/servers/foo/players/banned", scope.DefaultCluster, true},
		{RoleViewer, "GET", "/servers/foo/players", scope.DefaultCluster, true},
		// Module-declared actions run console commands → operator+. Status read.
		{RoleViewer, "POST", "/servers/foo/actions/run", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/servers/foo/actions/run", scope.DefaultCluster, true},
		{RoleAdmin, "POST", "/servers/foo/actions/run", scope.DefaultCluster, true},
		{RoleViewer, "GET", "/servers/foo/status", scope.DefaultCluster, true},
		{RoleOperator, "GET", "/servers/foo/status", scope.DefaultCluster, true},
		// Mods: listing viewer+, install/remove operator+.
		{RoleViewer, "GET", "/servers/foo/mods", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/servers/foo/mods/install", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/servers/foo/mods/install", scope.DefaultCluster, true},
		{RoleViewer, "DELETE", "/servers/foo/mods", scope.DefaultCluster, false},
		{RoleOperator, "DELETE", "/servers/foo/mods", scope.DefaultCluster, true},
		// ID-managed mods (ARK/Zomboid-style): same read/write split as the
		// file-based mods surface above — GET viewer+, the bulk-replace
		// PUT operator+ (it's a mod-management write, not a read).
		{RoleViewer, "GET", "/servers/foo/mods/ids", scope.DefaultCluster, true},
		{RoleViewer, "PUT", "/servers/foo/mods/ids", scope.DefaultCluster, false},
		{RoleOperator, "PUT", "/servers/foo/mods/ids", scope.DefaultCluster, true},
		{RoleAdmin, "PUT", "/servers/foo/mods/ids", scope.DefaultCluster, true},
		// Module + source management admin-only; reads viewer+.
		{RoleViewer, "GET", "/modules/catalog", scope.DefaultCluster, true},
		{RoleViewer, "GET", "/modules/sources", scope.DefaultCluster, true},
		{RoleOperator, "POST", "/modules", scope.DefaultCluster, false},
		{RoleAdmin, "POST", "/modules", scope.DefaultCluster, true},
		{RoleOperator, "POST", "/modules/sources", scope.DefaultCluster, false},
		{RoleAdmin, "POST", "/modules/sources", scope.DefaultCluster, true},
		{RoleOperator, "PUT", "/modules/sources/upstream", scope.DefaultCluster, false},
		{RoleAdmin, "PUT", "/modules/sources/upstream", scope.DefaultCluster, true},
		{RoleOperator, "DELETE", "/modules/sources/upstream", scope.DefaultCluster, false},
		{RoleAdmin, "DELETE", "/modules/sources/upstream", scope.DefaultCluster, true},
		// Backup destinations: read viewer+, write admin-only.
		{RoleViewer, "GET", "/backup-destinations", scope.DefaultCluster, true},
		{RoleOperator, "POST", "/backup-destinations", scope.DefaultCluster, false},
		{RoleAdmin, "POST", "/backup-destinations", scope.DefaultCluster, true},
		// Cluster reads viewer+; credential-minting POSTs admin-only.
		{RoleViewer, "GET", "/cluster", scope.DefaultCluster, true},
		{RoleViewer, "GET", "/cluster/stats", scope.DefaultCluster, true},
		{RoleOperator, "POST", "/cluster/nodes:join", scope.DefaultCluster, false},
		{RoleAdmin, "POST", "/cluster/nodes:join", scope.DefaultCluster, true},
		{RoleAdmin, "POST", "/cluster/kubeconfig", scope.DefaultCluster, true},
		// Config: admin-only, both read and write.
		{RoleViewer, "GET", "/admin/config", scope.DefaultCluster, false},
		{RoleAdmin, "GET", "/admin/config", scope.DefaultCluster, true},
		{RoleOperator, "PUT", "/admin/config/general", scope.DefaultCluster, false},
		{RoleAdmin, "PUT", "/admin/config/general", scope.DefaultCluster, true},
		// Notification test-sends share the config manage permission.
		{RoleViewer, "POST", "/admin/notifications/sinks/x/test", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/admin/notifications/sinks/x/test", scope.DefaultCluster, false},
		{RoleAdmin, "POST", "/admin/notifications/sinks/x/test", scope.DefaultCluster, true},
		// Mod-registry provider key secrets share the config manage permission.
		{RoleViewer, "PUT", "/admin/registries/curseforge/secret", scope.DefaultCluster, false},
		{RoleOperator, "PUT", "/admin/registries/curseforge/secret", scope.DefaultCluster, false},
		{RoleAdmin, "PUT", "/admin/registries/curseforge/secret", scope.DefaultCluster, true},
		{RoleAdmin, "DELETE", "/admin/registries/curseforge/secret", scope.DefaultCluster, true},
		// Restores: read viewer+, create operator+ (backups:restore).
		{RoleViewer, "GET", "/restores", scope.DefaultCluster, true},
		{RoleViewer, "POST", "/restores", scope.DefaultCluster, false},
		{RoleOperator, "POST", "/restores", scope.DefaultCluster, true},
		{RoleAdmin, "POST", "/restores", scope.DefaultCluster, true},
		// Events SSE is a namespaced read.
		{RoleViewer, "GET", "/events", scope.DefaultCluster, true},
		// Roles: read for all built-ins; manage admin-only.
		{RoleViewer, "GET", "/roles", scope.DefaultCluster, true},
		{RoleOperator, "GET", "/roles/permissions", scope.DefaultCluster, true},
		{RoleOperator, "POST", "/roles", scope.DefaultCluster, false},
		{RoleAdmin, "POST", "/roles", scope.DefaultCluster, true},
	}
	for _, tc := range cases {
		got := allow(users[tc.role], tc.method, tc.path, tc.cluster, ns)
		if got != tc.want {
			t.Errorf("allow(%s,%s,%s,%s) = %v, want %v", tc.role, tc.method, tc.path, tc.cluster, got, tc.want)
		}
	}
}

// TestAllow_PerNamespace proves that a role bound to a single namespace
// authorizes namespaced actions only there, and never confers
// cluster-scoped authority.
func TestAllow_PerNamespace(t *testing.T) {
	store := migratedStore(t)
	opPerms := loadRolePerms(t, store, RoleOperator)

	// Operator in "team-a" only — no cluster-wide binding.
	opInTeamA := &auth.User{Role: RoleOperator, Perms: map[string]map[string]map[string]struct{}{
		scope.DefaultCluster: {
			"team-a": opPerms,
		},
	}}
	// Admin in "team-a" only — namespaced wildcard, but not cluster-wide.
	adminInTeamA := &auth.User{Role: RoleAdmin, Perms: map[string]map[string]map[string]struct{}{
		scope.DefaultCluster: {
			"team-a": {"*": {}},
		},
	}}

	cases := []struct {
		name                      string
		u                         *auth.User
		method, path, cluster, ns string
		want                      bool
	}{
		{"op writes servers in its ns", opInTeamA, "POST", "/servers", scope.DefaultCluster, "team-a", true},
		{"op denied servers in other ns", opInTeamA, "POST", "/servers", scope.DefaultCluster, scope.DefaultNamespace, false},
		{"op denied servers read in other ns", opInTeamA, "GET", "/servers", scope.DefaultCluster, scope.DefaultNamespace, false},
		{"op reads servers in its ns", opInTeamA, "GET", "/servers", scope.DefaultCluster, "team-a", true},
		// Cluster-scoped action — a namespace binding never grants it.
		{"op denied user mgmt", opInTeamA, "POST", "/users", scope.DefaultCluster, "", false},
		{"ns-admin writes servers in its ns", adminInTeamA, "POST", "/servers", scope.DefaultCluster, "team-a", true},
		{"ns-admin denied servers elsewhere", adminInTeamA, "POST", "/servers", scope.DefaultCluster, scope.DefaultNamespace, false},
		// Namespace wildcard is namespaced-only: cluster authority denied.
		{"ns-admin denied user mgmt", adminInTeamA, "POST", "/users", scope.DefaultCluster, "", false},
		{"ns-admin denied audit", adminInTeamA, "GET", "/admin/audit", scope.DefaultCluster, "", false},
	}
	for _, tc := range cases {
		if got := allow(tc.u, tc.method, tc.path, tc.cluster, tc.ns); got != tc.want {
			t.Errorf("%s: allow=%v want %v", tc.name, got, tc.want)
		}
	}
}

// TestAllow_NilUserDenied is a fail-closed guard.
func TestAllow_NilUserDenied(t *testing.T) {
	if allow(nil, "GET", "/servers", scope.DefaultCluster, scope.DefaultNamespace) {
		t.Error("nil user allowed")
	}
}
