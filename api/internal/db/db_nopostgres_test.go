//go:build !postgres

package db

import (
	"context"
	"strings"
	"testing"
)

func TestOpenPostgres_NotCompiledIn(t *testing.T) {
	_, err := Open(context.Background(), "postgres", "host=x")
	if err == nil || !strings.Contains(err.Error(), "not compiled in") {
		t.Fatalf("got %v", err)
	}
}
