package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/api/internal/db"
)

// bareStore is a migrated store with no telemetry config row (telStore
// seeds one; some branches need its absence).
func bareStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), "sqlite", "file:"+filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func TestNew_DefaultsInterval(t *testing.T) {
	r := New(bareStore(t), telKube(), "http://x", "", "v1", 0)
	if r.interval != 24*time.Hour {
		t.Fatalf("interval = %v, want the 24h default for a non-positive interval", r.interval)
	}
}

func TestEnabled_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("absent config is disabled", func(t *testing.T) {
		r := New(bareStore(t), telKube(), "http://x", "", "v1", time.Hour)
		if r.enabled(ctx) {
			t.Fatal("want disabled when there is no telemetry config row")
		}
	})

	t.Run("malformed config is disabled", func(t *testing.T) {
		store := bareStore(t)
		if _, err := store.DB.Exec(
			`INSERT INTO config(key, value, updated_at) VALUES ('telemetry', '{bad', ?)`,
			"2026-01-01T00:00:00Z"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		r := New(store, telKube(), "http://x", "", "v1", time.Hour)
		if r.enabled(ctx) {
			t.Fatal("want disabled for a malformed telemetry config")
		}
	})
}

func TestCount_UnknownKind(t *testing.T) {
	r := New(bareStore(t), telKube(), "http://x", "", "v1", time.Hour)
	n, err := r.count(context.Background(), "bogus")
	if err != nil || n != 0 {
		t.Fatalf("count(bogus) = %d, %v; want 0, nil", n, err)
	}
}

func TestReportOnce_EndpointErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	r := New(telStore(t, true), telKube(), srv.URL, "", "v1", time.Hour)
	if err := r.reportOnce(context.Background()); err == nil {
		t.Fatal("want an error when the telemetry endpoint returns 500")
	}
}
