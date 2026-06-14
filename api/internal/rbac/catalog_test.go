package rbac

import "testing"

func TestValidPermission(t *testing.T) {
	for _, k := range []string{"servers:read", "users:manage", "audit:read", "config:manage", "cluster:manage"} {
		if !ValidPermission(k) {
			t.Errorf("ValidPermission(%q) = false, want true", k)
		}
	}
	// "*" is reserved for the built-in admin role and must never be a
	// grantable catalog key; unknown keys are invalid too.
	for _, k := range []string{"*", "", "servers:bogus", "unknown:read"} {
		if ValidPermission(k) {
			t.Errorf("ValidPermission(%q) = true, want false", k)
		}
	}
}

func TestNamespaced(t *testing.T) {
	cases := map[string]bool{
		"servers:read":      true,
		"backups:restore":   true,
		"schedules:write":   true,
		"destinations:read": true,
		"templates:read":    false,
		"modules:manage":    false,
		"users:manage":      false,
		"cluster:read":      false,
		"audit:read":        false,
		// Unknown keys (and the reserved "*") default to cluster-scoped.
		"*":           false,
		"unknown:key": false,
	}
	for k, want := range cases {
		if got := Namespaced(k); got != want {
			t.Errorf("Namespaced(%q) = %v, want %v", k, got, want)
		}
	}
}

// TestCatalog_Wellformed guards the catalog's structural invariants:
// every group and permission is labelled and keys are unique.
func TestCatalog_Wellformed(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range Catalog {
		if g.Resource == "" || g.Label == "" {
			t.Errorf("group %+v missing resource or label", g)
		}
		for _, p := range g.Permissions {
			if p.Key == "" || p.Label == "" {
				t.Errorf("permission %+v missing key or label", p)
			}
			if seen[p.Key] {
				t.Errorf("duplicate catalog key %q", p.Key)
			}
			seen[p.Key] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("catalog is empty")
	}
}

// TestCatalog_CoversSeededPermissions enforces the contract in catalog.go:
// every permission seeded by migrations/003_roles.sql must be a real
// catalog key (the admin "*" wildcard is the sole intentional exception).
func TestCatalog_CoversSeededPermissions(t *testing.T) {
	store := migratedStore(t) // rbac_test.go
	rows, err := store.DB.Query(`SELECT DISTINCT permission FROM role_permissions`)
	if err != nil {
		t.Fatalf("query seeded permissions: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if p == "*" {
			continue // admin wildcard, deliberately absent from the catalog
		}
		if !ValidPermission(p) {
			t.Errorf("seeded permission %q is not present in the catalog", p)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
}
