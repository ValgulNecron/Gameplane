package files

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWrite_OSCreateError exercises the os.Create error branch by
// targeting a path whose parent already exists as a non-directory.
// MkdirAll succeeds because filepath.Dir of "/file" is the root, which
// already exists; os.Create then fails because the target name is
// itself a directory.
func TestWrite_OSCreateError(t *testing.T) {
	srvURL, root := newServer(t)
	dir := filepath.Join(root, "is-a-dir")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// POST /files/write?path=/is-a-dir — os.Create on a directory fails.
	resp, err := http.Post(srvURL+"/files/write?path=/is-a-dir", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		t.Fatalf("write to a directory should not 204")
	}
}
