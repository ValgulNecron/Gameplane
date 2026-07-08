package auth

import (
	"context"
	"fmt"
)

// LoadPerms resolves the user's effective permission set from their role
// bindings, three-level map: cluster → namespace → permission set.
// It joins each binding's role to its permission rows, so a permission
// edit takes effect on the user's next request without re-issuing their
// session.
func (s *SessionStore) LoadPerms(ctx context.Context, userID int64) (map[string]map[string]map[string]struct{}, error) {
	rows, err := s.db.DB.QueryContext(ctx, `
		SELECT b.cluster, b.namespace, rp.permission
		FROM user_role_bindings b
		JOIN role_permissions rp ON rp.role_name = b.role_name
		WHERE b.user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("load permissions for user %d: %w", userID, err)
	}
	defer rows.Close()

	perms := map[string]map[string]map[string]struct{}{}
	for rows.Next() {
		var cluster, ns, perm string
		if err := rows.Scan(&cluster, &ns, &perm); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		if perms[cluster] == nil {
			perms[cluster] = map[string]map[string]struct{}{}
		}
		if perms[cluster][ns] == nil {
			perms[cluster][ns] = map[string]struct{}{}
		}
		perms[cluster][ns][perm] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permissions: %w", err)
	}
	return perms, nil
}

// Can reports whether the user holds perm. namespaced indicates whether the
// permission is scoped to a namespace+cluster (servers, backups, …) or is
// cluster-scoped control-plane state (users, roles, config, …).
//
//   - Cluster-scoped perms are NOT partitioned per target cluster: a
//     cluster-wide (namespace "*") binding in ANY cluster grants them. This is
//     why a backfilled admin (bound only on "local") keeps users:manage after
//     a second cluster is registered.
//   - Namespaced perms are gated by the request's target cluster (or the "*"
//     wildcard cluster): a cluster-wide binding on that cluster, or a binding
//     in the exact (cluster, namespace), grants them. A binding on one cluster
//     never confers the permission on another — that prevents cross-cluster
//     privilege escalation.
//   - The "*" permission wildcard (the built-in admin role) matches any perm
//     but is still subject to the same cluster gating for namespaced perms.
func (u *User) Can(perm string, namespaced bool, cluster, ns string) bool {
	if u == nil {
		return false
	}
	// cwHolds: does the user hold perm cluster-wide (namespace "*") on cluster ck?
	cwHolds := func(ck string) bool {
		return permSetHas(u.Perms[ck]["*"], "*") || permSetHas(u.Perms[ck]["*"], perm)
	}
	if !namespaced {
		// Control-plane perm: any cluster's cluster-wide binding grants it.
		for ck := range u.Perms {
			if cwHolds(ck) {
				return true
			}
		}
		return false
	}
	// Namespaced perm: gated by the target cluster or the "*" wildcard cluster.
	for _, ck := range []string{cluster, "*"} {
		if cwHolds(ck) {
			return true
		}
		if ns != "" && ns != "*" {
			if permSetHas(u.Perms[ck][ns], "*") || permSetHas(u.Perms[ck][ns], perm) {
				return true
			}
		}
	}
	return false
}

// permSetHas is nil-safe: indexing a nil map yields ok=false.
func permSetHas(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}
