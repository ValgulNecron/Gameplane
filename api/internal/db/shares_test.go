package db

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func newShareLinksStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func insertTestUser(t *testing.T, s *Store, username string) int64 {
	t.Helper()
	res, err := s.DB.Exec(`INSERT INTO users(username, role) VALUES (?, ?)`, username, "admin")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

func TestCreateShareLink_Success(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "alice")

	// Create a link.
	expiresAt := time.Now().Add(24 * time.Hour)
	rawToken, link, err := s.CreateShareLink(ctx, "default", "minecraft-server", userID, true, expiresAt)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Validate the returned link metadata.
	if link.Namespace != "default" {
		t.Errorf("namespace=%q, want default", link.Namespace)
	}
	if link.ServerName != "minecraft-server" {
		t.Errorf("server_name=%q, want minecraft-server", link.ServerName)
	}
	if link.CreatedBy != userID {
		t.Errorf("created_by=%d, want %d", link.CreatedBy, userID)
	}
	if !link.CanStart {
		t.Errorf("can_start=%v, want true", link.CanStart)
	}
	if link.RevokedAt != nil {
		t.Errorf("revoked_at=%v, want nil", link.RevokedAt)
	}

	// Validate the raw token format (32 bytes base64-url-encoded).
	// Base64 encoding of 32 bytes produces ~43 characters.
	if len(rawToken) < 40 || len(rawToken) > 50 {
		t.Errorf("raw token length=%d, expected ~43 (32 bytes base64)", len(rawToken))
	}

	// Verify the token is URL-safe base64 (no + or /).
	if strings.ContainsAny(rawToken, "+/") {
		t.Errorf("raw token contains non-URL-safe base64: %q", rawToken)
	}
}

func TestCreateShareLink_ExpiryInPast_Rejected(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "bob")

	// Try to create with an expiry in the past.
	expiresAt := time.Now().Add(-1 * time.Hour)
	_, _, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err == nil {
		t.Fatal("expected error for expiry in past, got nil")
	}
	if !errors.Is(err, ErrShareLinkExpiryInvalid) {
		t.Errorf("got %v, want ErrShareLinkExpiryInvalid", err)
	}
}

func TestCreateShareLink_ExpiryZero_Rejected(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "bob2")

	// Try to create with a zero expiry.
	_, _, err := s.CreateShareLink(ctx, "default", "server", userID, false, time.Time{})
	if err == nil {
		t.Fatal("expected error for zero expiry, got nil")
	}
	if !errors.Is(err, ErrShareLinkExpiryInvalid) {
		t.Errorf("got %v, want ErrShareLinkExpiryInvalid", err)
	}
}

func TestCreateShareLink_ExpiryExceedsMaximum_Rejected(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "bob3")

	// Try to create with an expiry beyond the maximum.
	expiresAt := time.Now().AddDate(0, 0, MaxShareLinkExpiryDays+1)
	_, _, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err == nil {
		t.Fatal("expected error for expiry exceeding maximum, got nil")
	}
	if !errors.Is(err, ErrShareLinkExpiryInvalid) {
		t.Errorf("got %v, want ErrShareLinkExpiryInvalid", err)
	}
}

func TestCreateShareLink_ExpiryAtMaximumBoundary_Accepted(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "bob4")

	// Create with an expiry exactly at the maximum.
	expiresAt := time.Now().AddDate(0, 0, MaxShareLinkExpiryDays)
	rawToken, link, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err != nil {
		t.Fatalf("create at maximum boundary: %v", err)
	}
	if rawToken == "" {
		t.Error("expected non-empty raw token")
	}
	if link.ID == "" {
		t.Error("expected non-empty link ID")
	}
}

func TestCreateAndLookup_RoundTrip(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "carol")

	// Create a link.
	expiresAt := time.Now().Add(24 * time.Hour)
	rawToken, created, err := s.CreateShareLink(ctx, "default", "server", userID, true, expiresAt)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Lookup the link by raw token.
	looked, err := s.LookupShareLink(ctx, rawToken)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	// Validate returned link matches created link.
	if looked.ID != created.ID {
		t.Errorf("id=%q, want %q", looked.ID, created.ID)
	}
	if looked.Namespace != created.Namespace {
		t.Errorf("namespace=%q, want %q", looked.Namespace, created.Namespace)
	}
	if looked.ServerName != created.ServerName {
		t.Errorf("server_name=%q, want %q", looked.ServerName, created.ServerName)
	}
	if looked.CreatedBy != created.CreatedBy {
		t.Errorf("created_by=%d, want %d", looked.CreatedBy, created.CreatedBy)
	}
	if looked.CanStart != created.CanStart {
		t.Errorf("can_start=%v, want %v", looked.CanStart, created.CanStart)
	}
}

func TestLookupShareLink_RawTokenNotStored(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "dave")

	// Create a link.
	expiresAt := time.Now().Add(24 * time.Hour)
	rawToken, _, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Query the raw row to verify the token_hash is a hash, not the token itself.
	var tokenHash string
	err = s.DB.QueryRow(`SELECT token_hash FROM share_links LIMIT 1`).Scan(&tokenHash)
	if err != nil {
		t.Fatalf("query token_hash: %v", err)
	}

	// The token_hash must be a SHA256 hex string (64 hex chars).
	// It should NOT be the raw token (which is much shorter base64).
	if tokenHash == rawToken {
		t.Error("token_hash is the raw token (should be hashed)")
	}

	// The token_hash should look like a SHA256 hex string (lowercase hex, 64 chars).
	if len(tokenHash) != 64 {
		t.Errorf("token_hash length=%d, want 64 (SHA256 hex)", len(tokenHash))
	}

	// Verify it's valid hex.
	if _, err := hex.DecodeString(tokenHash); err != nil {
		t.Errorf("token_hash is not valid hex: %v", err)
	}

	// Verify the stored hash matches the expected hash of the raw token.
	expectedHash := hashShareLinkToken(rawToken)
	if tokenHash != expectedHash {
		t.Errorf("token_hash=%q, expected %q", tokenHash, expectedHash)
	}
}

func TestLookupShareLink_Unknown_Invalid(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()

	// Lookup a completely bogus token.
	_, err := s.LookupShareLink(ctx, "nonexistent-token-xxx")
	if err != ErrShareLinkInvalid {
		t.Errorf("lookup unknown token: got %v, want ErrShareLinkInvalid", err)
	}
}

func TestLookupShareLink_Expired_Invalid(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "eve")

	// Create a link that expires immediately.
	expiresAt := time.Now().Add(-1 * time.Second)
	// Temporarily allow this by inserting directly to bypass validation.
	rawToken := generateShareLinkToken()
	tokenHash := hashShareLinkToken(rawToken)
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO share_links(id, namespace, server_name, created_by, can_start, token_hash, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"test-id", "default", "server", userID, 0, tokenHash, expiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert expired link: %v", err)
	}

	// Lookup must fail with ErrShareLinkInvalid.
	_, err = s.LookupShareLink(ctx, rawToken)
	if err != ErrShareLinkInvalid {
		t.Errorf("lookup expired: got %v, want ErrShareLinkInvalid", err)
	}
}

func TestLookupShareLink_Revoked_Invalid(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "frank")

	// Create a valid link.
	expiresAt := time.Now().Add(24 * time.Hour)
	rawToken, link, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Revoke it.
	if err := s.RevokeShareLink(ctx, link.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Lookup must fail with ErrShareLinkInvalid.
	_, err = s.LookupShareLink(ctx, rawToken)
	if err != ErrShareLinkInvalid {
		t.Errorf("lookup revoked: got %v, want ErrShareLinkInvalid", err)
	}
}

func TestLookupShareLink_UnknownExpiredRevoked_Same_Error(t *testing.T) {
	// Verify that unknown, expired, and revoked all return the same error,
	// so an attacker cannot distinguish between them.
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "grace")

	// Unknown token.
	_, unknownErr := s.LookupShareLink(ctx, "unknown-xxx")

	// Expired link.
	expiresAt := time.Now().Add(-1 * time.Second)
	expiredToken := generateShareLinkToken()
	expiredHash := hashShareLinkToken(expiredToken)
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO share_links(id, namespace, server_name, created_by, can_start, token_hash, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"expired-id", "default", "server", userID, 0, expiredHash, expiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert expired: %v", err)
	}
	_, expiredErr := s.LookupShareLink(ctx, expiredToken)

	// Revoked link.
	validToken := generateShareLinkToken()
	validHash := hashShareLinkToken(validToken)
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO share_links(id, namespace, server_name, created_by, can_start, token_hash, expires_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		"revoked-id", "default", "server", userID, 0, validHash, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert revoked: %v", err)
	}
	_, revokedErr := s.LookupShareLink(ctx, validToken)

	// All three must return the same error.
	if unknownErr != ErrShareLinkInvalid {
		t.Errorf("unknown: got %v, want ErrShareLinkInvalid", unknownErr)
	}
	if expiredErr != ErrShareLinkInvalid {
		t.Errorf("expired: got %v, want ErrShareLinkInvalid", expiredErr)
	}
	if revokedErr != ErrShareLinkInvalid {
		t.Errorf("revoked: got %v, want ErrShareLinkInvalid", revokedErr)
	}
}

func TestListShareLinks_Scoped(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "henry")

	// Create multiple links for different servers.
	exp := time.Now().Add(24 * time.Hour)

	// Server A in default namespace.
	_, _, err := s.CreateShareLink(ctx, "default", "server-a", userID, true, exp)
	if err != nil {
		t.Fatalf("create 1: %v", err)
	}

	// Server A in default namespace (another link).
	_, _, err = s.CreateShareLink(ctx, "default", "server-a", userID, false, exp)
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}

	// Server B in default namespace.
	_, _, err = s.CreateShareLink(ctx, "default", "server-b", userID, true, exp)
	if err != nil {
		t.Fatalf("create 3: %v", err)
	}

	// Server A in other namespace.
	_, _, err = s.CreateShareLink(ctx, "other", "server-a", userID, false, exp)
	if err != nil {
		t.Fatalf("create 4: %v", err)
	}

	// List for default/server-a must return exactly 2 links.
	links, err := s.ListShareLinks(ctx, "default", "server-a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 2 {
		t.Errorf("list default/server-a: got %d links, want 2", len(links))
	}

	// Verify all returned links match the namespace and server.
	for _, link := range links {
		if link.Namespace != "default" || link.ServerName != "server-a" {
			t.Errorf("list scoping failed: got %q/%q, want default/server-a", link.Namespace, link.ServerName)
		}
	}

	// List for default/server-b must return exactly 1 link.
	links, err = s.ListShareLinks(ctx, "default", "server-b")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("list default/server-b: got %d links, want 1", len(links))
	}

	// List for nonexistent server must return empty.
	links, err = s.ListShareLinks(ctx, "default", "nonexistent")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("list nonexistent: got %d links, want 0", len(links))
	}
}

func TestTouchShareLink_Updates_LastUsed(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "ivy")

	// Create a link.
	expiresAt := time.Now().Add(24 * time.Hour)
	_, link, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Initially last_used must be NULL.
	var lastUsedStr sql.NullString
	err = s.DB.QueryRow(`SELECT last_used FROM share_links WHERE id = ?`, link.ID).Scan(&lastUsedStr)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if lastUsedStr.Valid {
		t.Error("expected last_used to be NULL initially")
	}

	// Touch the link.
	if err := s.TouchShareLink(ctx, link.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// Now last_used should be set.
	err = s.DB.QueryRow(`SELECT last_used FROM share_links WHERE id = ?`, link.ID).Scan(&lastUsedStr)
	if err != nil {
		t.Fatalf("query after touch: %v", err)
	}
	if !lastUsedStr.Valid {
		t.Error("expected last_used to be set after touch")
	}

	// Parse the timestamp to ensure it's reasonable.
	lastUsed, err := time.Parse(time.RFC3339, lastUsedStr.String)
	if err != nil {
		t.Fatalf("parse last_used: %v", err)
	}

	// Verify it's recent (within 5 seconds).
	if time.Since(lastUsed) > 5*time.Second {
		t.Errorf("last_used=%v is too old", lastUsed)
	}
}

func TestLookupShareLink_DoesNotUpdateLastUsed(t *testing.T) {
	// Verify that LookupShareLink is a pure read and does NOT update last_used.
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "jack")

	// Create a link.
	expiresAt := time.Now().Add(24 * time.Hour)
	rawToken, link, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Initially last_used must be NULL.
	var lastUsedBefore sql.NullString
	err = s.DB.QueryRow(`SELECT last_used FROM share_links WHERE id = ?`, link.ID).Scan(&lastUsedBefore)
	if err != nil {
		t.Fatalf("query before: %v", err)
	}
	if lastUsedBefore.Valid {
		t.Error("expected last_used to be NULL before lookup")
	}

	// Lookup the link.
	_, err = s.LookupShareLink(ctx, rawToken)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}

	// last_used should still be NULL (lookup is a pure read).
	var lastUsedAfter sql.NullString
	err = s.DB.QueryRow(`SELECT last_used FROM share_links WHERE id = ?`, link.ID).Scan(&lastUsedAfter)
	if err != nil {
		t.Fatalf("query after: %v", err)
	}
	if lastUsedAfter.Valid {
		t.Error("expected last_used to remain NULL after lookup (lookup must not update it)")
	}
}

func TestRevokeShareLink_Success(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()
	userID := insertTestUser(t, s, "karl")

	// Create a link.
	expiresAt := time.Now().Add(24 * time.Hour)
	_, link, err := s.CreateShareLink(ctx, "default", "server", userID, false, expiresAt)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Revoke it.
	if err := s.RevokeShareLink(ctx, link.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Verify revoked_at is now set.
	var revokedAtStr sql.NullString
	err = s.DB.QueryRow(`SELECT revoked_at FROM share_links WHERE id = ?`, link.ID).Scan(&revokedAtStr)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !revokedAtStr.Valid {
		t.Error("expected revoked_at to be set after revoke")
	}

	// Parse the timestamp.
	revokedAt, err := time.Parse(time.RFC3339, revokedAtStr.String)
	if err != nil {
		t.Fatalf("parse revoked_at: %v", err)
	}

	// Verify it's recent.
	if time.Since(revokedAt) > 5*time.Second {
		t.Errorf("revoked_at=%v is too old", revokedAt)
	}
}

func TestRevokeShareLink_NotFound(t *testing.T) {
	s := newShareLinksStore(t)
	ctx := context.Background()

	// Try to revoke a nonexistent link.
	err := s.RevokeShareLink(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent link, got nil")
	}
}
