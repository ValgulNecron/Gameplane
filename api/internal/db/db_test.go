package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen_UnknownDriver(t *testing.T) {
	_, err := Open(context.Background(), "weird", "")
	if err == nil || !strings.Contains(err.Error(), "unknown db driver") {
		t.Fatalf("got %v", err)
	}
}

func TestOpen_SQLiteAndMigrate(t *testing.T) {
	s, err := Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if s.Driver != "sqlite" {
		t.Fatalf("driver=%q", s.Driver)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Idempotent: second Migrate is a no-op.
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	// Schema is in place.
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n < 3 {
		t.Fatalf("expected >=3 migrations applied, got %d", n)
	}
}

func TestMigrate_SeedsBuiltinRoles(t *testing.T) {
	s, err := Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// The three built-in roles exist and are flagged builtin.
	for _, name := range []string{"admin", "operator", "viewer"} {
		var builtin int
		if err := s.DB.QueryRow(`SELECT builtin FROM roles WHERE name = ?`, name).Scan(&builtin); err != nil {
			t.Fatalf("role %q missing: %v", name, err)
		}
		if builtin != 1 {
			t.Errorf("role %q builtin=%d, want 1", name, builtin)
		}
	}

	// admin holds the wildcard; viewer is read-only; operator can write servers.
	for _, tc := range []struct {
		role, perm string
	}{
		{"admin", "*"},
		{"operator", "servers:write"},
		{"operator", "backups:restore"},
		{"viewer", "servers:read"},
	} {
		var got int
		if err := s.DB.QueryRow(
			`SELECT COUNT(*) FROM role_permissions WHERE role_name = ? AND permission = ?`,
			tc.role, tc.perm).Scan(&got); err != nil {
			t.Fatalf("query %s/%s: %v", tc.role, tc.perm, err)
		}
		if got != 1 {
			t.Errorf("role %q missing permission %q", tc.role, tc.perm)
		}
	}

	// viewer must not hold any write/admin permission.
	var writes int
	if err := s.DB.QueryRow(
		`SELECT COUNT(*) FROM role_permissions WHERE role_name = 'viewer'
		   AND permission NOT LIKE '%:read'`).Scan(&writes); err != nil {
		t.Fatalf("viewer writes query: %v", err)
	}
	if writes != 0 {
		t.Errorf("viewer holds %d non-read permissions, want 0", writes)
	}
}

func TestOpen_SQLiteBadDSN(t *testing.T) {
	// modernc sqlite tolerates almost any DSN at Open; pinging is what
	// fails for an unwritable file. Use a path that can't be opened.
	_, err := Open(context.Background(), "sqlite", "file:/nonexistent/dir/x.db?_journal=BAD")
	if err == nil {
		t.Fatal("expected error for unwritable path")
	}
}

func TestRunMigration_RollbackOnFailure(t *testing.T) {
	s, err := Open(context.Background(), "sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Force a failure by running invalid SQL via runMigration.
	err = s.runMigration(context.Background(), "test_bad.sql", "NOT VALID SQL;\n")
	if err == nil {
		t.Fatal("expected migration to fail")
	}
}

func TestSplitStatements(t *testing.T) {
	in := "SELECT 1;\nSELECT 2;\n  \nSELECT 3"
	got := splitStatements(in)
	want := []string{"SELECT 1", "SELECT 2", "SELECT 3"}
	if len(got) != len(want) {
		t.Fatalf("got %d parts: %+v", len(got), got)
	}
	for i, s := range want {
		if got[i] != s {
			t.Errorf("[%d] %q != %q", i, got[i], s)
		}
	}
}

func TestMigrationApplied(t *testing.T) {
	s, _ := Open(context.Background(), "sqlite", ":memory:")
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	got, err := s.migrationApplied(context.Background(), "001_init.sql")
	if err != nil || !got {
		t.Fatalf("applied=%v err=%v", got, err)
	}
	got, err = s.migrationApplied(context.Background(), "999_never.sql")
	if err != nil || got {
		t.Fatalf("expected false for missing, got %v err=%v", got, err)
	}
}

// TestSqlitePath tests DSN path extraction for various formats.
func TestSqlitePath(t *testing.T) {
	tests := []struct {
		dsn  string
		want string
	}{
		{`file:/data/gameplane.db?_pragma=journal_mode(WAL)`, `/data/gameplane.db`},
		{`/data/gameplane.db`, `/data/gameplane.db`},
		{`/data/gameplane.db?mode=rwc`, `/data/gameplane.db`},
		{`:memory:`, ""},
		{`file::memory:`, ""},
		{`file:/tmp/test.db`, `/tmp/test.db`},
		{`file:/tmp/test.db?journal_mode=WAL&_pragma=cache_size(5000)`, `/tmp/test.db`},
		{``, ""},
	}
	for _, tc := range tests {
		t.Run(tc.dsn, func(t *testing.T) {
			got := sqlitePath(tc.dsn)
			if got != tc.want {
				t.Errorf("sqlitePath(%q) = %q, want %q", tc.dsn, got, tc.want)
			}
		})
	}
}

// TestAdoptLegacySQLite_TargetAbsentLegacyPresent tests adoption of kestrel.db.
func TestAdoptLegacySQLite_TargetAbsentLegacyPresent(t *testing.T) {
	tmpdir := t.TempDir()

	// Create a legacy kestrel.db with a table and a row.
	legacyPath := filepath.Join(tmpdir, "kestrel.db")
	legacyDB, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("create legacy db: %v", err)
	}
	if _, err := legacyDB.Exec(`CREATE TABLE test_data (id INTEGER, name TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO test_data VALUES (1, 'alice')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	legacyDB.Close()

	// Now adopt the legacy file to gameplane.db.
	targetPath := filepath.Join(tmpdir, "gameplane.db")
	dsn := "file:" + targetPath + "?_pragma=journal_mode(WAL)"
	if err := adoptLegacySQLite(dsn); err != nil {
		t.Fatalf("adoptLegacySQLite: %v", err)
	}

	// Verify target now exists.
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("target file not found: %v", err)
	}

	// Verify legacy is gone.
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy file still exists or unexpected error: %v", err)
	}

	// Verify data survived the rename.
	targetDB, err := sql.Open("sqlite", targetPath)
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	defer targetDB.Close()
	var id int
	var name string
	if err := targetDB.QueryRow(`SELECT id, name FROM test_data WHERE id = 1`).Scan(&id, &name); err != nil {
		t.Fatalf("query target: %v", err)
	}
	if id != 1 || name != "alice" {
		t.Errorf("data mismatch: got (%d, %q), want (1, alice)", id, name)
	}
}

// TestAdoptLegacySQLite_Sidecars tests that WAL sidecars move with the main file.
func TestAdoptLegacySQLite_Sidecars(t *testing.T) {
	tmpdir := t.TempDir()

	legacyPath := filepath.Join(tmpdir, "kestrel.db")
	targetPath := filepath.Join(tmpdir, "gameplane.db")

	// Create legacy DB with WAL to generate sidecars.
	legacyDB, err := sql.Open("sqlite", legacyPath+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	if _, err := legacyDB.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	legacyDB.Close()

	// Manually create the sidecar files if they don't exist
	// (they may not exist on first creation; we'll create them for testing).
	for _, suffix := range []string{"-wal", "-shm"} {
		sidecarPath := legacyPath + suffix
		if f, err := os.Create(sidecarPath); err == nil {
			f.Close()
		}
	}

	dsn := "file:" + targetPath + "?_pragma=journal_mode(WAL)"
	if err := adoptLegacySQLite(dsn); err != nil {
		t.Fatalf("adoptLegacySQLite: %v", err)
	}

	// Check that sidecars moved.
	for _, suffix := range []string{"-wal", "-shm"} {
		legacySidecar := legacyPath + suffix
		targetSidecar := targetPath + suffix
		if _, err := os.Stat(targetSidecar); err != nil {
			t.Errorf("target sidecar %s not found: %v", targetSidecar, err)
		}
		if _, err := os.Stat(legacySidecar); !os.IsNotExist(err) {
			t.Errorf("legacy sidecar %s still exists: %v", legacySidecar, err)
		}
	}
}

// TestAdoptLegacySQLite_TargetExists tests that we never overwrite an existing target.
func TestAdoptLegacySQLite_TargetExists(t *testing.T) {
	tmpdir := t.TempDir()

	legacyPath := filepath.Join(tmpdir, "kestrel.db")
	targetPath := filepath.Join(tmpdir, "gameplane.db")

	// Create both legacy and target files with distinct content.
	legacyDB, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	if _, err := legacyDB.Exec(`CREATE TABLE legacy_data (value TEXT)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO legacy_data VALUES ('from_legacy')`); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	legacyDB.Close()

	targetDB, err := sql.Open("sqlite", targetPath)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	if _, err := targetDB.Exec(`CREATE TABLE target_data (value TEXT)`); err != nil {
		t.Fatalf("create target table: %v", err)
	}
	if _, err := targetDB.Exec(`INSERT INTO target_data VALUES ('from_target')`); err != nil {
		t.Fatalf("insert target: %v", err)
	}
	targetDB.Close()

	// Attempt adoption.
	dsn := "file:" + targetPath
	if err := adoptLegacySQLite(dsn); err != nil {
		t.Fatalf("adoptLegacySQLite: %v", err)
	}

	// Verify target still has its original data.
	targetDB, err = sql.Open("sqlite", targetPath)
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	defer targetDB.Close()
	var val string
	if err := targetDB.QueryRow(`SELECT value FROM target_data WHERE value = 'from_target'`).Scan(&val); err != nil {
		t.Fatalf("target data lost: %v", err)
	}

	// Verify legacy still exists (not overwritten).
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy file missing: %v", err)
	}
}

// TestAdoptLegacySQLite_NeitherPresent tests fresh install behavior.
func TestAdoptLegacySQLite_NeitherPresent(t *testing.T) {
	tmpdir := t.TempDir()
	targetPath := filepath.Join(tmpdir, "gameplane.db")

	dsn := "file:" + targetPath + "?_pragma=journal_mode(WAL)"
	if err := adoptLegacySQLite(dsn); err != nil {
		t.Fatalf("adoptLegacySQLite: %v", err)
	}

	// Opening the database should create a fresh one without error.
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open after adopt: %v", err)
	}
	defer db.Close()

	// Verify we can create a table in the fresh database.
	if _, err := db.Exec(`CREATE TABLE test (id INTEGER)`); err != nil {
		t.Fatalf("create table in fresh db: %v", err)
	}
}

// TestAdoptLegacySQLite_MemoryDatabase tests that memory databases are untouched.
func TestAdoptLegacySQLite_MemoryDatabase(t *testing.T) {
	// These should be no-ops and not error.
	for _, dsn := range []string{`:memory:`, `file::memory:`} {
		if err := adoptLegacySQLite(dsn); err != nil {
			t.Errorf("adoptLegacySQLite(%q): %v", dsn, err)
		}
	}
}

// TestAdoptLegacySQLite_PostgresNotAffected tests that adoptLegacySQLite
// is only called for sqlite (already tested in Open), but we verify sqlitePath
// returns "" for empty/invalid DSNs to confirm the guard.
func TestAdoptLegacySQLite_EmptyDSN(t *testing.T) {
	if err := adoptLegacySQLite(""); err != nil {
		t.Errorf("adoptLegacySQLite(empty): %v", err)
	}
}

// TestOpen_AdoptsLegacySQLite verifies that Open() triggers adoption
// and migrations run successfully on the adopted file.
func TestOpen_AdoptsLegacySQLite(t *testing.T) {
	tmpdir := t.TempDir()

	// Create legacy database with schema and data.
	legacyPath := filepath.Join(tmpdir, "kestrel.db")
	legacyDB, err := sql.Open("sqlite", legacyPath)
	if err != nil {
		t.Fatalf("create legacy: %v", err)
	}
	// Create the schema_migrations table so Open/Migrate recognizes it as initialized.
	if _, err := legacyDB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO schema_migrations VALUES ('001_init.sql', datetime('now'))`); err != nil {
		t.Fatalf("seed migration: %v", err)
	}
	if _, err := legacyDB.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create users: %v", err)
	}
	if _, err := legacyDB.Exec(`INSERT INTO users VALUES ('user1', 'Alice')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	legacyDB.Close()

	// Open with a new target path — adoption should happen.
	targetPath := filepath.Join(tmpdir, "gameplane.db")
	dsn := "file:" + targetPath + "?_pragma=journal_mode(WAL)"
	store, err := Open(context.Background(), "sqlite", dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	// Verify data from legacy survives.
	var name string
	if err := store.DB.QueryRow(`SELECT name FROM users WHERE id = 'user1'`).Scan(&name); err != nil {
		t.Fatalf("query user: %v", err)
	}
	if name != "Alice" {
		t.Errorf("user data mismatch: got %q, want Alice", name)
	}

	// Verify migrations can run on the adopted database.
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Verify the migrations table has entries.
	var count int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query migrations: %v", err)
	}
	if count == 0 {
		t.Fatal("no migrations applied")
	}
}
