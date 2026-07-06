//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAPI_OwnerCollaboratorAccess exercises per-GameServer ownership and
// collaborator access control. The feature grants access to a specific
// GameServer based on owner and collaborator annotations, additively on top
// of namespace role bindings.
//
// Tested flows:
//   - Admin creates a GameServer and transfers it to owner-user.
//   - Owner can GET the server despite zero namespace role bindings.
//   - Owner can add collaborators via PUT /servers/{name}:collaborators with
//     the new username (server unions/dedupes).
//   - Collaborator can GET the server and run lifecycle operations (:start) with
//     no namespace role bindings.
//   - Collaborator cannot :transfer, :collaborators, or DELETE (owner-only).
//   - A non-related user cannot GET the server.
//
// Login budget: 3 non-admin logins (owner-user, collab-user, plus the admin
// setup login — well within the per-bucket limit).
func TestAPI_OwnerCollaboratorAccess(t *testing.T) {
	t.Parallel()

	const ns = "gameplane-games"
	const tmpl = "e2e-owner-collab-tmpl"
	gsName := fmt.Sprintf("e2e-owner-collab-gs-%d", time.Now().UnixNano())

	// Bootstrap and create admin client.
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	// Create a zero-permission custom role for the test users. POST /users
	// always mirrors the primary role into a cluster-wide binding, so a user
	// created with the viewer role could read every server through that
	// binding — the pre-add 403 subtest would fail, and the owner/collab
	// GET subtests would pass via the binding instead of proving the
	// ownership fallback. A no-permission role keeps both users' access
	// coming exclusively from the owner/collaborator annotations.
	// Registered for cleanup before the users so it is deleted after them
	// (role deletion requires no remaining bindings).
	noPermRole := fmt.Sprintf("e2e-owner-collab-noperm-%d", time.Now().UnixNano())
	t.Cleanup(func() { _, _, _ = admin.Delete("/roles/" + noPermRole) })
	roleResp, roleBody, err := admin.Post("/roles", map[string]any{
		"name":        noPermRole,
		"description": "e2e: zero permissions (ownership fallback tests)",
		"permissions": []string{},
	})
	if err != nil {
		t.Fatalf("create no-perm role: %v", err)
	}
	if roleResp.StatusCode != http.StatusCreated {
		t.Fatalf("create no-perm role: status=%d body=%s", roleResp.StatusCode, string(roleBody))
	}

	// Create two users whose only access can come from ownership annotations.
	ownerName, ownerPW, ownerID := envInstance.CreateUser(t, admin, noPermRole, "e2e-owner-collab-owner")
	collabName, collabPW, collabID := envInstance.CreateUser(t, admin, noPermRole, "e2e-owner-collab-collab")

	t.Cleanup(func() {
		_, _, _ = admin.Delete("/users/" + ownerID)
	})
	t.Cleanup(func() {
		_, _, _ = admin.Delete("/users/" + collabID)
	})

	// Clean up the GameServer at the end.
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	// Set up template and create GameServer as admin.
	applyBusyboxTemplate(t, tmpl)
	createGameServerViaAPI(t, admin, ns, gsName, tmpl)

	// Transfer ownership to owner-user.
	ownerIDInt, _ := strconv.ParseInt(ownerID, 10, 64)
	resp, body, err := admin.Post("/servers/"+gsName+":transfer", map[string]any{
		"userId": ownerIDInt,
	})
	if err != nil {
		t.Fatalf("admin POST :transfer: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("admin POST :transfer: status=%d body=%s", resp.StatusCode, string(body))
	}

	// Parse collaborator ID as int64 for the collaborators request.
	collabIDInt, _ := strconv.ParseInt(collabID, 10, 64)

	// Create authenticated clients for owner and collaborator.
	owner := envInstance.APIClient(t, ownerName, ownerPW)
	defer owner.Close()

	collab := envInstance.APIClient(t, collabName, collabPW)
	defer collab.Close()

	// Test: owner can GET the server despite having no namespace role bindings.
	t.Run("owner can GET server", func(t *testing.T) {
		resp, body, err := owner.Get("/servers/" + gsName + "?namespace=" + ns)
		if err != nil {
			t.Errorf("owner GET server: %v", err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("owner GET server: status=%d body=%s", resp.StatusCode, string(body))
		}
	})

	// Test: collaborator gets 403 on GET before being added to the list.
	t.Run("collab gets 403 GET before being added", func(t *testing.T) {
		resp, body, err := collab.Get("/servers/" + gsName + "?namespace=" + ns)
		if err != nil {
			t.Errorf("collab GET server: %v", err)
			return
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("collab GET server before add: status=%d want=403 body=%s", resp.StatusCode, string(body))
		}
	})

	// Test: owner can PUT :collaborators to add collab-user.
	t.Run("owner PUT :collaborators adds collaborator", func(t *testing.T) {
		resp, body, err := owner.Do(http.MethodPut, "/servers/"+gsName+":collaborators?namespace="+ns, map[string]any{
			"userIds": []int64{collabIDInt},
		})
		if err != nil {
			t.Errorf("owner PUT :collaborators: %v", err)
			return
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("owner PUT :collaborators: status=%d want=204 body=%s", resp.StatusCode, string(body))
		}
	})

	// Test: owner can GET /users/me/servers and it includes the server.
	t.Run("owner GET /users/me/servers includes owned server", func(t *testing.T) {
		resp, body, err := owner.Get("/users/me/servers")
		if err != nil {
			t.Errorf("owner GET /users/me/servers: %v", err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("owner GET /users/me/servers: status=%d body=%s", resp.StatusCode, string(body))
			return
		}
		if !strings.Contains(string(body), gsName) {
			t.Errorf("owner /users/me/servers does not include owned server %s: body=%s", gsName, string(body))
		}
	})

	// Test: collaborator can now GET the server (after being added).
	t.Run("collab can GET server after being added", func(t *testing.T) {
		resp, body, err := collab.Get("/servers/" + gsName + "?namespace=" + ns)
		if err != nil {
			t.Errorf("collab GET server: %v", err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("collab GET server after add: status=%d want=200 body=%s", resp.StatusCode, string(body))
		}
	})

	// Test: collaborator can run lifecycle operations without namespace role bindings.
	t.Run("collab can start server via lifecycle", func(t *testing.T) {
		resp, body, err := collab.Post("/servers/"+gsName+":start?namespace="+ns, nil)
		if err != nil {
			t.Errorf("collab POST :start: %v", err)
			return
		}
		// The server may legitimately fail to start, but the access check (status != 403)
		// proves the grant is working.
		if resp.StatusCode == http.StatusForbidden {
			t.Errorf("collab POST :start: status=403 (access denied despite collaborator grant) body=%s", string(body))
		}
	})

	// Test: collaborator can GET /users/me/servers and it includes the server.
	t.Run("collab GET /users/me/servers includes collaborated server", func(t *testing.T) {
		resp, body, err := collab.Get("/users/me/servers")
		if err != nil {
			t.Errorf("collab GET /users/me/servers: %v", err)
			return
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("collab GET /users/me/servers: status=%d body=%s", resp.StatusCode, string(body))
			return
		}
		if !strings.Contains(string(body), gsName) {
			t.Errorf("collab /users/me/servers does not include collaborated server %s: body=%s", gsName, string(body))
		}
	})

	// Test: collaborator gets 403 on POST :transfer (owner-only).
	t.Run("collab gets 403 on :transfer", func(t *testing.T) {
		resp, body, err := collab.Post("/servers/"+gsName+":transfer?namespace="+ns, map[string]any{
			"userId": ownerIDInt,
		})
		if err != nil {
			t.Errorf("collab POST :transfer: %v", err)
			return
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("collab POST :transfer: status=%d want=403 body=%s", resp.StatusCode, string(body))
		}
	})

	// Test: collaborator gets 403 on PUT :collaborators (owner-only).
	t.Run("collab gets 403 on :collaborators", func(t *testing.T) {
		resp, body, err := collab.Do(http.MethodPut, "/servers/"+gsName+":collaborators?namespace="+ns, map[string]any{
			"userIds": []int64{},
		})
		if err != nil {
			t.Errorf("collab PUT :collaborators: %v", err)
			return
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("collab PUT :collaborators: status=%d want=403 body=%s", resp.StatusCode, string(body))
		}
	})

	// Test: collaborator gets 403 on DELETE (owner-only).
	t.Run("collab gets 403 on DELETE", func(t *testing.T) {
		resp, body, err := collab.Delete("/servers/" + gsName + "?namespace=" + ns)
		if err != nil {
			t.Errorf("collab DELETE server: %v", err)
			return
		}
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("collab DELETE server: status=%d want=403 body=%s", resp.StatusCode, string(body))
		}
	})

	// Test: owner can DELETE the server (cleanup + permission check).
	t.Run("owner can DELETE server", func(t *testing.T) {
		resp, body, err := owner.Delete("/servers/" + gsName + "?namespace=" + ns)
		if err != nil {
			t.Errorf("owner DELETE server: %v", err)
			return
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("owner DELETE server: status=%d want=204 body=%s", resp.StatusCode, string(body))
		}
	})
}
