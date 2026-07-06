// Package rbac enforces permission-based access control on API routes.
//
// Roles are named sets of permissions (see catalog.go); users are bound
// to roles per namespace ("*" = cluster-wide). The rule table below maps
// each request to the single permission required to perform it, and the
// middleware checks the caller's resolved permission set — within the
// request's target namespace for namespaced permissions, cluster-wide
// for the rest. The built-in admin/operator/viewer roles are seeded so
// this reproduces the historical role matrix exactly.
//
// Owner and collaborator access is an additional fallback: when a
// namespace permission is denied and the request targets a specific
// GameServer, the middleware fetches the server and grants access if
// the caller is the owner or a collaborator (except for :transfer and
// :collaborators endpoints, which are owner-only).
package rbac

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// ServerFetcher fetches a GameServer so the middleware can evaluate
// owner/collaborator access. nil disables the fallback.
type ServerFetcher interface {
	GetServer(ctx context.Context, ns, name string) (*unstructured.Unstructured, error)
}

// Built-in role names. Custom roles may take any other (valid) name.
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// Ownership role constants for server access fallback.
const (
	roleOwner = iota
	roleCollaborator
	roleNone
)

// Middleware blocks requests the caller isn't permitted to make. It
// finds the matching rule, resolves the target namespace for namespaced
// permissions, and checks the caller's permission set. When that check
// fails on a namespaced permission and a concrete GameServer is
// extractable from the path, it fetches the server and allows the
// request if the caller is owner or collaborator. Owner-only operations
// (verb :transfer, :collaborators, :wipe-data, or DELETE on bare /servers/{name})
// are denied to collaborators. Invalid paths (trailing segments after a verb,
// e.g. /servers/a:transfer/extra) fail closed — the fallback does not apply.
// No matching rule means deny (fail-closed).
func Middleware(fetch ServerFetcher) func(http.Handler) http.Handler {
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
			if !allow(u, req.Method, req.URL.Path, ns) {
				// Try owner/collaborator fallback for namespaced server permissions.
				if Namespaced(r.perm) && fetch != nil &&
					(r.perm == "servers:read" || r.perm == "servers:write" || r.perm == "servers:console") {
					name, verb, ok := parseServerPath(req.URL.Path)
					if ok {
						obj, err := fetch.GetServer(req.Context(), ns, name)
						if err == nil && obj != nil {
							role := ownershipRole(obj, u.ID)
							if role != roleNone {
								// Owner-only operations: :transfer, :collaborators, :wipe-data, or DELETE with no verb.
								isOwnerOnly := verb == "transfer" || verb == "collaborators" || verb == "wipe-data" ||
									(req.Method == "DELETE" && verb == "")

								// Grant to owner, or to collaborator if not owner-only.
								if role == roleOwner || !isOwnerOnly {
									next.ServeHTTP(w, req)
									return
								}
							}
						}
					}
				}
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
	// Own profile: every authenticated user reads /users/me and their own
	// servers (/users/me/servers). The rest of /users is gated; must
	// precede the segment-wide users rules.
	{method: "GET", segment: "users", suffix: "/users/me", perm: ""},
	{method: "GET", segment: "users", suffix: "/users/me/servers", perm: ""},
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
	// Notification sink test-sends pair with editing the notifications
	// config section, so they share its manage permission.
	{segment: "admin", prefix: "/admin/notifications", perm: "config:manage"},
	// Identity-provider secrets pair with editing the auth config section.
	{segment: "admin", prefix: "/admin/auth", perm: "config:manage"},
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

// serverNameFromPath extracts the GameServer name from a path like
// /servers/{name}, /servers/{name}:verb, /servers/{name}/..., or
// /ws/servers/{name}/... Empty name ⇒ false (list endpoints have no name).
func serverNameFromPath(path string) (string, bool) {
	name, _, ok := parseServerPath(path)
	return name, ok
}

// parseServerPath extracts the server name and verb from a path like
// /servers/{name}, /servers/{name}:verb, /servers/{name}/..., or
// /ws/servers/{name}/...
// Returns (name, verb, ok) where verb is "" if no verb is present.
// Returns ok=false if the path has unexpected trailing segments after a verb
// (e.g., /servers/a:transfer/extra is invalid).
func parseServerPath(path string) (string, string, bool) {
	// Normalize to remove leading /
	trimmed := strings.TrimPrefix(path, "/")
	// Handle /ws/servers/... → servers/...
	if strings.HasPrefix(trimmed, "ws/") {
		trimmed = strings.TrimPrefix(trimmed, "ws/")
	}
	// Must start with servers/
	if !strings.HasPrefix(trimmed, "servers/") {
		return "", "", false
	}
	rest := strings.TrimPrefix(trimmed, "servers/")
	// Empty rest means /servers (list endpoint)
	if rest == "" {
		return "", "", false
	}

	// Find the end of the server name (before / or :)
	nameEndIdx := strings.IndexAny(rest, "/:")
	if nameEndIdx < 0 {
		// No / or :, the rest is the name
		return rest, "", true
	}

	name := rest[:nameEndIdx]
	if name == "" {
		return "", "", false
	}

	// Check what follows the name
	if rest[nameEndIdx] == ':' {
		// Extract the verb (everything between : and the next /)
		verbStart := nameEndIdx + 1
		verbEndIdx := strings.IndexByte(rest[verbStart:], '/')
		if verbEndIdx < 0 {
			// No trailing /, the rest is the verb
			verb := rest[verbStart:]
			if verb == "" {
				return "", "", false
			}
			return name, verb, true
		}
		// Trailing / after verb means trailing segments after verb (invalid, fail closed)
		return "", "", false
	}
	// rest[nameEndIdx] == '/', trailing segments without verb (OK, like /servers/a/files)
	return name, "", true
}

// ownershipRole returns the ownership role of a user in the server:
// owner (annotation gameplane.local/owner-id matches), collaborator
// (user ID in comma-separated gameplane.local/collaborators), or none.
func ownershipRole(obj *unstructured.Unstructured, userID int64) int {
	ann := obj.GetAnnotations()
	if ann == nil {
		return roleNone
	}
	// Check owner
	ownerIDStr := ann["gameplane.local/owner-id"]
	if ownerID, ok := parseUserID(ownerIDStr); ok && ownerID == userID {
		return roleOwner
	}
	// Check collaborators (comma-separated IDs)
	collabsStr := ann["gameplane.local/collaborators"]
	if collabsStr != "" {
		for _, id := range strings.Split(collabsStr, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if collabID, ok := parseUserID(id); ok && collabID == userID {
				return roleCollaborator
			}
		}
	}
	return roleNone
}

// parseUserID parses a string as int64 for ownership checks.
func parseUserID(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
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
