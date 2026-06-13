package rbac

import "testing"

func TestAllow(t *testing.T) {
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
		// of /users stays admin-only, and /users/me stays read-only for
		// non-admins.
		{RoleViewer, "GET", "/users/me", true},
		{RoleOperator, "GET", "/users/me", true},
		{RoleAdmin, "GET", "/users/me", true},
		{RoleViewer, "GET", "/users", false},
		{RoleViewer, "POST", "/users/me", false},
		{RoleViewer, "PUT", "/users/me", false},
		{RoleOperator, "GET", "/admin/audit", false},
		{RoleAdmin, "GET", "/admin/audit", true},
		{RoleViewer, "PATCH", "/servers/foo", false},
		// Prefix-confusion regression: /serverz must not inherit the
		// /servers rule set.
		{RoleAdmin, "POST", "/serverz", false},
		// Verb-suffix paths: /servers/foo:start must resolve to segment "servers".
		{RoleOperator, "POST", "/servers/foo:start", true},
		{RoleViewer, "POST", "/servers/foo:start", false},
		// WebSocket console forwards to RCON.Exec — viewers must be denied
		// even though the upgrade handshake is an HTTP GET.
		{RoleViewer, "GET", "/ws/servers/foo/console", false},
		{RoleOperator, "GET", "/ws/servers/foo/console", true},
		{RoleAdmin, "GET", "/ws/servers/foo/console", true},
		// WS logs tail is read-only → viewer ok.
		{RoleViewer, "GET", "/ws/servers/foo/logs", true},
		{RoleOperator, "GET", "/ws/servers/foo/logs", true},
		// PTY console attaches stdin to the game container — must require
		// operator+, like the RCON-backed /console endpoint above.
		{RoleViewer, "GET", "/ws/servers/foo/console-pty", false},
		{RoleOperator, "GET", "/ws/servers/foo/console-pty", true},
		{RoleAdmin, "GET", "/ws/servers/foo/console-pty", true},
		// Guard against the console-suffix rule being triggered by a
		// spoofed path like "/servers/foo/console" (segment "servers", not
		// "ws") — must still require operator via POST rules, and GET must
		// stay viewer-readable (it's a resource read, not the WS console).
		{RoleViewer, "GET", "/servers/foo/console", true},
		{RoleViewer, "POST", "/servers/foo/console", false},
		// Player moderation endpoints are POSTs under /servers and inherit
		// the operator+ requirement from the POST/servers rule. The banlist
		// is read-only, so viewer is fine.
		{RoleViewer, "POST", "/servers/foo/players/kick", false},
		{RoleOperator, "POST", "/servers/foo/players/kick", true},
		{RoleAdmin, "POST", "/servers/foo/players/kick", true},
		{RoleViewer, "POST", "/servers/foo/players/ban", false},
		{RoleOperator, "POST", "/servers/foo/players/ban", true},
		{RoleViewer, "POST", "/servers/foo/players/unban", false},
		{RoleOperator, "POST", "/servers/foo/players/unban", true},
		{RoleViewer, "GET", "/servers/foo/players/banned", true},
		{RoleViewer, "GET", "/servers/foo/players", true},
		// Module-declared actions run console commands → operator+, same
		// as the RCON console. Live status is a read → viewer+.
		{RoleViewer, "POST", "/servers/foo/actions/run", false},
		{RoleOperator, "POST", "/servers/foo/actions/run", true},
		{RoleAdmin, "POST", "/servers/foo/actions/run", true},
		{RoleViewer, "GET", "/servers/foo/status", true},
		{RoleOperator, "GET", "/servers/foo/status", true},
		// Mods: listing is viewer+, install/remove are operator+.
		{RoleViewer, "GET", "/servers/foo/mods", true},
		{RoleViewer, "POST", "/servers/foo/mods/install", false},
		{RoleOperator, "POST", "/servers/foo/mods/install", true},
		{RoleViewer, "DELETE", "/servers/foo/mods", false},
		{RoleOperator, "DELETE", "/servers/foo/mods", true},
		// Module + module-source management is admin-only; reads stay
		// viewer-accessible.
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
	}
	for _, tc := range cases {
		got := allow(tc.role, tc.method, tc.path)
		if got != tc.want {
			t.Errorf("allow(%s,%s,%s) = %v, want %v", tc.role, tc.method, tc.path, got, tc.want)
		}
	}
}
