package scope

import (
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
)

func TestResolve(t *testing.T) {
	saved := AllowedNamespaces
	t.Cleanup(func() { AllowedNamespaces = saved })
	AllowedNamespaces = []string{DefaultNamespace, "extra"}

	makeReq := func(ns string, role string) *url.Values {
		_ = ns
		_ = role
		return nil
	}
	_ = makeReq

	t.Run("missing param returns default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		got, err := Resolve(req)
		if err != nil || got != DefaultNamespace {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("default explicitly allowed for any role", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?namespace="+DefaultNamespace, nil)
		got, err := Resolve(req)
		if err != nil || got != DefaultNamespace {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("unknown namespace forbidden", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?namespace=other", nil)
		_, err := Resolve(req)
		if !errors.Is(err, ErrForbiddenNamespace) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("non-default rejected for viewer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?namespace=extra", nil)
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Role: "viewer"}))
		_, err := Resolve(req)
		if !errors.Is(err, ErrForbiddenNamespace) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("non-default rejected when no user in ctx", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?namespace=extra", nil)
		_, err := Resolve(req)
		if !errors.Is(err, ErrForbiddenNamespace) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("non-default allowed for operator", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?namespace=extra", nil)
		req = req.WithContext(auth.WithUser(req.Context(), &auth.User{Role: "operator"}))
		got, err := Resolve(req)
		if err != nil || got != "extra" {
			t.Fatalf("got %q err=%v", got, err)
		}
	})
}

func TestContains(t *testing.T) {
	if contains(nil, "x") {
		t.Fatal("empty slice should not contain anything")
	}
	if !contains([]string{"a", "b", "c"}, "b") {
		t.Fatal("expected hit")
	}
	if contains([]string{"a", "b"}, "z") {
		t.Fatal("expected miss")
	}
}
