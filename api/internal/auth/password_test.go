package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("verify correct password: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword("wrong guess", hash)
	if err != nil {
		t.Fatalf("verify wrong: err=%v", err)
	}
	if ok {
		t.Fatal("wrong password accepted")
	}
}

func TestHashPerCallDiffers(t *testing.T) {
	// Salt randomness means two hashes of the same password differ.
	a, _ := HashPassword("x")
	b, _ := HashPassword("x")
	if a == b {
		t.Fatal("hashes must differ across calls due to salt")
	}
}
