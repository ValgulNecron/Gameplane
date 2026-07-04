package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testFactorio(url string) *Factorio {
	f := newFactorio(&http.Client{Timeout: 5 * time.Second}, "gameplane-test")
	f.baseURL = url
	return f
}

const factorioCatalogJSON = `{"pagination":null,"results":[
	{"name":"krastorio2","title":"Krastorio 2","owner":"Krastor","summary":"Overhaul mod",
	 "downloads_count":900000,"category":"overhaul","thumbnail":"/assets/k2.png",
	 "latest_release":{"info_json":{"factorio_version":"1.1"}}},
	{"name":"flib","title":"Factorio Library","owner":"raiguard","summary":"Utility library",
	 "downloads_count":2500000,"thumbnail":"/assets/.thumb.png",
	 "latest_release":{"info_json":{"factorio_version":"2.0"}}},
	{"name":"aai-industry","title":"AAI Industry","owner":"Earendel","summary":"Industry overhaul",
	 "downloads_count":1200000,
	 "latest_release":{"info_json":{"factorio_version":"2.0"}}},
	{"name":"orphan","title":"No Release","owner":"nobody","summary":"never released","downloads_count":3}
]}`

func TestFactorioSearch(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mods" {
			t.Errorf("path = %q, want /api/mods", r.URL.Path)
		}
		if r.URL.Query().Get("page_size") != "max" {
			t.Errorf("page_size = %q, want max", r.URL.Query().Get("page_size"))
		}
		hits.Add(1)
		_, _ = w.Write([]byte(factorioCatalogJSON))
	}))
	defer srv.Close()
	f := testFactorio(srv.URL)

	got, err := f.Search(context.Background(), SearchQuery{GameVersion: "2.0"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Only the two 2.0 mods (krastorio2 is 1.1; orphan has no release),
	// downloads-descending.
	if len(got) != 2 || got[0].ID != "flib" || got[1].ID != "aai-industry" {
		t.Fatalf("hits = %+v, want flib then aai-industry", got)
	}
	if got[0].Provider != "factorio" || got[0].Author != "raiguard" {
		t.Errorf("hit = %+v", got[0])
	}
	// The portal's placeholder thumbnail is dropped; real ones are absolute.
	if got[0].IconURL != "" {
		t.Errorf("placeholder thumb should be dropped, got %q", got[0].IconURL)
	}
	if got[1].PageURL != "https://mods.factorio.com/mod/aai-industry" {
		t.Errorf("pageURL = %q", got[1].PageURL)
	}

	// Term search is substring over title/name/owner, case-insensitive.
	got, err = f.Search(context.Background(), SearchQuery{Term: "krastorio"})
	if err != nil || len(got) != 1 || got[0].ID != "krastorio2" {
		t.Fatalf("term search = %+v, %v", got, err)
	}
	if got[0].IconURL != "https://assets-mod.factorio.com/assets/k2.png" {
		t.Errorf("iconURL = %q", got[0].IconURL)
	}

	// The catalog is cached: both searches hit upstream once.
	if hits.Load() != 1 {
		t.Errorf("upstream fetches = %d, want 1 (cache)", hits.Load())
	}
}

func TestFactorioSearchModpackEmpty(t *testing.T) {
	got, err := testFactorio("http://unused").Search(context.Background(), SearchQuery{ProjectType: "modpack"})
	if err != nil || len(got) != 0 {
		t.Errorf("modpack search = %v, %v, want empty", got, err)
	}
}

func TestFactorioSearchPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(factorioCatalogJSON))
	}))
	defer srv.Close()
	f := testFactorio(srv.URL)

	got, err := f.Search(context.Background(), SearchQuery{Limit: 1})
	if err != nil || len(got) != 1 || got[0].ID != "flib" {
		t.Fatalf("limit=1 = %+v, %v", got, err)
	}
	got, err = f.Search(context.Background(), SearchQuery{Limit: 1, Offset: 1})
	if err != nil || len(got) != 1 || got[0].ID != "aai-industry" {
		t.Fatalf("offset=1 = %+v, %v", got, err)
	}
	got, err = f.Search(context.Background(), SearchQuery{Offset: 100})
	if err != nil || len(got) != 0 {
		t.Fatalf("offset past end = %+v, %v", got, err)
	}
}

func TestFactorioVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mods/flib/full" {
			t.Errorf("path = %q, want /api/mods/flib/full", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"name":"flib","releases":[
			{"version":"0.14.1","download_url":"/download/flib/old","file_name":"flib_0.14.1.zip",
			 "released_at":"2024-01-01T00:00:00Z","info_json":{"factorio_version":"1.1"}},
			{"version":"0.16.2","download_url":"/download/flib/new","file_name":"flib_0.16.2.zip",
			 "released_at":"2025-06-01T00:00:00Z","info_json":{"factorio_version":"2.0"}},
			{"version":"0.15.0","download_url":"/download/flib/mid","file_name":"flib_0.15.0.zip",
			 "released_at":"2024-11-01T00:00:00Z","info_json":{"factorio_version":"2.0"}}
		]}`))
	}))
	defer srv.Close()
	f := testFactorio(srv.URL)

	got, err := f.Versions(context.Background(), "flib", Filter{GameVersion: "2.0"})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	// 1.1 release filtered out; remaining sorted newest-first by released_at.
	if len(got) != 2 || got[0].VersionNumber != "0.16.2" || got[1].VersionNumber != "0.15.0" {
		t.Fatalf("versions = %+v", got)
	}
	file := got[0].Files[0]
	if file.Filename != "flib_0.16.2.zip" || file.DownloadURL != srv.URL+"/download/flib/new" {
		t.Errorf("file = %+v", file)
	}
	if !file.RequiresAuth {
		t.Error("portal downloads must be flagged RequiresAuth")
	}

	all, err := f.Versions(context.Background(), "flib", Filter{})
	if err != nil || len(all) != 3 {
		t.Fatalf("unfiltered versions = %+v, %v", all, err)
	}
}

func TestFactorioVersionsUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := testFactorio(srv.URL).Versions(context.Background(), "missing", Filter{}); err == nil {
		t.Fatal("want error on upstream 404")
	}
}

func TestFactorioModpackDeps(t *testing.T) {
	files, err := testFactorio("http://unused").ModpackDeps(context.Background(), "x")
	if err != nil || files != nil {
		t.Errorf("ModpackDeps = %v, %v, want nil, nil", files, err)
	}
}

func TestFactorioCatalogRefreshAfterTTL(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(factorioCatalogJSON))
	}))
	defer srv.Close()
	f := testFactorio(srv.URL)
	f.ttl = 0 // every call is stale

	for range 2 {
		if _, err := f.Search(context.Background(), SearchQuery{}); err != nil {
			t.Fatal(err)
		}
	}
	if hits.Load() != 2 {
		t.Errorf("upstream fetches = %d, want 2 (ttl expired)", hits.Load())
	}
}
