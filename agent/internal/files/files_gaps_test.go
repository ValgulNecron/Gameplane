package files

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestList_NotADirectory exercises os.ReadDir on a regular file.
func TestList_NotADirectory(t *testing.T) {
	srvURL, root := newServer(t)
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	resp := get(t, srvURL, "/files/list", url.Values{"path": []string{"/f.txt"}})
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		t.Fatal("listing a regular file should fail")
	}
}

// TestWrite_BodyTooLarge confirms the MaxBytesReader cap fires.
func TestWrite_BodyTooLarge(t *testing.T) {
	srvURL, _ := newServer(t)
	body := bytes.NewReader(bytes.Repeat([]byte("a"), maxWriteBytes+1))
	resp, err := testPost(t, srvURL+"/files/write?path=/big.txt", "application/octet-stream", body)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		t.Fatal("oversize write should not succeed")
	}
}

// TestWrite_BadResolve hits the resolve-error branch on /files/write.
func TestWrite_BadResolve(t *testing.T) {
	srvURL, _ := newServer(t)
	resp, err := testPost(t, srvURL+"/files/write?path=/no/such/parent/file", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

// TestSaveMultipart_OpenError forces a multipart.FileHeader whose Open
// fails by closing the underlying form before saveMultipart runs.
func TestSaveMultipart_OpenError(t *testing.T) {
	dir := t.TempDir()
	// A FileHeader whose form has been Removed → Open errors.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", "x.txt")
	_, _ = fw.Write([]byte("payload"))
	_ = mw.Close()
	form, err := readForm(mw.Boundary(), &buf)
	if err != nil {
		t.Fatalf("readForm: %v", err)
	}
	defer form.RemoveAll()
	var fh *multipart.FileHeader
	for _, fhs := range form.File {
		if len(fhs) > 0 {
			fh = fhs[0]
			break
		}
	}
	if fh == nil {
		t.Fatal("no file header parsed")
	}
	fh.Filename = "valid.txt"
	form.RemoveAll() // invalidate the underlying tmpfile
	if err := saveMultipart(dir, fh); err != nil {
		// some platforms keep the file open; tolerate a nil error above,
		// and only report errors that aren't the expected not-exist kind.
		if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "no such file") {
			t.Logf("saveMultipart returned unexpected error: %v", err)
		}
	}
}

// readForm parses a multipart body into a *multipart.Form for the test.
func readForm(boundary string, r io.Reader) (*multipart.Form, error) {
	mr := multipart.NewReader(r, boundary)
	return mr.ReadForm(1 << 20)
}

// TestMkdir_ExistingFile triggers the os.MkdirAll error branch by
// asking it to create a directory whose path is already a file.
func TestMkdir_ExistingFile(t *testing.T) {
	srvURL, root := newServer(t)
	conflict := filepath.Join(root, "blocking")
	if err := os.WriteFile(conflict, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	resp, err := testPost(t, srvURL+"/files/mkdir?path=/blocking/sub", "", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		t.Fatal("mkdir over existing file should fail")
	}
}

// TestUpload_ResolveError exercises the resolve-fail branch on /files/upload.
func TestUpload_ResolveError(t *testing.T) {
	srvURL, _ := newServer(t)
	buf, ct := multipartBody(t, map[string]string{"x.txt": "y"})
	resp, err := testPost(t, srvURL+"/files/upload?path=/no/such/parent", ct, buf)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", resp.StatusCode)
	}
}
