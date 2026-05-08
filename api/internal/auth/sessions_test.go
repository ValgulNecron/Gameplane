package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessions_CreateAndLookup(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)

	tok, csrf, err := store.Create(context.Background(), 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tok == csrf || tok == "" || csrf == "" {
		t.Fatalf("tokens look bad: %q %q", tok, csrf)
	}

	u, gotCSRF, err := store.lookup(context.Background(), tok)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if u.Username != "alice" || gotCSRF != csrf {
		t.Fatalf("lookup got %+v csrf=%q", u, gotCSRF)
	}
}

func TestSessions_Lookup_Expired(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)
	tok, _, err := store.Create(context.Background(), 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Force-expire the row directly.
	if _, err := s.DB.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`,
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), tok); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, _, err := store.lookup(context.Background(), tok); err == nil ||
		!strings.Contains(err.Error(), "expired") {
		t.Fatalf("got %v", err)
	}
}

func TestSessions_Lookup_CorruptExpires(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)
	tok, _, _ := store.Create(context.Background(), 1)
	if _, err := s.DB.Exec(`UPDATE sessions SET expires_at = 'garbage' WHERE token = ?`, tok); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, _, err := store.lookup(context.Background(), tok); err == nil ||
		!strings.Contains(err.Error(), "invalid") {
		t.Fatalf("got %v", err)
	}
	// Row should also be deleted.
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token = ?`, tok).Scan(&n)
	if n != 0 {
		t.Fatalf("corrupt row not removed (%d rows remaining)", n)
	}
}

func TestSessions_Authenticate(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)
	tok, csrf, _ := store.Create(context.Background(), 1)

	called := false
	h := store.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if u := UserFromContext(req.Context()); u == nil || u.Username != "alice" {
			t.Errorf("user not in ctx: %+v", u)
		}
		called = true
		w.WriteHeader(204)
	}))

	t.Run("no cookie returns 401", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("unknown token returns 401", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "ghost"})
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("read passes without csrf", func(t *testing.T) {
		called = false
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
		h.ServeHTTP(rr, req)
		if rr.Code != 204 || !called {
			t.Fatalf("code=%d called=%v", rr.Code, called)
		}
	})

	t.Run("mutating without csrf returns 403", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("mutating with valid csrf passes", func(t *testing.T) {
		called = false
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
		req.Header.Set(csrfHeader, csrf)
		h.ServeHTTP(rr, req)
		if rr.Code != 204 || !called {
			t.Fatalf("code=%d called=%v", rr.Code, called)
		}
	})
}

func TestSessions_DeleteForUser(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)
	_, _, _ = store.Create(context.Background(), 1)
	_, _, _ = store.Create(context.Background(), 1)
	if err := store.DeleteForUser(context.Background(), 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id=1`).Scan(&n)
	if n != 0 {
		t.Fatalf("expected 0 sessions, got %d", n)
	}
}

func TestSessions_HandleLogout(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)
	tok, _, _ := store.Create(context.Background(), 1)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
	store.HandleLogout().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("code=%d", rr.Code)
	}
	// Cookies cleared (MaxAge=-1).
	var clearedSession, clearedCSRF bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge < 0 {
			clearedSession = true
		}
		if c.Name == csrfCookie && c.MaxAge < 0 {
			clearedCSRF = true
		}
	}
	if !clearedSession || !clearedCSRF {
		t.Fatalf("cookies not cleared: %+v", rr.Result().Cookies())
	}
	// Row deleted.
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token=?`, tok).Scan(&n)
	if n != 0 {
		t.Fatalf("session row not deleted")
	}
}

func TestSessions_HandleLogout_NoCookie(t *testing.T) {
	s := newAuthDB(t)
	store := NewSessionStore(s)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/logout", nil)
	store.HandleLogout().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestSessions_GCOnce_DeletesExpired(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)
	tok, _, _ := store.Create(context.Background(), 1)
	_, _ = s.DB.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`,
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), tok)
	store.gcOnce(context.Background())
	var n int
	_ = s.DB.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
	if n != 0 {
		t.Fatalf("gc did not delete: %d remain", n)
	}
}

func TestSessions_StartGC(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "pw", "admin")
	store := NewSessionStore(s)
	tok, _, _ := store.Create(context.Background(), 1)
	_, _ = s.DB.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`,
		time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), tok)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.StartGC(ctx, 20*time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		_ = s.DB.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
		if n == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("StartGC did not delete expired session in time")
}

func TestStartGC_DefaultsZeroInterval(t *testing.T) {
	s := newAuthDB(t)
	store := NewSessionStore(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.StartGC(ctx, 0) // exercises the "<= 0 → 1h default" branch
}

func TestRandomToken_Distinct(t *testing.T) {
	a := randomToken()
	b := randomToken()
	if a == b || a == "" {
		t.Fatalf("tokens not distinct: %q %q", a, b)
	}
}

func TestUserFromContext_Nil(t *testing.T) {
	if u := UserFromContext(context.Background()); u != nil {
		t.Fatalf("expected nil, got %+v", u)
	}
}

func TestIsMutating(t *testing.T) {
	if isMutating(http.MethodGet) || isMutating(http.MethodHead) || isMutating(http.MethodOptions) {
		t.Fatal("read methods should be non-mutating")
	}
	if !isMutating(http.MethodPost) || !isMutating(http.MethodPut) ||
		!isMutating(http.MethodPatch) || !isMutating(http.MethodDelete) {
		t.Fatal("write methods should be mutating")
	}
}
