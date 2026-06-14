package files

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A not-yet-existing leaf whose PARENT is a symlink escaping the root must
// be rejected. resolve() can't EvalSymlinks the missing leaf, so it falls
// back to resolving the deepest existing ancestor (the parent) — which
// here points outside the root and must trip errPathOutOfRoot.
func TestResolve_ParentSymlinkEscapesRoot(t *testing.T) {
	srvURL, root := newServer(t)

	outside := t.TempDir()
	outsideResolved, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatalf("eval outside: %v", err)
	}
	if err := os.Symlink(outsideResolved, filepath.Join(root, "escparent")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	resp := get(t, srvURL, "/files/list", url.Values{"path": {"/escparent/ghost"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "escapes root") {
		t.Fatalf("body=%q want it to mention 'escapes root'", string(body))
	}
}
