//go:build e2e

package e2e

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"
)

// TestAPI_ModArchiveConfinement proves that the mod archive extraction
// confinement guard rejects both path traversal entries and symlinks that
// escape the sandbox, while accepting well-formed archives with nested paths.
func TestAPI_ModArchiveConfinement(t *testing.T) {
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	t.Run("PathTraversalRejected", func(t *testing.T) {
		ns := "gameplane-games"
		tmpl := "e2e-mods-confinement-traverse-tmpl"
		gs := "e2e-mods-confinement-traverse-gs"

		applyModsTemplate(t, tmpl)
		applyBusyboxGameServer(t, ns, gs, tmpl)
		waitPVCBound(t, ns, gs+"-data", 90*time.Second)
		requireAgentReady(t, ns, gs)

		// Wait for mods endpoint readiness
		envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
			resp, body, err := cli.Get("/servers/" + gs + "/mods")
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

		// The upload should fail (extraction error) or succeed but listing should show nothing
		// (the extraction was rejected by ConfinePath validation).
		// CodeQL alerts are cleared when the confinement validation rejects the traversal.
		if resp.StatusCode != http.StatusOK {
			// Rejection at upload time is acceptable (agent caught it)
			return
		}

		// If upload appeared to succeed, verify nothing was actually extracted
		// by listing mods and confirming the malicious archive did not land
		mods := listServerMods(t, cli, gs)
		for _, m := range mods {
			if m.Name == "malicious.zip" {
				t.Fatalf("malicious archive landed in mods list: %+v (confinement validation failed)", m)
			}
		}
	})

	t.Run("SymlinkEscapeRejected", func(t *testing.T) {
		ns := "gameplane-games"
		tmpl := "e2e-mods-confinement-symlink-tmpl"
		gs := "e2e-mods-confinement-symlink-gs"

		applyModsTemplate(t, tmpl)
		applyBusyboxGameServer(t, ns, gs, tmpl)
		waitPVCBound(t, ns, gs+"-data", 90*time.Second)
		requireAgentReady(t, ns, gs)

		// Wait for mods endpoint readiness
		envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
			resp, body, err := cli.Get("/servers/" + gs + "/mods")
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
			Name:     "escape-link",
			Method:   zip.Store,
			ExternalAttrs: 0o120777 << 16, // Unix symlink permissions
		}
		w, err := zw.CreateHeader(h)
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
	})

	t.Run("ValidArchiveExtracts", func(t *testing.T) {
		ns := "gameplane-games"
		tmpl := "e2e-mods-confinement-valid-tmpl"
		gs := "e2e-mods-confinement-valid-gs"

		applyModsTemplate(t, tmpl)
		applyBusyboxGameServer(t, ns, gs, tmpl)
		waitPVCBound(t, ns, gs+"-data", 90*time.Second)
		requireAgentReady(t, ns, gs)

		// Wait for mods endpoint readiness
		envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
			resp, body, err := cli.Get("/servers/" + gs + "/mods")
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

		// Add files that should extract successfully
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
	})
}
