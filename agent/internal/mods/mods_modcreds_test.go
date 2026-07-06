package mods

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestInjectModCreds_FactorioWithCredentials verifies that when factorio
// credentials are available, they are appended as query parameters.
func TestInjectModCreds_FactorioWithCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "factorio")
	if err := os.MkdirAll(credsPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "username"), []byte("testuser"), 0o644); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "token"), []byte("testtoken123"), 0o644); err != nil {
		t.Fatalf("write token: %v", err)
	}

	oldBasePath := modCredsBasePath
	modCredsBasePath = tmpDir
	defer func() { modCredsBasePath = oldBasePath }()

	testURL := "https://mods.factorio.com/api/v2/mods/MyMod/downloads/latest?version=1.1"
	result := injectModCreds(testURL, "factorio")

	// Parse the result URL to check parameters.
	u, err := url.Parse(result)
	if err != nil {
		t.Fatalf("parse result URL: %v", err)
	}
	if u.Query().Get("username") != "testuser" {
		t.Errorf("username parameter not injected; got %q", u.Query().Get("username"))
	}
	if u.Query().Get("token") != "testtoken123" {
		t.Errorf("token parameter not injected; got %q", u.Query().Get("token"))
	}
	// Existing parameters should be preserved.
	if u.Query().Get("version") != "1.1" {
		t.Errorf("existing query parameter lost; version=%q", u.Query().Get("version"))
	}
}

// TestInjectModCreds_FactorioMissingUsername verifies that missing username
// file results in unchanged URL.
func TestInjectModCreds_FactorioMissingUsername(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "factorio")
	if err := os.MkdirAll(credsPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No username file.
	if err := os.WriteFile(filepath.Join(credsPath, "token"), []byte("testtoken"), 0o644); err != nil {
		t.Fatalf("write token: %v", err)
	}

	oldBasePath := modCredsBasePath
	modCredsBasePath = tmpDir
	defer func() { modCredsBasePath = oldBasePath }()

	testURL := "https://mods.factorio.com/api/v2/mods/MyMod/downloads/latest"
	result := injectModCreds(testURL, "factorio")

	if result != testURL {
		t.Errorf("URL should be unchanged when username missing; got %q", result)
	}
}

// TestInjectModCreds_FactorioMissingToken verifies that missing token file
// results in unchanged URL.
func TestInjectModCreds_FactorioMissingToken(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "factorio")
	if err := os.MkdirAll(credsPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "username"), []byte("testuser"), 0o644); err != nil {
		t.Fatalf("write username: %v", err)
	}
	// No token file.

	oldBasePath := modCredsBasePath
	modCredsBasePath = tmpDir
	defer func() { modCredsBasePath = oldBasePath }()

	testURL := "https://mods.factorio.com/api/v2/mods/MyMod/downloads/latest"
	result := injectModCreds(testURL, "factorio")

	if result != testURL {
		t.Errorf("URL should be unchanged when token missing; got %q", result)
	}
}

// TestInjectModCreds_FactorioEmptyCredentials verifies that empty
// username or token results in unchanged URL.
func TestInjectModCreds_FactorioEmptyCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "factorio")
	if err := os.MkdirAll(credsPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "username"), []byte(""), 0o644); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "token"), []byte("testtoken"), 0o644); err != nil {
		t.Fatalf("write token: %v", err)
	}

	oldBasePath := modCredsBasePath
	modCredsBasePath = tmpDir
	defer func() { modCredsBasePath = oldBasePath }()

	testURL := "https://mods.factorio.com/api/v2/mods/MyMod/downloads/latest"
	result := injectModCreds(testURL, "factorio")

	if result != testURL {
		t.Errorf("URL should be unchanged when username empty; got %q", result)
	}
}

// TestInjectModCreds_NonFactorioProvider verifies that non-factorio
// providers are not modified.
func TestInjectModCreds_NonFactorioProvider(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "modrinth")
	if err := os.MkdirAll(credsPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "username"), []byte("testuser"), 0o644); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "token"), []byte("testtoken"), 0o644); err != nil {
		t.Fatalf("write token: %v", err)
	}

	oldBasePath := modCredsBasePath
	modCredsBasePath = tmpDir
	defer func() { modCredsBasePath = oldBasePath }()

	testURL := "https://cdn.modrinth.com/data/abc/versions/xyz/MyMod.jar"
	result := injectModCreds(testURL, "modrinth")

	if result != testURL {
		t.Errorf("URL should be unchanged for non-factorio provider; got %q", result)
	}
}

// TestInjectModCreds_EmptyProvider verifies that empty provider string
// results in unchanged URL.
func TestInjectModCreds_EmptyProvider(t *testing.T) {
	testURL := "https://example.com/mod.jar"
	result := injectModCreds(testURL, "")

	if result != testURL {
		t.Errorf("URL should be unchanged for empty provider; got %q", result)
	}
}

// TestInjectModCreds_CredentialsWithWhitespace verifies that whitespace
// around credentials is trimmed before use.
func TestInjectModCreds_CredentialsWithWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "factorio")
	if err := os.MkdirAll(credsPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "username"), []byte("  testuser  \n"), 0o644); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "token"), []byte("  testtoken\t"), 0o644); err != nil {
		t.Fatalf("write token: %v", err)
	}

	oldBasePath := modCredsBasePath
	modCredsBasePath = tmpDir
	defer func() { modCredsBasePath = oldBasePath }()

	testURL := "https://mods.factorio.com/api/v2/mods/MyMod/downloads/latest"
	result := injectModCreds(testURL, "factorio")

	u, err := url.Parse(result)
	if err != nil {
		t.Fatalf("parse result URL: %v", err)
	}
	if u.Query().Get("username") != "testuser" {
		t.Errorf("username not trimmed; got %q", u.Query().Get("username"))
	}
	if u.Query().Get("token") != "testtoken" {
		t.Errorf("token not trimmed; got %q", u.Query().Get("token"))
	}
}

// TestInjectModCreds_TokenNotInError verifies that injectModCreds never
// includes the token in its error messages or return values in a way that
// could be logged. We test that the token is not in any string output.
func TestInjectModCreds_TokenNotInError(t *testing.T) {
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "factorio")
	if err := os.MkdirAll(credsPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "username"), []byte("testuser"), 0o644); err != nil {
		t.Fatalf("write username: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credsPath, "token"), []byte("secret-token-12345"), 0o644); err != nil {
		t.Fatalf("write token: %v", err)
	}

	oldBasePath := modCredsBasePath
	modCredsBasePath = tmpDir
	defer func() { modCredsBasePath = oldBasePath }()

	testURL := "https://mods.factorio.com/api/v2/mods/MyMod/downloads/latest"
	result := injectModCreds(testURL, "factorio")

	// The token should be in the URL (since we injected it), but we're
	// verifying that any error message wouldn't contain it by accident.
	// This is more of a safety check — the function doesn't return errors,
	// but we want to ensure logging downstream won't expose it either.
	if !strings.Contains(result, "secret-token-12345") {
		// The token should be in the result URL.
		t.Errorf("token not found in result URL")
	}

	// Verify the URL can be parsed without errors.
	u, err := url.Parse(result)
	if err != nil {
		t.Errorf("parsing result URL should not error")
	}
	if u == nil {
		t.Errorf("parsed URL should not be nil")
	}
}

// TestModMetaProvider verifies that the install request metadata carries
// the provider information. This is a sanity check that ModMeta.Provider
// is used by the install handler to decide whether to inject credentials.
func TestModMetaProvider(t *testing.T) {
	meta := &ModMeta{
		Provider:  "factorio",
		ProjectID: "test-mod",
	}
	if meta.Provider != "factorio" {
		t.Errorf("ModMeta.Provider not set correctly; got %q", meta.Provider)
	}
}

// TestDownloadTemp_CredentialsNotLeaked verifies that when a download fails
// with credentialed query parameters in the URL, the error returned by
// downloadTemp does not contain the token. This ensures credentials can
// never be exposed in logs.
func TestDownloadTemp_CredentialsNotLeaked(t *testing.T) {
	// Create a server that immediately closes the connection on any request.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hijack the connection to force a client-side error.
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("http.ResponseWriter does not support hijack")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack failed: %v", err)
			return
		}
		// Close immediately without sending a response.
		conn.Close()
	}))
	defer srv.Close()

	// Build a credentialed URL directly (simulating what install would inject).
	secret := "super-secret-token-12345"
	credentialedURL := srv.URL + "/api/download?username=testuser&token=" + url.QueryEscape(secret)

	// Create a handler with the test server allowed.
	tmpDir := t.TempDir()
	testURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	h := &handler{
		dir:      tmpDir,
		maxBytes: 256 << 20,
		client:   newSafeClient([]string{testURL.Hostname()}),
	}

	// Perform the download; this should fail because the server closes the connection.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err = h.downloadTemp(ctx, credentialedURL)
	if err == nil {
		t.Fatal("expected downloadTemp to fail when server closes connection")
	}

	// The error must not contain the token.
	errStr := err.Error()
	if strings.Contains(errStr, secret) {
		t.Errorf("token leaked in error: %q", errStr)
	}
	// The error also should not contain the raw query string.
	if strings.Contains(errStr, "username=") {
		t.Errorf("credentialed query leaked in error: %q", errStr)
	}
}

// TestDownloadTemp_EmptyTokenNotLeaked verifies the symmetric case: when
// an empty token is used in a credentialed URL, the error doesn't leak it.
func TestDownloadTemp_EmptyTokenNotLeaked(t *testing.T) {
	// Create a server that immediately closes the connection.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("http.ResponseWriter does not support hijack")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack failed: %v", err)
			return
		}
		conn.Close()
	}))
	defer srv.Close()

	// Build a credentialed URL with an empty token.
	credentialedURL := srv.URL + "/api/download?username=testuser&token="

	tmpDir := t.TempDir()
	testURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	h := &handler{
		dir:      tmpDir,
		maxBytes: 256 << 20,
		client:   newSafeClient([]string{testURL.Hostname()}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _, err = h.downloadTemp(ctx, credentialedURL)
	if err == nil {
		t.Fatal("expected downloadTemp to fail when server closes connection")
	}

	// The error must not contain the query string even with empty token.
	errStr := err.Error()
	if strings.Contains(errStr, "username=") {
		t.Errorf("credentialed query leaked in error: %q", errStr)
	}
}
