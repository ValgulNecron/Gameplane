package auth

import (
	"context"
	"strings"
	"testing"
)

// TestResolveOrLinkUser_FirstLoginCreatesUser exercises the
// "no existing link → create user + oidc_link" branch.
func TestResolveOrLinkUser_FirstLoginCreatesUser(t *testing.T) {
	store := newAuthDB(t)
	o := &OIDC{}
	o.AttachStore(store)

	u, err := o.resolveOrLinkUser(context.Background(), "https://idp", "sub-1", "alice@x", "Alice", "viewer", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u.Username != "alice@x" || u.Role != "viewer" {
		t.Fatalf("got %+v", u)
	}

	// Calling again must hit the existing-link path and return the same row.
	u2, err := o.resolveOrLinkUser(context.Background(), "https://idp", "sub-1", "alice@x", "Alice", "viewer", false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if u2.ID != u.ID {
		t.Fatalf("expected reuse, got new id %d vs %d", u2.ID, u.ID)
	}
}

// TestResolveOrLinkUser_FallsBackToSubWhenEmailEmpty covers the
// `if baseUsername == "" { baseUsername = sub }` branch.
func TestResolveOrLinkUser_FallsBackToSubWhenEmailEmpty(t *testing.T) {
	store := newAuthDB(t)
	o := &OIDC{}
	o.AttachStore(store)
	u, err := o.resolveOrLinkUser(context.Background(), "https://idp", "subsub", "", "Anon", "viewer", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u.Username != "subsub" {
		t.Fatalf("got %q", u.Username)
	}
}

// TestPickUniqueUsername_Disambiguates covers the "username collision
// → suffix with sub" branch.
func TestPickUniqueUsername_Disambiguates(t *testing.T) {
	store := newAuthDB(t)
	// Seed a local user with the same base username an OIDC sign-in
	// would default to.
	if _, err := store.DB.Exec(
		`INSERT INTO users(username, display_name, email, role) VALUES (?, ?, ?, 'admin')`,
		"alice@x", "Alice", "alice@x",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	o := &OIDC{}
	o.AttachStore(store)
	u, err := o.resolveOrLinkUser(
		context.Background(), "https://idp", "subject-12345678ab", "alice@x", "Alice IdP", "viewer", false,
	)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u.Username == "alice@x" {
		t.Fatalf("expected suffixed username, got %q", u.Username)
	}
	if !strings.Contains(u.Username, "subject-") {
		t.Fatalf("expected suffix containing sub fragment, got %q", u.Username)
	}
}

// TestPickUniqueUsername_ShortSubKeepsAll covers the "len(suffix)<=8 →
// no truncation" branch.
func TestPickUniqueUsername_ShortSubKeepsAll(t *testing.T) {
	store := newAuthDB(t)
	if _, err := store.DB.Exec(
		`INSERT INTO users(username, display_name, email, role) VALUES (?, ?, ?, 'admin')`,
		"x", "X", "x",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	o := &OIDC{}
	o.AttachStore(store)
	u, err := o.resolveOrLinkUser(context.Background(), "https://idp", "abc", "x", "X", "viewer", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(u.Username, "abc") {
		t.Fatalf("expected full sub in username, got %q", u.Username)
	}
}
