package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/audit"
	"github.com/ValgulNecron/gameplane/api/internal/auth"
)

// TestAudit_RecordsAuthenticatedActor — the audit middleware must record
// the acting user (set on the actor holder by Authenticate), not
// "anonymous". Regression test for the context-propagation bug.
func TestAudit_RecordsAuthenticatedActor(t *testing.T) {
	store := newTestStore(t)
	a := audit.New(store)

	// Stand in for Authenticate: set the actor on the holder the audit
	// middleware installed into the request context.
	setActor := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth.SetActor(r.Context(), "alice")
			next.ServeHTTP(w, r)
		})
	}
	final := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })
	h := audit.Middleware(a)(setActor(final))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/servers", nil))

	evs, err := a.Page(httptest.NewRequest(http.MethodGet, "/x", nil), 10, 0)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("want 1 audit row, got %d", len(evs))
	}
	if evs[0].Actor != "alice" {
		t.Fatalf("actor = %q, want alice", evs[0].Actor)
	}
}

// TestAudit_AnonymousWhenUnauthenticated — a request with no
// authenticated actor (e.g. a login attempt) still logs as "anonymous".
func TestAudit_AnonymousWhenUnauthenticated(t *testing.T) {
	store := newTestStore(t)
	a := audit.New(store)
	final := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	h := audit.Middleware(a)(final)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/auth/login", nil))

	evs, err := a.Page(httptest.NewRequest(http.MethodGet, "/x", nil), 10, 0)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if len(evs) != 1 || evs[0].Actor != "anonymous" {
		t.Fatalf("want one anonymous row, got %+v", evs)
	}
}

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
