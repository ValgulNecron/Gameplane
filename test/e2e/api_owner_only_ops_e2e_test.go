//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestAPI_OwnerOnlyServerOperations_RequireOwnerOrAdmin checks that ownership
// transfer, collaborator edits, data wipe and server delete need the
// server's owner or an admin. An operator-role user, who holds the
// namespace servers:write permission, is refused on a server it does not
// own and allowed on its own; an admin is allowed on a server it does not
// own.
//
// Login budget: 1 e2e-admin login plus 1 login as the test's own
// operator-role user.
func TestAPI_OwnerOnlyServerOperations_RequireOwnerOrAdmin(t *testing.T) {
	t.Parallel()

	const ns = "gameplane-games"
	const tmpl = "e2e-owner-only-tmpl"
	suffix := time.Now().UnixNano()
	adminServer := fmt.Sprintf("e2e-owner-only-admin-%d", suffix)
	opServer := fmt.Sprintf("e2e-owner-only-op-%d", suffix)
	opServer2 := fmt.Sprintf("e2e-owner-only-op2-%d", suffix)

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	opName, opPW, opID := envInstance.CreateUser(t, admin, "operator", "e2e-owner-only-op")
	t.Cleanup(func() {
		r, _, _ := admin.Delete("/users/" + opID)
		if r != nil {
			_ = r.Body.Close()
		}
	})
	opIDInt, err := strconv.ParseInt(opID, 10, 64)
	if err != nil {
		t.Fatalf("parse operator id %q: %v", opID, err)
	}

	applyBusyboxTemplate(t, tmpl)
	createGameServerViaAPI(t, admin, ns, adminServer, tmpl)

	beforeObj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Get(context.Background(), adminServer, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s: %v", adminServer, err)
	}
	before := beforeObj.GetAnnotations()
	if before["gameplane.local/owner-id"] == "" {
		t.Fatalf("server %s has no owner annotation after an admin create: %v", adminServer, before)
	}
	beforeSuspend, _, _ := unstructured.NestedBool(beforeObj.Object, "spec", "suspend")

	op := envInstance.APIClient(t, opName, opPW)
	defer op.Close()

	refused := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"collaborator edit", http.MethodPut, "/servers/" + adminServer + ":collaborators?namespace=" + ns, map[string]any{"userIds": []int64{opIDInt}}},
		{"ownership transfer", http.MethodPost, "/servers/" + adminServer + ":transfer?namespace=" + ns, map[string]any{"userId": opIDInt}},
		{"data wipe", http.MethodPost, "/servers/" + adminServer + ":wipe-data?namespace=" + ns, map[string]any{"confirm": adminServer}},
		{"delete", http.MethodDelete, "/servers/" + adminServer + "?namespace=" + ns, nil},
	}
	for _, rq := range refused {
		t.Run("operator is refused "+rq.name+" on a server it does not own", func(t *testing.T) {
			resp, body, err := op.Do(rq.method, rq.path, rq.body)
			if err != nil {
				t.Fatalf("%s %s: %v", rq.method, rq.path, err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s: status=%d want=403 body=%s", rq.method, rq.path, resp.StatusCode, string(body))
			}
		})
	}

	t.Run("refused operations leave the server unchanged", func(t *testing.T) {
		obj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(context.Background(), adminServer, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("server %s should still exist: %v", adminServer, err)
		}
		ann := obj.GetAnnotations()
		for _, key := range []string{"gameplane.local/owner-id", "gameplane.local/owner", "gameplane.local/collaborators"} {
			if ann[key] != before[key] {
				t.Errorf("annotation %s = %q, want unchanged %q", key, ann[key], before[key])
			}
		}
		if ann["gameplane.local/wipe-data-requested"] != "" {
			t.Errorf("wipe was requested on %s: %q", adminServer, ann["gameplane.local/wipe-data-requested"])
		}
		if suspend, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspend != beforeSuspend {
			t.Errorf("spec.suspend = %v, want unchanged %v", suspend, beforeSuspend)
		}
	})

	t.Run("operator may delete a server it owns", func(t *testing.T) {
		createGameServerViaAPI(t, op, ns, opServer, tmpl)
		resp, body, err := op.Delete("/servers/" + opServer + "?namespace=" + ns)
		if err != nil {
			t.Fatalf("operator DELETE own server: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("operator DELETE own server: status=%d want=204 body=%s", resp.StatusCode, string(body))
		}
	})

	t.Run("admin may delete a server it does not own", func(t *testing.T) {
		createGameServerViaAPI(t, op, ns, opServer2, tmpl)
		resp, body, err := admin.Delete("/servers/" + opServer2 + "?namespace=" + ns)
		if err != nil {
			t.Fatalf("admin DELETE operator's server: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("admin DELETE operator's server: status=%d want=204 body=%s", resp.StatusCode, string(body))
		}
	})
}
