//go:build envtest

package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestDestinations_ClusterDispatch_Isolation proves that the Destinations
// handler correctly isolates objects per cluster: an unknown cluster returns
// 400, an empty cluster returns a valid 200 with an empty list, and the
// default local cluster returns 200.
func TestDestinations_ClusterDispatch_Isolation(t *testing.T) {
	t.Run("cluster=ghost is rejected with 400", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, "/backup-destinations?cluster=ghost", nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("GET /backup-destinations?cluster=ghost status = %d, want 400; body=%s", resp.StatusCode, readBody(t, resp))
		}
	})

	t.Run("cluster=other returns empty list", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, "/backup-destinations?cluster=other", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /backup-destinations?cluster=other status = %d, want 200; body=%s", resp.StatusCode, readBody(t, resp))
		}
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		resp.Body.Close()
		items, _ := result["items"].([]any)
		if len(items) != 0 {
			t.Errorf("GET /backup-destinations?cluster=other returned %d items, want 0 (empty cluster should have no destinations)", len(items))
		}
	})

	t.Run("no cluster param defaults to local", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, "/backup-destinations", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /backup-destinations status = %d, want 200; body=%s", resp.StatusCode, readBody(t, resp))
		}
		resp.Body.Close()
	})
}

// TestEvents_ClusterDispatch_UnknownClusterRejected proves that the Events
// handler (SSE stream) resolves the cluster BEFORE writing any response headers,
// so an unknown ?cluster= yields a clean 400 instead of a half-open stream.
func TestEvents_ClusterDispatch_UnknownClusterRejected(t *testing.T) {
	resp := doJSON(t, http.MethodGet, "/events?cluster=ghost", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("GET /events?cluster=ghost status = %d, want 400; body=%s", resp.StatusCode, readBody(t, resp))
	}
	resp.Body.Close()
}
