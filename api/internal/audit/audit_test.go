package audit

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
)

func newStore(t *testing.T) *db.Store {
	t.Helper()
	s, err := db.Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

// insertEvent writes one audit row with an explicit ts so retention tests can
// place events on either side of a cutoff. ts uses the same RFC3339 format the
// middleware writes.
func insertEvent(t *testing.T, s *db.Store, ts time.Time) {
	t.Helper()
	if _, err := s.DB.ExecContext(context.Background(),
		`INSERT INTO audit_events(ts, actor, method, path, status) VALUES (?, 'tester', 'POST', '/x', 200)`,
		ts.UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func countEvents(t *testing.T, s *db.Store) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_events`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestPrune(t *testing.T) {
	s := newStore(t)
	a := New(s)
	now := time.Now().UTC()

	insertEvent(t, s, now.Add(-48*time.Hour)) // old, should be pruned
	insertEvent(t, s, now.Add(-25*time.Hour)) // old, should be pruned
	insertEvent(t, s, now.Add(-1*time.Hour))  // recent, should survive

	deleted, err := a.Prune(context.Background(), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}
	if got := countEvents(t, s); got != 1 {
		t.Errorf("remaining = %d, want 1", got)
	}
}

func TestPrune_NothingOlderThanCutoff(t *testing.T) {
	s := newStore(t)
	a := New(s)
	now := time.Now().UTC()
	insertEvent(t, s, now.Add(-1*time.Hour))

	deleted, err := a.Prune(context.Background(), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
	if got := countEvents(t, s); got != 1 {
		t.Errorf("remaining = %d, want 1", got)
	}
}

// RunRetention with a non-positive window is a no-op that returns immediately;
// it must not prune anything or block.
func TestRunRetention_DisabledIsNoOp(t *testing.T) {
	s := newStore(t)
	a := New(s)
	insertEvent(t, s, time.Now().UTC().Add(-1000*time.Hour))

	a.RunRetention(context.Background(), 0, time.Hour,
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	if got := countEvents(t, s); got != 1 {
		t.Errorf("remaining = %d, want 1 (disabled retention must not prune)", got)
	}
}

// RunRetention sweeps once immediately on start, before the first tick. The
// context must stay live during that sweep (a pre-cancelled one would cancel
// the prune's DB op), so it runs in a goroutine and is cancelled only after
// the start-up prune is observed.
func TestRunRetention_SweepsOnStart(t *testing.T) {
	s := newStore(t)
	a := New(s)
	insertEvent(t, s, time.Now().UTC().Add(-48*time.Hour))

	// A long interval means the only sweep is the immediate start-up one.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		a.RunRetention(ctx, 24*time.Hour, time.Hour,
			slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()

	// The start-up sweep runs before the loop blocks on the ticker; poll
	// until it prunes (generous deadline keeps this non-flaky).
	deadline := time.Now().Add(2 * time.Second)
	for countEvents(t, s) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("start-up sweep did not prune within deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
}

func TestMiddleware_StdoutSink(t *testing.T) {
	s := newStore(t)
	var buf strings.Builder
	a := New(s, WithStdoutSink(slog.New(slog.NewJSONHandler(&buf, nil))))

	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	req := httptest.NewRequest("POST", "/api/v1/servers?name=alpha", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Username: "admin"}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	for _, want := range []string{
		`"actor":"admin"`, `"method":"POST"`, `"path":"/api/v1/servers"`,
		`"target":"alpha"`, `"status":201`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sink output missing %s in: %s", want, out)
		}
	}
	// The sink mirrors, it does not replace the DB record.
	if got := countEvents(t, s); got != 1 {
		t.Errorf("db events = %d, want 1", got)
	}
}

func TestMiddleware_NoSinkByDefault(t *testing.T) {
	s := newStore(t)
	a := New(s) // no WithStdoutSink → sink disabled
	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/api/v1/x", nil)
	h.ServeHTTP(httptest.NewRecorder(), req) // must not panic without a sink
	if got := countEvents(t, s); got != 1 {
		t.Errorf("db events = %d, want 1", got)
	}
}

func TestShouldLog(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{"GET", "/anything", false},
		{"HEAD", "/anything", false},
		{"OPTIONS", "/anything", false},
		{"POST", "/healthz", false},
		{"POST", "/metrics", false},
		{"POST", "/auth/oidc/callback", false},
		{"POST", "/api/v1/servers", true},
		{"DELETE", "/api/v1/servers/x", true},
		{"PUT", "/api/v1/users/admin", true},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if got := shouldLog(req); got != tc.want {
			t.Errorf("shouldLog(%s %s)=%v want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestMiddleware_RecordsMutatingRequest(t *testing.T) {
	s := newStore(t)
	a := New(s)

	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("POST", "/api/v1/servers?name=alpha", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Username: "admin"}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	rows, err := s.DB.Query(`SELECT actor, method, path, target, status FROM audit_events`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var got []struct {
		Actor, Method, Path, Target string
		Status                      int
	}
	for rows.Next() {
		var r struct {
			Actor, Method, Path, Target string
			Status                      int
		}
		if err := rows.Scan(&r.Actor, &r.Method, &r.Path, &r.Target, &r.Status); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 1 || got[0].Actor != "admin" || got[0].Status != 201 || got[0].Target != "alpha" {
		t.Fatalf("got %+v", got)
	}
}

func TestMiddleware_AnonymousActorWhenNoUser(t *testing.T) {
	s := newStore(t)
	a := New(s)
	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("POST", "/api/v1/anything", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	var actor string
	if err := s.DB.QueryRow(`SELECT actor FROM audit_events`).Scan(&actor); err != nil {
		t.Fatalf("query: %v", err)
	}
	if actor != "anonymous" {
		t.Fatalf("actor=%q", actor)
	}
}

func TestMiddleware_SkipsReads(t *testing.T) {
	s := newStore(t)
	a := New(s)
	h := Middleware(a)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest("GET", "/api/v1/servers", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&n)
	if n != 0 {
		t.Fatalf("expected no rows, got %d", n)
	}
}

func TestPage(t *testing.T) {
	s := newStore(t)
	a := New(s)
	for i := 0; i < 5; i++ {
		_, err := s.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, target, status, ip)
			VALUES (?, ?, 'POST', '/x', '', 201, '')`, "2026-01-0"+itoa(i+1)+"T00:00:00Z", "u")
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	req := httptest.NewRequest("GET", "/", nil)

	t.Run("default limit", func(t *testing.T) {
		got, err := a.Page(req, 0, 0) // limit<=0 → defaulted to 100
		if err != nil {
			t.Fatalf("page: %v", err)
		}
		if len(got) != 5 {
			t.Fatalf("got %d", len(got))
		}
	})

	t.Run("limit", func(t *testing.T) {
		got, err := a.Page(req, 2, 0)
		if err != nil {
			t.Fatalf("page: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d", len(got))
		}
	})

	t.Run("oversized limit defaulted", func(t *testing.T) {
		got, err := a.Page(req, 9999, 0)
		if err != nil {
			t.Fatalf("page: %v", err)
		}
		if len(got) > 100 {
			t.Fatalf("got %d (default cap is 100)", len(got))
		}
	})

	t.Run("before cursor", func(t *testing.T) {
		got, err := a.Page(req, 100, 3)
		if err != nil {
			t.Fatalf("page: %v", err)
		}
		for _, e := range got {
			if e.ID >= 3 {
				t.Fatalf("got id %d, expected <3", e.ID)
			}
		}
	})
}

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSON(rr, []string{"a", "b"})
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", rr.Header().Get("Content-Type"))
	}
	var got []string
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
