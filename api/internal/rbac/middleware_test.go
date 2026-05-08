package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
)

func TestMiddleware_Unauthenticated(t *testing.T) {
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestMiddleware_AllowsViewerRead(t *testing.T) {
	called := false
	h := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(204)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/servers", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Role: RoleViewer}))
	h.ServeHTTP(rr, req)
	if rr.Code != 204 || !called {
		t.Fatalf("code=%d called=%v", rr.Code, called)
	}
}

func TestMiddleware_DeniesViewerWrite(t *testing.T) {
	h := Middleware()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/servers", nil)
	req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Role: RoleViewer}))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestFirstSegment(t *testing.T) {
	cases := map[string]string{
		"/":            "",
		"/servers":     "servers",
		"/servers/foo": "servers",
		"users":        "users",
	}
	for in, want := range cases {
		if got := firstSegment(in); got != want {
			t.Errorf("firstSegment(%q)=%q want %q", in, got, want)
		}
	}
}
