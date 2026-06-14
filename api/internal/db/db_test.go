package db

import (
	"context"
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
