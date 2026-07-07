package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

func mountDestRouter(k *kube.Client) *chi.Mux {
	r := chi.NewRouter()
	reg := kube.NewRegistry(scope.DefaultCluster)
	reg.Set(scope.DefaultCluster, k)
	MountDestinations(r, reg)
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
			newDestSecret("gameplane-games", "default"),
			newDestSecret("gameplane-games", "second"),
			// A non-labelled secret that must NOT appear in the list.
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "gameplane-games"},
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
			newDestSecret("gameplane-games", "default"),
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wild", Namespace: "gameplane-games"}},
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

	t.Run("writes the operator-compatible repo key", func(t *testing.T) {
		body := map[string]any{"name": "repokey", "url": "s3://r", "password": "p"}
		if rr := do(t, r, "POST", "/backup-destinations/", body); rr.Code != 200 {
			t.Fatalf("create: %d %s", rr.Code, rr.Body)
		}
		sec, err := k.Typed.CoreV1().Secrets("gameplane-games").Get(context.Background(), "repokey", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get secret: %v", err)
		}
		// The operator's restic Jobs read RESTIC_REPOSITORY from key "repo";
		// writing "url" here is what broke the whole backup subsystem.
		if got := sec.StringData["repo"]; got != "s3://r" {
			t.Fatalf("secret key repo = %q, want s3://r (data=%v stringData=%v)", got, sec.Data, sec.StringData)
		}
		if _, ok := sec.StringData["url"]; ok {
			t.Fatal("secret must not use the legacy url key for the repo URL")
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
			newDestSecret("gameplane-games", "default"),
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "gameplane-games"}},
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
