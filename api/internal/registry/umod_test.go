package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testUmod(url string) *Umod {
	u := newUmod(&http.Client{Timeout: 5 * time.Second}, "gameplane-test")
	u.baseURL = url
	return u
}

func TestUmodSearchBrowse(t *testing.T) {
	var gotPath, gotQuery, gotSort, gotSortdir string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("query")
		gotSort = r.URL.Query().Get("sort")
		gotSortdir = r.URL.Query().Get("sortdir")
		_, _ = w.Write([]byte(`{"data":[
			{"slug":"vanish","title":"Vanish","description":"Go invisible","author":"Whispers88","icon_url":"https://assets.umod.org/i.png","downloads":472725,"url":"https://umod.org/plugins/vanish","download_url":"https://umod.org/plugins/Vanish.cs","latest_release_version":"2.1.4","latest_release_version_formatted":"v2.1.4"}
		]}`))
	}))
	defer srv.Close()

	got, err := testUmod(srv.URL).Search(context.Background(), SearchQuery{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/plugins/search.json" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty (browse)", gotQuery)
	}
	if gotSort != "downloads" || gotSortdir != "desc" {
		t.Errorf("sort=%q sortdir=%q, want downloads/desc default", gotSort, gotSortdir)
	}
	if len(got) != 1 || got[0].Title != "Vanish" || got[0].ID != "vanish" || got[0].Provider != "umod" {
		t.Errorf("hit = %+v", got)
	}
	if got[0].Downloads != 472725 {
		t.Errorf("downloads = %d", got[0].Downloads)
	}
}

func TestUmodSearchTermAndCategory(t *testing.T) {
	var gotQuery, gotCategory string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		gotCategory = r.URL.Query().Get("categories[]")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	if _, err := testUmod(srv.URL).Search(context.Background(), SearchQuery{Term: "vanish", Category: "rust"}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotQuery != "vanish" {
		t.Errorf("query = %q", gotQuery)
	}
	if gotCategory != "rust" {
		t.Errorf("categories[] = %q", gotCategory)
	}
}

func TestUmodSearchModpackEmpty(t *testing.T) {
	got, err := testUmod("http://unused.invalid").Search(context.Background(), SearchQuery{ProjectType: "modpack"})
	if err != nil || got != nil {
		t.Errorf("modpack search = %v, %v, want nil, nil (uMod has no modpacks)", got, err)
	}
}

func TestUmodVersionsFromHistoryEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/plugins/vanish/versions.json" {
			t.Errorf("path = %q, want the versions.json endpoint", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[
			{"version":"2.1.4","version_formatted":"v2.1.4","download_url":"https://umod.org/plugins/Vanish.cs?version=2.1.4"},
			{"version":"2.1.3","version_formatted":"v2.1.3","download_url":"https://umod.org/plugins/Vanish.cs?version=2.1.3"}
		]}`))
	}))
	defer srv.Close()

	got, err := testUmod(srv.URL).Versions(context.Background(), "vanish", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	f := got[0].Files
	if len(f) != 1 || f[0].Filename != "Vanish-2.1.4.cs" || !f[0].Primary {
		t.Errorf("file0 = %+v", f)
	}
	if f[0].DownloadURL != "https://umod.org/plugins/Vanish.cs?version=2.1.4" {
		t.Errorf("download url = %q", f[0].DownloadURL)
	}
}

func TestUmodVersionsFallsBackToLatestWhenHistoryEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/plugins/onlylatest/versions.json":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/plugins/onlylatest.json":
			_, _ = w.Write([]byte(`{"slug":"onlylatest","title":"Only Latest","download_url":"https://umod.org/plugins/OnlyLatest.cs","latest_release_version":"1.0.0"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := testUmod(srv.URL).Versions(context.Background(), "onlylatest", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 || got[0].VersionNumber != "1.0.0" {
		t.Fatalf("got = %+v", got)
	}
	if len(got[0].Files) != 1 || got[0].Files[0].Filename != "OnlyLatest-1.0.0.cs" {
		t.Errorf("files = %+v", got[0].Files)
	}
}

func TestUmodVersionsUnknownSlugErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := testUmod(srv.URL).Versions(context.Background(), "nope", Filter{}); err == nil {
		t.Fatal("expected an error for an unknown plugin slug")
	}
}

func TestUmodModpackDepsNil(t *testing.T) {
	got, err := newUmod(nil, "ua").ModpackDeps(context.Background(), "x")
	if err != nil || got != nil {
		t.Errorf("ModpackDeps = %v, %v", got, err)
	}
}

func TestUmodFilenameFromURL(t *testing.T) {
	for _, tc := range []struct{ url, version, want string }{
		{"https://umod.org/plugins/Vanish.cs?version=2.1.3", "2.1.3", "Vanish-2.1.3.cs"},
		{"https://umod.org/plugins/Vanish.cs", "", "Vanish.cs"},
		{"not a url at all", "1.0", "plugin-1.0.cs"},
	} {
		if got := umodFilenameFromURL(tc.url, tc.version); got != tc.want {
			t.Errorf("umodFilenameFromURL(%q, %q) = %q, want %q", tc.url, tc.version, got, tc.want)
		}
	}
}
