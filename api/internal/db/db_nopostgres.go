//go:build !postgres

package db

import (
	"context"
	"errors"
)

func openPostgres(_ context.Context, _ string) (*Store, error) {
	return nil, errors.New("postgres support not compiled in — rebuild with -tags postgres")
}
