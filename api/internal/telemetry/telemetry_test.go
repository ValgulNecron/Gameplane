package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

func telStore(t *testing.T, sendMetrics bool) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), "sqlite", "file:"+filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	val, _ := json.Marshal(map[string]bool{"sendMetrics": sendMetrics})
	if _, err := store.DB.Exec(
		`INSERT INTO config(key, value, updated_at) VALUES ('telemetry', ?, ?)`,
		string(val), "2026-01-01T00:00:00Z",
	); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	return store
}

func telKube(objs ...runtime.Object) *kube.Client {
	gvkr := map[schema.GroupVersionResource]string{
		kube.GVRs["servers"]:   "GameServerList",
		kube.GVRs["templates"]: "GameTemplateList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvkr, objs...)
	return &kube.Client{Dynamic: dyn}
}

func unstr(kind, name, ns string) *unstructured.Unstructured {
	o := &unstructured.Unstructured{}
	o.SetAPIVersion("gameplane.local/v1alpha1")
	o.SetKind(kind)
	o.SetName(name)
	if ns != "" {
		o.SetNamespace(ns)
	}
	return o
}

func TestReportOnce_EnabledPostsAnonymousCounts(t *testing.T) {
	var got payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	k := telKube(
		unstr("GameServer", "a", "gameplane-games"),
		unstr("GameServer", "b", "gameplane-games"),
		unstr("GameTemplate", "minecraft", ""),
	)
	r := New(telStore(t, true), k, srv.URL, "v1.2.3", time.Hour)
	if err := r.reportOnce(context.Background()); err != nil {
		t.Fatalf("reportOnce: %v", err)
	}
	if got.Version != "v1.2.3" || got.Servers != 2 || got.Templates != 1 {
		t.Fatalf("payload = %+v, want {v1.2.3 2 1}", got)
	}
}

func TestReportOnce_DisabledSkipsPost(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := New(telStore(t, false), telKube(), srv.URL, "v1", time.Hour)
	if err := r.reportOnce(context.Background()); err != nil {
		t.Fatalf("reportOnce: %v", err)
	}
	if hit {
		t.Fatal("telemetry must not POST when sendMetrics is off")
	}
}
