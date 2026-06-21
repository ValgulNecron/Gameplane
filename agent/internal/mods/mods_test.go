package mods

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kestrel-gg/kestrel/agent/internal/caps"
)

// allowLoopback lets the SSRF dial guard reach httptest servers (which
// bind to 127.0.0.1) for the duration of a test.
func allowLoopback(t *testing.T) {
	t.Helper()
	prev := ssrfPolicy
	ssrfPolicy = func(net.IP) bool { return true }
	t.Cleanup(func() { ssrfPolicy = prev })
}

func newSrv(t *testing.T, dataRoot string, spec *caps.Mods) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	Mount(r, dataRoot, spec)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path string, body any) (int, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, rdr)
	if err != nil {
		t.Fatalf("req: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func modsSpec(dir string, install *caps.ModInstall) *caps.Mods {
	return &caps.Mods{Path: dir, Extensions: []string{".jar"}, Install: install}
}

func TestList(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.jar", "b.jar", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(root, "mods", f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srv := newSrv(t, root, modsSpec("mods", nil))
	status, body := do(t, srv, http.MethodGet, "/mods", nil)
	if status != http.StatusOK {
		t.Fatalf("status=%d", status)
	}
	var got []Mod
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	// Only .jar files (extension filter) — notes.txt excluded.
	if len(got) != 2 {
		t.Fatalf("got %d mods: %+v", len(got), got)
	}
}

func TestList_MissingDirIsEmpty(t *testing.T) {
	srv := newSrv(t, t.TempDir(), modsSpec("mods", nil))
	status, body := do(t, srv, http.MethodGet, "/mods", nil)
	if status != http.StatusOK || strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestList_Unconfigured(t *testing.T) {
	srv := newSrv(t, t.TempDir(), nil)
	status, body := do(t, srv, http.MethodGet, "/mods", nil)
	if status != http.StatusOK || strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("status=%d body=%s", status, body)
	}
}

func TestList_SkipsSubdirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mods", "a.jar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newSrv(t, root, modsSpec("mods", nil))
	_, body := do(t, srv, http.MethodGet, "/mods", nil)
	var got []Mod
	_ = json.Unmarshal(body, &got)
	if len(got) != 1 || got[0].Name != "a.jar" {
		t.Fatalf("got %+v, want only a.jar", got)
	}
}

func TestList_NoExtensionFilterListsAll(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.jar", "readme.txt", "config.yml"} {
		if err := os.WriteFile(filepath.Join(root, "plugins", f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// No Extensions → every file counts as a mod.
	srv := newSrv(t, root, &caps.Mods{Path: "plugins"})
	_, body := do(t, srv, http.MethodGet, "/mods", nil)
	var got []Mod
	_ = json.Unmarshal(body, &got)
	if len(got) != 3 {
		t.Fatalf("got %d, want 3: %+v", len(got), got)
	}
}

func TestRemove_NonEmptyDirErrors(t *testing.T) {
	root := t.TempDir()
	// A non-empty subdirectory named like a mod can't be os.Remove'd,
	// exercising the generic error branch.
	if err := os.MkdirAll(filepath.Join(root, "mods", "pack"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mods", "pack", "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newSrv(t, root, modsSpec("mods", nil))
	status, _ := do(t, srv, http.MethodDelete, "/mods?name=pack", nil)
	if status != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", status)
	}
}

func TestNewHandler_PathEscapeDisables(t *testing.T) {
	// Defense in depth: a traversal path disables mods entirely.
	h := newHandler(t.TempDir(), &caps.Mods{Path: "../../escape"})
	if h.dir != "" {
		t.Fatalf("dir = %q, want empty (disabled)", h.dir)
	}
}

func TestInstall_HappyPath(t *testing.T) {
	allowLoopback(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("JARDATA"))
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))

	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))

	status, body := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/cool.jar"})
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	data, err := os.ReadFile(filepath.Join(root, "mods", "cool.jar"))
	if err != nil || string(data) != "JARDATA" {
		t.Fatalf("file = %q err=%v", data, err)
	}
}

func TestInstall_ExtractArchive(t *testing.T) {
	allowLoopback(t)

	// Build a Thunderstore-style zip: a plugin .dll plus package metadata.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"plugins/Cool.dll": "DLLBYTES",
		"manifest.json":    `{"name":"Cool"}`,
		"README.md":        "hi",
	} {
		f, _ := zw.Create(name)
		_, _ = f.Write([]byte(content))
	}
	_ = zw.Close()
	zipBytes := buf.Bytes()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBytes)
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))

	root := t.TempDir()
	spec := &caps.Mods{
		Path:       "plugins",
		Extensions: []string{".zip"},
		Extract:    true,
		Install:    &caps.ModInstall{AllowedHosts: []string{host}},
	}
	srv := newSrv(t, root, spec)

	// Install → unpacks into plugins/<name>/.
	status, body := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/x.zip", "name": "Owner-Cool-1.0.0.zip"})
	if status != http.StatusOK {
		t.Fatalf("install status=%d body=%s", status, body)
	}
	dll := filepath.Join(root, "plugins", "Owner-Cool-1.0.0", "plugins", "Cool.dll")
	if data, err := os.ReadFile(dll); err != nil || string(data) != "DLLBYTES" {
		t.Fatalf("extracted dll = %q err=%v", data, err)
	}

	// List → the per-mod folder shows as one mod.
	status, body = do(t, srv, http.MethodGet, "/mods", nil)
	if status != http.StatusOK {
		t.Fatalf("list status=%d", status)
	}
	var mods []Mod
	if err := json.Unmarshal(body, &mods); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(mods) != 1 || mods[0].Name != "Owner-Cool-1.0.0" {
		t.Fatalf("list = %+v, want one folder Owner-Cool-1.0.0", mods)
	}

	// Remove → the whole folder is gone.
	status, _ = do(t, srv, http.MethodDelete, "/mods?name=Owner-Cool-1.0.0", nil)
	if status != http.StatusNoContent {
		t.Fatalf("remove status=%d", status)
	}
	if _, err := os.Stat(filepath.Join(root, "plugins", "Owner-Cool-1.0.0")); !os.IsNotExist(err) {
		t.Fatalf("folder should be gone, stat err=%v", err)
	}
}

func TestInstall_ExplicitName(t *testing.T) {
	allowLoopback(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("DATA"))
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))

	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))
	// URL path has no usable basename; an explicit name is used instead.
	status, body := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/download?id=42", "name": "renamed.jar"})
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	if _, err := os.Stat(filepath.Join(root, "mods", "renamed.jar")); err != nil {
		t.Fatalf("renamed.jar missing: %v", err)
	}
}

func TestInstall_ContentLengthOverCap(t *testing.T) {
	allowLoopback(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Advertise a huge size up front so the cap rejects before reading.
		w.Header().Set("Content-Length", "999999999")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	srv := newSrv(t, t.TempDir(), modsSpec("mods",
		&caps.ModInstall{AllowedHosts: []string{host}, MaxSizeMB: 1}))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/big.jar"})
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want 413", status)
	}
}

func TestInstall_UpstreamError(t *testing.T) {
	allowLoopback(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	srv := newSrv(t, t.TempDir(), modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/missing.jar"})
	if status != http.StatusBadGateway {
		t.Fatalf("status=%d, want 502", status)
	}
}

func TestInstall_RedirectToDisallowedHostBlocked(t *testing.T) {
	allowLoopback(t)
	// Redirect to a host not on the allowlist must be refused by
	// CheckRedirect even though the initial host is allowed.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://evil.example.com/x.jar", http.StatusFound)
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))
	srv := newSrv(t, t.TempDir(), modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/start.jar"})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", status)
	}
}

func TestRemove_Unconfigured(t *testing.T) {
	srv := newSrv(t, t.TempDir(), nil)
	status, _ := do(t, srv, http.MethodDelete, "/mods?name=x.jar", nil)
	if status != http.StatusNotImplemented {
		t.Fatalf("status=%d, want 501", status)
	}
}

func TestInstall_HostNotAllowed(t *testing.T) {
	allowLoopback(t)
	srv := newSrv(t, t.TempDir(), modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{"cdn.example.com"}}))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": "https://evil.example.net/x.jar"})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", status)
	}
}

func TestInstall_BlocksLoopbackBySSRFGuard(t *testing.T) {
	// No allowLoopback: the default guard must refuse a 127.0.0.1 target
	// even though the host is on the allowlist.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data"))
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))

	srv := newSrv(t, t.TempDir(), modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/x.jar"})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (SSRF guard)", status)
	}
}

func TestInstall_SizeCap(t *testing.T) {
	allowLoopback(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("A"), 5<<20)) // 5 MiB
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))

	srv := newSrv(t, t.TempDir(), modsSpec("mods",
		&caps.ModInstall{AllowedHosts: []string{host}, MaxSizeMB: 1}))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/big.jar"})
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want 413", status)
	}
}

func TestInstall_WrongExtension(t *testing.T) {
	allowLoopback(t)
	srv := newSrv(t, t.TempDir(), modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{"cdn.example.com"}}))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": "https://cdn.example.com/notamod.zip"})
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", status)
	}
}

func TestInstall_BadURL(t *testing.T) {
	srv := newSrv(t, t.TempDir(), modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{"x"}}))
	for _, u := range []string{"", "ftp://x/y.jar", "/relative.jar", "not a url"} {
		status, _ := do(t, srv, http.MethodPost, "/mods/install", map[string]string{"url": u})
		if status != http.StatusBadRequest {
			t.Errorf("url %q: status=%d, want 400", u, status)
		}
	}
}

func TestInstall_Disabled(t *testing.T) {
	// Mods configured but no install policy → installs are 501.
	srv := newSrv(t, t.TempDir(), modsSpec("mods", nil))
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": "https://cdn.example.com/x.jar"})
	if status != http.StatusNotImplemented {
		t.Fatalf("status=%d, want 501", status)
	}
}

func TestRemove(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "mods", "gone.jar")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newSrv(t, root, modsSpec("mods", nil))

	if status, _ := do(t, srv, http.MethodDelete, "/mods?name=gone.jar", nil); status != http.StatusNoContent {
		t.Fatalf("delete status=%d", status)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("file should be gone")
	}
	if status, _ := do(t, srv, http.MethodDelete, "/mods?name=missing.jar", nil); status != http.StatusNotFound {
		t.Fatalf("missing delete status=%d, want 404", status)
	}
}

func TestRemove_RejectsTraversal(t *testing.T) {
	srv := newSrv(t, t.TempDir(), modsSpec("mods", nil))
	for _, n := range []string{"../escape", "a/b", "..", ".hidden", ""} {
		status, _ := do(t, srv, http.MethodDelete, "/mods?name="+n, nil)
		if status != http.StatusBadRequest {
			t.Errorf("name %q: status=%d, want 400", n, status)
		}
	}
}

func TestSafeName(t *testing.T) {
	bad := []string{"", "..", ".", "a/b", "a\\b", "../x", ".dotfile", "x\ny", strings.Repeat("a", 201)}
	for _, n := range bad {
		if _, err := safeName(n); err == nil {
			t.Errorf("safeName(%q) should fail", n)
		}
	}
	good := []string{"cool.jar", "Mod_1.2.3.jar", "a-b_c.zip"}
	for _, n := range good {
		if got, err := safeName(n); err != nil || got != n {
			t.Errorf("safeName(%q) = %q, %v", n, got, err)
		}
	}
}

func TestHostAllowed(t *testing.T) {
	allow := []string{"cdn.modrinth.com", ".curseforge.com"}
	yes := []string{"cdn.modrinth.com", "CDN.Modrinth.com", "edge.curseforge.com", "curseforge.com"}
	no := []string{"modrinth.com", "evil.com", "curseforge.com.evil.com", "fakecdn.modrinth.com.evil"}
	for _, h := range yes {
		if !hostAllowed(h, allow) {
			t.Errorf("hostAllowed(%q) = false, want true", h)
		}
	}
	for _, h := range no {
		if hostAllowed(h, allow) {
			t.Errorf("hostAllowed(%q) = true, want false", h)
		}
	}
}

func TestArchiveFolderName(t *testing.T) {
	cases := map[string]string{
		"Owner-Mod-1.0.0.zip":    "Owner-Mod-1.0.0",
		"Owner-Mod-1.0.0.tar.gz": "Owner-Mod-1.0.0",
		"Owner-Mod-1.0.0.tgz":    "Owner-Mod-1.0.0",
		"Owner-Mod-1.0.0.TGZ":    "Owner-Mod-1.0.0", // case-insensitive
		"plainname":              "plainname",       // no known archive ext
	}
	for in, want := range cases {
		if got := archiveFolderName(in); got != want {
			t.Errorf("archiveFolderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnzipInto_RejectsZipSlip(t *testing.T) {
	zipPath := makeZip(t, map[string]string{"good.txt": "ok", "../evil.txt": "pwned"})
	dst := filepath.Join(t.TempDir(), "out")
	if err := unzipInto(zipPath, dst, 1<<20); err != nil {
		t.Fatalf("unzipInto: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "good.txt")); err != nil {
		t.Fatalf("good.txt should be extracted: %v", err)
	}
	// The traversal entry is skipped — nothing lands beside the staging dir.
	if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "evil.txt")); !os.IsNotExist(err) {
		t.Fatalf("evil.txt escaped the staging dir: %v", err)
	}
}

func TestUnzipInto_SizeCap(t *testing.T) {
	zipPath := makeZip(t, map[string]string{"big.bin": strings.Repeat("A", 4096)})
	dst := filepath.Join(t.TempDir(), "out")
	if err := unzipInto(zipPath, dst, 1024); !errors.Is(err, errTooLarge) {
		t.Fatalf("unzipInto over cap = %v, want errTooLarge", err)
	}
}

func TestUnzipInto_BadArchive(t *testing.T) {
	dir := t.TempDir()
	notZip := filepath.Join(dir, "x.zip")
	if err := os.WriteFile(notZip, []byte("definitely not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := unzipInto(notZip, filepath.Join(dir, "out"), 1<<20); !errors.Is(err, errBadArchive) {
		t.Fatalf("unzipInto bad archive = %v, want errBadArchive", err)
	}
}

func TestInstall_ExtractBadArchive(t *testing.T) {
	allowLoopback(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("definitely not a zip"))
	}))
	defer upstream.Close()
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(upstream.URL, "http://"))

	root := t.TempDir()
	spec := &caps.Mods{
		Path:       "plugins",
		Extensions: []string{".zip"},
		Extract:    true,
		Install:    &caps.ModInstall{AllowedHosts: []string{host}},
	}
	srv := newSrv(t, root, spec)
	status, _ := do(t, srv, http.MethodPost, "/mods/install",
		map[string]string{"url": upstream.URL + "/x.zip", "name": "Owner-Bad-1.0.0.zip"})
	if status != http.StatusBadGateway {
		t.Fatalf("status=%d, want 502 (bad archive)", status)
	}
}

// The SSRF IP-classification table (isPublic / IsAllowed) now lives in the
// shared netguard module's TestIsPublic; the agent tests cover only its use
// of the guard (default-refuses loopback, redirect host re-check).
