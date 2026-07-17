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

// testCurseforgeGame wraps testCurseforge with a configured game id,
// mirroring testSteamApp — the way tests exercise the Provider surface a
// template's registry config actually resolves to.
func testCurseforgeGame(url string, gameID int32) *curseforgeGame {
	return &curseforgeGame{cf: testCurseforge(url), gameID: gameID}
}

func TestCurseforgeSearch(t *testing.T) {
	var gotKey, gotGameID, gotClass, gotLoader, gotSort, gotFilter, gotIndex, gotGV string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mods/search" {
			t.Errorf("path = %q, want /mods/search", r.URL.Path)
		}
		gotKey = r.Header.Get("x-api-key")
		gotGameID = r.URL.Query().Get("gameId")
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

	got, err := testCurseforgeGame(srv.URL, cfGameMinecraft).Search(context.Background(), SearchQuery{
		Term: "jei", Loader: "fabric", GameVersion: "1.21.4", Sort: "downloads", Offset: 50, Limit: 20,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotKey != "secret-key" {
		t.Errorf("x-api-key = %q", gotKey)
	}
	if gotGameID != "432" {
		t.Errorf("gameId = %q, want 432 (minecraft)", gotGameID)
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
	if _, err := testCurseforgeGame(srv.URL, cfGameMinecraft).Search(context.Background(), SearchQuery{ProjectType: "modpack", Limit: 20}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotClass != "4471" {
		t.Errorf("classId = %q, want 4471 (modpacks)", gotClass)
	}
}

// TestCurseforgeSearchUsesConfiguredGameID is the regression guard for the
// "ARK's mod browser shows Minecraft mods" bug: the game id sent upstream
// must be the one configured on the template's provider (curseforgeGameID),
// never the hardcoded Minecraft id. It also proves a non-Minecraft game
// gets no classId filter, since CurseForge's mod/modpack class ids (6,
// 4471) are Minecraft-specific and would silently zero out another game's
// results.
func TestCurseforgeSearchUsesConfiguredGameID(t *testing.T) {
	const arkGameID = 83374 // ARK: Survival Ascended, from CurseForge's /v1/games.
	var gotGameID, gotClass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotGameID = r.URL.Query().Get("gameId")
		gotClass = r.URL.Query().Get("classId")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	if _, err := testCurseforgeGame(srv.URL, arkGameID).Search(context.Background(), SearchQuery{Term: "s+"}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotGameID != "83374" {
		t.Errorf("gameId = %q, want 83374 (ARK: Survival Ascended), not the hardcoded Minecraft id", gotGameID)
	}
	if gotClass != "" {
		t.Errorf("classId = %q, want unset for a non-Minecraft game", gotClass)
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

// TestCurseforgeGameDelegates proves curseforgeGame's Versions and
// ModpackDeps just forward to the underlying engine (game-agnostic, unlike
// Search) — mirroring steamApp's equivalent wrapper-delegation coverage.
func TestCurseforgeGameDelegates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mods/238222/files" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	game := testCurseforgeGame(srv.URL, 83374)
	if _, err := game.Versions(context.Background(), "238222", Filter{}); err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if files, err := game.ModpackDeps(context.Background(), "238222"); err != nil || files != nil {
		t.Fatalf("ModpackDeps = %v, %v; want nil, nil", files, err)
	}
}

func TestCurseforgeUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := testCurseforgeGame(srv.URL, cfGameMinecraft).Search(context.Background(), SearchQuery{Term: "x"}); err == nil {
		t.Fatal("expected error on 403")
	}
}
