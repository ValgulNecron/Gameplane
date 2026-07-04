//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// TestAPI_ModUpload proves the direct-upload path end to end: a multipart
// file posted to the API is proxied to the agent, lands on the mod volume,
// and is recorded in the install manifest as provider "upload" (so update
// checks skip it). Uses the same mods-capable busybox template as the
// manifest test; needs no egress (nothing is downloaded).
func TestAPI_ModUpload(t *testing.T) {
	t.Parallel()

	ns := "gameplane-games"
	tmpl := "e2e-mods-upload-tmpl"
	gs := "e2e-mods-upload-gs"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	applyModsTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	requireAgentReady(t, ns, gs)
	envInstance.Eventually(t, 30*time.Second, func() (bool, string) {
		resp, body, err := cli.Get("/servers/" + gs + "/mods")
		if err != nil {
			return false, "GET /mods: " + err.Error()
		}
		if resp.StatusCode != http.StatusOK {
			return false, "status=" + http.StatusText(resp.StatusCode) + " body=" + string(body)
		}
		return true, ""
	})

	// Build the multipart body the dashboard's Upload mode sends.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "uploaded-e2e.bin")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write([]byte("uploaded by gameplane e2e")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, cli.BaseURL+"/servers/"+gs+"/mods/upload", &buf)
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload expected 200, got %d body=%q", resp.StatusCode, string(body))
	}
	var uploaded modEntry
	if err := json.Unmarshal(body, &uploaded); err != nil {
		t.Fatalf("decode upload response: %v body=%q", err, string(body))
	}
	if uploaded.Name != "uploaded-e2e.bin" || uploaded.Meta == nil || uploaded.Meta.Provider != "upload" {
		t.Fatalf("upload response = %+v, want provider upload", uploaded)
	}

	// The listing shows the upload as a managed (provider "upload") mod.
	mods := listServerMods(t, cli, gs)
	if len(mods) != 1 || mods[0].Name != "uploaded-e2e.bin" {
		t.Fatalf("mods after upload = %+v", mods)
	}
	if m := mods[0].Meta; m == nil || m.Provider != "upload" || m.InstalledAt == "" {
		t.Fatalf("manifest meta after upload = %+v", mods[0].Meta)
	}

	// Cleanup path works for uploads too.
	delResp, delBody, err := cli.Delete("/servers/" + gs + "/mods?name=" + url.QueryEscape("uploaded-e2e.bin"))
	if err != nil {
		t.Fatalf("DELETE /mods: %v", err)
	}
	if delResp.StatusCode/100 != 2 {
		t.Fatalf("remove expected 2xx, got %d body=%q", delResp.StatusCode, string(delBody))
	}
}
