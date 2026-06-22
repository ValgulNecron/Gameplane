package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
