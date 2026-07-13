package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testNexusGame(url, domain string) *nexusGame {
	n := newNexus(&http.Client{Timeout: 5 * time.Second}, "gameplane-test", "secret-key")
	n.baseURL = url
	return &nexusGame{nexus: n, domain: domain}
}

func TestNexusSearchDigitTermResolvesByID(t *testing.T) {
	var gotPath, gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("apikey")
		_, _ = w.Write([]byte(`{"mod_id":42,"name":"SMAPI","summary":"Modding API","picture_url":"https://x/p.jpg","author":"Pathoschild","version":"4.0.0"}`))
	}))
	defer srv.Close()

	g := testNexusGame(srv.URL, "stardewvalley")
	got, err := g.Search(context.Background(), SearchQuery{Term: "42", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/v1/games/stardewvalley/mods/42.json" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAPIKey != "secret-key" {
		t.Errorf("apikey header = %q, want secret-key", gotAPIKey)
	}
	if len(got) != 1 || got[0].ID != "42" || got[0].Title != "SMAPI" || got[0].Provider != "nexus" {
		t.Fatalf("got = %+v", got)
	}
	if got[0].PageURL != "https://www.nexusmods.com/stardewvalley/mods/42" {
		t.Errorf("PageURL = %q", got[0].PageURL)
	}
}

func TestNexusSearchDigitTermNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	g := testNexusGame(srv.URL, "stardewvalley")
	got, err := g.Search(context.Background(), SearchQuery{Term: "999999", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty for an unknown mod id", got)
	}
}

func TestNexusSearchTextFiltersTrending(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[
			{"mod_id":1,"name":"SMAPI","summary":"Modding API","author":"Pathoschild"},
			{"mod_id":2,"name":"Content Patcher","summary":"Load content packs","author":"Pathoschild"},
			{"mod_id":3,"name":"Automate","summary":"Auto machines","author":"Pathoschild"}
		]`))
	}))
	defer srv.Close()

	g := testNexusGame(srv.URL, "stardewvalley")
	got, err := g.Search(context.Background(), SearchQuery{Term: "content", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/v1/games/stardewvalley/mods/trending.json" {
		t.Errorf("path = %q, want trending listing", gotPath)
	}
	if len(got) != 1 || got[0].Title != "Content Patcher" {
		t.Fatalf("got = %+v, want just Content Patcher (substring match)", got)
	}
}

func TestNexusSearchEmptyTermReturnsWholeTrendingPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"mod_id":1,"name":"SMAPI"},
			{"mod_id":2,"name":"Content Patcher"}
		]`))
	}))
	defer srv.Close()

	g := testNexusGame(srv.URL, "stardewvalley")
	got, err := g.Search(context.Background(), SearchQuery{Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2 (unfiltered trending page)", len(got))
	}
}

func TestNexusSearchModpackReturnsEmptyWithoutRequest(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	g := testNexusGame(srv.URL, "stardewvalley")
	got, err := g.Search(context.Background(), SearchQuery{ProjectType: "modpack", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if hit {
		t.Error("Nexus has no Collections integration; a modpack query should never hit the API")
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}

// TestNexusVersionsAlwaysEmptyFiles is the headline behavior: Nexus never
// mints a download link server-side (premium-gated, IP-bound, and the
// agent's download path does no content-type validation) — Versions must
// never fabricate a File even when the mod resolves successfully.
func TestNexusVersionsAlwaysEmptyFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"mod_id":42,"name":"SMAPI","version":"4.0.0"}`))
	}))
	defer srv.Close()

	g := testNexusGame(srv.URL, "stardewvalley")
	got, err := g.Versions(context.Background(), "42", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d versions, want 1", len(got))
	}
	if len(got[0].Files) != 0 {
		t.Fatalf("Files = %+v, want empty — Nexus never mints a download link here", got[0].Files)
	}
	if got[0].VersionNumber != "4.0.0" {
		t.Errorf("VersionNumber = %q, want 4.0.0", got[0].VersionNumber)
	}
}

func TestNexusVersionsUnknownID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	g := testNexusGame(srv.URL, "stardewvalley")
	got, err := g.Versions(context.Background(), "999999", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty for an unknown mod id", got)
	}
}

func TestNexusModpackDepsIsNoOp(t *testing.T) {
	g := testNexusGame("http://unused.invalid", "stardewvalley")
	files, err := g.ModpackDeps(context.Background(), "1")
	if err != nil || files != nil {
		t.Fatalf("ModpackDeps = %v, %v; want nil, nil", files, err)
	}
}

// TestSetNexusKeyGating proves the end-to-end claim: nexus is hidden with
// no key configured and becomes available/selectable once one exists,
// exactly like curseforge's existing key-gating.
func TestSetNexusKeyGating(t *testing.T) {
	ctx := context.Background()

	noKey := NewSet("test", StaticKeys(map[string]string{}))
	if noKey.Available(ctx, "nexus") {
		t.Error("nexus should be unavailable without a key")
	}
	if _, ok := noKey.For(ctx, Config{Provider: "nexus", Community: "stardewvalley"}); ok {
		t.Error("nexus without a key should not be selectable")
	}

	withKey := NewSet("test", StaticKeys(map[string]string{"nexus": "nexus-key"}))
	if !withKey.Available(ctx, "nexus") {
		t.Error("nexus should be available with a key")
	}
	if _, ok := withKey.For(ctx, Config{Provider: "nexus", Community: "stardewvalley"}); !ok {
		t.Error("nexus with a key and a community/domain should be selectable")
	}
	// A key alone isn't enough — the template must also declare a domain.
	if _, ok := withKey.For(ctx, Config{Provider: "nexus"}); ok {
		t.Error("nexus without a Community/domain should not be selectable")
	}
}
