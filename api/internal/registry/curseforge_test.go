package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testCurseforge(url string) *Curseforge {
	c := newCurseforge(&http.Client{Timeout: 5 * time.Second}, "gameplane-test", "secret-key")
	c.baseURL = url
	return c
}

func TestCurseforgeSearch(t *testing.T) {
	var gotKey, gotClass, gotLoader, gotSort, gotFilter, gotIndex, gotGV string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mods/search" {
			t.Errorf("path = %q, want /mods/search", r.URL.Path)
		}
		gotKey = r.Header.Get("x-api-key")
		gotClass = r.URL.Query().Get("classId")
		gotLoader = r.URL.Query().Get("modLoaderType")
		gotSort = r.URL.Query().Get("sortField")
		gotFilter = r.URL.Query().Get("searchFilter")
		gotIndex = r.URL.Query().Get("index")
		gotGV = r.URL.Query().Get("gameVersion")
		_, _ = w.Write([]byte(`{"data":[
			{"id":238222,"name":"JEI","slug":"jei","summary":"Items","downloadCount":99,"authors":[{"name":"mezz"}],"logo":{"url":"https://media.forgecdn.net/x.png"},"links":{"websiteUrl":"https://www.curseforge.com/minecraft/mc-mods/jei"}}
		]}`))
	}))
	defer srv.Close()

	got, err := testCurseforge(srv.URL).Search(context.Background(), SearchQuery{
		Term: "jei", Loader: "fabric", GameVersion: "1.21.4", Sort: "downloads", Offset: 50, Limit: 20,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotKey != "secret-key" {
		t.Errorf("x-api-key = %q", gotKey)
	}
	if gotClass != "6" {
		t.Errorf("classId = %q, want 6 (mods)", gotClass)
	}
	if gotLoader != "4" {
		t.Errorf("modLoaderType = %q, want 4 (fabric)", gotLoader)
	}
	if gotSort != "6" {
		t.Errorf("sortField = %q, want 6 (TotalDownloads)", gotSort)
	}
	if gotFilter != "jei" || gotIndex != "50" || gotGV != "1.21.4" {
		t.Errorf("filter=%q index=%q gv=%q", gotFilter, gotIndex, gotGV)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "238222" || got[0].Title != "JEI" || got[0].Author != "mezz" ||
		got[0].Downloads != 99 || got[0].Provider != "curseforge" {
		t.Errorf("hit = %+v", got[0])
	}
}

func TestCurseforgeModpackClass(t *testing.T) {
	var gotClass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClass = r.URL.Query().Get("classId")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	if _, err := testCurseforge(srv.URL).Search(context.Background(), SearchQuery{ProjectType: "modpack", Limit: 20}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotClass != "4471" {
		t.Errorf("classId = %q, want 4471 (modpacks)", gotClass)
	}
}

func TestCurseforgeVersionsSkipsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mods/238222/files" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[
			{"id":1,"displayName":"JEI 1.0","fileName":"jei-1.0.jar","downloadUrl":"https://edge.forgecdn.net/jei-1.0.jar","fileLength":100,"gameVersions":["1.21.4","Fabric"]},
			{"id":2,"displayName":"JEI 0.9","fileName":"jei-0.9.jar","downloadUrl":"","fileLength":90,"gameVersions":["1.20.1"]}
		]}`))
	}))
	defer srv.Close()
	got, err := testCurseforge(srv.URL).Versions(context.Background(), "238222", Filter{Loader: "fabric", GameVersion: "1.21.4"})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	// The file with no downloadUrl (author opted out) is skipped.
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Files[0].DownloadURL != "https://edge.forgecdn.net/jei-1.0.jar" || got[0].Files[0].Size != 100 {
		t.Errorf("file = %+v", got[0].Files[0])
	}
}

func TestCurseforgeModpackDepsNil(t *testing.T) {
	got, err := newCurseforge(nil, "ua", "k").ModpackDeps(context.Background(), "x")
	if err != nil || got != nil {
		t.Errorf("ModpackDeps = %v, %v", got, err)
	}
}

func TestCurseforgeUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := testCurseforge(srv.URL).Search(context.Background(), SearchQuery{Term: "x"}); err == nil {
		t.Fatal("expected error on 403")
	}
}
