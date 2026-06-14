package db

import (
	"context"
	"testing"
)

func TestConfigValue(t *testing.T) {
	s := newRBACStore(t) // migrated store helper (rbac_test.go)
	ctx := context.Background()
	const want = `{"instanceName":"kestrel"}`
	if _, err := s.DB.Exec(`INSERT INTO config(key, value) VALUES ('general', ?)`, want); err != nil {
		t.Fatalf("insert config: %v", err)
	}

	v, ok, err := s.ConfigValue(ctx, "general")
	if err != nil || !ok {
		t.Fatalf("ConfigValue(general): v=%q ok=%v err=%v", v, ok, err)
	}
	if v != want {
		t.Fatalf("value = %q, want %q", v, want)
	}

	// Missing key: present=false, no error.
	if v, ok, err := s.ConfigValue(ctx, "absent"); err != nil || ok || v != "" {
		t.Fatalf("ConfigValue(absent): v=%q ok=%v err=%v", v, ok, err)
	}
}

func TestConfigValue_ClosedDB(t *testing.T) {
	s := newRBACStore(t)
	_ = s.Close()
	if _, _, err := s.ConfigValue(context.Background(), "general"); err == nil {
		t.Fatal("expected error querying a closed DB")
	}
}
