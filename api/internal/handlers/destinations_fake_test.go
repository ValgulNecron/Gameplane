package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

func mountDestRouter(k *kube.Client) *chi.Mux {
	r := chi.NewRouter()
	MountDestinations(r, k)
	return r
}

func newDestSecret(ns, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{destinationLabel: "true"},
		},
		StringData: map[string]string{"url": "s3://bucket", "password": "hunter2"},
	}
}

func TestDestinations_List(t *testing.T) {
	k := &kube.Client{
		Dynamic: fakeKubeClient().Dynamic,
		Typed: kubefake.NewClientset(
			newDestSecret("kestrel-games", "default"),
			newDestSecret("kestrel-games", "second"),
			// A non-labelled secret that must NOT appear in the list.
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "kestrel-games"},
			},
		),
	}
	r := mountDestRouter(k)
	rr := do(t, r, "GET", "/backup-destinations/", nil)
	if rr.Code != 200 {
		t.Fatalf("got %d %s", rr.Code, rr.Body)
	}
	var out destinationListResp
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("got %d items", len(out.Items))
	}
}

func TestDestinations_Get(t *testing.T) {
	k := &kube.Client{
		Dynamic: fakeKubeClient().Dynamic,
		Typed: kubefake.NewClientset(
			newDestSecret("kestrel-games", "default"),
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "kestrel-games"}},
		),
	}
	r := mountDestRouter(k)

	rr := do(t, r, "GET", "/backup-destinations/default", nil)
	if rr.Code != 200 {
		t.Fatalf("get default: %d %s", rr.Code, rr.Body)
	}

	rr = do(t, r, "GET", "/backup-destinations/wild", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("non-destination should 404, got %d", rr.Code)
	}

	rr = do(t, r, "GET", "/backup-destinations/missing", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing should 404, got %d", rr.Code)
	}
}

func TestDestinations_Create(t *testing.T) {
	k := &kube.Client{
		Dynamic: fakeKubeClient().Dynamic,
		Typed:   kubefake.NewClientset(),
	}
	r := mountDestRouter(k)

	t.Run("happy path", func(t *testing.T) {
		body := map[string]any{"name": "alpha", "url": "s3://x", "password": "p"}
		rr := do(t, r, "POST", "/backup-destinations/", body)
		if rr.Code != 200 {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		body := map[string]any{"name": "BAD", "url": "x", "password": "p"}
		rr := do(t, r, "POST", "/backup-destinations/", body)
		if rr.Code == 200 {
			t.Fatal("BAD should be rejected")
		}
	})

	t.Run("missing url", func(t *testing.T) {
		body := map[string]any{"name": "x", "password": "p"}
		rr := do(t, r, "POST", "/backup-destinations/", body)
		if rr.Code == 200 {
			t.Fatal("missing url should fail")
		}
	})

	t.Run("bad json", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/backup-destinations/", strings.NewReader("not json"))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code == 200 {
			t.Fatal("bogus body created a destination")
		}
	})
}

func TestDestinations_Delete(t *testing.T) {
	k := &kube.Client{
		Dynamic: fakeKubeClient().Dynamic,
		Typed: kubefake.NewClientset(
			newDestSecret("kestrel-games", "default"),
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "kestrel-games"}},
		),
	}
	r := mountDestRouter(k)

	t.Run("non-destination not found", func(t *testing.T) {
		rr := do(t, r, "DELETE", "/backup-destinations/other", nil)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got %d", rr.Code)
		}
	})

	t.Run("missing not found", func(t *testing.T) {
		rr := do(t, r, "DELETE", "/backup-destinations/ghost", nil)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got %d", rr.Code)
		}
	})

	t.Run("happy path", func(t *testing.T) {
		rr := do(t, r, "DELETE", "/backup-destinations/default", nil)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("got %d %s", rr.Code, rr.Body)
		}
	})
}
