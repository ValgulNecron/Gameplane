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
