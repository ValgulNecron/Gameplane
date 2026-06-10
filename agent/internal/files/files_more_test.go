package files

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// TestDownload_ResolveError exercises the resolve-fail branch.
func TestDownload_ResolveError(t *testing.T) {
	srvURL, _ := newServer(t)
	resp := get(t, srvURL, "/files/download", url.Values{"path": []string{"/no/such/parent/file"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

// TestRead_ResolveError covers the resolve-fail branch on /files/read.
func TestRead_ResolveError(t *testing.T) {
	srvURL, _ := newServer(t)
	resp := get(t, srvURL, "/files/read", url.Values{"path": []string{"/no/such/parent/file"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

// TestList_ResolveError covers the resolve-fail branch on /files/list.
func TestList_ResolveError(t *testing.T) {
	srvURL, _ := newServer(t)
	resp := get(t, srvURL, "/files/list", url.Values{"path": []string{"/no/such/parent/file"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

// TestDelete_ResolveError covers the resolve-fail branch on /files/delete.
func TestDelete_ResolveError(t *testing.T) {
	srvURL, _ := newServer(t)
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, srvURL+"/files/delete?path=/no/such/parent/file", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

// TestList_PermissionDenied makes the directory unreadable so
// os.ReadDir fails with EACCES, exercising the httpErr→403 branch.
func TestList_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root — permissions are bypassed")
	}
	srvURL, root := newServer(t)
	hidden := filepath.Join(root, "secret")
	if err := os.Mkdir(hidden, 0o000); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(hidden, 0o755); _ = os.RemoveAll(hidden) })
	resp := get(t, srvURL, "/files/list", url.Values{"path": []string{"/secret"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("got %d", resp.StatusCode)
	}
}
