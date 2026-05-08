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
	}
	for _, tc := range cases {
		got := allow(tc.role, tc.method, tc.path)
		if got != tc.want {
			t.Errorf("allow(%s,%s,%s) = %v, want %v", tc.role, tc.method, tc.path, got, tc.want)
		}
	}
}
