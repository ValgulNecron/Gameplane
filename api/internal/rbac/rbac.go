// Package rbac enforces role-based access control on API routes.
//
// Roles:
//   - admin    — full access, including users/RBAC + admin settings
//   - operator — create/update/delete GameServers + backups + templates
//   - viewer   — read-only access
//
// Scope granularity past the top-level role (per-namespace, per-server)
// is expressed in a future users.scope JSON column — v1 ships with
// coarse cluster-wide roles, which matches the Users & RBAC screen.
package rbac

import (
	"net/http"
	"strings"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
)

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// Middleware blocks requests based on the caller's role. The route
// table below maps method+firstSegment to a minimum role; if no rule
// matches, access is denied (fail-closed).
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			u := auth.UserFromContext(req.Context())
			if u == nil {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			if !allow(u.Role, req.Method, req.URL.Path) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// rule matches on the first path segment (e.g. "servers" from
// "/servers/foo") rather than on a free-form prefix — "/servers" no
// longer matches "/serverz" or "/servers2". pathSuffix narrows further
// when a single first-segment isn't specific enough — WS routes share
// segment "ws" but differ in behavior between console (writes via RCON)
// and logs (read-only).
type rule struct {
	method     string // "" = any
	segment    string // "" = any first segment (used for the catch-all read rule)
	pathSuffix string // "" = any; otherwise strings.HasSuffix(path, pathSuffix)
	role       string
}

var rules = []rule{
	// Admin-only
	{method: "", segment: "admin", role: RoleAdmin},
	// Own profile: any authenticated role may read /users/me (the
	// dashboard loads it after login). Must precede the admin-only
	// catch-all for the rest of /users.
	{method: "GET", segment: "users", pathSuffix: "/users/me", role: RoleViewer},
	{method: "", segment: "users", role: RoleAdmin},

	// Writes on server/template/backup resources → operator+
	{method: "POST", segment: "servers", role: RoleOperator},
	{method: "PUT", segment: "servers", role: RoleOperator},
	{method: "PATCH", segment: "servers", role: RoleOperator},
	{method: "DELETE", segment: "servers", role: RoleOperator},
	{method: "POST", segment: "templates", role: RoleOperator},
	{method: "DELETE", segment: "templates", role: RoleOperator},
	{method: "POST", segment: "backups", role: RoleOperator},
	{method: "DELETE", segment: "backups", role: RoleOperator},
	{method: "POST", segment: "schedules", role: RoleOperator},
	{method: "PUT", segment: "schedules", role: RoleOperator},
	{method: "DELETE", segment: "schedules", role: RoleOperator},

	// Backup destinations are restic-repo Secrets. Reads expose only
	// the (already-public) URL plus a hasPassword bool, so viewers can
	// see what's configured. Writes touch live credentials → admin only.
	{method: "POST", segment: "backup-destinations", role: RoleAdmin},
	{method: "PUT", segment: "backup-destinations", role: RoleAdmin},
	{method: "DELETE", segment: "backup-destinations", role: RoleAdmin},

	// Module install/upgrade/uninstall and module-source management are
	// admin-only — they make the cluster pull arbitrary bundles and
	// create cluster-scoped GameTemplates. Reads (catalog, sources,
	// installed list) follow the global viewer+ rule below.
	{method: "POST", segment: "modules", role: RoleAdmin},
	{method: "PUT", segment: "modules", role: RoleAdmin},
	{method: "PATCH", segment: "modules", role: RoleAdmin},
	{method: "DELETE", segment: "modules", role: RoleAdmin},

	// WebSocket console executes RCON commands (/stop, /op, /ban, …).
	// The upgrade is an HTTP GET but the stream is read-write, so it
	// doesn't fit the usual "GET = safe" shape. Require operator+.
	// Must come before the generic segment:"ws" rule below.
	{method: "GET", segment: "ws", pathSuffix: "/console", role: RoleOperator},
	// PTY console attaches the user to the game container's stdin via
	// kubectl-attach. Same write-capability profile as RCON console.
	{method: "GET", segment: "ws", pathSuffix: "/console-pty", role: RoleOperator},
	// Other WS endpoints (logs tail) are read-only — viewer is fine.
	{method: "GET", segment: "ws", role: RoleViewer},

	// Everyone else (including viewer) can read. Empty segment matches
	// any top-level resource.
	{method: "GET", segment: "", role: RoleViewer},
	{method: "HEAD", segment: "", role: RoleViewer},
}

var rank = map[string]int{RoleViewer: 0, RoleOperator: 1, RoleAdmin: 2}

// firstSegment returns the first path segment, stripping any trailing
// verb suffix ("/servers/foo:start" → "servers").
func firstSegment(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(trimmed, '/'); i >= 0 {
		return trimmed[:i]
	}
	return trimmed
}

func allow(role, method, path string) bool {
	seg := firstSegment(path)
	for _, r := range rules {
		if r.method != "" && r.method != method {
			continue
		}
		if r.segment != "" && r.segment != seg {
			continue
		}
		if r.pathSuffix != "" && !strings.HasSuffix(path, r.pathSuffix) {
			continue
		}
		return rank[role] >= rank[r.role]
	}
	return false
}
