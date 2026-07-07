package kube

import (
	"sync"
	"testing"
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

func TestRegistryConcurrency(t *testing.T) {
	r := NewRegistry("local")
	var wg sync.WaitGroup
	numGoroutines := 50

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			clusterID := "cluster-" + string(rune('a'+idx%26))
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
