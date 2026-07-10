package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/audit"
	"github.com/ValgulNecron/gameplane/api/internal/auth"
)

// auditGet issues a GET against a MountAudit test server and returns the response.
func auditGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return resp
}

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

// TestMountAudit_Verify exercises the tamper-evidence endpoint through
// MountAudit + the RBAC-covered path prefix: a clean chain reports OK, and a
// DB-level tamper is caught and reported with the offending row id.
func TestMountAudit_Verify(t *testing.T) {
	store := newTestStore(t)
	a := audit.New(store)
	h := audit.Middleware(a)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i := 0; i < 3; i++ {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/servers", nil))
	}

	r := chi.NewRouter()
	MountAudit(r, a)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/verify")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var result audit.VerifyResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result.OK || result.Checked != 3 {
		t.Fatalf("result = %+v, want OK with Checked=3", result)
	}

	// Tamper directly at the DB level, then verify again through the endpoint.
	if _, err := store.DB.Exec(`UPDATE audit_events SET actor = 'attacker' WHERE id = 2`); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	resp2 := auditGet(t, srv.URL+"/admin/audit/verify")
	defer resp2.Body.Close()
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.OK || result.FirstBadID != 2 {
		t.Fatalf("result = %+v, want a break reported at row 2", result)
	}
}

// TestMountAudit_VerifyDBError — Verify's own query failure (not just Page's)
// must also surface as a 500 through the endpoint, not a panic or a false OK.
func TestMountAudit_VerifyDBError(t *testing.T) {
	store := newTestStore(t)
	a := audit.New(store)
	if _, err := store.DB.ExecContext(context.Background(), `DROP TABLE audit_events`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	r := chi.NewRouter()
	MountAudit(r, a)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/verify")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", resp.StatusCode)
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

func TestMountAudit_ExportCSV(t *testing.T) {
	store := newTestStore(t)
	for i := 0; i < 3; i++ {
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, target, status, ip)
			VALUES (?, 'admin', 'POST', '/servers', 'mc-1', 201, '10.0.0.1')`,
			"2026-01-0"+strconv.Itoa(i+1)+"T00:00:00Z"); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type=%q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".csv") {
		t.Fatalf("content-disposition=%q", cd)
	}
	recs, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(recs) != 4 { // header + 3 rows
		t.Fatalf("rows=%d, want 4", len(recs))
	}
	if recs[0][0] != "id" || recs[0][2] != "actor" {
		t.Fatalf("header=%v", recs[0])
	}
	// Oldest-first: the first data row is the earliest ts.
	if recs[1][1] != "2026-01-01T00:00:00Z" || recs[1][2] != "admin" {
		t.Fatalf("first row=%v", recs[1])
	}
}

func TestMountAudit_ExportJSON(t *testing.T) {
	store := newTestStore(t)
	for i := 0; i < 2; i++ {
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, status)
			VALUES (?, 'admin', 'POST', '/x', 201)`, "2026-01-0"+strconv.Itoa(i+1)+"T00:00:00Z"); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?format=json")
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type=%q", ct)
	}
	var got []audit.Event
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2", len(got))
	}
}

func TestMountAudit_ExportBadFormat(t *testing.T) {
	store := newTestStore(t)
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?format=xml")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", resp.StatusCode)
	}
}

func TestMountAudit_ExportTimeWindow(t *testing.T) {
	store := newTestStore(t)
	for _, ts := range []string{"2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", "2026-03-01T00:00:00Z"} {
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, status)
			VALUES (?, 'admin', 'POST', '/x', 201)`, ts); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?format=json&since=2026-01-15T00:00:00Z&until=2026-02-15T00:00:00Z")
	defer resp.Body.Close()
	var got []audit.Event
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].TS != "2026-02-01T00:00:00Z" {
		t.Fatalf("window export = %+v, want only the Feb event", got)
	}
}

func TestMountAudit_ExportFilteredByActor(t *testing.T) {
	store := newTestStore(t)
	rows := []struct {
		actor  string
		method string
		status int
	}{
		{"alice", "POST", 201},
		{"bob", "DELETE", 204},
		{"Alice", "PATCH", 200},
	}
	for i, row := range rows {
		ts := "2026-01-0" + strconv.Itoa(i+1) + "T00:00:00Z"
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, status)
			VALUES (?, ?, ?, '/x', ?)`, ts, row.actor, row.method, row.status); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?format=json&actor=alice")
	defer resp.Body.Close()
	var got []audit.Event
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (case-insensitive match of alice + Alice)", len(got))
	}
	if got[0].Actor != "alice" || got[1].Actor != "Alice" {
		t.Fatalf("actors = %q/%q, want alice/Alice", got[0].Actor, got[1].Actor)
	}
}

func TestMountAudit_ExportFilteredByMethod(t *testing.T) {
	store := newTestStore(t)
	rows := []struct {
		actor  string
		method string
		status int
	}{
		{"admin", "POST", 201},
		{"admin", "DELETE", 204},
		{"admin", "PATCH", 200},
	}
	for i, row := range rows {
		ts := "2026-01-0" + strconv.Itoa(i+1) + "T00:00:00Z"
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, status)
			VALUES (?, ?, ?, '/x', ?)`, ts, row.actor, row.method, row.status); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?format=json&method=DELETE")
	defer resp.Body.Close()
	var got []audit.Event
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1 DELETE", len(got))
	}
	if got[0].Method != "DELETE" {
		t.Fatalf("method = %q, want DELETE", got[0].Method)
	}
}

func TestMountAudit_ExportFilteredByStatus(t *testing.T) {
	store := newTestStore(t)
	rows := []struct {
		actor  string
		status int
	}{
		{"admin", 200},
		{"admin", 201},
		{"admin", 400},
		{"admin", 500},
		{"admin", 502},
	}
	for i, row := range rows {
		ts := "2026-01-0" + strconv.Itoa(i+1) + "T00:00:00Z"
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, status)
			VALUES (?, ?, 'POST', '/x', ?)`, ts, row.actor, row.status); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// Test 2xx filter
	resp := auditGet(t, srv.URL+"/admin/audit/export?format=json&status=2xx")
	defer resp.Body.Close()
	var got []audit.Event
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode 2xx: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("2xx: got %d events, want 2", len(got))
	}
	for _, e := range got {
		if e.Status < 200 || e.Status >= 300 {
			t.Fatalf("2xx filter: got status %d, want 200-299", e.Status)
		}
	}

	// Test 5xx filter
	resp = auditGet(t, srv.URL+"/admin/audit/export?format=json&status=5xx")
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode 5xx: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("5xx: got %d events, want 2", len(got))
	}
	for _, e := range got {
		if e.Status < 500 || e.Status >= 600 {
			t.Fatalf("5xx filter: got status %d, want 500-599", e.Status)
		}
	}
}

func TestMountAudit_ExportLiteralPercentInActor(t *testing.T) {
	store := newTestStore(t)
	actors := []string{"a%b", "axxb"}
	for i, actor := range actors {
		ts := "2026-01-0" + strconv.Itoa(i+1) + "T00:00:00Z"
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, status)
			VALUES (?, ?, 'POST', '/x', 200)`, ts, actor); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?format=json&actor=a%25b")
	defer resp.Body.Close()
	var got []audit.Event
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1 (exact match of a%%b)", len(got))
	}
	if got[0].Actor != "a%b" {
		t.Fatalf("actor = %q, want a%%b", got[0].Actor)
	}
}

func TestMountAudit_ExportBadMethod(t *testing.T) {
	store := newTestStore(t)
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?method=BOGUS")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", resp.StatusCode)
	}
}

func TestMountAudit_ExportBadStatus(t *testing.T) {
	store := newTestStore(t)
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export?status=9xx")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", resp.StatusCode)
	}
}

func TestMountAudit_ExportBareNoFilters(t *testing.T) {
	store := newTestStore(t)
	rows := []struct {
		actor  string
		method string
	}{
		{"alice", "POST"},
		{"bob", "DELETE"},
		{"charlie", "PATCH"},
	}
	for i, row := range rows {
		ts := "2026-01-0" + strconv.Itoa(i+1) + "T00:00:00Z"
		if _, err := store.DB.Exec(`INSERT INTO audit_events(ts, actor, method, path, status)
			VALUES (?, ?, ?, '/x', 201)`, ts, row.actor, row.method); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	r := chi.NewRouter()
	MountAudit(r, audit.New(store))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp := auditGet(t, srv.URL+"/admin/audit/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, want 200", resp.StatusCode)
	}
	recs, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(recs) != 4 { // header + 3 rows
		t.Fatalf("got %d records, want 4 (header + 3 data rows)", len(recs))
	}
}
