package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
	"github.com/kestrel-gg/kestrel/api/internal/db"
)

// dsnIn returns a sqlite DSN pointing at a fresh file under t.TempDir().
// :memory: is unusable here because db.Open caps MaxOpenConns to 1 but we
// still re-open the same DB inside bootstrapAdmin (it owns its own *Store);
// a temp file lets the second open see the first call's writes.
func dsnIn(t *testing.T) string {
	t.Helper()
	return "file:" + filepath.Join(t.TempDir(), "k.db") + "?_pragma=journal_mode(WAL)"
}

func runBootstrap(t *testing.T, dsn, stdin string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"--db-driver=sqlite", "--db-dsn=" + dsn}, args...)
	var stderr bytes.Buffer
	err := bootstrapAdmin(context.Background(), full, strings.NewReader(stdin), &stderr)
	return stderr.String(), err
}

func mustOpen(t *testing.T, dsn string) *db.Store {
	t.Helper()
	s, err := db.Open(context.Background(), "sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBootstrap_Insert(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	out, err := runBootstrap(t, dsn, "",
		"--username=admin", "--password=correct-horse-battery", "--email=a@b.test",
	)
	if err != nil {
		t.Fatalf("bootstrap: %v (stderr=%q)", err, out)
	}
	if !strings.Contains(out, "created user \"admin\"") {
		t.Fatalf("expected created log, got %q", out)
	}

	s := mustOpen(t, dsn)
	var role, hash, display, email string
	err = s.DB.QueryRow(`SELECT role, pw_hash, display_name, email FROM users WHERE username='admin'`).
		Scan(&role, &hash, &display, &email)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if role != "admin" {
		t.Fatalf("role=%q want admin", role)
	}
	if display != "admin" {
		t.Fatalf("display_name=%q want default to username", display)
	}
	if email != "a@b.test" {
		t.Fatalf("email=%q", email)
	}
	ok, err := auth.VerifyPassword("correct-horse-battery", hash)
	if err != nil || !ok {
		t.Fatalf("password did not verify (ok=%v err=%v)", ok, err)
	}
}

func TestBootstrap_PasswordStdin(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	_, err := runBootstrap(t, dsn, "fromstdin-password\n",
		"--username=admin", "--password-stdin",
	)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	s := mustOpen(t, dsn)
	var hash string
	if err := s.DB.QueryRow(`SELECT pw_hash FROM users WHERE username='admin'`).Scan(&hash); err != nil {
		t.Fatalf("select: %v", err)
	}
	ok, _ := auth.VerifyPassword("fromstdin-password", hash)
	if !ok {
		t.Fatal("stdin password did not verify")
	}
}

func TestBootstrap_PasswordEnv(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "env-supplied-pass")
	_, err := runBootstrap(t, dsn, "", "--username=admin")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	s := mustOpen(t, dsn)
	var hash string
	if err := s.DB.QueryRow(`SELECT pw_hash FROM users WHERE username='admin'`).Scan(&hash); err != nil {
		t.Fatalf("select: %v", err)
	}
	ok, _ := auth.VerifyPassword("env-supplied-pass", hash)
	if !ok {
		t.Fatal("env password did not verify")
	}
}

func TestBootstrap_RejectsShortPassword(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	_, err := runBootstrap(t, dsn, "", "--username=admin", "--password=short")
	if err == nil || !strings.Contains(err.Error(), "too short") {
		t.Fatalf("expected too-short error, got %v", err)
	}
}

func TestBootstrap_RejectsMissingPassword(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	_, err := runBootstrap(t, dsn, "", "--username=admin")
	if err == nil || !strings.Contains(err.Error(), "password required") {
		t.Fatalf("expected password-required error, got %v", err)
	}
}

func TestBootstrap_RejectsConflictingPasswordSources(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	_, err := runBootstrap(t, dsn, "ignored\n",
		"--username=admin", "--password=in-flag", "--password-stdin",
	)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually-exclusive error, got %v", err)
	}
}

func TestBootstrap_RejectsMissingUsername(t *testing.T) {
	_, err := runBootstrap(t, dsnIn(t), "", "--password=correct-horse-battery")
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("expected username-required error, got %v", err)
	}
}

func TestBootstrap_RefusesOverwriteWithoutForce(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	if _, err := runBootstrap(t, dsn, "",
		"--username=admin", "--password=correct-horse-battery"); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	_, err := runBootstrap(t, dsn, "",
		"--username=admin", "--password=different-correct-horse")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected --force advisory, got %v", err)
	}

	// Original hash must be untouched.
	s := mustOpen(t, dsn)
	var hash string
	if err := s.DB.QueryRow(`SELECT pw_hash FROM users WHERE username='admin'`).Scan(&hash); err != nil {
		t.Fatalf("select: %v", err)
	}
	ok, _ := auth.VerifyPassword("correct-horse-battery", hash)
	if !ok {
		t.Fatal("original password no longer verifies after rejected overwrite")
	}
}

func TestBootstrap_ForceUpdatesPassword(t *testing.T) {
	dsn := dsnIn(t)
	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	if _, err := runBootstrap(t, dsn, "",
		"--username=admin", "--password=original-correct-horse"); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}
	out, err := runBootstrap(t, dsn, "",
		"--username=admin", "--password=fresh-rotation-secret", "--force",
	)
	if err != nil {
		t.Fatalf("force bootstrap: %v", err)
	}
	if !strings.Contains(out, "updated user") {
		t.Fatalf("expected updated log, got %q", out)
	}

	s := mustOpen(t, dsn)
	var hash string
	if err := s.DB.QueryRow(`SELECT pw_hash FROM users WHERE username='admin'`).Scan(&hash); err != nil {
		t.Fatalf("select: %v", err)
	}
	if ok, _ := auth.VerifyPassword("original-correct-horse", hash); ok {
		t.Fatal("old password still verifies after --force")
	}
	if ok, _ := auth.VerifyPassword("fresh-rotation-secret", hash); !ok {
		t.Fatal("new password does not verify after --force")
	}
}

func TestBootstrap_ForcePromotesToAdmin(t *testing.T) {
	dsn := dsnIn(t)
	// Pre-seed a non-admin user directly so we can confirm --force
	// also rewrites role to admin (the install path may have created
	// a viewer-role row before someone realised admin was missing).
	s := mustOpen(t, dsn)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := s.DB.Exec(
		`INSERT INTO users(username, display_name, email, role, pw_hash) VALUES (?,?,?,?,?)`,
		"alice", "Alice", "alice@example.com", "viewer", "placeholder",
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Setenv("KESTREL_ADMIN_PASSWORD", "")
	if _, err := runBootstrap(t, dsn, "",
		"--username=alice", "--password=promotion-correct-horse", "--force",
	); err != nil {
		t.Fatalf("force bootstrap: %v", err)
	}
	var role string
	if err := s.DB.QueryRow(`SELECT role FROM users WHERE username='alice'`).Scan(&role); err != nil {
		t.Fatalf("select: %v", err)
	}
	if role != "admin" {
		t.Fatalf("role=%q want admin after --force", role)
	}
}
