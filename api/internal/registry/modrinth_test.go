package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testModrinth(url string) *Modrinth {
	m := newModrinth(&http.Client{Timeout: 5 * time.Second}, "kestrel-test")
	m.baseURL = url
	return m
}

func TestModrinthSearch(t *testing.T) {
	var gotFacets, gotQuery, gotLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %q, want /search", r.URL.Path)
		}
		gotFacets = r.URL.Query().Get("facets")
		gotQuery = r.URL.Query().Get("query")
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[
			{"project_id":"AABBCC","slug":"sodium","title":"Sodium","description":"Fast","author":"jelly","icon_url":"https://cdn.modrinth.com/i.png","downloads":1234},
			{"project_id":"DDEEFF","title":"NoSlug","author":"who","downloads":5}
		]}`))
	}))
	defer srv.Close()

	got, err := testModrinth(srv.URL).Search(context.Background(), SearchQuery{
		Term: "perf", Loader: "fabric", GameVersion: "1.21.4", Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotQuery != "perf" || gotLimit != "10" {
		t.Errorf("query=%q limit=%q", gotQuery, gotLimit)
	}
	if want := `[["categories:fabric"],["versions:1.21.4"]]`; gotFacets != want {
		t.Errorf("facets = %q, want %q", gotFacets, want)
	}
	if len(got) != 2 {
		t.Fatalf("len(hits) = %d, want 2", len(got))
	}
	if got[0].ID != "AABBCC" || got[0].Title != "Sodium" || got[0].Downloads != 1234 {
		t.Errorf("hit[0] = %+v", got[0])
	}
	if got[0].Provider != "modrinth" {
		t.Errorf("provider = %q", got[0].Provider)
	}
	if got[0].PageURL != "https://modrinth.com/project/sodium" {
		t.Errorf("pageURL = %q", got[0].PageURL)
	}
	// No slug → page URL falls back to the project id.
	if got[1].PageURL != "https://modrinth.com/project/DDEEFF" {
		t.Errorf("hit[1] pageURL = %q", got[1].PageURL)
	}
}

func TestModrinthSearchNoFacets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f := r.URL.Query().Get("facets"); f != "" {
			t.Errorf("facets should be empty, got %q", f)
		}
		_, _ = w.Write([]byte(`{"hits":[]}`))
	}))
	defer srv.Close()

	got, err := testModrinth(srv.URL).Search(context.Background(), SearchQuery{Term: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestModrinthVersions(t *testing.T) {
	var gotLoaders, gotGV string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/project/sodium/version" {
			t.Errorf("path = %q", r.URL.Path)
		}
		gotLoaders = r.URL.Query().Get("loaders")
		gotGV = r.URL.Query().Get("game_versions")
		_, _ = w.Write([]byte(`[
			{"id":"old","name":"v1","version_number":"1.0","game_versions":["1.21.4"],"loaders":["fabric"],"date_published":"2024-01-01T00:00:00Z","files":[{"url":"https://cdn.modrinth.com/a.jar","filename":"a.jar","primary":true,"size":10}]},
			{"id":"new","name":"v2","version_number":"2.0","game_versions":["1.21.4"],"loaders":["fabric"],"date_published":"2025-06-01T00:00:00Z","files":[{"url":"https://cdn.modrinth.com/b.jar","filename":"b.jar","primary":true,"size":20}]}
		]`))
	}))
	defer srv.Close()

	got, err := testModrinth(srv.URL).Versions(context.Background(), "sodium", Filter{Loader: "fabric", GameVersion: "1.21.4"})
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if gotLoaders != `["fabric"]` || gotGV != `["1.21.4"]` {
		t.Errorf("loaders=%q game_versions=%q", gotLoaders, gotGV)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Newest first (date_published 2025 before 2024).
	if got[0].ID != "new" {
		t.Errorf("first id = %q, want new (newest-first)", got[0].ID)
	}
	if len(got[0].Files) != 1 || got[0].Files[0].DownloadURL != "https://cdn.modrinth.com/b.jar" {
		t.Errorf("files = %+v", got[0].Files)
	}
}

func TestModrinthUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := testModrinth(srv.URL).Search(context.Background(), SearchQuery{Term: "x"}); err == nil {
		t.Fatal("expected error on 500")
	}
}
