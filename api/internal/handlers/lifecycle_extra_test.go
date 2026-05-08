package handlers

import (
	"testing"
)

// TestLifecycle_RestartMissingServer exercises the first-Patch error
// branch in restartHandler.
func TestLifecycle_RestartMissingServer(t *testing.T) {
	k := fakeKubeClient()
	r := mountLifecycleRouter(k)
	rr := do(t, r, "POST", "/servers/ghost:restart", nil)
	if rr.Code == 202 {
		t.Fatal("missing server should not restart successfully")
	}
}

// TestLifecycle_StartScopeError forces resolveNS to deny via an unknown
// namespace query param.
func TestLifecycle_StartScopeError(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountLifecycleRouter(k)
	rr := do(t, r, "POST", "/servers/alpha:start?namespace=forbidden", nil)
	if rr.Code == 202 {
		t.Fatal("forbidden namespace should not patch")
	}
}

// TestLifecycle_CloneScopeError mirrors the clone-side scope check.
func TestLifecycle_CloneScopeError(t *testing.T) {
	k := fakeKubeClient(newServerObj("kestrel-games", "alpha"))
	r := mountLifecycleRouter(k)
	rr := do(t, r, "POST", "/servers/alpha:clone?namespace=forbidden",
		map[string]any{"newName": "beta"})
	if rr.Code == 200 {
		t.Fatal("forbidden namespace should not clone")
	}
}
