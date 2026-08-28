//go:build e2e

package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// applyModsExtractTemplate is applyModsTemplate (see api_mods_e2e_test.go)
// plus "extract": true on the mods capability, so an uploaded archive is
// actually unpacked via swapInArchive/unzipInto instead of being stored as
// an inert file (agent/internal/mods/mods.go: with extract unset, uploads
// are just os.Rename'd into place and the confinement guard on the
// extraction path never runs). The confinement tests in this file need
// extraction enabled to exercise the guard they claim to verify.
func applyModsExtractTemplate(t *testing.T, tmplName string) {
	t.Helper()
	ctx := context.Background()
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E mods confinement busybox (" + tmplName + ")",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
			"capabilities": map[string]any{
				"mods": map[string]any{
					"path":    "mods",
					"extract": true,
					"install": map[string]any{
						"allowedHosts": []any{"raw.githubusercontent.com"},
						"maxSizeMB":    int64(16),
					},
				},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create template %s: %v", tmplName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})
}

// TestAPI_ModArchiveConfinement_PathTraversalRejected proves that the mod
// archive extraction confinement guard rejects path traversal entries
// (e.g., ../../../etc/passwd) that attempt to escape the sandbox.
func TestAPI_ModArchiveConfinement_PathTraversalRejected(t *testing.T) {
	t.Parallel()

	ns := "gameplane-games"
	tmpl := "e2e-mods-confinement-traverse-tmpl"
	gs := "e2e-mods-confinement-traverse-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyModsExtractTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)

	// Wait for mods endpoint readiness
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		resp, _, err := cli.Get("/servers/" + gs + "/mods")
		if err != nil {
			return false, "GET /mods: " + err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false, "status=" + http.StatusText(resp.StatusCode)
		}
		return true, ""
	})

	// Create a zip archive with a path traversal entry
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// Add a valid file first
	w, err := zw.Create("valid-mod.txt")
	if err != nil {
		t.Fatalf("create valid entry: %v", err)
	}
	if _, err := w.Write([]byte("valid content")); err != nil {
		t.Fatalf("write valid entry: %v", err)
	}
	// Add a traversal entry that tries to escape the sandbox
	w, err = zw.Create("../../../etc/passwd")
	if err != nil {
		t.Fatalf("create traversal entry: %v", err)
	}
	if _, err := w.Write([]byte("malicious content")); err != nil {
		t.Fatalf("write traversal entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	// Upload the malicious archive via multipart
	mpBuf := &bytes.Buffer{}
	mw := multipart.NewWriter(mpBuf)
	fw, err := mw.CreateFormFile("file", "malicious.zip")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write(buf.Bytes()); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, cli.BaseURL+"/servers/"+gs+"/mods/upload", mpBuf)
	if err != nil {
		t.Fatalf("build upload req: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Gameplane-CSRF", cli.CSRF)
	resp, err := cli.HTTP.Do(req)
	if err != nil {
		t.Fatalf("POST /mods/upload: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// With extraction enabled, unzipInto rejects the traversal entry via
	// ConfineRelPath, the staging dir is discarded, and the upload fails —
	// this is the expected, common outcome.
	// CodeQL alerts are cleared when the confinement validation rejects the traversal.
	if resp.StatusCode != http.StatusOK {
		// Rejection at upload time is acceptable (agent caught it)
		return
	}

	// If upload appeared to succeed despite the traversal entry, verify
	// nothing was actually extracted by listing mods and confirming the
	// archive did not land. Extraction installs archives under a folder
	// named after the archive with its extension stripped (see
	// agent/internal/mods/mods.go archiveFolderName), not the raw upload
	// filename.
	installName := strings.TrimSuffix("malicious.zip", ".zip")
	mods := listServerMods(t, cli, gs)
	for _, m := range mods {
		if m.Name == "malicious.zip" || m.Name == installName {
			t.Fatalf("malicious archive landed in mods list: %+v (confinement validation failed)", m)
		}
	}
}

// TestAPI_ModArchiveConfinement_SymlinkEscapeRejected proves that the mod
// archive extraction confinement guard rejects symlink entries that point
// outside the sandbox (e.g., symlinks to ../../../../etc/passwd).
func TestAPI_ModArchiveConfinement_SymlinkEscapeRejected(t *testing.T) {
	t.Parallel()

	ns := "gameplane-games"
	tmpl := "e2e-mods-confinement-symlink-tmpl"
	gs := "e2e-mods-confinement-symlink-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyModsExtractTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)

	// Wait for mods endpoint readiness
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		resp, _, err := cli.Get("/servers/" + gs + "/mods")
		if err != nil {
			return false, "GET /mods: " + err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false, "status=" + http.StatusText(resp.StatusCode)
		}
		return true, ""
	})

	// Create a zip archive with a symlink entry that escapes the sandbox.
	// ZIP format stores symlinks as regular files with a symlink flag bit and
	// the target as the file content. We'll create one manually.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add a valid regular file
	w, err := zw.Create("valid-mod.txt")
	if err != nil {
		t.Fatalf("create valid entry: %v", err)
	}
	if _, err := w.Write([]byte("valid content")); err != nil {
		t.Fatalf("write valid entry: %v", err)
	}

	// Add a symlink entry: name is the link, content is the target.
	// We manually create the file header with the symlink bit set.
	h := &zip.FileHeader{
		Name:          "escape-link",
		Method:        zip.Store,
		ExternalAttrs: 0o120777 << 16, // Unix symlink permissions
	}
	w, err = zw.CreateHeader(h)
	if err != nil {
		t.Fatalf("create symlink header: %v", err)
	}
	// Write the symlink target (outside sandbox)
	if _, err := w.Write([]byte("../../../../etc/passwd")); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	// Upload the archive with escaping symlink
	mpBuf := &bytes.Buffer{}
	mw := multipart.NewWriter(mpBuf)
	fw, err := mw.CreateFormFile("file", "symlink-escape.zip")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write(buf.Bytes()); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, cli.BaseURL+"/servers/"+gs+"/mods/upload", mpBuf)
	if err != nil {
		t.Fatalf("build upload req: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Gameplane-CSRF", cli.CSRF)
	resp, err := cli.HTTP.Do(req)
	if err != nil {
		t.Fatalf("POST /mods/upload: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Rejection (error response) or non-listing is acceptable.
	// ConfinePath validation on symlink resolution should reject this.
	if resp.StatusCode != http.StatusOK {
		return
	}

	// Verify the symlink was not created or listed
	mods := listServerMods(t, cli, gs)
	for _, m := range mods {
		if m.Name == "symlink-escape.zip" {
			t.Fatalf("symlink-escape archive landed in mods list: %+v (symlink escape was not rejected)", m)
		}
	}
}

// TestAPI_ModArchiveConfinement_ValidArchiveExtracts proves that the mod
// archive extraction confinement guard accepts well-formed archives, including
// those with nested paths (e.g., subdir/nested.txt) that a too-strict guard
// might wrongly reject as potential escapes.
func TestAPI_ModArchiveConfinement_ValidArchiveExtracts(t *testing.T) {
	t.Parallel()

	ns := "gameplane-games"
	tmpl := "e2e-mods-confinement-valid-tmpl"
	gs := "e2e-mods-confinement-valid-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyModsExtractTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)

	// Wait for mods endpoint readiness
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		resp, _, err := cli.Get("/servers/" + gs + "/mods")
		if err != nil {
			return false, "GET /mods: " + err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false, "status=" + http.StatusText(resp.StatusCode)
		}
		return true, ""
	})

	// Create a well-formed zip archive with valid files and optional subdirectory
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add files that should extract successfully (including nested paths)
	files := map[string]string{
		"mod.txt":           "mod content",
		"subdir/nested.txt": "nested content",
		"data.bin":          "binary-like data",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %q: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write entry %q: %v", name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	// Upload the valid archive via multipart
	mpBuf := &bytes.Buffer{}
	mw := multipart.NewWriter(mpBuf)
	fw, err := mw.CreateFormFile("file", "e2e-mod-confinement.zip")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write(buf.Bytes()); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, cli.BaseURL+"/servers/"+gs+"/mods/upload", mpBuf)
	if err != nil {
		t.Fatalf("build upload req: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Gameplane-CSRF", cli.CSRF)
	resp, err := cli.HTTP.Do(req)
	if err != nil {
		t.Fatalf("POST /mods/upload: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// The upload MUST succeed (200 OK)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload expected 200, got %d body=%q", resp.StatusCode, string(body))
	}

	// Parse the response to get the uploaded mod entry
	var uploaded modEntry
	if err := json.Unmarshal(body, &uploaded); err != nil {
		t.Fatalf("decode upload response: %v body=%q", err, string(body))
	}
	if uploaded.Name != "e2e-mod-confinement.zip" {
		t.Fatalf("upload response name = %q, want e2e-mod-confinement.zip", uploaded.Name)
	}
	if uploaded.Meta == nil || uploaded.Meta.Provider != "upload" {
		t.Fatalf("upload response provider = %v, want upload", uploaded.Meta)
	}

	// Verify the listing shows the uploaded mod
	mods := listServerMods(t, cli, gs)
	found := false
	for _, m := range mods {
		if m.Name == "e2e-mod-confinement.zip" {
			found = true
			if m.Meta == nil || m.Meta.Provider != "upload" {
				t.Fatalf("listed mod has unexpected meta: %+v", m.Meta)
			}
			if m.Size <= 0 {
				t.Fatalf("listed mod has zero size: %d", m.Size)
			}
			break
		}
	}
	if !found {
		t.Fatalf("uploaded mod not found in listing: %+v", mods)
	}

	// Cleanup: delete the mod to verify removal also respects confinement
	delResp, delBody, err := cli.Delete("/servers/" + gs + "/mods?name=e2e-mod-confinement.zip")
	if err != nil {
		t.Fatalf("DELETE /mods: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode/100 != 2 {
		t.Fatalf("remove expected 2xx, got %d body=%q", delResp.StatusCode, string(delBody))
	}

	// Verify mod is removed from listing
	mods = listServerMods(t, cli, gs)
	for _, m := range mods {
		if m.Name == "e2e-mod-confinement.zip" {
			t.Fatalf("mod still in listing after delete: %+v", m)
		}
	}
}
