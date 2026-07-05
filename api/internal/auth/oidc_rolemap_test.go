package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestExtractGroups(t *testing.T) {
	cases := []struct {
		name      string
		claims    map[string]any
		claimName string
		want      []string
	}{
		{"missing claim", map[string]any{"sub": "x"}, "groups", nil},
		{"nil claim", map[string]any{"groups": nil}, "groups", nil},
		{"string array", map[string]any{"groups": []any{"a", "b"}}, "groups", []string{"a", "b"}},
		{"skips non-strings", map[string]any{"groups": []any{"a", 42, true, "b"}}, "groups", []string{"a", "b"}},
		{"empty array", map[string]any{"groups": []any{}}, "groups", nil},
		{"single string", map[string]any{"groups": "admins"}, "groups", []string{"admins"}},
		{"non-string non-array claim", map[string]any{"groups": 7.0}, "groups", nil},
		{"custom claim name", map[string]any{"roles": []any{"ops"}}, "roles", []string{"ops"}},
		{"empty claimName defaults to groups", map[string]any{"groups": []any{"g"}}, "", []string{"g"}},
		{"custom name misses default claim", map[string]any{"groups": []any{"g"}}, "roles", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractGroups(tc.claims, tc.claimName)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestComputeRole(t *testing.T) {
	mappings := &RoleMappings{
		Admin:    []string{"gp-admins"},
		Operator: []string{"gp-ops", "gp-sre"},
		Viewer:   []string{"gp-view"},
	}
	cases := []struct {
		name   string
		groups []string
		pol    *ProviderPolicy
		role   string
		deny   bool
	}{
		{"nil policy", []string{"gp-admins"}, nil, "viewer", false},
		{"nil mappings", []string{"gp-admins"}, &ProviderPolicy{DefaultRole: "deny"}, "viewer", false},
		{"admin match", []string{"gp-admins"}, &ProviderPolicy{RoleMappings: mappings}, "admin", false},
		{"operator match", []string{"gp-sre"}, &ProviderPolicy{RoleMappings: mappings}, "operator", false},
		{"viewer match", []string{"gp-view"}, &ProviderPolicy{RoleMappings: mappings}, "viewer", false},
		{
			"most privileged wins",
			[]string{"gp-view", "gp-ops", "gp-admins"},
			&ProviderPolicy{RoleMappings: mappings},
			"admin", false,
		},
		{
			"operator beats viewer",
			[]string{"gp-view", "gp-ops"},
			&ProviderPolicy{RoleMappings: mappings},
			"operator", false,
		},
		{"no match empty default", []string{"other"}, &ProviderPolicy{RoleMappings: mappings}, "viewer", false},
		{
			"no match viewer default",
			[]string{"other"},
			&ProviderPolicy{RoleMappings: mappings, DefaultRole: "viewer"},
			"viewer", false,
		},
		{
			"no match operator default",
			[]string{"other"},
			&ProviderPolicy{RoleMappings: mappings, DefaultRole: "operator"},
			"operator", false,
		},
		{
			"no match admin default",
			[]string{"other"},
			&ProviderPolicy{RoleMappings: mappings, DefaultRole: "admin"},
			"admin", false,
		},
		{
			"no match deny",
			[]string{"other"},
			&ProviderPolicy{RoleMappings: mappings, DefaultRole: "deny"},
			"", true,
		},
		{
			"no groups at all deny",
			nil,
			&ProviderPolicy{RoleMappings: mappings, DefaultRole: "deny"},
			"", true,
		},
		{
			"match beats deny default",
			[]string{"gp-ops"},
			&ProviderPolicy{RoleMappings: mappings, DefaultRole: "deny"},
			"operator", false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, deny := computeRole(tc.groups, tc.pol)
			if role != tc.role || deny != tc.deny {
				t.Fatalf("got (%q, %v) want (%q, %v)", role, deny, tc.role, tc.deny)
			}
		})
	}
}

// callbackViaIDP drives one full HandleCallback round against the fake
// IdP, returning the recorder. The IdP must have nonce set before calling.
func callbackViaIDP(t *testing.T, o *OIDC, sessions *SessionStore, nonce string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc&code=auth-code-1", nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "abc"})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: nonce})
	o.HandleCallback(sessions)(rr, req)
	return rr
}

// TestHandleCallback_RoleMappingFirstLogin — first login through a
// provider with mappings creates the user with the mapped role and a
// matching '*' role binding.
func TestHandleCallback_RoleMappingFirstLogin(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.nonce = "nonce-map"
	idp.groups = []string{"gp-admins"}

	o, err := NewOIDCWithPolicy(context.Background(), idp.issuer(), "client-1", "secret",
		"https://app/cb", &ProviderPolicy{
			RoleMappings: &RoleMappings{Admin: []string{"gp-admins"}},
		})
	if err != nil {
		t.Fatalf("NewOIDCWithPolicy: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)

	rr := callbackViaIDP(t, o, NewSessionStore(store), "nonce-map")
	if rr.Code != http.StatusFound {
		t.Fatalf("code=%d body=%q", rr.Code, rr.Body)
	}
	var role string
	if err := store.DB.QueryRow(
		`SELECT role FROM users WHERE email = ?`, idp.email).Scan(&role); err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if role != "admin" {
		t.Fatalf("role=%q want admin", role)
	}
	var bindingRole string
	if err := store.DB.QueryRow(`
		SELECT b.role_name FROM user_role_bindings b
		JOIN users u ON u.id = b.user_id
		WHERE u.email = ? AND b.namespace = '*'`, idp.email).Scan(&bindingRole); err != nil {
		t.Fatalf("binding not created: %v", err)
	}
	if bindingRole != "admin" {
		t.Fatalf("binding role=%q want admin", bindingRole)
	}
}

// TestHandleCallback_RoleMappingResync — the user's IdP groups change
// between logins; the second login demotes both users.role and the '*'
// binding. The IdP is authoritative when mappings are configured.
func TestHandleCallback_RoleMappingResync(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.groups = []string{"gp-admins"}

	o, err := NewOIDCWithPolicy(context.Background(), idp.issuer(), "client-1", "secret",
		"https://app/cb", &ProviderPolicy{
			RoleMappings: &RoleMappings{
				Admin:    []string{"gp-admins"},
				Operator: []string{"gp-ops"},
			},
		})
	if err != nil {
		t.Fatalf("NewOIDCWithPolicy: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)
	sessions := NewSessionStore(store)

	idp.nonce = "nonce-1"
	if rr := callbackViaIDP(t, o, sessions, "nonce-1"); rr.Code != http.StatusFound {
		t.Fatalf("first login code=%d body=%q", rr.Code, rr.Body)
	}

	// Admin membership revoked at the IdP; only the operator group is left.
	idp.groups = []string{"gp-ops"}
	idp.nonce = "nonce-2"
	if rr := callbackViaIDP(t, o, sessions, "nonce-2"); rr.Code != http.StatusFound {
		t.Fatalf("second login code=%d body=%q", rr.Code, rr.Body)
	}

	var role, bindingRole string
	if err := store.DB.QueryRow(`SELECT role FROM users WHERE email = ?`, idp.email).Scan(&role); err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := store.DB.QueryRow(`
		SELECT b.role_name FROM user_role_bindings b
		JOIN users u ON u.id = b.user_id
		WHERE u.email = ? AND b.namespace = '*'`, idp.email).Scan(&bindingRole); err != nil {
		t.Fatalf("binding: %v", err)
	}
	if role != "operator" || bindingRole != "operator" {
		t.Fatalf("role=%q binding=%q, want operator/operator", role, bindingRole)
	}
	// Exactly one cluster-wide binding — resync must repoint, not stack.
	var n int
	if err := store.DB.QueryRow(`
		SELECT COUNT(*) FROM user_role_bindings b
		JOIN users u ON u.id = b.user_id
		WHERE u.email = ? AND b.namespace = '*'`, idp.email).Scan(&n); err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if n != 1 {
		t.Fatalf("cluster bindings = %d, want 1", n)
	}
}

// TestHandleCallback_NoMappingKeepsManualPromotion — a provider without
// role mappings must never touch a stored role: a user promoted to admin
// by hand stays admin across re-logins.
func TestHandleCallback_NoMappingKeepsManualPromotion(t *testing.T) {
	idp := newFakeIDP(t, "client-1")

	o, err := NewOIDC(context.Background(), idp.issuer(), "client-1", "secret", "https://app/cb")
	if err != nil {
		t.Fatalf("NewOIDC: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)
	sessions := NewSessionStore(store)

	idp.nonce = "nonce-1"
	if rr := callbackViaIDP(t, o, sessions, "nonce-1"); rr.Code != http.StatusFound {
		t.Fatalf("first login code=%d body=%q", rr.Code, rr.Body)
	}

	// Manual promotion, as the users handler would do it.
	if _, err := store.DB.Exec(`UPDATE users SET role = 'admin' WHERE email = ?`, idp.email); err != nil {
		t.Fatalf("promote: %v", err)
	}

	idp.nonce = "nonce-2"
	if rr := callbackViaIDP(t, o, sessions, "nonce-2"); rr.Code != http.StatusFound {
		t.Fatalf("second login code=%d body=%q", rr.Code, rr.Body)
	}
	var role string
	if err := store.DB.QueryRow(`SELECT role FROM users WHERE email = ?`, idp.email).Scan(&role); err != nil {
		t.Fatalf("user: %v", err)
	}
	if role != "admin" {
		t.Fatalf("role=%q, manual promotion must survive re-login", role)
	}
}

// TestHandleCallback_ResyncKeepsLastUserManager — the sole admin's IdP
// groups now map only to viewer; the resync must SKIP the demotion (the
// install would otherwise lose its last user-manager) while the login
// itself still succeeds.
func TestHandleCallback_ResyncKeepsLastUserManager(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.groups = []string{"gp-admins"}

	o, err := NewOIDCWithPolicy(context.Background(), idp.issuer(), "client-1", "secret",
		"https://app/cb", &ProviderPolicy{
			RoleMappings: &RoleMappings{
				Admin:  []string{"gp-admins"},
				Viewer: []string{"gp-view"},
			},
		})
	if err != nil {
		t.Fatalf("NewOIDCWithPolicy: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)
	sessions := NewSessionStore(store)

	// First login makes the user the install's only admin.
	idp.nonce = "nonce-1"
	if rr := callbackViaIDP(t, o, sessions, "nonce-1"); rr.Code != http.StatusFound {
		t.Fatalf("first login code=%d body=%q", rr.Code, rr.Body)
	}

	// Admin group revoked at the IdP; only a viewer-mapped group remains.
	idp.groups = []string{"gp-view"}
	idp.nonce = "nonce-2"
	if rr := callbackViaIDP(t, o, sessions, "nonce-2"); rr.Code != http.StatusFound {
		t.Fatalf("second login must still succeed: code=%d body=%q", rr.Code, rr.Body)
	}

	var role, bindingRole string
	if err := store.DB.QueryRow(`SELECT role FROM users WHERE email = ?`, idp.email).Scan(&role); err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := store.DB.QueryRow(`
		SELECT b.role_name FROM user_role_bindings b
		JOIN users u ON u.id = b.user_id
		WHERE u.email = ? AND b.namespace = '*'`, idp.email).Scan(&bindingRole); err != nil {
		t.Fatalf("binding: %v", err)
	}
	if role != "admin" || bindingRole != "admin" {
		t.Fatalf("role=%q binding=%q, last user-manager must keep admin", role, bindingRole)
	}
}

// TestHandleCallback_ResyncDemotesWhenAnotherManagerExists — the inverse
// of the lockout guard: with a second admin present, the same demotion
// login applies as normal.
func TestHandleCallback_ResyncDemotesWhenAnotherManagerExists(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.groups = []string{"gp-admins"}

	o, err := NewOIDCWithPolicy(context.Background(), idp.issuer(), "client-1", "secret",
		"https://app/cb", &ProviderPolicy{
			RoleMappings: &RoleMappings{
				Admin:  []string{"gp-admins"},
				Viewer: []string{"gp-view"},
			},
		})
	if err != nil {
		t.Fatalf("NewOIDCWithPolicy: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)
	sessions := NewSessionStore(store)

	idp.nonce = "nonce-1"
	if rr := callbackViaIDP(t, o, sessions, "nonce-1"); rr.Code != http.StatusFound {
		t.Fatalf("first login code=%d body=%q", rr.Code, rr.Body)
	}

	// A second user-manager exists, so the guard must not fire.
	if _, err := store.DB.Exec(
		`INSERT INTO users(username, display_name, email, role) VALUES ('root2', 'Root Two', 'root2@x', 'admin')`,
	); err != nil {
		t.Fatalf("seed second admin: %v", err)
	}

	idp.groups = []string{"gp-view"}
	idp.nonce = "nonce-2"
	if rr := callbackViaIDP(t, o, sessions, "nonce-2"); rr.Code != http.StatusFound {
		t.Fatalf("second login code=%d body=%q", rr.Code, rr.Body)
	}

	var role, bindingRole string
	if err := store.DB.QueryRow(`SELECT role FROM users WHERE email = ?`, idp.email).Scan(&role); err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := store.DB.QueryRow(`
		SELECT b.role_name FROM user_role_bindings b
		JOIN users u ON u.id = b.user_id
		WHERE u.email = ? AND b.namespace = '*'`, idp.email).Scan(&bindingRole); err != nil {
		t.Fatalf("binding: %v", err)
	}
	if role != "viewer" || bindingRole != "viewer" {
		t.Fatalf("role=%q binding=%q, want viewer/viewer (another manager exists)", role, bindingRole)
	}
}

// TestHandleCallback_DefaultRoleDeny — defaultRole=deny plus no matching
// group refuses the login with 403 and creates no user row.
func TestHandleCallback_DefaultRoleDeny(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	idp.nonce = "nonce-deny"
	idp.groups = []string{"unrelated-group"}

	o, err := NewOIDCWithPolicy(context.Background(), idp.issuer(), "client-1", "secret",
		"https://app/cb", &ProviderPolicy{
			RoleMappings: &RoleMappings{Admin: []string{"gp-admins"}},
			DefaultRole:  "deny",
		})
	if err != nil {
		t.Fatalf("NewOIDCWithPolicy: %v", err)
	}
	store := newAuthDB(t)
	o.AttachStore(store)

	rr := callbackViaIDP(t, o, NewSessionStore(store), "nonce-deny")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%q, want 403", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "login not permitted") {
		t.Fatalf("body=%q", rr.Body.String())
	}
	var n int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if n != 0 {
		t.Fatalf("users = %d, deny must not create a row", n)
	}
}

// TestHandleStart_ExtraScopesInAuthCodeURL — the start redirect must
// request the policy's extra scopes exactly once each, after the base
// set, without duplicating a base scope listed again in the policy.
func TestHandleStart_ExtraScopesInAuthCodeURL(t *testing.T) {
	idp := newFakeIDP(t, "client-1")
	o, err := NewOIDCWithPolicy(context.Background(), idp.issuer(), "client-1", "secret",
		"https://app/cb", &ProviderPolicy{Scopes: []string{"groups", "email"}})
	if err != nil {
		t.Fatalf("NewOIDCWithPolicy: %v", err)
	}
	rr := httptest.NewRecorder()
	o.HandleStart()(rr, httptest.NewRequest("GET", "/auth/oidc/start", nil))
	if rr.Code != http.StatusFound {
		t.Fatalf("code=%d", rr.Code)
	}
	loc, err := url.Parse(rr.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	scope := loc.Query().Get("scope")
	if scope != "openid profile email groups" {
		t.Fatalf("scope=%q want %q (base set + deduped extras, order preserved)",
			scope, "openid profile email groups")
	}
}
