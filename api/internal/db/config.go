package db

import (
	"context"
	"database/sql"
)

// ConfigValue returns the raw JSON value stored for an admin config
// section (the `config` table is a simple key→JSON map written by the
// /admin/config handler) and whether the key was present. Uses a
// no-parameter scan so it's portable across the sqlite and postgres
// drivers without per-driver placeholder rebinding.
func (s *Store) ConfigValue(ctx context.Context, key string) (string, bool, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT key, value FROM config")
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return "", false, err
		}
		if k == key {
			return v, true, nil
		}
	}
	return "", false, rows.Err()
}

// ConfigValueTx is ConfigValue but reads through an open transaction, so it
// does not need a second connection (required under sqlite's single-conn cap).
func ConfigValueTx(ctx context.Context, tx *sql.Tx, key string) (string, bool, error) {
	rows, err := tx.QueryContext(ctx, "SELECT key, value FROM config")
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return "", false, err
		}
		if k == key {
			return v, true, nil
		}
	}
	return "", false, rows.Err()
}
