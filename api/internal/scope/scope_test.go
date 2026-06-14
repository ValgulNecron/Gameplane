package scope

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestResolve(t *testing.T) {
	saved := AllowedNamespaces
	t.Cleanup(func() { AllowedNamespaces = saved })
	AllowedNamespaces = []string{DefaultNamespace, "extra"}

	t.Run("missing param returns default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		got, err := Resolve(req)
		if err != nil || got != DefaultNamespace {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("default explicitly allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?namespace="+DefaultNamespace, nil)
		got, err := Resolve(req)
		if err != nil || got != DefaultNamespace {
			t.Fatalf("got %q err=%v", got, err)
		}
	})

	t.Run("allow-listed extra namespace resolves", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/?namespace=extra", nil)
		got, err := Resolve(req)
		if err != nil || got != "extra" {
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
