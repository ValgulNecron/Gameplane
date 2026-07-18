package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRejectRemoteCluster_TableDriven is the guard's core logic in
// isolation: a non-local ?cluster= selector must write a 404 (and report
// "rejected"); an absent, blank, or explicitly-local selector must let the
// caller proceed.
func TestRejectRemoteCluster_TableDriven(t *testing.T) {
	cases := []struct {
		name         string
		query        string
		wantRejected bool
	}{
		{"no cluster param", "", false},
		{"explicit local cluster", "?cluster=local", false},
		{"blank cluster param trims to empty", "?cluster=%20%20", false},
		{"remote cluster", "?cluster=remote-1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/servers/alpha/mods/ids"+tc.query, nil)
			rr := httptest.NewRecorder()
			got := rejectRemoteCluster(rr, req)
			if got != tc.wantRejected {
				t.Errorf("rejectRemoteCluster() = %v, want %v", got, tc.wantRejected)
			}
			if tc.wantRejected && rr.Code != http.StatusNotFound {
				t.Errorf("code = %d, want 404", rr.Code)
			}
			if !tc.wantRejected && rr.Code != http.StatusOK {
				// httptest.ResponseRecorder defaults to 200 when nothing wrote
				// a status — a pass-through must not have touched w at all.
				t.Errorf("code = %d, want 200 (untouched)", rr.Code)
			}
		})
	}
}

// TestModIDs_RejectsNonLocalCluster proves the guard is wired onto
// MountModIDs' GET and PUT — the handlers hold a bare *kube.Client (the
// LOCAL cluster only), so a non-local ?cluster= must 404 rather than
// silently acting on the local, same-named GameServer.
func TestModIDs_RejectsNonLocalCluster(t *testing.T) {
	k := fakeKubeClient(
		newIDListTemplateObj("ark"),
		newModIDsServerObj("gameplane-games", "alpha", "ark", nil),
	)
	r := mountModIDsRouter(k)

	if rr := do(t, r, "GET", "/servers/alpha/mods/ids?cluster=remote-1", nil); rr.Code != http.StatusNotFound {
		t.Errorf("GET: got %d, want 404: %s", rr.Code, rr.Body)
	}
	if rr := do(t, r, "PUT", "/servers/alpha/mods/ids?cluster=remote-1", []ModID{{ID: "1"}}); rr.Code != http.StatusNotFound {
		t.Errorf("PUT: got %d, want 404: %s", rr.Code, rr.Body)
	}
}

// TestModIDs_LocalClusterStillWorks proves the guard doesn't regress the
// single-cluster (and explicit-local) case.
func TestModIDs_LocalClusterStillWorks(t *testing.T) {
	k := fakeKubeClient(
		newIDListTemplateObj("ark"),
		newModIDsServerObj("gameplane-games", "alpha", "ark", nil),
	)
	r := mountModIDsRouter(k)

	for _, q := range []string{"", "?cluster=local"} {
		rr := do(t, r, "GET", "/servers/alpha/mods/ids"+q, nil)
		if rr.Code != http.StatusOK {
			t.Errorf("query=%q: got %d, want 200: %s", q, rr.Code, rr.Body)
		}
	}
}

// TestModUpdates_RejectsNonLocalCluster is the mod-update-check twin of
// TestModIDs_RejectsNonLocalCluster.
func TestModUpdates_RejectsNonLocalCluster(t *testing.T) {
	lister := &fakeModLister{mods: nil}
	r := mountUpdatesRouter(updatesFixtureKube(), fakeSet{p: &fakeVersionsProvider{}}, lister)

	rr := do(t, r, "GET", "/servers/alpha/mods/updates?cluster=remote-1", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404: %s", rr.Code, rr.Body)
	}
}

// TestModUpdates_LocalClusterStillWorks proves the guard doesn't regress
// the single-cluster (and explicit-local) case.
func TestModUpdates_LocalClusterStillWorks(t *testing.T) {
	lister := &fakeModLister{mods: nil}
	r := mountUpdatesRouter(updatesFixtureKube(), fakeSet{p: &fakeVersionsProvider{}}, lister)

	for _, q := range []string{"", "?cluster=local"} {
		rr := do(t, r, "GET", "/servers/alpha/mods/updates"+q, nil)
		if rr.Code != http.StatusOK {
			t.Errorf("query=%q: got %d, want 200: %s", q, rr.Code, rr.Body)
		}
	}
}
