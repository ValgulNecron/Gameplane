package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

// seedAccountRows gives userID one row in every table tied to an account and
// one active share link, and returns that link's raw token.
func seedAccountRows(t *testing.T, s *Store, userID int64, tag string) string {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO oidc_links(user_id, issuer, subject, email) VALUES (?, ?, ?, ?)`,
			[]any{userID, "https://idp.example", "sub-" + tag, tag + "@example.com"}},
		{`INSERT INTO user_preferences(user_id) VALUES (?)`, []any{userID}},
		{`INSERT INTO sessions(token, user_id, csrf_token, expires_at) VALUES (?, ?, ?, ?)`,
			[]any{"session-" + tag, userID, "csrf-" + tag, time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}},
		{`INSERT INTO api_tokens(token, user_id, name) VALUES (?, ?, ?)`, []any{"api-token-" + tag, userID, tag}},
		{`INSERT INTO user_role_bindings(user_id, role_name, cluster, namespace) VALUES (?, 'viewer', 'local', '*')`, []any{userID}},
	} {
		if _, err := s.DB.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatalf("seed %q: %v", stmt.sql, err)
		}
	}
	token, _, err := s.CreateShareLink(ctx, "local", "default", "server-"+tag, userID, false, nil)
	if err != nil {
		t.Fatalf("seed share link: %v", err)
	}
	return token
}

// accountRowCount counts userID's rows across the tables tied to an account.
func accountRowCount(t *testing.T, s *Store, userID int64) int {
	t.Helper()
	total := 0
	for _, q := range []string{
		`SELECT COUNT(*) FROM users WHERE id = ?`,
		`SELECT COUNT(*) FROM oidc_links WHERE user_id = ?`,
		`SELECT COUNT(*) FROM user_preferences WHERE user_id = ?`,
		`SELECT COUNT(*) FROM sessions WHERE user_id = ?`,
		`SELECT COUNT(*) FROM api_tokens WHERE user_id = ?`,
		`SELECT COUNT(*) FROM user_role_bindings WHERE user_id = ?`,
	} {
		var n int
		if err := s.DB.QueryRowContext(context.Background(), q, userID).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		total += n
	}
	return total
}

func TestDeleteUser_RemovesAccountRowsAndRevokesShareLinks(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	gone := insertTestUser(t, s, "leaving")
	kept := insertTestUser(t, s, "staying")
	goneToken := seedAccountRows(t, s, gone, "leaving")
	keptToken := seedAccountRows(t, s, kept, "staying")

	if err := s.DeleteUser(ctx, gone); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	if n := accountRowCount(t, s, gone); n != 0 {
		t.Errorf("deleted user still has %d account rows", n)
	}
	if _, err := s.LookupShareLink(ctx, goneToken); !errors.Is(err, ErrShareLinkInvalid) {
		t.Errorf("deleted user's share link: got %v, want ErrShareLinkInvalid", err)
	}
	var revoked int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM share_links WHERE created_by = ? AND revoked_at IS NOT NULL`, gone).Scan(&revoked); err != nil {
		t.Fatalf("count revoked links: %v", err)
	}
	if revoked != 1 {
		t.Errorf("revoked share links for deleted user = %d, want 1 (revoked, not deleted)", revoked)
	}

	// Another account's rows and links are untouched.
	if n := accountRowCount(t, s, kept); n != 6 {
		t.Errorf("other user's account rows = %d, want 6", n)
	}
	if _, err := s.LookupShareLink(ctx, keptToken); err != nil {
		t.Errorf("other user's share link no longer resolves: %v", err)
	}
}

func TestDeleteUser_UnknownIDIsNoOp(t *testing.T) {
	s := newShareLinksStore(t)
	if err := s.DeleteUser(context.Background(), 424242); err != nil {
		t.Fatalf("DeleteUser(unknown): %v", err)
	}
}

func TestMigrate_AccountCleanupClearsRowsOfRemovedUsers(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	gone := insertTestUser(t, s, "removed-earlier")
	kept := insertTestUser(t, s, "still-here")
	goneToken := seedAccountRows(t, s, gone, "removed-earlier")
	keptToken := seedAccountRows(t, s, kept, "still-here")

	// A user row removed on its own, the way user deletes worked before
	// DeleteUser, leaves its other rows behind.
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, gone); err != nil {
		t.Fatalf("delete users row: %v", err)
	}

	// Run the cleanup migration again over that state.
	const name = "012_account_removal_cleanup.sql"
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = ?`, name); err != nil {
		t.Fatalf("unmark migration: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if n := accountRowCount(t, s, gone); n != 0 {
		t.Errorf("removed user still has %d account rows after cleanup", n)
	}
	if _, err := s.LookupShareLink(ctx, goneToken); !errors.Is(err, ErrShareLinkInvalid) {
		t.Errorf("removed user's share link after cleanup: got %v, want ErrShareLinkInvalid", err)
	}
	links, err := s.ListShareLinks(ctx, "local", "default", "server-removed-earlier")
	if err != nil {
		t.Fatalf("list share links: %v", err)
	}
	if len(links) != 1 || links[0].RevokedAt == nil {
		t.Errorf("removed user's share link should be kept and revoked, got %+v", links)
	}

	if n := accountRowCount(t, s, kept); n != 6 {
		t.Errorf("other user's account rows = %d, want 6", n)
	}
	if _, err := s.LookupShareLink(ctx, keptToken); err != nil {
		t.Errorf("other user's share link no longer resolves: %v", err)
	}
}
