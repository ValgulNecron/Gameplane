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

func TestStream_StatusFilterSentinel(t *testing.T) {
	s := newStore(t)
	a := New(s)
	ts := time.Now().UTC().Format(time.RFC3339)

	// Insert one 2xx event and one 5xx event
	if _, err := s.DB.ExecContext(context.Background(),
		`INSERT INTO audit_events(ts, actor, method, path, status) VALUES (?, 'tester', 'POST', '/x', 200)`,
		ts,
	); err != nil {
		t.Fatalf("insert 2xx: %v", err)
	}

	if _, err := s.DB.ExecContext(context.Background(),
		`INSERT INTO audit_events(ts, actor, method, path, status) VALUES (?, 'tester', 'POST', '/y', 500)`,
		ts,
	); err != nil {
		t.Fatalf("insert 5xx: %v", err)
	}

	// Call Stream directly with StatusMin=0, StatusMax=299 (meaning "filter to 2xx")
	// This exercises the sentinel: StatusMax should gate the filter, not StatusMin.
	filter := StreamFilter{StatusMin: 0, StatusMax: 299}
	var got []Event
	err := a.Stream(context.Background(), filter, func(e Event) error {
		got = append(got, e)
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Status != 200 {
		t.Fatalf("got status %d, want 200", got[0].Status)
	}
	if got[0].Path != "/x" {
		t.Fatalf("got path %q, want /x", got[0].Path)
	}
}

// ---- tamper-evidence (hash chain) ----

func postEvent(h http.Handler) {
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/servers", nil))
}

// TestInsertChained_LinksConsecutiveRows checks that each row's prev_hash
// equals the previous row's hash, and that hash is exactly the documented
// SHA-256(prev_hash || canonical(row)) — not just "some non-empty string".
func TestInsertChained_LinksConsecutiveRows(t *testing.T) {
	s := newStore(t)
	a := New(s)
	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i := 0; i < 3; i++ {
		postEvent(h)
	}

	rows, err := s.DB.Query(
		`SELECT id, ts, actor, method, path, COALESCE(target,''), status, COALESCE(ip,''),
		        COALESCE(prev_hash,''), COALESCE(hash,'')
		 FROM audit_events ORDER BY id ASC`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	expectedPrev := "" // genesis
	count := 0
	for rows.Next() {
		var e Event
		var prevHash, hash string
		if err := rows.Scan(&e.ID, &e.TS, &e.Actor, &e.Method, &e.Path, &e.Target, &e.Status, &e.IP, &prevHash, &hash); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if prevHash != expectedPrev {
			t.Fatalf("row %d: prev_hash=%q, want %q", e.ID, prevHash, expectedPrev)
		}
		if want := computeHash(prevHash, e); want != hash {
			t.Fatalf("row %d: hash=%q, want %q (recomputed)", e.ID, hash, want)
		}
		expectedPrev = hash
		count++
	}
	if count != 3 {
		t.Fatalf("got %d rows, want 3", count)
	}
}

// TestInsertChained_RestartsChainAfterLegacyRow — a row written before
// migration 005 shipped has no hash. The first row inserted afterward must
// restart the chain at genesis (prev_hash ""), not fail or chain off a NULL.
func TestInsertChained_RestartsChainAfterLegacyRow(t *testing.T) {
	s := newStore(t)
	a := New(s)
	insertEvent(t, s, time.Now().UTC()) // pre-chain row: no prev_hash/hash

	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	postEvent(h)

	var prevHash string
	if err := s.DB.QueryRow(
		`SELECT COALESCE(prev_hash,'') FROM audit_events ORDER BY id DESC LIMIT 1`,
	).Scan(&prevHash); err != nil {
		t.Fatalf("query: %v", err)
	}
	if prevHash != "" {
		t.Fatalf("prev_hash = %q, want genesis (empty) right after a legacy row", prevHash)
	}

	// Verify must also skip the legacy row rather than treating it as a break.
	result, err := a.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.OK || result.Checked != 1 {
		t.Fatalf("result = %+v, want OK with Checked=1 (legacy row excluded)", result)
	}
}

func TestVerify_CleanChain(t *testing.T) {
	s := newStore(t)
	a := New(s)
	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i := 0; i < 4; i++ {
		postEvent(h)
	}

	result, err := a.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.OK || result.Checked != 4 {
		t.Fatalf("result = %+v, want OK with Checked=4", result)
	}
}

// TestVerify_DetectsUpdatedRow — a DB-level UPDATE against a chained row
// (bypassing the API entirely) must be caught: the row's stored hash no
// longer matches its recomputed content.
func TestVerify_DetectsUpdatedRow(t *testing.T) {
	s := newStore(t)
	a := New(s)
	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i := 0; i < 3; i++ {
		postEvent(h)
	}

	if _, err := s.DB.Exec(`UPDATE audit_events SET actor = 'attacker' WHERE id = 2`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	result, err := a.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.OK {
		t.Fatal("expected the tampered row to be detected")
	}
	if result.FirstBadID != 2 {
		t.Fatalf("FirstBadID = %d, want 2", result.FirstBadID)
	}
}

// TestVerify_DetectsDeletedRow — a DB-level DELETE removes a link from the
// chain; the next surviving row's prev_hash no longer matches.
func TestVerify_DetectsDeletedRow(t *testing.T) {
	s := newStore(t)
	a := New(s)
	h := Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i := 0; i < 3; i++ {
		postEvent(h)
	}

	if _, err := s.DB.Exec(`DELETE FROM audit_events WHERE id = 2`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	result, err := a.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.OK {
		t.Fatal("expected the deletion to be detected")
	}
	if result.FirstBadID != 3 {
		t.Fatalf("FirstBadID = %d, want 3 (its prev_hash now dangles)", result.FirstBadID)
	}
}

// TestVerify_ChecksOutAfterPruneCheckpoint — Prune deletes old rows but must
// leave a checkpoint behind so the surviving rows still verify, and so a
// subsequent insert keeps chaining off the checkpoint instead of silently
// restarting at genesis.
func TestVerify_ChecksOutAfterPruneCheckpoint(t *testing.T) {
	s := newStore(t)
	a := New(s)
	ctx := context.Background()
	now := time.Now().UTC()

	old := now.Add(-48 * time.Hour)
	for i := 0; i < 3; i++ {
		if err := a.insertChained(ctx, old.Add(time.Duration(i)*time.Minute).Format(time.RFC3339),
			"tester", "POST", "/x", "", 200, ""); err != nil {
			t.Fatalf("insertChained old: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := a.insertChained(ctx, now.Add(time.Duration(i)*time.Minute).Format(time.RFC3339),
			"tester", "POST", "/y", "", 200, ""); err != nil {
			t.Fatalf("insertChained recent: %v", err)
		}
	}

	deleted, err := a.Prune(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}

	result, err := a.Verify(ctx)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.OK {
		t.Fatalf("result = %+v, want a pruned table to still verify via its checkpoint", result)
	}
	if result.Checked != 2 {
		t.Fatalf("Checked = %d, want 2 (only the surviving rows)", result.Checked)
	}

	raw, ok, err := s.ConfigValue(ctx, chainConfigKey)
	if err != nil || !ok {
		t.Fatalf("checkpoint missing: ok=%v err=%v", ok, err)
	}
	if !strings.Contains(raw, `"id"`) || !strings.Contains(raw, `"hash"`) {
		t.Fatalf("checkpoint payload = %q, missing expected fields", raw)
	}

	// A further insert must chain off the checkpoint, not restart at genesis.
	if err := a.insertChained(ctx, now.Add(10*time.Minute).Format(time.RFC3339),
		"tester", "POST", "/z", "", 200, ""); err != nil {
		t.Fatalf("insertChained after prune: %v", err)
	}
	result, err = a.Verify(ctx)
	if err != nil {
		t.Fatalf("verify after further insert: %v", err)
	}
	if !result.OK || result.Checked != 3 {
		t.Fatalf("result = %+v, want OK with Checked=3", result)
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
