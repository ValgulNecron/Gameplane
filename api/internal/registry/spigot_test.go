package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSpigot(url string) *Spigot {
	s := newSpigot(&http.Client{Timeout: 5 * time.Second}, "gameplane-test")
	s.baseURL = url
	return s
}

func TestSpigotSearchBrowseEmptyTerm(t *testing.T) {
	var gotPath, gotSort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSort = r.URL.Query().Get("sort")
		_, _ = w.Write([]byte(`[{"id":2124,"name":"SkinsRestorer","tag":"Restore skins","downloads":18796712,"icon":{"url":"data/x.jpg"}}]`))
	}))
	defer srv.Close()

	got, err := testSpigot(srv.URL).Search(context.Background(), SearchQuery{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/resources" {
		t.Errorf("path = %q, want /resources (browse)", gotPath)
	}
	if gotSort != "-downloads" {
		t.Errorf("sort = %q, want -downloads", gotSort)
	}
	if len(got) != 1 || got[0].Title != "SkinsRestorer" || got[0].ID != "2124" || got[0].Provider != "spigot" {
		t.Errorf("hit = %+v", got)
	}
	if got[0].IconURL != "https://www.spigotmc.org/data/x.jpg" {
		t.Errorf("icon = %q", got[0].IconURL)
	}
	if got[0].PageURL != "https://www.spigotmc.org/resources/2124/" {
		t.Errorf("page = %q", got[0].PageURL)
	}
}

func TestSpigotSearchTerm(t *testing.T) {
	var gotPath, gotField string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotField = r.URL.Query().Get("field")
		_, _ = w.Write([]byte(`[{"id":9089,"name":"EssentialsX","tag":"Essentials suite","downloads":100}]`))
	}))
	defer srv.Close()

	got, err := testSpigot(srv.URL).Search(context.Background(), SearchQuery{Term: "essentials", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/search/resources/essentials" {
		t.Errorf("path = %q", gotPath)
	}
	if gotField != "name" {
		t.Errorf("field = %q, want name", gotField)
	}
	if len(got) != 1 || got[0].Title != "EssentialsX" {
		t.Errorf("hit = %+v", got)
	}
}

func TestSpigotSearchModpackEmpty(t *testing.T) {
	got, err := testSpigot("http://unused.invalid").Search(context.Background(), SearchQuery{ProjectType: "modpack"})
	if err != nil || got != nil {
		t.Errorf("modpack search = %v, %v, want nil, nil (Spigot has no modpacks)", got, err)
	}
}

func TestSpigotVersionsHostedFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/resources/28140":
			_, _ = w.Write([]byte(`{"id":28140,"name":"LuckPerms","external":false,"premium":false}`))
		case r.URL.Path == "/resources/28140/versions":
			_, _ = w.Write([]byte(`[{"id":590885,"name":"5.5.0"},{"id":544174,"name":"5.4.131"}]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := testSpigot(srv.URL).Versions(context.Background(), "28140", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	f := got[0].Files
	if len(f) != 1 {
		t.Fatalf("files = %+v, want 1 (hosted resource)", f)
	}
	if !strings.HasSuffix(f[0].DownloadURL, "/resources/28140/versions/590885/download") {
		t.Errorf("download url = %q", f[0].DownloadURL)
	}
	if f[0].Filename != "LuckPerms-5.5.0.jar" {
		t.Errorf("filename = %q", f[0].Filename)
	}
	if !f[0].Primary {
		t.Error("expected primary file")
	}
}

func TestSpigotVersionsExternalResourceHasNoFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/resources/9089":
			_, _ = w.Write([]byte(`{"id":9089,"name":"EssentialsX","external":true,"premium":false}`))
		case r.URL.Path == "/resources/9089/versions":
			_, _ = w.Write([]byte(`[{"id":639442,"name":"2.22.0"}]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := testSpigot(srv.URL).Versions(context.Background(), "9089", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	// The version itself is still reported (title/number), but with zero
	// files — the dashboard renders "No compatible files." for it. Do NOT
	// invent a DownloadURL.
	if len(got[0].Files) != 0 {
		t.Errorf("files = %+v, want empty for an external resource", got[0].Files)
	}
	if got[0].VersionNumber != "2.22.0" {
		t.Errorf("version number = %q", got[0].VersionNumber)
	}
}

func TestSpigotVersionsPremiumResourceHasNoFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/resources/555":
			_, _ = w.Write([]byte(`{"id":555,"name":"PaidPlugin","external":false,"premium":true}`))
		case r.URL.Path == "/resources/555/versions":
			_, _ = w.Write([]byte(`[{"id":1,"name":"1.0"}]`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	got, err := testSpigot(srv.URL).Versions(context.Background(), "555", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 || len(got[0].Files) != 0 {
		t.Errorf("got = %+v, want 1 version with zero files", got)
	}
}

func TestSpigotModpackDepsNil(t *testing.T) {
	got, err := newSpigot(nil, "ua").ModpackDeps(context.Background(), "x")
	if err != nil || got != nil {
		t.Errorf("ModpackDeps = %v, %v", got, err)
	}
}

func TestSpigotUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if _, err := testSpigot(srv.URL).Search(context.Background(), SearchQuery{Term: "x"}); err == nil {
		t.Fatal("expected error on 503")
	}
}
