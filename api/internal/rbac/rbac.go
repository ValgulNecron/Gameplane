// Package rbac enforces permission-based access control on API routes.
//
// Roles are named sets of permissions (see catalog.go); users are bound
// to roles per namespace ("*" = cluster-wide). The rule table below maps
// each request to the single permission required to perform it, and the
// middleware checks the caller's resolved permission set — within the
// request's target namespace for namespaced permissions, cluster-wide
// for the rest. The built-in admin/operator/viewer roles are seeded so
// this reproduces the historical role matrix exactly.
package rbac

import (
	"net/http"
	"strings"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// Built-in role names. Custom roles may take any other (valid) name.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// Middleware blocks requests the caller isn't permitted to make. It
// finds the matching rule, resolves the target namespace for namespaced
// permissions, and checks the caller's permission set. No matching rule
// means deny (fail-closed).
func Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			u := auth.UserFromContext(req.Context())
			if u == nil {
				http.Error(w, "unauthenticated", http.StatusUnauthorized)
				return
			}
			r, ok := match(req.Method, req.URL.Path)
			if !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if r.perm == "" { // any authenticated user
				next.ServeHTTP(w, req)
				return
			}
			ns := ""
			if Namespaced(r.perm) {
				resolved, err := scope.Resolve(req)
				if err != nil {
					http.Error(w, "namespace not permitted", http.StatusBadRequest)
					return
				}
				ns = resolved
			}
			if !u.Can(r.perm, Namespaced(r.perm), ns) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// rule maps a request shape to the permission required to perform it.
// The first matching rule wins. method "" matches any method; segment ""
// matches any first path segment; prefix/suffix narrow further when a
// first segment isn't specific enough (e.g. /admin/audit vs /admin/config,
// or the write-capable WS /console vs the read-only logs tail).
type rule struct {
	method  string
	segment string
	prefix  string // strings.HasPrefix(path, prefix)
	suffix  string // strings.HasSuffix(path, suffix)
	perm    string // required permission; "" = any authenticated user
}

var rules = []rule{
	// Own profile: every authenticated user reads /users/me. The rest of
	// /users is gated; must precede the segment-wide users rules.
	{method: "GET", segment: "users", suffix: "/users/me", perm: ""},
	{method: "GET", segment: "users", perm: "users:read"},
	{segment: "users", perm: "users:manage"},

	// Roles + the permission catalog.
	{method: "GET", segment: "roles", perm: "roles:read"},
	{segment: "roles", perm: "roles:manage"},

	// Admin area. Audit is a read; config splits read vs manage; anything
	// else under /admin defaults to the admin wildcard (fail-closed-ish).
	{method: "GET", segment: "admin", prefix: "/admin/audit", perm: "audit:read"},
	{method: "GET", segment: "admin", prefix: "/admin/config", perm: "config:read"},
	{segment: "admin", prefix: "/admin/config", perm: "config:manage"},
	{segment: "admin", perm: "*"},

	// Namespaced game resources. Reads vs writes; the catch-all GET below
	// never fires for these because the explicit read rule matches first.
	{method: "GET", segment: "servers", perm: "servers:read"},
	{segment: "servers", perm: "servers:write"},
	{method: "GET", segment: "templates", perm: "templates:read"},
	{segment: "templates", perm: "templates:write"},
	{method: "GET", segment: "backups", perm: "backups:read"},
	{segment: "backups", perm: "backups:write"},
	{method: "GET", segment: "schedules", perm: "schedules:read"},
	{segment: "schedules", perm: "schedules:write"},
	{method: "GET", segment: "restores", perm: "backups:read"},
	{segment: "restores", perm: "backups:restore"},

	// Backup destinations are restic-repo Secrets: reads expose only the
	// (already-public) URL + a hasPassword bool; writes touch credentials.
	{method: "GET", segment: "backup-destinations", perm: "destinations:read"},
	{segment: "backup-destinations", perm: "destinations:manage"},

	// Module install/upgrade/uninstall + source management are cluster-wide.
	{method: "GET", segment: "modules", perm: "modules:read"},
	{segment: "modules", perm: "modules:manage"},

	// Cluster reads (nodes, version, storage) vs credential-minting ops.
	{method: "GET", segment: "cluster", perm: "cluster:read"},
	{segment: "cluster", perm: "cluster:manage"},

	// Events SSE is a namespaced, multiplexed read.
	{method: "GET", segment: "events", perm: "servers:read"},

	// WebSocket console runs RCON/PTY commands (write-capable, despite the
	// GET upgrade); the logs tail is read-only.
	{method: "GET", segment: "ws", suffix: "/console", perm: "servers:console"},
	{method: "GET", segment: "ws", suffix: "/console-pty", perm: "servers:console"},
	{method: "GET", segment: "ws", perm: "servers:read"},

	// Any other authenticated read of an unlisted path.
	{method: "GET", segment: "", perm: ""},
	{method: "HEAD", segment: "", perm: ""},
}

// firstSegment returns the first path segment, stripping any trailing
// verb suffix ("/servers/foo:start" → "servers").
func firstSegment(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(trimmed, '/'); i >= 0 {
		return trimmed[:i]
	}
	return trimmed
}

// match returns the first rule that applies to (method, path).
func match(method, path string) (rule, bool) {
	seg := firstSegment(path)
	for _, r := range rules {
		if r.method != "" && r.method != method {
			continue
		}
		if r.segment != "" && r.segment != seg {
			continue
		}
		if r.prefix != "" && !strings.HasPrefix(path, r.prefix) {
			continue
		}
		if r.suffix != "" && !strings.HasSuffix(path, r.suffix) {
			continue
		}
		return r, true
	}
	return rule{}, false
}

// allow is the pure authorization check, factored out for testing. ns is
// the already-resolved target namespace (ignored for cluster-scoped
// permissions).
func allow(u *auth.User, method, path, ns string) bool {
	r, ok := match(method, path)
	if !ok {
		return false
	}
	if r.perm == "" {
		return true
	}
	return u.Can(r.perm, Namespaced(r.perm), ns)
}
