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
	return &auth.User{Role: role, Perms: map[string]map[string]struct{}{
		"*": loadRolePerms(t, store, role),
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
		role, method, path string
		want               bool
	}{
		{RoleViewer, "GET", "/servers", true},
		{RoleViewer, "POST", "/servers", false},
		{RoleOperator, "POST", "/servers", true},
		{RoleOperator, "DELETE", "/users/1", false},
		{RoleAdmin, "DELETE", "/users/1", true},
		// Own profile: every authenticated role reads /users/me; the rest
		// of /users stays admin-only, and /users/me stays read-only.
		{RoleViewer, "GET", "/users/me", true},
		{RoleOperator, "GET", "/users/me", true},
		{RoleAdmin, "GET", "/users/me", true},
		{RoleViewer, "GET", "/users", false},
		{RoleOperator, "GET", "/users", false},
		{RoleAdmin, "GET", "/users", true},
		{RoleViewer, "POST", "/users/me", false},
		{RoleViewer, "PUT", "/users/me", false},
		{RoleOperator, "GET", "/admin/audit", false},
		{RoleAdmin, "GET", "/admin/audit", true},
		{RoleViewer, "PATCH", "/servers/foo", false},
		// Prefix-confusion regression: /serverz must not inherit /servers.
		{RoleAdmin, "POST", "/serverz", false},
		// Verb-suffix paths: /servers/foo:start resolves to segment "servers".
		{RoleOperator, "POST", "/servers/foo:start", true},
		{RoleViewer, "POST", "/servers/foo:start", false},
		// WebSocket console forwards to RCON.Exec — viewers denied even
		// though the upgrade handshake is a GET.
		{RoleViewer, "GET", "/ws/servers/foo/console", false},
		{RoleOperator, "GET", "/ws/servers/foo/console", true},
		{RoleAdmin, "GET", "/ws/servers/foo/console", true},
		// WS logs tail is read-only → viewer ok.
		{RoleViewer, "GET", "/ws/servers/foo/logs", true},
		{RoleOperator, "GET", "/ws/servers/foo/logs", true},
		// PTY console attaches stdin → operator+, like RCON console.
		{RoleViewer, "GET", "/ws/servers/foo/console-pty", false},
		{RoleOperator, "GET", "/ws/servers/foo/console-pty", true},
		{RoleAdmin, "GET", "/ws/servers/foo/console-pty", true},
		// Guard against the console-suffix rule triggering on a spoofed
		// resource path "/servers/foo/console" (segment "servers", not "ws").
		{RoleViewer, "GET", "/servers/foo/console", true},
		{RoleViewer, "POST", "/servers/foo/console", false},
		// Player moderation are POSTs under /servers → operator+; banlist read.
		{RoleViewer, "POST", "/servers/foo/players/kick", false},
		{RoleOperator, "POST", "/servers/foo/players/kick", true},
		{RoleAdmin, "POST", "/servers/foo/players/kick", true},
		{RoleViewer, "POST", "/servers/foo/players/ban", false},
		{RoleOperator, "POST", "/servers/foo/players/ban", true},
		{RoleViewer, "POST", "/servers/foo/players/unban", false},
		{RoleOperator, "POST", "/servers/foo/players/unban", true},
		{RoleViewer, "GET", "/servers/foo/players/banned", true},
		{RoleViewer, "GET", "/servers/foo/players", true},
		// Module-declared actions run console commands → operator+. Status read.
		{RoleViewer, "POST", "/servers/foo/actions/run", false},
		{RoleOperator, "POST", "/servers/foo/actions/run", true},
		{RoleAdmin, "POST", "/servers/foo/actions/run", true},
		{RoleViewer, "GET", "/servers/foo/status", true},
		{RoleOperator, "GET", "/servers/foo/status", true},
		// Mods: listing viewer+, install/remove operator+.
		{RoleViewer, "GET", "/servers/foo/mods", true},
		{RoleViewer, "POST", "/servers/foo/mods/install", false},
		{RoleOperator, "POST", "/servers/foo/mods/install", true},
		{RoleViewer, "DELETE", "/servers/foo/mods", false},
		{RoleOperator, "DELETE", "/servers/foo/mods", true},
		// Module + source management admin-only; reads viewer+.
		{RoleViewer, "GET", "/modules/catalog", true},
		{RoleViewer, "GET", "/modules/sources", true},
		{RoleOperator, "POST", "/modules", false},
		{RoleAdmin, "POST", "/modules", true},
		{RoleOperator, "POST", "/modules/sources", false},
		{RoleAdmin, "POST", "/modules/sources", true},
		{RoleOperator, "PUT", "/modules/sources/upstream", false},
		{RoleAdmin, "PUT", "/modules/sources/upstream", true},
		{RoleOperator, "DELETE", "/modules/sources/upstream", false},
		{RoleAdmin, "DELETE", "/modules/sources/upstream", true},
		// Backup destinations: read viewer+, write admin-only.
		{RoleViewer, "GET", "/backup-destinations", true},
		{RoleOperator, "POST", "/backup-destinations", false},
		{RoleAdmin, "POST", "/backup-destinations", true},
		// Cluster reads viewer+; credential-minting POSTs admin-only.
		{RoleViewer, "GET", "/cluster", true},
		{RoleViewer, "GET", "/cluster/stats", true},
		{RoleOperator, "POST", "/cluster/nodes:join", false},
		{RoleAdmin, "POST", "/cluster/nodes:join", true},
		{RoleAdmin, "POST", "/cluster/kubeconfig", true},
		// Config: admin-only, both read and write.
		{RoleViewer, "GET", "/admin/config", false},
		{RoleAdmin, "GET", "/admin/config", true},
		{RoleOperator, "PUT", "/admin/config/general", false},
		{RoleAdmin, "PUT", "/admin/config/general", true},
		// Notification test-sends share the config manage permission.
		{RoleViewer, "POST", "/admin/notifications/sinks/x/test", false},
		{RoleOperator, "POST", "/admin/notifications/sinks/x/test", false},
		{RoleAdmin, "POST", "/admin/notifications/sinks/x/test", true},
		// Restores: read viewer+, create operator+ (backups:restore).
		{RoleViewer, "GET", "/restores", true},
		{RoleViewer, "POST", "/restores", false},
		{RoleOperator, "POST", "/restores", true},
		{RoleAdmin, "POST", "/restores", true},
		// Events SSE is a namespaced read.
		{RoleViewer, "GET", "/events", true},
		// Roles: read for all built-ins; manage admin-only.
		{RoleViewer, "GET", "/roles", true},
		{RoleOperator, "GET", "/roles/permissions", true},
		{RoleOperator, "POST", "/roles", false},
		{RoleAdmin, "POST", "/roles", true},
	}
	for _, tc := range cases {
		got := allow(users[tc.role], tc.method, tc.path, ns)
		if got != tc.want {
			t.Errorf("allow(%s,%s,%s) = %v, want %v", tc.role, tc.method, tc.path, got, tc.want)
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
	opInTeamA := &auth.User{Role: RoleOperator, Perms: map[string]map[string]struct{}{
		"team-a": opPerms,
	}}
	// Admin in "team-a" only — namespaced wildcard, but not cluster-wide.
	adminInTeamA := &auth.User{Role: RoleAdmin, Perms: map[string]map[string]struct{}{
		"team-a": {"*": {}},
	}}

	cases := []struct {
		name             string
		u                *auth.User
		method, path, ns string
		want             bool
	}{
		{"op writes servers in its ns", opInTeamA, "POST", "/servers", "team-a", true},
		{"op denied servers in other ns", opInTeamA, "POST", "/servers", scope.DefaultNamespace, false},
		{"op denied servers read in other ns", opInTeamA, "GET", "/servers", scope.DefaultNamespace, false},
		{"op reads servers in its ns", opInTeamA, "GET", "/servers", "team-a", true},
		// Cluster-scoped action — a namespace binding never grants it.
		{"op denied user mgmt", opInTeamA, "POST", "/users", "", false},
		{"ns-admin writes servers in its ns", adminInTeamA, "POST", "/servers", "team-a", true},
		{"ns-admin denied servers elsewhere", adminInTeamA, "POST", "/servers", scope.DefaultNamespace, false},
		// Namespace wildcard is namespaced-only: cluster authority denied.
		{"ns-admin denied user mgmt", adminInTeamA, "POST", "/users", "", false},
		{"ns-admin denied audit", adminInTeamA, "GET", "/admin/audit", "", false},
	}
	for _, tc := range cases {
		if got := allow(tc.u, tc.method, tc.path, tc.ns); got != tc.want {
			t.Errorf("%s: allow=%v want %v", tc.name, got, tc.want)
		}
	}
}

// TestAllow_NilUserDenied is a fail-closed guard.
func TestAllow_NilUserDenied(t *testing.T) {
	if allow(nil, "GET", "/servers", scope.DefaultNamespace) {
		t.Error("nil user allowed")
	}
}
