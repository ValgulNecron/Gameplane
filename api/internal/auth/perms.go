package auth

import (
	"context"
	"fmt"
)

// LoadPerms resolves the user's effective permission set from their role
// bindings, keyed by namespace ("*" = cluster-wide). It joins each
// binding's role to its permission rows, so a permission edit takes
// effect on the user's next request without re-issuing their session.
func (s *SessionStore) LoadPerms(ctx context.Context, userID int64) (map[string]map[string]struct{}, error) {
	rows, err := s.db.DB.QueryContext(ctx, `
		SELECT b.namespace, rp.permission
		FROM user_role_bindings b
		JOIN role_permissions rp ON rp.role_name = b.role_name
		WHERE b.user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("load permissions for user %d: %w", userID, err)
	}
	defer rows.Close()

	perms := map[string]map[string]struct{}{}
	for rows.Next() {
		var ns, perm string
		if err := rows.Scan(&ns, &perm); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		if perms[ns] == nil {
			perms[ns] = map[string]struct{}{}
		}
		perms[ns][perm] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permissions: %w", err)
	}
	return perms, nil
}

// Can reports whether the user holds perm. namespaced indicates whether
// the permission is scoped to a namespace (servers, backups, …) or
// cluster-scoped (users, roles, config, …).
//
//   - A cluster-wide ("*") binding holding the "*" wildcard (the built-in
//     admin role) grants everything.
//   - A cluster-wide binding holding perm grants it anywhere.
//   - For a namespaced permission, a binding in the target namespace ns
//     (holding perm or the "*" wildcard) also grants it.
//   - A namespace-scoped binding NEVER confers a cluster-scoped
//     permission — that matches Kubernetes Role vs ClusterRole semantics.
func (u *User) Can(perm string, namespaced bool, ns string) bool {
	if u == nil {
		return false
	}
	if permSetHas(u.Perms["*"], "*") {
		return true
	}
	if permSetHas(u.Perms["*"], perm) {
		return true
	}
	if namespaced && ns != "" && ns != "*" {
		if permSetHas(u.Perms[ns], "*") || permSetHas(u.Perms[ns], perm) {
			return true
		}
	}
	return false
}

// permSetHas is nil-safe: indexing a nil map yields ok=false.
func permSetHas(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}
