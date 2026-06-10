package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newServer mounts files on a temp root and returns the server URL + the
// resolved root path (resolved via EvalSymlinks because t.TempDir() may
// return a path containing symlinks on macOS — the production resolver
// would fight us otherwise).
func newServer(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval root: %v", err)
	}
	r := chi.NewRouter()
	Mount(r, resolved)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv.URL, resolved
}

func get(t *testing.T, srvURL, path string, q url.Values) *http.Response {
	t.Helper()
	full := srvURL + path
	if q != nil {
		full += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, full, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return resp
}

func testPost(t *testing.T, url, contentType string, body io.Reader) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

func TestList(t *testing.T) {
	srvURL, root := newServer(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp := get(t, srvURL, "/files/list", url.Values{"path": []string{"/"}})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var ents []Entry
	if err := json.NewDecoder(resp.Body).Decode(&ents); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(ents) != 2 {
		t.Fatalf("got %d entries: %+v", len(ents), ents)
	}
	names := map[string]bool{}
	for _, e := range ents {
		names[e.Name] = e.Dir
	}
	if names["a.txt"] != false || names["sub"] != true {
		t.Fatalf("entries=%+v", ents)
	}
}

func TestList_NotFound(t *testing.T) {
	srvURL, _ := newServer(t)
	resp := get(t, srvURL, "/files/list", url.Values{"path": []string{"/missing"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestRead(t *testing.T) {
	srvURL, root := newServer(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Run("success", func(t *testing.T) {
		resp := get(t, srvURL, "/files/read", url.Values{"path": []string{"/a.txt"}})
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "hi" {
			t.Fatalf("body=%q", body)
		}
	})

	t.Run("directory rejected", func(t *testing.T) {
		resp := get(t, srvURL, "/files/read", url.Values{"path": []string{"/"}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resp := get(t, srvURL, "/files/read", url.Values{"path": []string{"/nope"}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("too large", func(t *testing.T) {
		big := filepath.Join(root, "big.bin")
		f, err := os.Create(big)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := f.Truncate((2 << 20) + 1); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		_ = f.Close()
		resp := get(t, srvURL, "/files/read", url.Values{"path": []string{"/big.bin"}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("bad resolve generic 400", func(t *testing.T) {
		// Parent dir doesn't exist → EvalSymlinks fails → "bad request"
		// (not the verbatim errPathOutOfRoot path).
		resp := get(t, srvURL, "/files/read", url.Values{"path": []string{"/no/such/parent/file"}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "bad request") {
			t.Fatalf("body=%q", body)
		}
	})

	t.Run("symlink-escape returns errPathOutOfRoot verbatim", func(t *testing.T) {
		outside := t.TempDir()
		outsideResolved, _ := filepath.EvalSymlinks(outside)
		// Place a real file outside, then a symlink inside root pointing
		// to that file's parent. Reading the symlink will trigger resolve's
		// post-EvalSymlinks prefix check and return errPathOutOfRoot.
		if err := os.WriteFile(filepath.Join(outsideResolved, "secret"), []byte("x"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		link := filepath.Join(root, "esc-r")
		if err := os.Symlink(outsideResolved, link); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(link) })
		resp := get(t, srvURL, "/files/read", url.Values{"path": []string{"/esc-r/secret"}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "escapes root") {
			t.Fatalf("body=%q", body)
		}
	})
}

func TestDownload(t *testing.T) {
	srvURL, root := newServer(t)
	if err := os.WriteFile(filepath.Join(root, "f.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Run("success sets Content-Disposition", func(t *testing.T) {
		resp := get(t, srvURL, "/files/download", url.Values{"path": []string{"/f.bin"}})
		defer resp.Body.Close()
		if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "f.bin") {
			t.Fatalf("Content-Disposition=%q", cd)
		}
	})

	t.Run("directory rejected", func(t *testing.T) {
		resp := get(t, srvURL, "/files/download", url.Values{"path": []string{"/"}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resp := get(t, srvURL, "/files/download", url.Values{"path": []string{"/nope"}})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})
}

func TestWrite(t *testing.T) {
	srvURL, root := newServer(t)

	t.Run("writes file in root", func(t *testing.T) {
		body := bytes.NewBufferString("hello world")
		resp, err := testPost(t, srvURL+"/files/write?path=/a.txt", "text/plain", body)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", resp.StatusCode, readBody(resp))
		}
		got, _ := os.ReadFile(filepath.Join(root, "a.txt"))
		if string(got) != "hello world" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("rejects symlink-escape via resolve", func(t *testing.T) {
		// Place a symlink inside root pointing outside; writing through
		// it must be rejected.
		outside := t.TempDir()
		outsideResolved, _ := filepath.EvalSymlinks(outside)
		link := filepath.Join(root, "escape")
		if err := os.Symlink(outsideResolved, link); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(link) })
		// Targeting the symlink itself: resolve will follow and reject.
		resp, err := testPost(t, srvURL+"/files/write?path=/escape", "text/plain", strings.NewReader("x"))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})
}

func TestUpload(t *testing.T) {
	srvURL, root := newServer(t)

	t.Run("happy path", func(t *testing.T) {
		buf, ct := multipartBody(t, map[string]string{"a.txt": "alpha", "b.txt": "beta"})
		resp, err := testPost(t, srvURL+"/files/upload?path=/up", ct, buf)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", resp.StatusCode, readBody(resp))
		}
		a, _ := os.ReadFile(filepath.Join(root, "up", "a.txt"))
		b, _ := os.ReadFile(filepath.Join(root, "up", "b.txt"))
		if string(a) != "alpha" || string(b) != "beta" {
			t.Fatalf("a=%q b=%q", a, b)
		}
	})

	t.Run("rejects upload through symlink to outside", func(t *testing.T) {
		outside := t.TempDir()
		outsideResolved, _ := filepath.EvalSymlinks(outside)
		link := filepath.Join(root, "esc-up")
		if err := os.Symlink(outsideResolved, link); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(link) })
		buf, ct := multipartBody(t, map[string]string{"x.txt": "x"})
		resp, err := testPost(t, srvURL+"/files/upload?path=/esc-up", ct, buf)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("multipart parse error", func(t *testing.T) {
		// Wrong Content-Type (no multipart boundary) → ParseMultipartForm fails.
		resp, err := testPost(t, srvURL+"/files/upload?path=/", "text/plain", strings.NewReader("not multipart"))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})
}

func TestSaveMultipart_RejectsBadFilenames(t *testing.T) {
	dir := t.TempDir()
	cases := []string{".", "..", string(os.PathSeparator)}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			fh := makeFileHeader(t, name, "x")
			err := saveMultipart(dir, fh)
			if err == nil || !strings.Contains(err.Error(), "invalid filename") {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestSaveMultipart_RejectsOversize(t *testing.T) {
	dir := t.TempDir()
	fh := makeFileHeader(t, "big", "x")
	fh.Size = maxUploadFileBytes + 1 // lie about size
	err := saveMultipart(dir, fh)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("got %v", err)
	}
}

func TestUpload_TooManyFiles(t *testing.T) {
	srvURL, _ := newServer(t)
	files := map[string]string{}
	for i := 0; i <= maxUploadFiles; i++ {
		files[filepath.Join("f"+itoa(i)+".txt")] = "x"
	}
	buf, ct := multipartBody(t, files)
	resp, err := testPost(t, srvURL+"/files/upload?path=/many", ct, buf)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestMkdir(t *testing.T) {
	srvURL, root := newServer(t)
	resp, err := testPost(t, srvURL+"/files/mkdir?path=/single", "", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", resp.StatusCode, readBody(resp))
	}
	fi, err := os.Stat(filepath.Join(root, "single"))
	if err != nil || !fi.IsDir() {
		t.Fatalf("dir not created: %v", err)
	}
}

func TestMkdir_BadResolve(t *testing.T) {
	// Parent path doesn't exist → resolve fails on its EvalSymlinks call.
	srvURL, _ := newServer(t)
	resp, err := testPost(t, srvURL+"/files/mkdir?path=/no/such/parent/dir", "", nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestDelete(t *testing.T) {
	srvURL, root := newServer(t)
	if err := os.WriteFile(filepath.Join(root, "f"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "d", "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "d", "sub", "g"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Run("non-recursive removes file", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, srvURL+"/files/delete?path=/f", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		if _, err := os.Stat(filepath.Join(root, "f")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("still exists: %v", err)
		}
	})

	t.Run("non-recursive on non-empty dir errors", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, srvURL+"/files/delete?path=/d", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNoContent {
			t.Fatal("expected error on non-empty dir")
		}
	})

	t.Run("recursive removes dir tree", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, srvURL+"/files/delete?path=/d&recursive=true", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("refuses to delete root", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, srvURL+"/files/delete?path=/&recursive=true", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("bad resolve on delete (parent missing)", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodDelete, srvURL+"/files/delete?path=/no/such/parent/file", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})
}

func TestResolve_SymlinkEscape(t *testing.T) {
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("eval root: %v", err)
	}
	outside := t.TempDir()
	resolvedOutside, err := filepath.EvalSymlinks(outside)
	if err != nil {
		t.Fatalf("eval outside: %v", err)
	}

	// Place a symlink inside root that points outside.
	link := filepath.Join(resolved, "escape")
	if err := os.Symlink(resolvedOutside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	h := &handler{root: resolved}
	if _, err := h.resolve("/escape"); !errors.Is(err, errPathOutOfRoot) {
		t.Fatalf("got %v", err)
	}
}

func TestResolve_EmptyPathReturnsRoot(t *testing.T) {
	h := &handler{root: "/data"}
	got, err := h.resolve("")
	if err != nil || got != "/data" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestResolve_ParentMissing(t *testing.T) {
	root := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(root)
	h := &handler{root: resolved}
	// Parent dir doesn't exist either; EvalSymlinks fails on it.
	if _, err := h.resolve("/no/such/parent/file"); err == nil {
		t.Fatal("expected error")
	}
}

func TestHttpErr_PermissionMappedTo403(t *testing.T) {
	rr := httptest.NewRecorder()
	httpErr(rr, os.ErrPermission)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestHttpErr_GenericMappedTo500(t *testing.T) {
	rr := httptest.NewRecorder()
	httpErr(rr, errors.New("disk on fire"))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "disk on fire") {
		t.Fatal("error message leaked")
	}
}

// helpers

func multipartBody(t *testing.T, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, body := range files {
		fw, err := mw.CreateFormFile("files", name)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close mw: %v", err)
	}
	return &buf, mw.FormDataContentType()
}

func makeFileHeader(t *testing.T, name, body string) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", name)
	_, _ = fw.Write([]byte(body))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, fhs := range req.MultipartForm.File {
		for _, fh := range fhs {
			return fh
		}
	}
	t.Fatal("no header parsed")
	return nil
}

func readBody(r *http.Response) string {
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
