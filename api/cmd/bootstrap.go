package main

import (
	"bufio"
	"context"
	"database/sql"
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
	username      string
	password      string
	passwordStdin bool
	email         string
	displayName   string
	force         bool
	dbDriver      string
	dbDSN         string
}

func (b *bootstrapFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&b.username, "username", "", "admin username (required)")
	fs.StringVar(&b.password, "password", "", "admin password; or set GAMEPLANE_ADMIN_PASSWORD, or use --password-stdin")
	fs.BoolVar(&b.passwordStdin, "password-stdin", false, "read password from stdin (single line)")
	fs.StringVar(&b.email, "email", "", "optional email")
	fs.StringVar(&b.displayName, "display-name", "", "optional display name (defaults to username)")
	fs.BoolVar(&b.force, "force", false, "if user exists, overwrite password and promote to admin")
	fs.StringVar(&b.dbDriver, "db-driver", envOr("GAMEPLANE_DB_DRIVER", "sqlite"), "sqlite or postgres")
	fs.StringVar(&b.dbDSN, "db-dsn", envOr("GAMEPLANE_DB_DSN", "file:/data/kestrel.db?_pragma=journal_mode(WAL)"), "DSN")
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
