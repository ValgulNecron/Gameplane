package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

func TestStampOwner(t *testing.T) {
	obj := newServerObj("kestrel-games", "alpha")
	req := httptest.NewRequest("POST", "/servers", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{ID: 7, Username: "alice"}))
	stampOwner(obj, req)
	ann := obj.GetAnnotations()
	if ann[ownerAnnotation] != "alice" || ann[ownerIDAnnotation] != "7" {
		t.Fatalf("owner annotations = %v", ann)
	}
}

func TestOwnership_Transfer(t *testing.T) {
	store := newTestStore(t)
	uid := seedUser(t, store, "bob", "viewer", "")
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := chi.NewRouter()
	MountOwnership(r, k, store)

	t.Run("rejects an unknown user", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/alpha:transfer", map[string]any{"userId": 999999})
		if rr.Code != 404 {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})

	t.Run("requires a userId", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/alpha:transfer", map[string]any{})
		if rr.Code != 400 {
			t.Fatalf("got %d", rr.Code)
		}
	})

	t.Run("transfers to a valid user", func(t *testing.T) {
		rr := do(t, r, "POST", "/servers/alpha:transfer", map[string]any{"userId": uid})
		if rr.Code != 204 {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
		obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace("kestrel-games").Get(t.Context(), "alpha", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		ann := obj.GetAnnotations()
		if ann[ownerAnnotation] != "bob" {
			t.Errorf("owner = %q, want bob", ann[ownerAnnotation])
		}
	})
}
