package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestConfig_GetAll_DBError forces the QueryContext to fail by dropping
// the config table out from under the handler.
func TestConfig_GetAll_DBError(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.DB.ExecContext(context.Background(), `DROP TABLE config`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	r := chi.NewRouter()
	MountConfig(r, store, false)
	rr := newRR()
	r.ServeHTTP(rr, httpReq("GET", "/admin/config/", ""))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestConfig_GetAll_SkipsUnknownKeys verifies the loop's "skip rows
// for sections we no longer recognize" branch.
func TestConfig_GetAll_SkipsUnknownKeys(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.DB.ExecContext(context.Background(),
		`INSERT INTO config(key, value, updated_at) VALUES ('legacy', '{"x":1}', datetime('now'))`,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	r := chi.NewRouter()
	MountConfig(r, store, false)
	rr := newRR()
	r.ServeHTTP(rr, httpReq("GET", "/admin/config/", ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestConfig_Put_DBError tries to write to a dropped table so the
// upsert fails with a 5xx (covers the httperr.Write path on Exec err).
func TestConfig_Put_DBError(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.DB.ExecContext(context.Background(), `DROP TABLE config`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	r := chi.NewRouter()
	MountConfig(r, store, false)
	rr := newRR()
	r.ServeHTTP(rr, httpReq("PUT", "/admin/config/general", `{"instanceName":"k","defaultNamespace":"n"}`))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
}
