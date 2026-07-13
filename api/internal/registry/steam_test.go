package registry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSteamApp(url string, appID int32) *steamApp {
	s := newSteam(&http.Client{Timeout: 5 * time.Second}, "gameplane-test", "secret-key")
	s.baseURL = url
	return &steamApp{steam: s, appID: appID}
}

func TestSteamSearchFacetsByAppID(t *testing.T) {
	var gotPath, gotKey, gotAppID, gotFileType, gotQueryType, gotSearchText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.URL.Query().Get("key")
		gotAppID = r.URL.Query().Get("appid")
		gotFileType = r.URL.Query().Get("filetype")
		gotQueryType = r.URL.Query().Get("query_type")
		gotSearchText = r.URL.Query().Get("search_text")
		_, _ = w.Write([]byte(`{"response":{"publishedfiledetails":[
			{"publishedfileid":"555","result":1,"title":"Wiremod","preview_url":"https://x/wire.png","subscriptions":42}
		]}}`))
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	got, err := app.Search(context.Background(), SearchQuery{Term: "wire", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPath != "/IPublishedFileService/QueryFiles/v1/" {
		t.Errorf("path = %q", gotPath)
	}
	if gotKey != "secret-key" {
		t.Errorf("key = %q, want secret-key", gotKey)
	}
	if gotAppID != "4000" {
		t.Errorf("appid = %q, want 4000", gotAppID)
	}
	if gotFileType != "0" {
		t.Errorf("filetype = %q, want 0 (items)", gotFileType)
	}
	if gotQueryType != "12" {
		t.Errorf("query_type = %q, want 12 (text search)", gotQueryType)
	}
	if gotSearchText != "wire" {
		t.Errorf("search_text = %q, want wire", gotSearchText)
	}
	if len(got) != 1 || got[0].ID != "555" || got[0].Title != "Wiremod" || got[0].Provider != "steam" {
		t.Fatalf("got = %+v", got)
	}
	if got[0].PageURL != "https://steamcommunity.com/sharedfiles/filedetails/?id=555" {
		t.Errorf("PageURL = %q", got[0].PageURL)
	}
}

func TestSteamSearchNoTermUsesTrendQueryType(t *testing.T) {
	var gotQueryType, gotSearchText, gotDays string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQueryType = r.URL.Query().Get("query_type")
		gotDays = r.URL.Query().Get("days")
		_, gotSearchTextSet := r.URL.Query()["search_text"]
		if gotSearchTextSet {
			gotSearchText = r.URL.Query().Get("search_text")
		}
		_, _ = w.Write([]byte(`{"response":{"publishedfiledetails":[]}}`))
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	if _, err := app.Search(context.Background(), SearchQuery{Limit: 10}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotQueryType != "3" {
		t.Errorf("query_type = %q, want 3 (ranked by trend)", gotQueryType)
	}
	if gotSearchText != "" {
		t.Errorf("search_text = %q, want unset", gotSearchText)
	}
	// M2: RankedByTrend effectively requires "days" — omitting it can make
	// browse return empty. Third-party clients use 7.
	if gotDays != "7" {
		t.Errorf("days = %q, want 7 (required by RankedByTrend)", gotDays)
	}
}

func TestSteamSearchModpackUsesCollectionsFiletype(t *testing.T) {
	var gotFileType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFileType = r.URL.Query().Get("filetype")
		_, _ = w.Write([]byte(`{"response":{"publishedfiledetails":[]}}`))
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	if _, err := app.Search(context.Background(), SearchQuery{ProjectType: "modpack", Limit: 10}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	// M1: asserted against the literal Steamworks enum value (not the
	// code's own constant) so a regression back to the wrong value (1,
	// which is ItemsMtx — individual microtransaction items, not
	// Collections) would actually fail this test.
	// EPublishedFileInfoMatchingFileType: Items=0, ItemsMtx=1,
	// ItemsReadyToUse=2, Collections=3, Artwork=4, ...
	if gotFileType != "3" {
		t.Errorf("filetype = %q, want 3 (collections) for a modpack query", gotFileType)
	}
}

// TestSteamResolveByIDUsesDescriptionFallback is M3:
// ISteamRemoteStorage/GetPublishedFileDetails (the resolve-by-id path used
// by every digit-term Search and by Versions' existence check) returns the
// blurb as "description", not "short_description"/"file_description" (the
// QueryFiles field names) — so a resolved item must still surface a
// description via that field.
func TestSteamResolveByIDUsesDescriptionFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"result":1,"resultcount":1,"publishedfiledetails":[
			{"publishedfileid":"1","result":1,"title":"Wiremod","consumer_app_id":4000,"description":"A wiring mod"}
		]}}`))
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	got, err := app.Search(context.Background(), SearchQuery{Term: "1", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Description != "A wiring mod" {
		t.Fatalf("got = %+v, want description from GetPublishedFileDetails' \"description\" field", got)
	}
}

func TestSteamSearchDigitTermUsesGetPublishedFileDetails(t *testing.T) {
	var queryFilesHit bool
	var detailsPath, detailsMethod, detailsBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/IPublishedFileService/QueryFiles/v1/":
			queryFilesHit = true
			_, _ = w.Write([]byte(`{"response":{"publishedfiledetails":[]}}`))
		case "/ISteamRemoteStorage/GetPublishedFileDetails/v1/":
			detailsPath = r.URL.Path
			detailsMethod = r.Method
			b, _ := io.ReadAll(r.Body)
			detailsBody = string(b)
			_, _ = w.Write([]byte(`{"response":{"result":1,"resultcount":1,"publishedfiledetails":[
				{"publishedfileid":"180077636","result":1,"title":"Stargate Carter Pack","consumer_app_id":4000,"preview_url":"https://x/p.jpg","subscriptions":9001}
			]}}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	got, err := app.Search(context.Background(), SearchQuery{Term: "180077636", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if queryFilesHit {
		t.Error("a digit-only term should not hit QueryFiles")
	}
	if detailsPath != "/ISteamRemoteStorage/GetPublishedFileDetails/v1/" {
		t.Errorf("details path = %q", detailsPath)
	}
	if detailsMethod != http.MethodPost {
		t.Errorf("details method = %q, want POST", detailsMethod)
	}
	if !strings.Contains(detailsBody, "publishedfileids%5B0%5D=180077636") {
		t.Errorf("details body = %q, want publishedfileids[0]=180077636", detailsBody)
	}
	if len(got) != 1 || got[0].ID != "180077636" || got[0].Title != "Stargate Carter Pack" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSteamSearchDigitTermCrossAppMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"result":1,"resultcount":1,"publishedfiledetails":[
			{"publishedfileid":"1","result":1,"consumer_app_id":730}
		]}}`))
	}))
	defer srv.Close()

	// appID 4000 (GMod), but the resolved item belongs to app 730 (CS2).
	app := testSteamApp(srv.URL, 4000)
	got, err := app.Search(context.Background(), SearchQuery{Term: "1", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v, want no match for a cross-app id", got)
	}
}

func TestSteamSearchDigitTermNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"result":1,"resultcount":1,"publishedfiledetails":[
			{"publishedfileid":"1","result":9}
		]}}`))
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	got, err := app.Search(context.Background(), SearchQuery{Term: "1", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v, want no match for result=9 (not found)", got)
	}
}

// TestSteamVersionsAlwaysEmpty is the headline behavior: Workshop content
// has no HTTP download URL, so Versions must report zero versions — not one
// Version with an empty Files slice — even when the upstream item resolves
// successfully. A lone Files-less Version would instead surface a
// populated version dropdown next to a permanently-disabled Install
// button; the dashboard's empty state is keyed on zero *versions*.
func TestSteamVersionsAlwaysEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"result":1,"resultcount":1,"publishedfiledetails":[
			{"publishedfileid":"555","result":1,"title":"Wiremod","consumer_app_id":4000}
		]}}`))
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	got, err := app.Versions(context.Background(), "555", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v, want zero versions — Workshop content has no HTTP download URL", got)
	}
}

func TestSteamVersionsUnknownID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":{"result":1,"resultcount":1,"publishedfiledetails":[
			{"publishedfileid":"1","result":9}
		]}}`))
	}))
	defer srv.Close()

	app := testSteamApp(srv.URL, 4000)
	got, err := app.Versions(context.Background(), "1", Filter{})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty for an unknown id", got)
	}
}

func TestSteamModpackDepsIsNoOp(t *testing.T) {
	app := testSteamApp("http://unused.invalid", 4000)
	files, err := app.ModpackDeps(context.Background(), "1")
	if err != nil || files != nil {
		t.Fatalf("ModpackDeps = %v, %v; want nil, nil", files, err)
	}
}

// TestSteamKeyNeverLeaksOnTransportError is the B1 regression test: Steam's
// QueryFiles takes the API key as a query parameter, and Go's
// *url.Error.Error() embeds the full request URL — query string included.
// A transport-level failure (DNS blip, TLS failure, timeout, connection
// refused — simulated here by closing the listener before the request)
// must never let that key escape into the returned error, since the error
// can reach an HTTP response body (httperr.WriteCode on a 5xx) or a log
// line that flows to the audit/syslog/S3 sinks.
func TestSteamKeyNeverLeaksOnTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedURL := srv.URL
	srv.Close() // listener now closed; any request to closedURL fails to connect.

	s := newSteam(&http.Client{Timeout: 5 * time.Second}, "gameplane-test", "s3cret-steam-key")
	s.baseURL = closedURL
	app := &steamApp{steam: s, appID: 4000}

	_, err := app.Search(context.Background(), SearchQuery{Term: "wire", Limit: 10})
	if err == nil {
		t.Fatal("expected a transport error against a closed listener")
	}
	if strings.Contains(err.Error(), "s3cret-steam-key") {
		t.Fatalf("error leaks the API key: %v", err)
	}
}

// TestSetSteamKeyGating proves the end-to-end claim: steam is hidden with
// no key configured and becomes available/selectable once one exists,
// exactly like curseforge's existing key-gating.
func TestSetSteamKeyGating(t *testing.T) {
	ctx := context.Background()

	noKey := NewSet("test", StaticKeys(map[string]string{}))
	if noKey.Available(ctx, "steam") {
		t.Error("steam should be unavailable without a key")
	}
	if _, ok := noKey.For(ctx, Config{Provider: "steam", SteamAppID: 4000}); ok {
		t.Error("steam without a key should not be selectable")
	}

	withKey := NewSet("test", StaticKeys(map[string]string{"steam": "steam-key"}))
	if !withKey.Available(ctx, "steam") {
		t.Error("steam should be available with a key")
	}
	if _, ok := withKey.For(ctx, Config{Provider: "steam", SteamAppID: 4000}); !ok {
		t.Error("steam with a key and an appID should be selectable")
	}
	// A key alone isn't enough — the template must also facet to an app.
	if _, ok := withKey.For(ctx, Config{Provider: "steam"}); ok {
		t.Error("steam without a SteamAppID should not be selectable")
	}
}
