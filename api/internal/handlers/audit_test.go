package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kestrel-gg/kestrel/api/internal/audit"
)

func TestMountAudit_HappyPath(t *testing.T) {
	store := newTestStore(t)
	a := audit.New(store)
	for i := 0; i < 3; i++ {
		_, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, target, status, ip)
			VALUES (?, 'admin', 'POST', '/x', '', 201, '')`, "2026-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, a)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/admin/audit?limit=2", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var got []audit.Event
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d", len(got))
	}
}

func TestMountAudit_DBError(t *testing.T) {
	store := newTestStore(t)
	a := audit.New(store)
	// Drop the table so Page errors out.
	if _, err := store.DB.ExecContext(context.Background(), `DROP TABLE audit_events`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	r := chi.NewRouter()
	MountAudit(r, a)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/admin/audit", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
