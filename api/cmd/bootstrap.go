package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
)

// bootstrapAdmin seeds (or, with --force, resets) the initial admin user
// so that fresh installs don't require hand-writing SQL. It runs the same
// schema migrations and password hashing as the API itself, so a row
// created here is indistinguishable from one created via the dashboard.
//
// stdin/stderr are injected so tests can drive the command without
// touching the real os.Stdin/os.Stderr.
func bootstrapAdmin(ctx context.Context, args []string, stdin io.Reader, stderr io.Writer) error {
	var bf bootstrapFlags
	fs := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bf.bind(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Break-glass without a user reset: --enable-local-login alone just
	// force-enables the local provider in the auth config row — for the
	// "local disabled in Admin Settings, OIDC broken" lockout, where the
	// admin's password still works once the method is re-enabled.
	if bf.enableLocalLogin && bf.username == "" {
		store, err := db.Open(ctx, bf.dbDriver, bf.dbDSN)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer store.Close()
		if err := store.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		return enableLocalLogin(ctx, store, stderr)
	}

	if bf.username == "" {
		return errors.New("--username is required")
	}
	pw, err := resolvePassword(&bf, stdin)
	if err != nil {
		return err
	}
	if len(pw) < auth.MinPasswordLen {
		return fmt.Errorf("password too short (minimum %d characters)", auth.MinPasswordLen)
	}

	store, err := db.Open(ctx, bf.dbDriver, bf.dbDSN)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if bf.enableLocalLogin {
		if err := enableLocalLogin(ctx, store, stderr); err != nil {
			return err
		}
	}

	hash, err := auth.HashPassword(pw)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	display := bf.displayName
	if display == "" {
		display = bf.username
	}

	var existingID int64
	row := store.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, bf.username)
	switch err := row.Scan(&existingID); {
	case errors.Is(err, sql.ErrNoRows):
		res, err := store.DB.ExecContext(ctx,
			`INSERT INTO users(username, display_name, email, role, pw_hash) VALUES (?, ?, ?, 'admin', ?)`,
			bf.username, display, bf.email, hash,
		)
		if err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		newID, _ := res.LastInsertId()
		// Mirror the admin role into a cluster-wide role binding so RBAC
		// resolves the user's permissions (this runs after Migrate, so the
		// migration's backfill never saw this row).
		if err := store.SetClusterRoleBinding(ctx, nil, newID, "admin"); err != nil {
			return fmt.Errorf("bind admin role: %w", err)
		}
		fmt.Fprintf(stderr, "bootstrap-admin: created user %q\n", bf.username)
		return nil
	case err != nil:
		return fmt.Errorf("lookup user: %w", err)
	}

	if !bf.force {
		return fmt.Errorf("user %q already exists; pass --force to overwrite password and promote to admin", bf.username)
	}
	if _, err := store.DB.ExecContext(ctx,
		`UPDATE users
		   SET pw_hash = ?, role = 'admin', display_name = ?, email = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		hash, display, bf.email, existingID,
	); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if err := store.SetClusterRoleBinding(ctx, nil, existingID, "admin"); err != nil {
		return fmt.Errorf("bind admin role: %w", err)
	}
	fmt.Fprintf(stderr, "bootstrap-admin: updated user %q\n", bf.username)
	return nil
}

type bootstrapFlags struct {
	username         string
	password         string
	passwordStdin    bool
	email            string
	displayName      string
	force            bool
	enableLocalLogin bool
	dbDriver         string
	dbDSN            string
}

func (b *bootstrapFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&b.username, "username", "", "admin username (required unless only --enable-local-login is used)")
	fs.StringVar(&b.password, "password", "", "admin password; or set GAMEPLANE_ADMIN_PASSWORD, or use --password-stdin")
	fs.BoolVar(&b.passwordStdin, "password-stdin", false, "read password from stdin (single line)")
	fs.StringVar(&b.email, "email", "", "optional email")
	fs.StringVar(&b.displayName, "display-name", "", "optional display name (defaults to username)")
	fs.BoolVar(&b.force, "force", false, "if user exists, overwrite password and promote to admin")
	fs.BoolVar(&b.enableLocalLogin, "enable-local-login", false, "break-glass: force-enable the local provider in the auth config (usable alone, without --username)")
	fs.StringVar(&b.dbDriver, "db-driver", envOr("GAMEPLANE_DB_DRIVER", "sqlite"), "sqlite or postgres")
	fs.StringVar(&b.dbDSN, "db-dsn", envOr("GAMEPLANE_DB_DSN", "file:/data/gameplane.db?_pragma=journal_mode(WAL)"), "DSN")
}

// enableLocalLogin flips (or injects) the local provider's enabled flag
// in the persisted auth config, preserving every other provider. A
// missing row already means "local enabled", so it's left absent.
func enableLocalLogin(ctx context.Context, store *db.Store, stderr io.Writer) error {
	raw, ok, err := store.ConfigValue(ctx, "auth")
	if err != nil {
		return fmt.Errorf("read auth config: %w", err)
	}
	if !ok {
		fmt.Fprintln(stderr, "bootstrap-admin: no auth config row — local login is already enabled by default")
		return nil
	}
	var cfg struct {
		Providers []map[string]any `json:"providers"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return fmt.Errorf("parse auth config: %w", err)
	}
	found := false
	for _, p := range cfg.Providers {
		if kind, _ := p["kind"].(string); kind == "local" {
			p["enabled"] = true
			found = true
		}
	}
	if !found {
		cfg.Providers = append(cfg.Providers, map[string]any{
			"name": "local", "kind": "local", "enabled": true,
		})
	}
	canon, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if _, err := store.DB.ExecContext(ctx,
		`INSERT INTO config(key, value, updated_at)
		 VALUES ('auth', ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET
		     value      = excluded.value,
		     updated_at = excluded.updated_at`,
		string(canon),
	); err != nil {
		return fmt.Errorf("write auth config: %w", err)
	}
	fmt.Fprintln(stderr, "bootstrap-admin: local login re-enabled in the auth config")
	return nil
}

// resolvePassword picks the password from exactly one source. Order of
// precedence: --password-stdin, then --password, then GAMEPLANE_ADMIN_PASSWORD.
// Combining --password-stdin with --password is rejected so an operator
// can't accidentally hash the wrong value when piping.
func resolvePassword(bf *bootstrapFlags, stdin io.Reader) (string, error) {
	if bf.passwordStdin && bf.password != "" {
		return "", errors.New("--password-stdin and --password are mutually exclusive")
	}
	if bf.passwordStdin {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		// Trim only trailing newline / CR — passwords with leading or
		// internal whitespace are valid, so don't TrimSpace here.
		return strings.TrimRight(line, "\r\n"), nil
	}
	if bf.password != "" {
		return bf.password, nil
	}
	if env := os.Getenv("GAMEPLANE_ADMIN_PASSWORD"); env != "" {
		return env, nil
	}
	return "", errors.New("password required: provide --password, --password-stdin, or GAMEPLANE_ADMIN_PASSWORD")
}
