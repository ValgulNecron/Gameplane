package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ShareLink is a signed, expiring, revocable token granting read access to a
// single GameServer's status and connection address, optionally with start capability.
type ShareLink struct {
	ID         string     // unique identifier
	Namespace  string     // Kubernetes namespace
	ServerName string     // GameServer name
	CreatedBy  int64      // user ID who created it
	CanStart   bool       // whether the link grants start capability
	ExpiresAt  time.Time  // mandatory expiry timestamp
	RevokedAt  *time.Time // NULL = active; set for audit trail
	CreatedAt  time.Time  // when created
	LastUsed   *time.Time // when last accessed via LookupShareLink
}

// ErrShareLinkInvalid is returned by LookupShareLink when a token is unknown,
// expired, or revoked. The caller cannot distinguish between these cases —
// an attacker probing tokens must not learn that one existed.
var ErrShareLinkInvalid = errors.New("invalid share link")

// ErrShareLinkExpiryInvalid is returned by CreateShareLink when the expiry
// timestamp is invalid (zero, in the past, or beyond the maximum allowed).
var ErrShareLinkExpiryInvalid = errors.New("share link expiry invalid")

// MaxShareLinkExpiryDays is the maximum lifetime in days for a share link.
const MaxShareLinkExpiryDays = 90

// CreateShareLink mints a new share link and returns the raw token, which is
// never stored and never recoverable afterwards. Expiry is mandatory and must
// be in the future, and must not exceed MaxShareLinkExpiryDays.
func (s *Store) CreateShareLink(ctx context.Context, ns, serverName string, createdBy int64, canStart bool, expiresAt time.Time) (rawToken string, link ShareLink, err error) {
	// Validate expiry.
	now := time.Now()
	if expiresAt.IsZero() {
		return "", ShareLink{}, fmt.Errorf("share link expiry: %w", ErrShareLinkExpiryInvalid)
	}
	if expiresAt.Before(now) {
		return "", ShareLink{}, fmt.Errorf("share link expiry in the past: %w", ErrShareLinkExpiryInvalid)
	}
	maxExpiry := now.AddDate(0, 0, MaxShareLinkExpiryDays)
	if expiresAt.After(maxExpiry) {
		return "", ShareLink{}, fmt.Errorf("share link expiry exceeds maximum: %w", ErrShareLinkExpiryInvalid)
	}

	// Generate a random ID (12 bytes base64-encoded).
	id := generateShareLinkID()

	// Generate a random token (32 bytes base64-encoded).
	rawToken = generateShareLinkToken()

	// Hash the token with SHA-256.
	tokenHash := hashShareLinkToken(rawToken)

	// Insert into the database.
	canStartInt := 0
	if canStart {
		canStartInt = 1
	}
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO share_links(id, namespace, server_name, created_by, can_start, token_hash, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, ns, serverName, createdBy, canStartInt, tokenHash, expiresAt.Format(time.RFC3339))
	if err != nil {
		return "", ShareLink{}, fmt.Errorf("insert share link: %w", err)
	}

	// Return the raw token (never persisted) and the link metadata.
	link = ShareLink{
		ID:         id,
		Namespace:  ns,
		ServerName: serverName,
		CreatedBy:  createdBy,
		CanStart:   canStart,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now(),
	}

	return rawToken, link, nil
}

// LookupShareLink resolves a raw token. It returns ErrShareLinkInvalid for an
// unknown, expired, or revoked token — the caller cannot distinguish these cases.
// This is a pure read operation. Call TouchShareLink separately to update last_used.
func (s *Store) LookupShareLink(ctx context.Context, rawToken string) (ShareLink, error) {
	// Hash the token.
	tokenHash := hashShareLinkToken(rawToken)

	// Query by hash for O(1) lookup.
	var link ShareLink
	var canStartInt int
	var revokedAtStr sql.NullString
	var lastUsedStr sql.NullString
	var expiresAtStr string

	err := s.DB.QueryRowContext(ctx,
		`SELECT id, namespace, server_name, created_by, can_start, expires_at, revoked_at, created_at, last_used
		 FROM share_links WHERE token_hash = ?`,
		tokenHash).Scan(
		&link.ID,
		&link.Namespace,
		&link.ServerName,
		&link.CreatedBy,
		&canStartInt,
		&expiresAtStr,
		&revokedAtStr,
		&link.CreatedAt,
		&lastUsedStr,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return ShareLink{}, ErrShareLinkInvalid
	}
	if err != nil {
		return ShareLink{}, fmt.Errorf("lookup share link: %w", err)
	}

	// Parse timestamps.
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		return ShareLink{}, fmt.Errorf("parse expires_at: %w", err)
	}
	link.ExpiresAt = expiresAt

	// Parse revoked_at if present.
	if revokedAtStr.Valid {
		revokedAt, err := time.Parse(time.RFC3339, revokedAtStr.String)
		if err != nil {
			return ShareLink{}, fmt.Errorf("parse revoked_at: %w", err)
		}
		link.RevokedAt = &revokedAt
	}

	// Parse last_used if present.
	if lastUsedStr.Valid {
		lastUsed, err := time.Parse(time.RFC3339, lastUsedStr.String)
		if err != nil {
			return ShareLink{}, fmt.Errorf("parse last_used: %w", err)
		}
		link.LastUsed = &lastUsed
	}

	// Convert can_start from int.
	link.CanStart = canStartInt != 0

	// Check if the link is revoked.
	if link.RevokedAt != nil {
		return ShareLink{}, ErrShareLinkInvalid
	}

	// Check if the link is expired.
	if time.Now().After(link.ExpiresAt) {
		return ShareLink{}, ErrShareLinkInvalid
	}

	return link, nil
}

// ListShareLinks returns all share links for a given server (namespace, server_name),
// active and revoked alike. Results are ordered by created_at descending.
func (s *Store) ListShareLinks(ctx context.Context, ns, serverName string) ([]ShareLink, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, namespace, server_name, created_by, can_start, expires_at, revoked_at, created_at, last_used
		 FROM share_links WHERE namespace = ? AND server_name = ?
		 ORDER BY created_at DESC`,
		ns, serverName)
	if err != nil {
		return nil, fmt.Errorf("list share links: %w", err)
	}
	defer rows.Close()

	var links []ShareLink
	for rows.Next() {
		var link ShareLink
		var canStartInt int
		var revokedAtStr sql.NullString
		var lastUsedStr sql.NullString
		var expiresAtStr string
		var createdAtStr string

		if err := rows.Scan(
			&link.ID,
			&link.Namespace,
			&link.ServerName,
			&link.CreatedBy,
			&canStartInt,
			&expiresAtStr,
			&revokedAtStr,
			&createdAtStr,
			&lastUsedStr,
		); err != nil {
			return nil, fmt.Errorf("scan share link: %w", err)
		}

		// Parse timestamps.
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse expires_at: %w", err)
		}
		link.ExpiresAt = expiresAt

		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		link.CreatedAt = createdAt

		// Parse revoked_at if present.
		if revokedAtStr.Valid {
			revokedAt, err := time.Parse(time.RFC3339, revokedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("parse revoked_at: %w", err)
			}
			link.RevokedAt = &revokedAt
		}

		// Parse last_used if present.
		if lastUsedStr.Valid {
			lastUsed, err := time.Parse(time.RFC3339, lastUsedStr.String)
			if err != nil {
				return nil, fmt.Errorf("parse last_used: %w", err)
			}
			link.LastUsed = &lastUsed
		}

		// Convert can_start from int.
		link.CanStart = canStartInt != 0

		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate share links: %w", err)
	}

	return links, nil
}

// RevokeShareLink marks a share link as revoked by setting revoked_at to the
// current timestamp. Revocation is auditable (never a delete).
func (s *Store) RevokeShareLink(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE share_links SET revoked_at = datetime('now') WHERE id = ?`,
		id)
	if err != nil {
		return fmt.Errorf("revoke share link: %w", err)
	}

	// Check if the row was found.
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("share link not found: %w", errors.New("unknown id"))
	}

	return nil
}

// TouchShareLink updates the last_used timestamp for a share link, recording
// when it was last accessed. Used by LookupShareLink internally.
func (s *Store) TouchShareLink(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE share_links SET last_used = datetime('now') WHERE id = ?`,
		id)
	if err != nil {
		return fmt.Errorf("touch share link: %w", err)
	}
	return nil
}

// generateShareLinkID generates a random 12-byte string encoded as base64-url.
func generateShareLinkID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateShareLinkToken generates a random 32-byte token encoded as base64-url.
func generateShareLinkToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// hashShareLinkToken returns the hex-encoded SHA-256 hash of the raw token.
func hashShareLinkToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}
