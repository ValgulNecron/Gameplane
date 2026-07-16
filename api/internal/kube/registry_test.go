package kube

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry("local")
	if r.DefaultID() != "local" {
		t.Errorf("DefaultID() = %q, want %q", r.DefaultID(), "local")
	}
	if r.Default() != nil {
		t.Errorf("Default() = %v, want nil", r.Default())
	}
}

func TestRegistrySetGet(t *testing.T) {
	r := NewRegistry("local")
	c := &Client{}

	r.Set("local", c)
	got, ok := r.Get("local")
	if !ok {
		t.Error("Get(local) returned ok=false, want true")
	}
	if got != c {
		t.Errorf("Get(local) returned different pointer, want same *Client")
	}

	got2, ok2 := r.Get("missing")
	if ok2 {
		t.Error("Get(missing) returned ok=true, want false")
	}
	if got2 != nil {
		t.Errorf("Get(missing) returned %v, want nil", got2)
	}
}

func TestRegistryDefault(t *testing.T) {
	r := NewRegistry("local")
	c := &Client{}

	r.Set("local", c)
	if r.Default() != c {
		t.Errorf("Default() returned different pointer after Set, want same *Client")
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry("local")
	c := &Client{}

	r.Set("local", c)
	r.Remove("local")

	got, ok := r.Get("local")
	if ok {
		t.Error("Get(local) returned ok=true after Remove, want false")
	}
	if got != nil {
		t.Errorf("Get(local) returned %v after Remove, want nil", got)
	}
}

func TestRegistryIDs(t *testing.T) {
	r := NewRegistry("local")

	// Empty registry should have empty IDs.
	ids := r.IDs()
	if len(ids) != 0 {
		t.Errorf("IDs() on empty registry returned %v, want empty", ids)
	}

	// Add IDs out of order and verify they come back sorted.
	r.Set("prod", &Client{})
	r.Set("local", &Client{})
	r.Set("staging", &Client{})

	ids = r.IDs()
	want := []string{"local", "prod", "staging"}
	if len(ids) != len(want) {
		t.Errorf("IDs() returned %d items, want %d", len(ids), len(want))
	}
	for i, id := range ids {
		if i >= len(want) {
			break
		}
		if id != want[i] {
			t.Errorf("IDs()[%d] = %q, want %q", i, id, want[i])
		}
	}
}

func TestRegistryGetServer_UnregisteredCluster(t *testing.T) {
	r := NewRegistry("local")
	if _, err := r.GetServer(context.Background(), "ghost", "gameplane-games", "alpha"); err == nil {
		t.Fatal("GetServer on unregistered cluster returned nil error, want error")
	}
}

func TestRegistryGetServer_RegisteredCluster(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":      "alpha",
			"namespace": "gameplane-games",
		},
	}}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		GVRs["servers"]: "GameServerList",
	}, obj)
	r := NewRegistry("local")
	r.Set("local", &Client{Dynamic: dyn})

	got, err := r.GetServer(context.Background(), "local", "gameplane-games", "alpha")
	if err != nil {
		t.Fatalf("GetServer() error = %v", err)
	}
	if got.GetName() != "alpha" {
		t.Errorf("GetServer() name = %q, want alpha", got.GetName())
	}
}

// TestRegistryConcurrency hammers the registry from many goroutines to prove
// its locking holds (run this with -race for the real signal).
//
// Each goroutine MUST own a unique cluster id. An earlier version keyed on
// idx%26, so with 50 goroutines two of them shared every id — each Set its own
// *Client, then asserted Get returned *its* pointer. That is not true under its
// own concurrency: the sibling's Set legitimately wins, and its Remove can make
// a Get return ok=false. The test only passed when the scheduler happened to
// interleave kindly, and flaked otherwise. Unique ids make every assertion here
// deterministic while still exercising concurrent access to the shared map.
func TestRegistryConcurrency(t *testing.T) {
	r := NewRegistry("local")
	var wg sync.WaitGroup
	numGoroutines := 50

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			clusterID := fmt.Sprintf("cluster-%d", idx)
			c := &Client{}

			r.Set(clusterID, c)
			got, ok := r.Get(clusterID)
			if !ok {
				t.Errorf("goroutine %d: Get returned ok=false after Set", idx)
			}
			if got != c {
				t.Errorf("goroutine %d: Get returned different pointer after Set", idx)
			}

			_ = r.IDs()
			r.Remove(clusterID)

			_, ok = r.Get(clusterID)
			if ok {
				t.Errorf("goroutine %d: Get returned ok=true after Remove", idx)
			}
		}(i)
	}
	wg.Wait()
}
