package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ValgulNecron/gameplane/api/internal/db"
)

func newAuthDB(t *testing.T) *db.Store {
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

func seedUser(t *testing.T, s *db.Store, username, pw, role string) {
	t.Helper()
	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	_, err = s.DB.Exec(
		`INSERT INTO users(username, display_name, email, role, pw_hash) VALUES (?,?,?,?,?)`,
		username, username, username+"@example.com", role, hash,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestLogin_RejectsNonJSONContentType(t *testing.T) {
	s := newAuthDB(t)
	l := NewLocal(s)
	ss := NewSessionStore(s)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("user=alice"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	l.HandleLogin(ss).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestLogin_BadJSON(t *testing.T) {
	s := newAuthDB(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	NewLocal(s).HandleLogin(NewSessionStore(s)).ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestLogin_UnknownUser_TimingPath(t *testing.T) {
	s := newAuthDB(t)
	body := strings.NewReader(`{"username":"ghost","password":"hunter2"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/json")
	NewLocal(s).HandleLogin(NewSessionStore(s)).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "rightpw", "admin")
	body := strings.NewReader(`{"username":"alice","password":"wrong"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/json")
	NewLocal(s).HandleLogin(NewSessionStore(s)).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestLogin_Success_SetsCookies(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "hunter2", "admin")
	body := strings.NewReader(`{"username":"alice","password":"hunter2"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/json")
	NewLocal(s).HandleLogin(NewSessionStore(s)).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body)
	}
	var got loginResp
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.User.Username != "alice" || got.CSRF == "" {
		t.Fatalf("got %+v", got)
	}
	cookies := rr.Result().Cookies()
	var session, csrf bool
	for _, c := range cookies {
		if c.Name == sessionCookie {
			session = true
		}
		if c.Name == csrfCookie {
			csrf = true
		}
	}
	if !session || !csrf {
		t.Fatalf("cookies not set: %+v", cookies)
	}
}

func TestLogin_PerUserRateLimit(t *testing.T) {
	s := newAuthDB(t)
	seedUser(t, s, "alice", "hunter2", "admin")
	// Drain the per-user bucket for "alice".
	for i := 0; i < 6; i++ {
		LoginUserLimiter.AllowUser("alice")
	}
	body := strings.NewReader(`{"username":"alice","password":"hunter2"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", body)
	req.Header.Set("Content-Type", "application/json")
	NewLocal(s).HandleLogin(NewSessionStore(s)).ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("code=%d", rr.Code)
	}
}
