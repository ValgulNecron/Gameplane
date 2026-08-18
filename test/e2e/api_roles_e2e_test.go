//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAPI_CustomRole_Lifecycle exercises creating a custom role, assigning
// it to a user, the in-use delete guard, and finally deleting the role
// once it's unused.
func TestAPI_CustomRole_Lifecycle(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	const roleName = "e2e-support"
	t.Cleanup(func() {
		r, _, _ := admin.Delete("/roles/" + roleName)
		if r != nil {
			r.Body.Close()
		}
	})

	// Create a read-only custom role.
	resp, body, err := admin.Post("/roles", map[string]any{
		"name":        roleName,
		"description": "Read-only helper",
		"permissions": []string{"servers:read", "backups:read"},
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create role status=%d body=%s", resp.StatusCode, string(body))
	}

	// It shows up in the listing.
	listResp, listBody, err := admin.Get("/roles")
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	if !strings.Contains(string(listBody), roleName) {
		t.Fatalf("custom role not in listing: %s", string(listBody))
	}

	// Reject the wildcard / unknown permissions on a custom role.
	r, _, _ := admin.Post("/roles", map[string]any{
		"name": "e2e-bad", "permissions": []string{"*"},
	})
	if r != nil {
		defer func() { _ = r.Body.Close() }()
	}
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("wildcard role: status=%d want 400", r.StatusCode)
	}

	// Assign the role to a user.
	supportName, supportPW, supportID := envInstance.CreateUser(t, admin, roleName, "e2e-support-user")
	supportCleanup := func() {
		r, _, _ := admin.Delete("/users/" + supportID)
		if r != nil {
			r.Body.Close()
		}
	}
	t.Cleanup(supportCleanup)

	support := envInstance.APIClient(t, supportName, supportPW)
	defer support.Close()

	// The read-only role can read servers but not write them.
	r, _, _ = support.Get("/servers")
	if r.StatusCode != http.StatusOK {
		t.Errorf("support GET /servers: status=%d want 200", r.StatusCode)
	}
	r.Body.Close()
	r, _, _ = support.Post("/servers", map[string]any{
		"apiVersion": "gameplane.local/v1alpha1", "kind": "GameServer",
		"metadata": map[string]any{"name": "e2e-support-nope", "namespace": "gameplane-games"},
		"spec":     map[string]any{"templateRef": map[string]any{"name": "nope"}},
	})
	if r.StatusCode != http.StatusForbidden {
		t.Errorf("support POST /servers: status=%d want 403", r.StatusCode)
	}
	r.Body.Close()

	// A role assigned to a user can't be deleted.
	r, _, _ = admin.Delete("/roles/" + roleName)
	if r.StatusCode != http.StatusConflict {
		t.Errorf("delete in-use role: status=%d want 409", r.StatusCode)
	}
	r.Body.Close()

	// Once the user is gone, the role deletes cleanly.
	r, _, _ = admin.Delete("/users/" + supportID)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete support user: status=%d", r.StatusCode)
	}
	r.Body.Close()
	r, _, _ = admin.Delete("/roles/" + roleName)
	if r.StatusCode != http.StatusNoContent {
		t.Errorf("delete unused role: status=%d want 204", r.StatusCode)
	}
	r.Body.Close()
}

// TestAPI_BuiltinRole_Immutable proves the seeded roles are protected: the
// admin role can't be edited and no built-in role can be deleted.
func TestAPI_BuiltinRole_Immutable(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	r, _, _ := admin.Patch("/roles/admin", map[string]any{
		"permissions": []string{"servers:read"},
	})
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("edit admin role: status=%d want 400", r.StatusCode)
	}
	r.Body.Close()
	for _, name := range []string{"admin", "operator", "viewer"} {
		r, _, _ := admin.Delete("/roles/" + name)
		if r.StatusCode != http.StatusBadRequest {
			t.Errorf("delete builtin %q: status=%d want 400", name, r.StatusCode)
		}
		r.Body.Close()
	}
}

// TestAPI_PerNamespaceBinding_GrantsScopedAccess proves that a per-namespace
// role binding authorizes namespaced writes in that namespace, while a
// namespace binding never confers cluster-scoped authority.
func TestAPI_PerNamespaceBinding_GrantsScopedAccess(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	const tmplName = "e2e-binding-tmpl"
	applyBusyboxTemplate(t, tmplName)

	viewerName, viewerPW, viewerID := envInstance.CreateUser(t, admin, "viewer", "e2e-binding")
	t.Cleanup(func() {
		r, _, _ := admin.Delete("/users/" + viewerID)
		if r != nil {
			r.Body.Close()
		}
	})

	// As a plain viewer, creating a server is forbidden.
	v1 := envInstance.APIClient(t, viewerName, viewerPW)
	gsSpec := func(name string) map[string]any {
		return map[string]any{
			"apiVersion": "gameplane.local/v1alpha1", "kind": "GameServer",
			"metadata": map[string]any{"name": name, "namespace": "gameplane-games"},
			"spec":     map[string]any{"templateRef": map[string]any{"name": tmplName}},
		}
	}
	r, _, _ := v1.Post("/servers", gsSpec("e2e-binding-denied"))
	if r.StatusCode != http.StatusForbidden {
		t.Errorf("viewer POST /servers: status=%d want 403", r.StatusCode)
	}
	r.Body.Close()
	v1.Close()

	// Grant operator in the default namespace only.
	r, b, _ := admin.Post("/users/"+viewerID+"/bindings", map[string]any{
		"roleName": "operator", "namespace": "gameplane-games",
	})
	if r.StatusCode != http.StatusCreated {
		t.Fatalf("add binding: status=%d body=%s", r.StatusCode, string(b))
	}
	r.Body.Close()

	// The binding cleared the user's sessions; re-authenticate.
	v2 := envInstance.APIClient(t, viewerName, viewerPW)
	defer v2.Close()

	gsName := fmt.Sprintf("e2e-binding-ok-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace("gameplane-games").
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})
	// Now the namespaced write is authorized (200/201, not 403).
	r, b, _ = v2.Post("/servers", gsSpec(gsName))
	if r.StatusCode == http.StatusForbidden {
		t.Errorf("scoped operator POST /servers: status=%d (forbidden) body=%s", r.StatusCode, string(b))
	}
	r.Body.Close()
	// But a cluster-scoped action stays denied — a namespace binding never
	// grants cluster authority.
	r, _, _ = v2.Post("/users", map[string]string{
		"username": "e2e-binding-should-not", "password": "irrelevant-pw-1234567", "role": "viewer",
	})
	if r.StatusCode != http.StatusForbidden {
		t.Errorf("scoped operator POST /users: status=%d want 403", r.StatusCode)
	}
	r.Body.Close()

	// Remove the binding again.
	r, _, _ = admin.Delete("/users/" + viewerID + "/bindings/operator/gameplane-games")
	if r.StatusCode != http.StatusNoContent {
		t.Errorf("remove binding: status=%d want 204", r.StatusCode)
	}
	r.Body.Close()
}
