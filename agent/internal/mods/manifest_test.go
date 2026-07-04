package mods

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
)

// jarServer serves the same bytes for every path and returns its allowlist
// host, mirroring how install tests reach an httptest upstream.
func jarServer(t *testing.T, payload []byte) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	host, _, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	return srv, host
}

func listMods(t *testing.T, srv *httptest.Server) []Mod {
	t.Helper()
	status, body := do(t, srv, http.MethodGet, "/mods", nil)
	if status != http.StatusOK {
		t.Fatalf("list status=%d body=%s", status, body)
	}
	var got []Mod
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return got
}

func readManifest(t *testing.T, dir string) map[string]*ModMeta {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return m.Mods
}

func TestInstall_RecordsMeta(t *testing.T) {
	allowLoopback(t)
	upstream, host := jarServer(t, []byte("JAR"))
	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))

	status, body := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL: upstream.URL + "/sodium-0.6.13.jar",
		Meta: &ModMeta{
			Provider: "modrinth", ProjectID: "AANobbMI", ProjectName: "Sodium",
			VersionID: "v613", VersionNumber: "0.6.13", GameVersion: "1.21.4", Loader: "fabric",
		},
	})
	if status != http.StatusOK {
		t.Fatalf("install status=%d body=%s", status, body)
	}
	var installed Mod
	if err := json.Unmarshal(body, &installed); err != nil {
		t.Fatal(err)
	}
	if installed.Meta == nil || installed.Meta.InstalledAt == "" || installed.Meta.SourceURL == "" {
		t.Fatalf("install response meta not stamped: %+v", installed.Meta)
	}

	got := listMods(t, srv)
	if len(got) != 1 || got[0].Meta == nil {
		t.Fatalf("list = %+v, want one managed mod", got)
	}
	m := got[0].Meta
	if m.Provider != "modrinth" || m.ProjectID != "AANobbMI" || m.VersionID != "v613" ||
		m.Loader != "fabric" || m.GameVersion != "1.21.4" {
		t.Fatalf("meta = %+v", m)
	}
	if m.SourceURL != upstream.URL+"/sodium-0.6.13.jar" {
		t.Fatalf("sourceUrl = %q", m.SourceURL)
	}
}

func TestInstall_ReplacesSwapsFileAndMeta(t *testing.T) {
	allowLoopback(t)
	upstream, host := jarServer(t, []byte("JAR"))
	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))

	if status, body := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL:  upstream.URL + "/sodium-0.6.9.jar",
		Meta: &ModMeta{Provider: "modrinth", ProjectID: "AANobbMI", VersionID: "v609"},
	}); status != http.StatusOK {
		t.Fatalf("first install status=%d body=%s", status, body)
	}
	if status, body := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL:      upstream.URL + "/sodium-0.6.13.jar",
		Replaces: "sodium-0.6.9.jar",
		Meta:     &ModMeta{Provider: "modrinth", ProjectID: "AANobbMI", VersionID: "v613"},
	}); status != http.StatusOK {
		t.Fatalf("upgrade status=%d body=%s", status, body)
	}

	if _, err := os.Stat(filepath.Join(root, "mods", "sodium-0.6.9.jar")); !os.IsNotExist(err) {
		t.Fatalf("old jar should be gone, stat err=%v", err)
	}
	got := listMods(t, srv)
	if len(got) != 1 || got[0].Name != "sodium-0.6.13.jar" {
		t.Fatalf("list = %+v, want only the upgraded jar", got)
	}
	if got[0].Meta == nil || got[0].Meta.VersionID != "v613" {
		t.Fatalf("meta = %+v, want versionId v613", got[0].Meta)
	}
	if entries := readManifest(t, filepath.Join(root, "mods")); len(entries) != 1 {
		t.Fatalf("manifest = %+v, want single entry", entries)
	}
}

func TestInstall_ReplacesExtractFolder(t *testing.T) {
	allowLoopback(t)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("plugins/Cool.dll")
	_, _ = f.Write([]byte("DLL"))
	_ = zw.Close()
	upstream, host := jarServer(t, buf.Bytes())

	root := t.TempDir()
	spec := &caps.Mods{
		Path:       "plugins",
		Extensions: []string{".zip"},
		Extract:    true,
		Install:    &caps.ModInstall{AllowedHosts: []string{host}},
	}
	srv := newSrv(t, root, spec)

	if status, body := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL: upstream.URL + "/x.zip", Name: "Owner-Cool-1.0.0.zip",
		Meta: &ModMeta{Provider: "thunderstore", ProjectID: "Owner/Cool", VersionID: "1.0.0"},
	}); status != http.StatusOK {
		t.Fatalf("first install status=%d body=%s", status, body)
	}
	if status, body := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL: upstream.URL + "/x.zip", Name: "Owner-Cool-2.0.0.zip",
		Replaces: "Owner-Cool-1.0.0",
		Meta:     &ModMeta{Provider: "thunderstore", ProjectID: "Owner/Cool", VersionID: "2.0.0"},
	}); status != http.StatusOK {
		t.Fatalf("upgrade status=%d body=%s", status, body)
	}

	if _, err := os.Stat(filepath.Join(root, "plugins", "Owner-Cool-1.0.0")); !os.IsNotExist(err) {
		t.Fatalf("old folder should be gone, stat err=%v", err)
	}
	got := listMods(t, srv)
	if len(got) != 1 || got[0].Name != "Owner-Cool-2.0.0" || got[0].Meta == nil || got[0].Meta.VersionID != "2.0.0" {
		t.Fatalf("list = %+v, want only Owner-Cool-2.0.0 with meta", got)
	}
}

func TestInstall_WithoutMetaClearsStaleEntry(t *testing.T) {
	allowLoopback(t)
	upstream, host := jarServer(t, []byte("JAR"))
	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))

	if status, _ := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL:  upstream.URL + "/cool.jar",
		Meta: &ModMeta{Provider: "modrinth", ProjectID: "p1", VersionID: "v1"},
	}); status != http.StatusOK {
		t.Fatal("managed install failed")
	}
	// Reinstalling the same name from a plain URL loses the identity: the
	// manifest must not keep claiming the old provenance.
	if status, _ := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL: upstream.URL + "/cool.jar",
	}); status != http.StatusOK {
		t.Fatal("plain reinstall failed")
	}
	got := listMods(t, srv)
	if len(got) != 1 || got[0].Meta != nil {
		t.Fatalf("list = %+v, want one unmanaged mod", got)
	}
}

func TestList_MergesManagedAndUnmanaged(t *testing.T) {
	allowLoopback(t)
	upstream, host := jarServer(t, []byte("JAR"))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	// An out-of-band file predates the managed install.
	if err := os.WriteFile(filepath.Join(root, "mods", "handmade.jar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))
	if status, _ := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL:  upstream.URL + "/managed.jar",
		Meta: &ModMeta{Provider: "modrinth", ProjectID: "p1"},
	}); status != http.StatusOK {
		t.Fatal("install failed")
	}

	byName := map[string]*ModMeta{}
	for _, m := range listMods(t, srv) {
		byName[m.Name] = m.Meta
	}
	if len(byName) != 2 {
		t.Fatalf("list = %+v, want 2 mods", byName)
	}
	if byName["handmade.jar"] != nil {
		t.Fatal("out-of-band file must be unmanaged")
	}
	if byName["managed.jar"] == nil {
		t.Fatal("installed mod must carry meta")
	}
}

func TestList_CorruptManifestDegradesToUnmanaged(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "mods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jar"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newSrv(t, root, modsSpec("mods", nil))
	got := listMods(t, srv)
	if len(got) != 1 || got[0].Meta != nil {
		t.Fatalf("list = %+v, want one unmanaged mod despite corrupt manifest", got)
	}
}

func TestUpdateManifest_PrunesMissingFiles(t *testing.T) {
	allowLoopback(t)
	upstream, host := jarServer(t, []byte("JAR"))
	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))

	if status, _ := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL:  upstream.URL + "/ghost.jar",
		Meta: &ModMeta{Provider: "modrinth", ProjectID: "ghost"},
	}); status != http.StatusOK {
		t.Fatal("install failed")
	}
	// Delete the file out-of-band; the entry is stale now.
	if err := os.Remove(filepath.Join(root, "mods", "ghost.jar")); err != nil {
		t.Fatal(err)
	}
	// Any mutating operation self-heals the manifest.
	if status, _ := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL:  upstream.URL + "/other.jar",
		Meta: &ModMeta{Provider: "modrinth", ProjectID: "other"},
	}); status != http.StatusOK {
		t.Fatal("second install failed")
	}
	entries := readManifest(t, filepath.Join(root, "mods"))
	if _, stale := entries["ghost.jar"]; stale {
		t.Fatalf("manifest = %+v, stale ghost.jar entry must be pruned", entries)
	}
	if _, ok := entries["other.jar"]; !ok {
		t.Fatalf("manifest = %+v, other.jar entry missing", entries)
	}
}

func TestRemove_DropsManifestEntry(t *testing.T) {
	allowLoopback(t)
	upstream, host := jarServer(t, []byte("JAR"))
	root := t.TempDir()
	srv := newSrv(t, root, modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))

	if status, _ := do(t, srv, http.MethodPost, "/mods/install", installReq{
		URL:  upstream.URL + "/gone.jar",
		Meta: &ModMeta{Provider: "modrinth", ProjectID: "p1"},
	}); status != http.StatusOK {
		t.Fatal("install failed")
	}
	if status, _ := do(t, srv, http.MethodDelete, "/mods?name=gone.jar", nil); status != http.StatusNoContent {
		t.Fatal("remove failed")
	}
	if entries := readManifest(t, filepath.Join(root, "mods")); len(entries) != 0 {
		t.Fatalf("manifest = %+v, want empty after remove", entries)
	}
}

func TestList_ManifestFileIsInvisible(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "mods")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte(`{"version":1,"mods":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// No extension filter so any visible file would be listed.
	srv := newSrv(t, root, &caps.Mods{Path: "mods"})
	if got := listMods(t, srv); len(got) != 0 {
		t.Fatalf("list = %+v, manifest must not appear", got)
	}
}

func TestInstall_RejectsBadReplaces(t *testing.T) {
	allowLoopback(t)
	upstream, host := jarServer(t, []byte("JAR"))
	srv := newSrv(t, t.TempDir(), modsSpec("mods", &caps.ModInstall{AllowedHosts: []string{host}}))
	for _, bad := range []string{"../escape", "a/b", manifestName} {
		status, _ := do(t, srv, http.MethodPost, "/mods/install", installReq{
			URL: upstream.URL + "/x.jar", Replaces: bad,
		})
		if status != http.StatusBadRequest {
			t.Errorf("replaces %q: status=%d, want 400", bad, status)
		}
	}
}

func TestSafeName_RejectsManifest(t *testing.T) {
	if _, err := safeName(manifestName); err == nil {
		t.Fatal("safeName must reject the manifest file name")
	}
}

func TestManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &manifest{Version: 1, Mods: map[string]*ModMeta{
		"a.jar": {Provider: "modrinth", ProjectID: "p", VersionID: "v"},
	}}
	if err := m.save(dir); err != nil {
		t.Fatal(err)
	}
	got := loadManifest(dir)
	if got.Version != 1 || len(got.Mods) != 1 || got.Mods["a.jar"].ProjectID != "p" {
		t.Fatalf("round trip = %+v", got)
	}
}

func TestLoadManifest_MissingIsEmpty(t *testing.T) {
	got := loadManifest(filepath.Join(t.TempDir(), "nope"))
	if got.Version != 1 || len(got.Mods) != 0 {
		t.Fatalf("loadManifest missing dir = %+v, want empty", got)
	}
}

func TestLoadManifest_NullModsMap(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, manifestName), []byte(`{"version":1,"mods":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadManifest(dir)
	if got.Mods == nil {
		t.Fatal("Mods map must be non-nil after load")
	}
}
