// Package auth handles user authentication (local + OIDC) and session
// lifecycle for the Gameplane API.
//
// Passwords are hashed with argon2id using parameters aligned with
// OWASP 2023 guidance for responsive dashboards: 64 MiB, t=3, p=2.
// (The OWASP table suggests p=4 for high-core servers; we pick p=2 to
// stay responsive on the single/dual-core nodes that small k3s installs
// run on — see argonThreads comment.) Encoded format follows the PHC
// string scheme so hashes remain verifiable if we later bump params.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime   = 3
	argonMemory = 64 * 1024
	// 2 threads matches OWASP 2023 guidance and stays responsive on
	// single- and dual-core nodes (common in small k3s deployments).
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// MinPasswordLen is the floor enforced anywhere a user picks or resets a
// password — HTTP user CRUD, password reset, and the bootstrap-admin CLI.
// Short enough to not block legitimate use, long enough that argon2id
// makes brute force impractical. Matches the design.pen reset-modal copy.
const MinPasswordLen = 12

// dummyHash is a valid argon2id hash of a random password nobody knows.
// It exists so the login handler can run a full VerifyPassword even on
// not-found / OIDC-only accounts, keeping the response-time envelope
// constant. Without it, a "user does not exist" reply returns in µs
// while a real wrong-password reply takes ~argon2 cost, and attackers
// enumerate valid usernames by timing.
var dummyHash = func() string {
	r := make([]byte, 32)
	if _, err := rand.Read(r); err != nil {
		// rand.Read only fails if the OS CSPRNG is broken; in that case
		// we can't safely serve auth anyway.
		panic("crypto/rand: " + err.Error())
	}
	h, err := HashPassword(string(r))
	if err != nil {
		panic("init dummy hash: " + err.Error())
	}
	return h
}()

// VerifyDummy runs an argon2id compare against a hash nobody can match.
// Use it on login paths where the outcome is already "deny" but you
// want the time envelope to match a real verify. pw is discarded.
func VerifyDummy(pw string) {
	_, _ = VerifyPassword(pw, dummyHash)
}

func HashPassword(pw string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(pw, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}
	var mem, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads); err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	if len(want) != argonKeyLen {
		return false, fmt.Errorf("invalid hash length")
	}
	got := argon2.IDKey([]byte(pw), salt, time, mem, threads, argonKeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
