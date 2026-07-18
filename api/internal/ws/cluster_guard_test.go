package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// TestRejectRemoteCluster_TableDriven is the guard's core logic in
// isolation: a non-local ?cluster= selector must 404 before the wrapped
// handler ever runs; an absent, blank, or explicitly-local selector must
// reach it.
func TestRejectRemoteCluster_TableDriven(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantCalled bool
		wantCode   int
	}{
		{"no cluster param", "", true, http.StatusOK},
		{"explicit local cluster", "?cluster=local", true, http.StatusOK},
		{"blank cluster param trims to empty", "?cluster=%20%20", true, http.StatusOK},
		{"remote cluster", "?cluster=remote-1", false, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			h := rejectRemoteCluster(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/servers/alpha/status"+tc.query, nil)
			rr := httptest.NewRecorder()
			h(rr, req)
			if called != tc.wantCalled {
				t.Errorf("handler called = %v, want %v", called, tc.wantCalled)
			}
			if rr.Code != tc.wantCode {
				t.Errorf("code = %d, want %d", rr.Code, tc.wantCode)
			}
		})
	}
}

// TestMount_RejectsNonLocalCluster proves the guard is actually wired onto
// the real Mount() routes — not just unit-tested in isolation — across a
// representative sample of the proxy, action, and mod routes.
func TestMount_RejectsNonLocalCluster(t *testing.T) {
	r := chi.NewRouter()
	Mount(r, nil, "", "", "")
	cases := []struct{ method, path string }{
		{"GET", "/ws/servers/alpha/console"},
		{"GET", "/ws/servers/alpha/logs"},
		{"GET", "/servers/alpha/logs/download"},
		{"GET", "/servers/alpha/files/list"},
		{"POST", "/servers/alpha/actions/run"},
		{"GET", "/servers/alpha/status"},
		{"GET", "/servers/alpha/mods"},
		{"POST", "/servers/alpha/mods/install"},
		{"DELETE", "/servers/alpha/mods"},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path+"?cluster=remote-1", nil)
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s %s?cluster=remote-1: got %d, want 404", tc.method, tc.path, rr.Code)
		}
	}
}

// TestMount_LocalClusterStillReachesHandler proves the guard doesn't
// regress the single-cluster (and explicit-local) case: the request must
// still reach the proxy handler, which then answers its own dev-mode 503
// (no mTLS material configured) — never the guard's 404.
func TestMount_LocalClusterStillReachesHandler(t *testing.T) {
	r := chi.NewRouter()
	Mount(r, nil, "", "", "")
	for _, q := range []string{"", "?cluster=local"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/servers/alpha/status"+q, nil)
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("query=%q: got %d, want 503 (reached the proxy handler)", q, rr.Code)
		}
	}
}

// TestMountAttach_RejectsNonLocalCluster covers the PTY console route,
// which attaches via the API's own in-cluster kubeconfig rather than the
// agent proxy — a separate code path from Mount's own routes, so it needs
// its own regression coverage.
func TestMountAttach_RejectsNonLocalCluster(t *testing.T) {
	r := chi.NewRouter()
	mountAttach(r, &kube.Client{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/servers/alpha/console-pty?cluster=remote-1", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rr.Code)
	}
}

// TestMountAttach_LocalClusterReachesHandler proves an absent cluster
// selector still reaches attachProxy.handle — it then fails
// websocket.Accept (this is a plain HTTP request, not a real upgrade),
// but that failure is never a 404.
func TestMountAttach_LocalClusterReachesHandler(t *testing.T) {
	r := chi.NewRouter()
	mountAttach(r, &kube.Client{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/servers/alpha/console-pty", nil)
	r.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatal("got 404, guard should have passed an absent cluster selector through to the handler")
	}
}

// TestMountPodLogs_RejectsNonLocalCluster mirrors the attach guard for the
// other Kubernetes-API-direct route (pod-log streaming during startup).
func TestMountPodLogs_RejectsNonLocalCluster(t *testing.T) {
	r := chi.NewRouter()
	mountPodLogs(r, &kube.Client{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/servers/alpha/logs/pod?cluster=remote-1", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rr.Code)
	}
}

// TestMountPodLogs_LocalClusterReachesHandler is the pod-logs twin of
// TestMountAttach_LocalClusterReachesHandler.
func TestMountPodLogs_LocalClusterReachesHandler(t *testing.T) {
	r := chi.NewRouter()
	mountPodLogs(r, &kube.Client{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/servers/alpha/logs/pod", nil)
	r.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatal("got 404, guard should have passed an absent cluster selector through to the handler")
	}
}
