package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testHangar(url string) *Hangar {
	h := newHangar(&http.Client{Timeout: 5 * time.Second}, "kestrel-test")
	h.baseURL = url
	return h
}

func TestHangarSearch(t *testing.T) {
	var gotQuery, gotSort, gotLimit, gotOffset string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects" {
			t.Errorf("path = %q, want /projects", r.URL.Path)
		}
		gotQuery = r.URL.Query().Get("query")
		gotSort = r.URL.Query().Get("sort")
		gotLimit = r.URL.Query().Get("limit")
		gotOffset = r.URL.Query().Get("offset")
		_, _ = w.Write([]byte(`{"result":[
			{"name":"EssentialsX","description":"Ess","avatarUrl":"https://hangarcdn/a.png","namespace":{"owner":"EssentialsX","slug":"Essentials"},"stats":{"downloads":5000}},
			{"name":"NoNs","namespace":{"owner":"","slug":""},"stats":{"downloads":1}}
		]}`))
	}))
	defer srv.Close()

	got, err := testHangar(srv.URL).Search(context.Background(), SearchQuery{Term: "ess", Sort: "downloads", Offset: 10, Limit: 20})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotQuery != "ess" || gotSort != "-downloads" || gotLimit != "20" || gotOffset != "10" {
		t.Errorf("q=%q sort=%q limit=%q offset=%q", gotQuery, gotSort, gotLimit, gotOffset)
	}
	// The result with an empty namespace is skipped.
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "EssentialsX/Essentials" || got[0].Author != "EssentialsX" || got[0].Provider != "hangar" {
		t.Errorf("hit = %+v", got[0])
	}
}

func TestHangarSearchModpackEmpty(t *testing.T) {
	got, err := testHangar("http://unused").Search(context.Background(), SearchQuery{ProjectType: "modpack"})
	if err != nil || got != nil {
		t.Errorf("modpack search = %v, %v", got, err)
	}
}

func TestHangarVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/EssentialsX/Essentials/versions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":[
			{"name":"2.20.1","downloads":{"PAPER":{"fileInfo":{"name":"EssentialsX-2.20.1.jar","sizeBytes":123},"externalUrl":null,"downloadUrl":"https://hangarcdn.papermc.io/EssentialsX-2.20.1.jar"}},"platformDependencies":{"PAPER":["1.21","1.20.4"]}},
			{"name":"2.19","downloads":{"PAPER":{"fileInfo":{"name":null,"sizeBytes":0},"externalUrl":"https://github.com/x/releases/2.19.jar","downloadUrl":null}},"platformDependencies":{"PAPER":["1.19"]}}
		]}`))
	}))
	defer srv.Close()

	got, err := testHangar(srv.URL).Versions(context.Background(), "EssentialsX/Essentials", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Files[0].DownloadURL != "https://hangarcdn.papermc.io/EssentialsX-2.20.1.jar" ||
		got[0].Files[0].Filename != "EssentialsX-2.20.1.jar" || got[0].Files[0].Size != 123 {
		t.Errorf("v0 file = %+v", got[0].Files[0])
	}
	// No direct downloadUrl → externalUrl; no fileInfo.name → synthesized filename.
	if got[1].Files[0].DownloadURL != "https://github.com/x/releases/2.19.jar" {
		t.Errorf("v1 downloadUrl = %q", got[1].Files[0].DownloadURL)
	}
	if got[1].Files[0].Filename != "Essentials-2.19.jar" {
		t.Errorf("v1 filename = %q", got[1].Files[0].Filename)
	}
}

func TestHangarVersionsConstructedURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// No downloadUrl and no externalUrl → engine builds the versioned
		// download endpoint.
		_, _ = w.Write([]byte(`{"result":[
			{"name":"1.0","downloads":{"PAPER":{"fileInfo":{"name":"p.jar","sizeBytes":1},"externalUrl":null,"downloadUrl":null}},"platformDependencies":{"PAPER":["1.21"]}}
		]}`))
	}))
	defer srv.Close()
	got, err := testHangar(srv.URL).Versions(context.Background(), "Owner/Plug", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	want := srv.URL + "/projects/Owner/Plug/versions/1.0/PAPER/download"
	if got[0].Files[0].DownloadURL != want {
		t.Errorf("constructed url = %q, want %q", got[0].Files[0].DownloadURL, want)
	}
}

func TestHangarBadID(t *testing.T) {
	got, err := testHangar("http://unused").Versions(context.Background(), "noslash", Filter{})
	if err != nil || got != nil {
		t.Errorf("bad id = %v, %v", got, err)
	}
}
