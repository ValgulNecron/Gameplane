//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAPI_RBAC_ViewerCannotMutate_Matrix asserts the full read/write
// matrix the rbac.Middleware enforces for a viewer-role user. The sister
// test TestAPI_RBAC_ViewerCannotMutate already covers POST /servers/*:start;
// this one walks every protected segment so a regression that flips
// any single rule shows up in CI.
func TestAPI_RBAC_ViewerCannotMutate_Matrix(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	viewerName, viewerPW, viewerID := envInstance.CreateUser(t, admin, "viewer", "e2e-rbac-vmatrix")
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/users/" + viewerID)
	})

	viewer := envInstance.APIClient(t, viewerName, viewerPW)
	defer viewer.Close()

	type call struct {
		name, method, path string
		body               any
		want               int
	}
	cases := []call{
		// Admin-only segments — viewer must be 403.
		{"POST /users", http.MethodPost, "/users", map[string]string{
			"username": "rbac-should-not-create",
			"password": "irrelevant-pw-1234567",
			"role":     "viewer",
		}, http.StatusForbidden},
		{"DELETE /users/{viewerSelf}", http.MethodDelete, "/users/" + viewerID, nil, http.StatusForbidden},
		{"GET /admin/audit", http.MethodGet, "/admin/audit?limit=10", nil, http.StatusForbidden},
		{"POST /backup-destinations", http.MethodPost, "/backup-destinations", map[string]any{
			"name":     "rbac-fake-dest",
			"url":      "rest:http://example.invalid/repo",
			"password": "irrelevant",
		}, http.StatusForbidden},
		{"POST /modules", http.MethodPost, "/modules", map[string]any{
			"name":   "rbac-fake-mod",
			"source": map[string]any{"name": "rbac-fake-source"},
		}, http.StatusForbidden},

		// Operator-only segments — viewer must be 403.
		{"POST /servers/{name}:start", http.MethodPost, "/servers/no-such-server:start", nil, http.StatusForbidden},
		{"POST /servers", http.MethodPost, "/servers", map[string]any{
			"apiVersion": "gameplane.local/v1alpha1",
			"kind":       "GameServer",
			"metadata":   map[string]any{"name": "rbac-fake", "namespace": "gameplane-games"},
			"spec":       map[string]any{"templateRef": map[string]any{"name": "rbac-fake-tmpl"}},
		}, http.StatusForbidden},
		{"POST /backups", http.MethodPost, "/backups", map[string]any{
			"apiVersion": "gameplane.local/v1alpha1",
			"kind":       "Backup",
			"metadata":   map[string]any{"name": "rbac-fake", "namespace": "gameplane-games"},
			"spec":       map[string]any{},
		}, http.StatusForbidden},
		{"POST /schedules", http.MethodPost, "/schedules", map[string]any{
			"apiVersion": "gameplane.local/v1alpha1",
			"kind":       "BackupSchedule",
			"metadata":   map[string]any{"name": "rbac-fake", "namespace": "gameplane-games"},
			"spec":       map[string]any{},
		}, http.StatusForbidden},
		{"DELETE /servers/no-such", http.MethodDelete, "/servers/no-such", nil, http.StatusForbidden},

		// Reads — viewer is allowed (the catch-all viewer+ rule).
		{"GET /servers", http.MethodGet, "/servers", nil, http.StatusOK},
		{"GET /modules/catalog", http.MethodGet, "/modules/catalog", nil, http.StatusOK},
		{"GET /backup-destinations", http.MethodGet, "/backup-destinations", nil, http.StatusOK},
	}

	for _, tc := range cases {
		resp, body, err := viewer.Do(tc.method, tc.path, tc.body)
		if err != nil {
			t.Errorf("%s: do: %v", tc.name, err)
			continue
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status=%d want=%d body=%s", tc.name, resp.StatusCode, tc.want, string(body))
		}
	}
}

// TestAPI_RBAC_OperatorCanWriteServers_NotUsers proves the operator
// role can write GameServer-shaped resources but is still locked out of
// admin-only segments (users, audit, destinations, modules write paths).
func TestAPI_RBAC_OperatorCanWriteServers_NotUsers(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	opName, opPW, opID := envInstance.CreateUser(t, admin, "operator", "e2e-rbac-op")
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/users/" + opID)
	})

	op := envInstance.APIClient(t, opName, opPW)
	defer op.Close()

	// Set up a real GameTemplate so the operator's POST /servers
	// actually exercises the create path (an invalid spec would 400
	// before RBAC is even consulted in some shapes; we want to confirm
	// the create succeeds for an operator-role caller).
	const tmplName = "e2e-rbac-op-tmpl"
	applyBusyboxTemplate(t, tmplName)

	const ns = "gameplane-games"
	gsName := fmt.Sprintf("e2e-rbac-op-gs-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	resp, body, err := op.Post("/servers", map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec":       map[string]any{"templateRef": map[string]any{"name": tmplName}},
	})
	if err != nil {
		t.Fatalf("operator POST /servers: %v", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("operator POST /servers: status=%d body=%s", resp.StatusCode, string(body))
	}

	type call struct {
		name, method, path string
		body               any
		want               int
	}
	denied := []call{
		{"POST /users", http.MethodPost, "/users", map[string]string{
			"username": "rbac-op-should-not-create",
			"password": "irrelevant-pw-1234567",
			"role":     "viewer",
		}, http.StatusForbidden},
		{"DELETE /users/{operatorSelf}", http.MethodDelete, "/users/" + opID, nil, http.StatusForbidden},
		{"GET /admin/audit", http.MethodGet, "/admin/audit?limit=10", nil, http.StatusForbidden},
		{"POST /backup-destinations", http.MethodPost, "/backup-destinations", map[string]any{
			"name":     "rbac-op-fake-dest",
			"url":      "rest:http://example.invalid/repo",
			"password": "irrelevant",
		}, http.StatusForbidden},
		{"POST /modules", http.MethodPost, "/modules", map[string]any{
			"name":   "rbac-op-fake-mod",
			"source": map[string]any{"name": "rbac-op-fake-source"},
		}, http.StatusForbidden},
	}
	for _, tc := range denied {
		r, b, err := op.Do(tc.method, tc.path, tc.body)
		if err != nil {
			t.Errorf("%s: do: %v", tc.name, err)
			continue
		}
		if r.StatusCode != tc.want {
			t.Errorf("%s: status=%d want=%d body=%s", tc.name, r.StatusCode, tc.want, string(b))
		}
	}
}

// TestAPI_OperatorCannotInviteUsers: operator-role users can write
// GameServer-shaped resources, but creating users is admin-only. This
// case is already implicitly covered by the operator-RBAC matrix above,
// but is broken out as a separate test so a regression that flips the
// /users gate produces a clearly-named failure rather than getting
// buried inside a tabular row.
func TestAPI_OperatorCannotInviteUsers(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	opName, opPW, opID := envInstance.CreateUser(t, admin, "operator", "e2e-rbac-op-invite")
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/users/" + opID)
	})

	op := envInstance.APIClient(t, opName, opPW)
	defer op.Close()

	resp, body, err := op.Post("/users", map[string]string{
		"username": "rbac-op-should-not-invite",
		"password": "irrelevant-pw-1234567",
		"role":     "viewer",
	})
	if err != nil {
		t.Fatalf("operator POST /users: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("operator POST /users: status=%d want=403 body=%s",
			resp.StatusCode, string(body))
	}
}

// TestAPI_RBAC_AdminCanReachAll is a tiny safety-net asserting that
// every admin-gated segment returns 200 when called as admin. Catches
// regressions where a global middleware change accidentally bounces the
// admin role too — without this, an over-tight rule could go unnoticed
// behind the simpler "viewer is 403" tests.
func TestAPI_RBAC_AdminCanReachAll(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	checks := []struct {
		name, path string
	}{
		{"GET /users", "/users"},
		{"GET /admin/audit", "/admin/audit?limit=10"},
		{"GET /servers", "/servers"},
		{"GET /modules/catalog", "/modules/catalog"},
		{"GET /backup-destinations", "/backup-destinations"},
	}
	for _, c := range checks {
		resp, body, err := admin.Get(c.path)
		if err != nil {
			t.Errorf("%s: do: %v", c.name, err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status=%d body=%s", c.name, resp.StatusCode, string(body))
		}
	}
}
