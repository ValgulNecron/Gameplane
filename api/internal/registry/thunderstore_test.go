package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const tsCatalog = `[
	{"name":"ValheimPlus","full_name":"valheimPlus-ValheimPlus","owner":"valheimPlus","package_url":"https://thunderstore.io/c/valheim/p/valheimPlus/ValheimPlus/","rating_score":500,"is_deprecated":false,"versions":[
		{"name":"ValheimPlus","full_name":"valheimPlus-ValheimPlus-0.9.9","version_number":"0.9.9","description":"Tweaks","icon":"https://gcdn.thunderstore.io/v.png","download_url":"https://thunderstore.io/package/download/valheimPlus/ValheimPlus/0.9.9/","downloads":100,"file_size":4096}
	]},
	{"name":"BepInExPack","full_name":"denikson-BepInExPack_Valheim","owner":"denikson","package_url":"https://thunderstore.io/c/valheim/p/denikson/BepInExPack_Valheim/","rating_score":900,"is_deprecated":false,"versions":[
		{"name":"BepInExPack","full_name":"denikson-BepInExPack_Valheim-5.4.2202","version_number":"5.4.2202","description":"Loader","icon":"https://gcdn.thunderstore.io/b.png","download_url":"https://thunderstore.io/package/download/denikson/BepInExPack_Valheim/5.4.2202/","downloads":9000,"file_size":8192}
	]},
	{"name":"OldMod","full_name":"x-OldMod","owner":"x","package_url":"u","rating_score":1,"is_deprecated":true,"versions":[]}
]`

func newTSServer(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/c/valheim/api/v1/package/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tsCatalog))
	}))
}

func testThunderstore(url string) *thunderstoreCommunity {
	ts := newThunderstore(&http.Client{Timeout: 5 * time.Second}, "kestrel-test")
	ts.baseURL = url
	return &thunderstoreCommunity{ts: ts, community: "valheim"}
}

func TestThunderstoreSearchRanksAndFilters(t *testing.T) {
	srv := newTSServer(t, nil)
	defer srv.Close()

	got, err := testThunderstore(srv.URL).Search(context.Background(), SearchQuery{Term: "", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Deprecated package excluded → 2 results, ranked by rating_score desc.
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (deprecated excluded)", len(got))
	}
	if got[0].Title != "BepInExPack" {
		t.Errorf("top = %q, want BepInExPack (highest rating)", got[0].Title)
	}
	if got[0].ID != "denikson-BepInExPack_Valheim" || got[0].Provider != "thunderstore" {
		t.Errorf("hit = %+v", got[0])
	}
	if got[0].Downloads != 9000 {
		t.Errorf("downloads = %d, want 9000", got[0].Downloads)
	}
}

func TestThunderstoreSearchTerm(t *testing.T) {
	srv := newTSServer(t, nil)
	defer srv.Close()
	got, err := testThunderstore(srv.URL).Search(context.Background(), SearchQuery{Term: "valheimplus"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Title != "ValheimPlus" {
		t.Fatalf("got %+v, want only ValheimPlus", got)
	}
}

func TestThunderstoreVersions(t *testing.T) {
	srv := newTSServer(t, nil)
	defer srv.Close()
	got, err := testThunderstore(srv.URL).Versions(context.Background(), "denikson-BepInExPack_Valheim", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	f := got[0].Files[0]
	if f.DownloadURL != "https://thunderstore.io/package/download/denikson/BepInExPack_Valheim/5.4.2202/" {
		t.Errorf("download = %q", f.DownloadURL)
	}
	if f.Filename != "denikson-BepInExPack_Valheim-5.4.2202.zip" || !f.Primary {
		t.Errorf("file = %+v", f)
	}
}

func TestThunderstoreVersionsNotFound(t *testing.T) {
	srv := newTSServer(t, nil)
	defer srv.Close()
	if _, err := testThunderstore(srv.URL).Versions(context.Background(), "nope", Filter{}); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestThunderstoreCaches(t *testing.T) {
	var hits int32
	srv := newTSServer(t, &hits)
	defer srv.Close()
	c := testThunderstore(srv.URL)
	for i := 0; i < 3; i++ {
		if _, err := c.Search(context.Background(), SearchQuery{Term: "x"}); err != nil {
			t.Fatalf("Search: %v", err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("upstream hits = %d, want 1 (cached)", got)
	}
}
