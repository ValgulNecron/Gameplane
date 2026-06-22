package handlers

import (
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func templateObj(name string, labels map[string]string) *unstructured.Unstructured {
	o := newServerObj("", name)
	o.Object["apiVersion"] = "gameplane.gg/v1alpha1"
	o.Object["kind"] = "GameTemplate"
	delete(o.Object["metadata"].(map[string]any), "namespace")
	if labels != nil {
		o.SetLabels(labels)
	}
	return o
}

// updateHandler's success paths (namespaced + cluster) and its own decode
// error path are distinct from createHandler's; the existing fake tests
// only reach the managed-template block. Cover the rest here.

func TestResources_Update_NamespacedSuccess(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountResourcesRouter(k)
	body := map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": "alpha", "namespace": "kestrel-games"},
		"spec":       map[string]any{"templateRef": map[string]any{"name": "minecraft"}, "suspended": true},
	}
	rr := do(t, r, "PUT", "/servers/alpha", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("put namespaced: got %d %s, want 200", rr.Code, rr.Body)
	}
}

func TestResources_Update_ClusterUnmanagedSuccess(t *testing.T) {
	k := fakeKubeClient(templateObj("minecraft", nil)) // no managed-by label
	r := mountResourcesRouter(k)
	body := map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": "minecraft"},
		"spec":       map[string]any{"image": "y", "game": "minecraft", "version": "2"},
	}
	rr := do(t, r, "PUT", "/templates/minecraft", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("put cluster unmanaged: got %d %s, want 200", rr.Code, rr.Body)
	}
}

func TestResources_Update_BadJSON(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountResourcesRouter(k)
	rr := doRaw(t, r, "PUT", "/servers/alpha", "not json")
	if rr.Code < 400 {
		t.Fatalf("put bad json: got %d, want a client/server error", rr.Code)
	}
}

// A managed template missing the module-name label falls back to the
// template name in the conflict message.
func TestResources_ManagedTemplate_ModNameFallback(t *testing.T) {
	k := fakeKubeClient(templateObj("minecraft", map[string]string{"gameplane.gg/managed-by": "Module"}))
	r := mountResourcesRouter(k)
	body := map[string]any{
		"apiVersion": "gameplane.gg/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": "minecraft"},
		"spec":       map[string]any{"image": "x"},
	}
	rr := do(t, r, "PUT", "/templates/minecraft", body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("put managed (no module-name): got %d %s, want 409", rr.Code, rr.Body)
	}
	if !strings.Contains(rr.Body.String(), "minecraft") {
		t.Fatalf("conflict message should name the template: %s", rr.Body.String())
	}
}
