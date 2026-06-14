package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

func mountLifecycleRouter(k *kube.Client) *chi.Mux {
	r := chi.NewRouter()
	MountLifecycle(r, k)
	return r
}

func newServerObj(ns, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kestrel.gg/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": name, "namespace": ns},
		"spec":       map[string]any{"templateRef": map[string]any{"name": "minecraft"}},
	}}
}

func TestLifecycle_StartStop(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountLifecycleRouter(k)

	for _, verb := range []string{"start", "stop"} {
		rr := do(t, r, "POST", "/servers/alpha:"+verb, nil)
		if rr.Code != 202 {
			t.Fatalf("%s: %d %s", verb, rr.Code, rr.Body)
		}
	}
}

func TestLifecycle_Restart(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountLifecycleRouter(k)
	rr := do(t, r, "POST", "/servers/alpha:restart", nil)
	if rr.Code != 202 {
		t.Fatalf("got %d %s", rr.Code, rr.Body)
	}
}

func TestLifecycle_StartUnknown(t *testing.T) {
	k := fakeKubeClient()
	r := mountLifecycleRouter(k)
	rr := do(t, r, "POST", "/servers/missing:start", nil)
	if rr.Code == 202 {
		t.Fatal("missing server should not patch successfully")
	}
}

func TestLifecycle_WipeData(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountLifecycleRouter(k)

	t.Run("requires matching confirmation", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/alpha:wipe-data", map[string]any{"confirm": "wrong"})
		if rr.Code != 400 {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})

	t.Run("suspends and stamps the wipe annotation", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/alpha:wipe-data", map[string]any{"confirm": "alpha"})
		if rr.Code != 202 {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
		obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace("kestrel-games").Get(t.Context(), "alpha", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		suspend, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
		if !suspend {
			t.Error("expected suspend=true after wipe request")
		}
		ann := obj.GetAnnotations()
		if ann[wipeRequestedAnnotation] == "" {
			t.Errorf("expected wipe-requested annotation, got %v", ann)
		}
	})
}

func TestLifecycle_Clone(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountLifecycleRouter(k)

	t.Run("happy path", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/alpha:clone", map[string]any{"newName": "beta"})
		if rr.Code != 200 {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})

	t.Run("missing newName", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/alpha:clone", map[string]any{})
		if rr.Code != 400 {
			t.Fatalf("got %d", rr.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/servers/alpha:clone", strings.NewReader("bogus"))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != 400 {
			t.Fatalf("got %d", rr.Code)
		}
	})

	t.Run("source missing", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/ghost:clone", map[string]any{"newName": "z"})
		if rr.Code == 200 {
			t.Fatal("ghost source should not clone")
		}
	})
}
