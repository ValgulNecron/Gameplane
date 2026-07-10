// Package db wraps the API's user/session/audit store. SQLite is the
// default so homelab installs have zero external dependencies; Postgres
// is opt-in via --db-driver=postgres --db-dsn=...
//
// Schema migrations live alongside this package as plain .sql files,
// applied in filename order at startup.
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	// Pure-Go SQLite driver; registered as "sqlite" via database/sql.
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	DB     *sql.DB
	Driver string
}

func Open(ctx context.Context, driver, dsn string) (*Store, error) {
	switch driver {
	case "sqlite":
		// Before opening the database, attempt to adopt any legacy kestrel.db file.
		// This is safe because we only rename if the target does not exist.
		if err := adoptLegacySQLite(dsn); err != nil {
			return nil, err
		}

		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(1) // modernc sqlite + WAL is safe with multiple readers; cap for simplicity
		if err := db.PingContext(ctx); err != nil {
			return nil, err
		}
		return &Store{DB: db, Driver: "sqlite"}, nil
	case "postgres":
		// pgx driver lives in a separate build tag to keep the default
		// binary small; see db_postgres.go for the registration.
		return openPostgres(ctx, dsn)
	default:
		return nil, fmt.Errorf("unknown db driver %q", driver)
	}
}

func (s *Store) Close() error { return s.DB.Close() }

// Migrate applies every .sql file under migrations/ in order. Each
// file is run in a single transaction; failures are fatal.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`,
	); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := s.migrationApplied(ctx, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		content, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := s.runMigration(ctx, name, string(content)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) migrationApplied(ctx context.Context, name string) (bool, error) {
	var v string
	err := s.DB.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version=?`, name).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) runMigration(ctx context.Context, name, sqlText string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range splitStatements(sqlText) {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, applied_at) VALUES (?, datetime('now'))`, name,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// splitStatements is a quick-and-dirty splitter that breaks on ";\n".
// Good enough for the short migrations we'll ship; replace with a real
// parser if we start needing dollar-quoted PL/pgSQL blocks.
func splitStatements(s string) []string {
	parts := strings.Split(s, ";\n")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// sqlitePath extracts the filesystem path from a SQLite DSN.
// Returns "" for non-file DSNs (e.g., :memory:, file::memory:).
// Handles DSN formats like "file:/path/db.db?_pragma=..." and bare "/path/db.db".
func sqlitePath(dsn string) string {
	// Strip the "file:" prefix if present.
	if strings.HasPrefix(dsn, "file:") {
		dsn = dsn[5:]
	}

	// If it's a memory database, return empty.
	if dsn == ":memory:" || strings.HasPrefix(dsn, ":memory:") {
		return ""
	}

	// Strip query parameters (everything from the first ?).
	if idx := strings.IndexByte(dsn, '?'); idx != -1 {
		dsn = dsn[:idx]
	}

	// If nothing remains, it's not a valid file path.
	if dsn == "" {
		return ""
	}

	return dsn
}

// adoptLegacySQLite renames kestrel.db to the target DSN path if the target
// doesn't exist but the legacy file does. This handles Kestrel → Gameplane
// upgrades where the DSN changed from kestrel.db to gameplane.db.
// Also moves the -wal WAL sidecar if present (but not -shm, which is rebuilt).
// Returns a fatal error if the rename fails; silently succeeds if either
// the target exists (do not overwrite) or the legacy file is absent.
func adoptLegacySQLite(dsn string) error {
	targetPath := sqlitePath(dsn)

	// Not a file-based SQLite database; nothing to adopt.
	if targetPath == "" {
		return nil
	}

	// Check if target already exists. If it does, never overwrite.
	if _, err := os.Stat(targetPath); err == nil {
		// Target exists; nothing to do.
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		// Some other error (permission, etc.); propagate it.
		return fmt.Errorf("checking target file %s: %w", targetPath, err)
	}

	// Target does not exist. Check for legacy kestrel.db in the same directory.
	dir := filepath.Dir(targetPath)
	legacyPath := filepath.Join(dir, "kestrel.db")

	legacyInfo, err := os.Stat(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		// Legacy file does not exist either; this is a fresh install.
		return nil
	} else if err != nil {
		// Some error other than "not exist"; propagate it.
		return fmt.Errorf("checking legacy file %s: %w", legacyPath, err)
	}

	// Guard: legacy must be a regular file, not a directory or other type.
	// A directory would be renamed into place, then sql.Open would fail confusingly.
	if !legacyInfo.Mode().IsRegular() {
		// Not a regular file; skip adoption silently (leave it alone).
		return nil
	}

	// Legacy file exists and target does not. Move the -wal sidecar FIRST (if present),
	// then the main database file. This ordering ensures that a partial failure always
	// leaves the original kestrel.db intact, so the next startup re-adopts cleanly
	// instead of silently skipping adoption with an orphaned WAL.
	legacyWAL := legacyPath + "-wal"
	targetWAL := targetPath + "-wal"
	if _, err := os.Stat(legacyWAL); err == nil {
		// WAL sidecar exists; move it first.
		if err := os.Rename(legacyWAL, targetWAL); err != nil {
			return fmt.Errorf("rename %s to %s: %w", legacyWAL, targetWAL, err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		// Some error other than "not exist"; propagate it.
		return fmt.Errorf("checking legacy WAL %s: %w", legacyWAL, err)
	}

	// Now rename the main database file (this is the point of no return for the rename pair).
	if err := os.Rename(legacyPath, targetPath); err != nil {
		return fmt.Errorf("rename %s to %s: %w", legacyPath, targetPath, err)
	}

	// Note: -shm (shared memory index) is not moved because SQLite rebuilds it automatically.
	// Moving it adds a failure mode for zero benefit.

	slog.Warn("adopted legacy SQLite database", "old", legacyPath, "new", targetPath)
	return nil
}
