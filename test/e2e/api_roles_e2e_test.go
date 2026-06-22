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
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	const roleName = "e2e-support"
	t.Cleanup(func() { _, _, _ = admin.Delete("/roles/" + roleName) })

	// Create a read-only custom role.
	resp, body, err := admin.Post("/roles", map[string]any{
		"name":        roleName,
		"description": "Read-only helper",
		"permissions": []string{"servers:read", "backups:read"},
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create role status=%d body=%s", resp.StatusCode, string(body))
	}

	// It shows up in the listing.
	_, listBody, err := admin.Get("/roles")
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if !strings.Contains(string(listBody), roleName) {
		t.Fatalf("custom role not in listing: %s", string(listBody))
	}

	// Reject the wildcard / unknown permissions on a custom role.
	if r, _, _ := admin.Post("/roles", map[string]any{
		"name": "e2e-bad", "permissions": []string{"*"},
	}); r.StatusCode != http.StatusBadRequest {
		t.Errorf("wildcard role: status=%d want 400", r.StatusCode)
	}

	// Assign the role to a user.
	supportName, supportPW, supportID := envInstance.CreateUser(t, admin, roleName, "e2e-support-user")
	supportCleanup := func() { _, _, _ = admin.Delete("/users/" + supportID) }
	t.Cleanup(supportCleanup)

	support := envInstance.APIClient(t, supportName, supportPW)
	defer support.Close()

	// The read-only role can read servers but not write them.
	if r, _, _ := support.Get("/servers"); r.StatusCode != http.StatusOK {
		t.Errorf("support GET /servers: status=%d want 200", r.StatusCode)
	}
	if r, _, _ := support.Post("/servers", map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1", "kind": "GameServer",
		"metadata": map[string]any{"name": "e2e-support-nope", "namespace": "gameplane-games"},
		"spec":     map[string]any{"templateRef": map[string]any{"name": "nope"}},
	}); r.StatusCode != http.StatusForbidden {
		t.Errorf("support POST /servers: status=%d want 403", r.StatusCode)
	}

	// A role assigned to a user can't be deleted.
	if r, _, _ := admin.Delete("/roles/" + roleName); r.StatusCode != http.StatusConflict {
		t.Errorf("delete in-use role: status=%d want 409", r.StatusCode)
	}

	// Once the user is gone, the role deletes cleanly.
	if r, _, _ := admin.Delete("/users/" + supportID); r.StatusCode != http.StatusNoContent {
		t.Fatalf("delete support user: status=%d", r.StatusCode)
	}
	if r, _, _ := admin.Delete("/roles/" + roleName); r.StatusCode != http.StatusNoContent {
		t.Errorf("delete unused role: status=%d want 204", r.StatusCode)
	}
}

// TestAPI_BuiltinRole_Immutable proves the seeded roles are protected: the
// admin role can't be edited and no built-in role can be deleted.
func TestAPI_BuiltinRole_Immutable(t *testing.T) {
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	if r, _, _ := admin.Patch("/roles/admin", map[string]any{
		"permissions": []string{"servers:read"},
	}); r.StatusCode != http.StatusBadRequest {
		t.Errorf("edit admin role: status=%d want 400", r.StatusCode)
	}
	for _, name := range []string{"admin", "operator", "viewer"} {
		if r, _, _ := admin.Delete("/roles/" + name); r.StatusCode != http.StatusBadRequest {
			t.Errorf("delete builtin %q: status=%d want 400", name, r.StatusCode)
		}
	}
}

// TestAPI_PerNamespaceBinding_GrantsScopedAccess proves that a per-namespace
// role binding authorizes namespaced writes in that namespace, while a
// namespace binding never confers cluster-scoped authority.
func TestAPI_PerNamespaceBinding_GrantsScopedAccess(t *testing.T) {
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	const tmplName = "e2e-binding-tmpl"
	applyBusyboxTemplate(t, tmplName)

	viewerName, viewerPW, viewerID := envInstance.CreateUser(t, admin, "viewer", "e2e-binding")
	t.Cleanup(func() { _, _, _ = admin.Delete("/users/" + viewerID) })

	// As a plain viewer, creating a server is forbidden.
	v1 := envInstance.APIClient(t, viewerName, viewerPW)
	gsSpec := func(name string) map[string]any {
		return map[string]any{
			"apiVersion": "gameplane.gg/v1alpha1", "kind": "GameServer",
			"metadata": map[string]any{"name": name, "namespace": "gameplane-games"},
			"spec":     map[string]any{"templateRef": map[string]any{"name": tmplName}},
		}
	}
	if r, _, _ := v1.Post("/servers", gsSpec("e2e-binding-denied")); r.StatusCode != http.StatusForbidden {
		t.Errorf("viewer POST /servers: status=%d want 403", r.StatusCode)
	}
	v1.Close()

	// Grant operator in the default namespace only.
	if r, b, _ := admin.Post("/users/"+viewerID+"/bindings", map[string]any{
		"roleName": "operator", "namespace": "gameplane-games",
	}); r.StatusCode != http.StatusCreated {
		t.Fatalf("add binding: status=%d body=%s", r.StatusCode, string(b))
	}

	// The binding cleared the user's sessions; re-authenticate.
	v2 := envInstance.APIClient(t, viewerName, viewerPW)
	defer v2.Close()

	gsName := fmt.Sprintf("e2e-binding-ok-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace("gameplane-games").
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})
	// Now the namespaced write is authorized (200/201, not 403).
	if r, b, _ := v2.Post("/servers", gsSpec(gsName)); r.StatusCode == http.StatusForbidden {
		t.Errorf("scoped operator POST /servers: status=%d (forbidden) body=%s", r.StatusCode, string(b))
	}
	// But a cluster-scoped action stays denied — a namespace binding never
	// grants cluster authority.
	if r, _, _ := v2.Post("/users", map[string]string{
		"username": "e2e-binding-should-not", "password": "irrelevant-pw-1234567", "role": "viewer",
	}); r.StatusCode != http.StatusForbidden {
		t.Errorf("scoped operator POST /users: status=%d want 403", r.StatusCode)
	}

	// Remove the binding again.
	if r, _, _ := admin.Delete("/users/" + viewerID + "/bindings/operator/gameplane-games"); r.StatusCode != http.StatusNoContent {
		t.Errorf("remove binding: status=%d want 204", r.StatusCode)
	}
}
