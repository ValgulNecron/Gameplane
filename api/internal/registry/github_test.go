package registry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testGitHubRepo(url string) *githubRepo {
	gh := newGitHub(&http.Client{Timeout: 5 * time.Second}, "gameplane-test")
	gh.baseURL = url
	return &githubRepo{gh: gh, owner: "someorg", repo: "somemod"}
}

const githubReleasesFixture = `[
	{"id":1,"tag_name":"v2.0.0","name":"2.0.0 - Big Update","draft":false,"author":{"login":"alice"},
	 "assets":[{"name":"mod-2.0.0.jar","browser_download_url":"https://github.com/someorg/somemod/releases/download/v2.0.0/mod-2.0.0.jar","size":1024}]},
	{"id":2,"tag_name":"v1.0.0","name":"","draft":false,"author":{"login":"alice"},
	 "assets":[{"name":"mod-1.0.0.jar","browser_download_url":"https://github.com/someorg/somemod/releases/download/v1.0.0/mod-1.0.0.jar","size":512}]},
	{"id":3,"tag_name":"v0.9.0-draft","name":"WIP","draft":true,"author":{"login":"alice"},"assets":[]}
]`

func TestGitHubSearchFiltersDraftsAndTerm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/someorg/somemod/releases" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(githubReleasesFixture))
	}))
	defer srv.Close()

	// No term: drafts excluded, everything else returned.
	got, err := testGitHubRepo(srv.URL).Search(context.Background(), SearchQuery{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (draft excluded)", len(got))
	}
	if got[0].Title != "2.0.0 - Big Update" || got[0].ID != "1" || got[0].Provider != "github" {
		t.Errorf("hit0 = %+v", got[0])
	}
	// Blank release name falls back to the tag.
	if got[1].Title != "v1.0.0" {
		t.Errorf("hit1 title = %q, want tag fallback v1.0.0", got[1].Title)
	}

	// Term matches by title or tag substring.
	filtered, err := testGitHubRepo(srv.URL).Search(context.Background(), SearchQuery{Term: "big", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "1" {
		t.Fatalf("filtered = %+v, want just release 1", filtered)
	}
}

func TestGitHubSearchModpackEmpty(t *testing.T) {
	got, err := testGitHubRepo("http://unused.invalid").Search(context.Background(), SearchQuery{ProjectType: "modpack"})
	if err != nil || got != nil {
		t.Errorf("modpack search = %v, %v, want nil, nil (GitHub Releases has no modpacks)", got, err)
	}
}

func TestGitHubVersionsReturnsReleaseAssets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/someorg/somemod/releases/1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":1,"tag_name":"v2.0.0","name":"2.0.0","draft":false,
			"assets":[
				{"name":"mod-2.0.0.jar","browser_download_url":"https://github.com/someorg/somemod/releases/download/v2.0.0/mod-2.0.0.jar","size":1024},
				{"name":"mod-2.0.0-sources.jar","browser_download_url":"https://github.com/someorg/somemod/releases/download/v2.0.0/mod-2.0.0-sources.jar","size":2048}
			]}`))
	}))
	defer srv.Close()

	got, err := testGitHubRepo(srv.URL).Versions(context.Background(), "1", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	v := got[0]
	if v.VersionNumber != "v2.0.0" || len(v.Files) != 2 {
		t.Fatalf("version = %+v", v)
	}
	if v.Files[0].DownloadURL != "https://github.com/someorg/somemod/releases/download/v2.0.0/mod-2.0.0.jar" || !v.Files[0].Primary {
		t.Errorf("file0 = %+v", v.Files[0])
	}
	if v.Files[1].Primary {
		t.Error("only the first file should be marked primary")
	}
}

func TestGitHubVersionsSkipsAssetsWithNoURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":1,"tag_name":"v1","assets":[{"name":"broken","browser_download_url":""}]}`))
	}))
	defer srv.Close()
	got, err := testGitHubRepo(srv.URL).Versions(context.Background(), "1", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 || len(got[0].Files) != 0 {
		t.Errorf("got = %+v, want 1 version with zero files", got)
	}
}

func TestGitHubRateLimitedClassic403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := testGitHubRepo(srv.URL).Search(context.Background(), SearchQuery{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, errGitHubRateLimited) {
		t.Errorf("err = %v, want wrapped errGitHubRateLimited", err)
	}
}

func TestGitHubRateLimitedSecondary429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, err := testGitHubRepo(srv.URL).Search(context.Background(), SearchQuery{})
	if !errors.Is(err, errGitHubRateLimited) {
		t.Errorf("err = %v, want wrapped errGitHubRateLimited", err)
	}
}

func TestGitHubForbiddenWithQuotaLeftIsNotRateLimit(t *testing.T) {
	// A 403 with remaining quota > 0 is a real permission error (e.g. a
	// private repo), not a rate limit — must not be misreported as one.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "42")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := testGitHubRepo(srv.URL).Search(context.Background(), SearchQuery{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if errors.Is(err, errGitHubRateLimited) {
		t.Error("a 403 with quota remaining should not be reported as rate-limited")
	}
}

func TestGitHubReleaseNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := testGitHubRepo(srv.URL).Versions(context.Background(), "999", Filter{}); err == nil {
		t.Fatal("expected error for unknown release")
	}
}

func TestGitHubModpackDepsNil(t *testing.T) {
	r := testGitHubRepo("http://unused.invalid")
	got, err := r.ModpackDeps(context.Background(), "x")
	if err != nil || got != nil {
		t.Errorf("ModpackDeps = %v, %v", got, err)
	}
}
