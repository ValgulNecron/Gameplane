package files

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Creating a path whose intermediate directories don't exist must succeed:
// resolve() walks up to the nearest existing ancestor (here the root) so the
// handlers' MkdirAll can create the whole subtree. Before the fix, resolve()
// only checked the immediate parent and rejected the request when it was
// absent, making the handlers' MkdirAll unreachable.
func TestMkdir_CreatesMissingAncestors(t *testing.T) {
	srvURL, root := newServer(t)

	resp, err := testPost(t, srvURL+"/files/mkdir?path=/one/two/three", "application/json", nil)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d want 204", resp.StatusCode)
	}
	info, err := os.Stat(filepath.Join(root, "one", "two", "three"))
	if err != nil || !info.IsDir() {
		t.Fatalf("nested dir not created: err=%v", err)
	}
}

// notADirPath plants a regular file under root and returns a request path that
// tries to descend into it, so resolve() hits an ENOTDIR error — a non-NotExist
// EvalSymlinks failure — and the handler returns a generic 400 "bad request".
// A merely-missing parent now resolves successfully (nested creation), so this
// is how the handlers' resolve-error branch is exercised.
func notADirPath(t *testing.T, root string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "notadir"), []byte("x"), 0o600); err != nil {
		t.Fatalf("plant file: %v", err)
	}
	return "/notadir/child"
}

func TestWrite_CreatesMissingAncestors(t *testing.T) {
	srvURL, root := newServer(t)

	resp, err := testPost(t, srvURL+"/files/write?path=/a/b/c.txt", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d want 204", resp.StatusCode)
	}
	got, err := os.ReadFile(filepath.Join(root, "a", "b", "c.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("content=%q want hello", got)
	}
}
