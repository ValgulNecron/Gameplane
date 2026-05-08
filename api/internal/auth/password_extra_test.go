package auth

import (
	"strings"
	"testing"
)

func TestVerifyPassword_BadFormat(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-phc-string"); err == nil {
		t.Fatal("expected unsupported-hash-format error")
	}
}

func TestVerifyPassword_BadVersion(t *testing.T) {
	enc := "$argon2id$v=NOT-A-NUMBER$m=64,t=3,p=2$AAAA$AAAA"
	if _, err := VerifyPassword("x", enc); err == nil {
		t.Fatal("expected version parse error")
	}
}

func TestVerifyPassword_BadParams(t *testing.T) {
	enc := "$argon2id$v=19$m=garbled$AAAA$AAAA"
	if _, err := VerifyPassword("x", enc); err == nil {
		t.Fatal("expected params parse error")
	}
}

func TestVerifyPassword_BadSalt(t *testing.T) {
	enc := "$argon2id$v=19$m=64,t=3,p=2$!!!$AAAA"
	if _, err := VerifyPassword("x", enc); err == nil {
		t.Fatal("expected salt decode error")
	}
}

func TestVerifyPassword_BadKey(t *testing.T) {
	enc := "$argon2id$v=19$m=64,t=3,p=2$AAAA$!!!"
	if _, err := VerifyPassword("x", enc); err == nil {
		t.Fatal("expected key decode error")
	}
}

func TestVerifyPassword_BadKeyLength(t *testing.T) {
	// A valid-base64 but wrong-length key triggers the explicit length check.
	enc := "$argon2id$v=19$m=64,t=3,p=2$AAAA$AAAA"
	_, err := VerifyPassword("x", enc)
	if err == nil || !strings.Contains(err.Error(), "invalid hash length") {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyPassword_RoundTrip(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("hunter2", hash)
	if err != nil || !ok {
		t.Fatalf("verify ok=%v err=%v", ok, err)
	}
	ok, _ = VerifyPassword("wrong", hash)
	if ok {
		t.Fatal("wrong password should not verify")
	}
}
