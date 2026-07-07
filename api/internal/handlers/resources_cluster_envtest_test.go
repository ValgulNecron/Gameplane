//go:build envtest

package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestResources_ClusterDispatch_Isolation proves the "one apiserver" landmine
// doesn't apply: a GameServer created against the envtest ("local") cluster
// is visible with no `?cluster=` param and with `?cluster=local` (backward
// compatible default), is NOT visible on the empty fake "other" cluster
// (objects never leak across clusters), and an unregistered `?cluster=ghost`
// is rejected with 400 rather than 500 or a silent 200.
func TestResources_ClusterDispatch_Isolation(t *testing.T) {
	name := uniqueResourceName("disp")
	body := map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": "minecraft"},
		},
	}

	resp := doJSON(t, http.MethodPost, "/servers", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /servers status = %d, want 201; body=%s", resp.StatusCode, readBody(t, resp))
	}

	t.Run("no cluster param lists it (default local)", func(t *testing.T) {
		if !listContains(t, "/servers", name) {
			t.Errorf("GET /servers did not include %q", name)
		}
	})

	t.Run("cluster=local lists it", func(t *testing.T) {
		if !listContains(t, "/servers?cluster=local", name) {
			t.Errorf("GET /servers?cluster=local did not include %q", name)
		}
	})

	t.Run("cluster=other is empty — no cross-cluster leak", func(t *testing.T) {
		if listContains(t, "/servers?cluster=other", name) {
			t.Errorf("GET /servers?cluster=other unexpectedly included %q — cross-cluster leak", name)
		}
	})

	t.Run("cluster=ghost is rejected with 400", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, "/servers?cluster=ghost", nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("GET /servers?cluster=ghost status = %d, want 400; body=%s", resp.StatusCode, readBody(t, resp))
		}
	})
}

// listContains GETs path and reports whether the decoded list's items
// contain an object named name.
func listContains(t *testing.T, path, name string) bool {
	t.Helper()
	resp := doJSON(t, http.MethodGet, path, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200; body=%s", path, resp.StatusCode, readBody(t, resp))
	}
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	resp.Body.Close()
	items, _ := raw["items"].([]any)
	for _, it := range items {
		obj, _ := it.(map[string]any)
		md, _ := obj["metadata"].(map[string]any)
		if md["name"] == name {
			return true
		}
	}
	return false
}
