package mods

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
)

// postUpload sends a multipart upload with the given filename + payload.
func postUpload(t *testing.T, srv *httptest.Server, field, filename string, payload []byte) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/mods/upload", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func TestUpload_HappyPath(t *testing.T) {
	root := t.TempDir()
	// No install (allowlist) block on purpose: uploads must work without one.
	srv := newSrv(t, root, modsSpec("mods", nil))

	status, body := postUpload(t, srv, "file", "custom.jar", []byte("JARDATA"))
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	var installed Mod
	if err := json.Unmarshal(body, &installed); err != nil {
		t.Fatal(err)
	}
	if installed.Name != "custom.jar" || installed.Size != int64(len("JARDATA")) {
		t.Fatalf("installed = %+v", installed)
	}
	if installed.Meta == nil || installed.Meta.Provider != "upload" || installed.Meta.InstalledAt == "" {
		t.Fatalf("meta = %+v, want provider upload with installedAt", installed.Meta)
	}
	data, err := os.ReadFile(filepath.Join(root, "mods", "custom.jar"))
	if err != nil || string(data) != "JARDATA" {
		t.Fatalf("file = %q err=%v", data, err)
	}
	// Manifest records the upload.
	entries := readManifest(t, filepath.Join(root, "mods"))
	if m, ok := entries["custom.jar"]; !ok || m.Provider != "upload" {
		t.Fatalf("manifest = %+v", entries)
	}
}

func TestUpload_ExtractArchive(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("plugins/Cool.dll")
	_, _ = f.Write([]byte("DLL"))
	_ = zw.Close()

	root := t.TempDir()
	spec := &caps.Mods{Path: "plugins", Extensions: []string{".zip"}, Extract: true}
	srv := newSrv(t, root, spec)

	status, body := postUpload(t, srv, "file", "Owner-Cool-1.0.0.zip", buf.Bytes())
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	dll := filepath.Join(root, "plugins", "Owner-Cool-1.0.0", "plugins", "Cool.dll")
	if data, err := os.ReadFile(dll); err != nil || string(data) != "DLL" {
		t.Fatalf("extracted = %q err=%v", data, err)
	}
	// The upload temp file is gone; only the folder remains.
	entries, _ := os.ReadDir(filepath.Join(root, "plugins"))
	for _, e := range entries {
		if e.Name() != "Owner-Cool-1.0.0" && e.Name() != manifestName {
			t.Fatalf("unexpected leftover %q", e.Name())
		}
	}
}

func TestUpload_ExtractBadArchive(t *testing.T) {
	root := t.TempDir()
	spec := &caps.Mods{Path: "plugins", Extensions: []string{".zip"}, Extract: true}
	srv := newSrv(t, root, spec)
	status, _ := postUpload(t, srv, "file", "bad.zip", []byte("not a zip"))
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", status)
	}
}

func TestUpload_RejectsWrongExtension(t *testing.T) {
	srv := newSrv(t, t.TempDir(), modsSpec("mods", nil)) // .jar only
	status, _ := postUpload(t, srv, "file", "notes.txt", []byte("x"))
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", status)
	}
}

func TestUpload_RejectsBadName(t *testing.T) {
	srv := newSrv(t, t.TempDir(), &caps.Mods{Path: "mods"})
	for _, bad := range []string{".hidden", manifestName} {
		status, _ := postUpload(t, srv, "file", bad, []byte("x"))
		if status != http.StatusBadRequest {
			t.Errorf("name %q: status=%d, want 400", bad, status)
		}
	}
}

func TestUpload_SizeCap(t *testing.T) {
	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{
		AllowedHosts: []string{"cdn.example.com"}, MaxSizeMB: 1,
	}))
	status, _ := postUpload(t, srv, "file", "big.jar", bytes.Repeat([]byte("A"), 2<<20))
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d, want 413", status)
	}
	// Nothing lands in the mods dir.
	if _, err := os.Stat(filepath.Join(root, "mods", "big.jar")); !os.IsNotExist(err) {
		t.Fatalf("oversize upload must not persist, stat err=%v", err)
	}
}

func TestUpload_MissingFileField(t *testing.T) {
	srv := newSrv(t, t.TempDir(), &caps.Mods{Path: "mods"})
	status, _ := postUpload(t, srv, "wrongfield", "x.jar", []byte("x"))
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", status)
	}
}

func TestUpload_Unconfigured(t *testing.T) {
	srv := newSrv(t, t.TempDir(), nil)
	status, _ := postUpload(t, srv, "file", "x.jar", []byte("x"))
	if status != http.StatusNotImplemented {
		t.Fatalf("status=%d, want 501", status)
	}
}
