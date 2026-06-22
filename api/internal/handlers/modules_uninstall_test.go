package handlers

import (
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestUninstall_BlockedByGameServer covers the 409 path where a
// referenced GameServer prevents removal.
func TestUninstall_BlockedByGameServer(t *testing.T) {
	mod := newModule("minecraft", map[string]any{
		"source":  map[string]any{"name": "u"},
		"name":    "minecraft",
		"version": "1.0",
	})
	_ = unstructured.SetNestedField(mod.Object, "minecraft", "status", "appliedTemplate")

	// A GameServer that uses the template — uninstall must be blocked.
	gs := newServerObj("gameplane-games", "alpha")
	_ = unstructured.SetNestedField(gs.Object, "minecraft", "spec", "templateRef", "name")

	k := fakeKubeClient(mod, gs)
	r := mountModulesRouter(k)
	rr := do(t, r, "DELETE", "/modules/minecraft", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("got %d %s", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "in use by") {
		t.Fatalf("body=%q", rr.Body)
	}
}

// TestUninstall_NotFound — uninstallBlocker returns "" + err so we fall
// through to the Delete path, which itself 404s on a missing object.
func TestUninstall_NotFound(t *testing.T) {
	k := fakeKubeClient()
	r := mountModulesRouter(k)
	rr := do(t, r, "DELETE", "/modules/ghost", nil)
	if rr.Code == http.StatusNoContent {
		t.Fatal("missing module should not 204")
	}
}

// TestUpgrade_NotFound — patch on missing module surfaces 404 via
// httperr.Write.
func TestUpgrade_NotFound(t *testing.T) {
	k := fakeKubeClient()
	r := mountModulesRouter(k)
	rr := do(t, r, "PATCH", "/modules/ghost", map[string]any{"version": "1.0"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestUpgrade_BadJSON — body parsing failure surfaces 400/500-ish.
func TestUpgrade_BadJSON(t *testing.T) {
	k := fakeKubeClient(newModule("alpha", map[string]any{
		"source": map[string]any{"name": "u"}, "name": "x", "version": "1",
	}))
	r := mountModulesRouter(k)
	req := httpReq("PATCH", "/modules/alpha", "not json")
	rr := newRR()
	r.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatal("bogus body should not succeed")
	}
}
