//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// oidcHelmSession drives one OIDC login against the Helm-seeded "helm"
// provider, like oidcHelmLogin, and returns an APIClient for the resulting
// session (CSRF token taken from the gameplane_csrf cookie) plus the user id
// GET /users/me reports. The client reuses the caller's port-forward, so its
// Close is a no-op.
func oidcHelmSession(t *testing.T, apiBase, idpBase, sub, email string, groups []string) (*APIClient, int64) {
	t.Helper()
	jar := newInsecureCookieJar()
	cli := &http.Client{Jar: jar, Timeout: 30 * time.Second, CheckRedirect: oidcNoRedirect}

	startResp := oidcDo(t, cli, http.MethodGet, apiBase+"/auth/oidc/helm/start", nil)
	_ = startResp.Body.Close()
	if startResp.StatusCode != http.StatusFound {
		t.Fatalf("oidc start status = %d, want 302", startResp.StatusCode)
	}
	authorizeURL, err := url.Parse(startResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse start Location: %v", err)
	}
	idpURL, err := url.Parse(idpBase)
	if err != nil {
		t.Fatalf("parse idp base: %v", err)
	}
	authorizeURL.Scheme = idpURL.Scheme
	authorizeURL.Host = idpURL.Host
	q := authorizeURL.Query()
	q.Set("sub", sub)
	q.Set("email", email)
	q.Set("groups", strings.Join(groups, ","))
	authorizeURL.RawQuery = q.Encode()

	authorizeResp := oidcDo(t, cli, http.MethodGet, authorizeURL.String(), nil)
	_ = authorizeResp.Body.Close()
	if authorizeResp.StatusCode != http.StatusFound {
		t.Fatalf("fake idp authorize status = %d, want 302", authorizeResp.StatusCode)
	}
	callbackLoc, err := url.Parse(authorizeResp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parse authorize Location: %v", err)
	}

	callbackResp := oidcDo(t, cli, http.MethodGet, apiBase+"/auth/oidc/helm/callback?"+callbackLoc.RawQuery, nil)
	cbBody, _ := io.ReadAll(callbackResp.Body)
	_ = callbackResp.Body.Close()
	if callbackResp.StatusCode != http.StatusFound {
		t.Fatalf("oidc callback status = %d, want 302: %s", callbackResp.StatusCode, string(cbBody))
	}

	meResp := oidcDo(t, cli, http.MethodGet, apiBase+"/users/me", nil)
	meBody, _ := io.ReadAll(meResp.Body)
	_ = meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("/users/me %d: %s", meResp.StatusCode, string(meBody))
	}
	var me struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(meBody, &me); err != nil || me.ID == 0 {
		t.Fatalf("decode /users/me id: %v\n%s", err, string(meBody))
	}

	csrf := ""
	for _, c := range jar.Cookies(nil) {
		if c.Name == "gameplane_csrf" {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("oidc login set no gameplane_csrf cookie")
	}
	return &APIClient{BaseURL: apiBase, CSRF: csrf, HTTP: cli}, me.ID
}

// publicShareGet resolves a share token anonymously and returns the status
// and body.
func publicShareGet(t *testing.T, apiBase, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, apiBase+"/shares/"+token, nil)
	if err != nil {
		t.Fatalf("build share request: %v", err)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET /shares/<token>: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// TestAPI_AccountRemoval_RevokesSharesAndAllowsSSOReprovision checks what
// deleting a user account removes: a share link the user created stops
// resolving, and the same IdP subject can sign in again afterwards, as a
// fresh user.
//
// Budget: one e2e-admin login (APIClient) and two OIDC callbacks
// (auth.OIDCCallbackLimiter, burst 10/min). The SSO user is put in the
// Helm-seeded admin group so it can create a share link; e2e-admin remains
// a user manager, so the delete passes the lockout guard. Writes no auth
// config and no roleMappings.
func TestAPI_AccountRemoval_RevokesSharesAndAllowsSSOReprovision(t *testing.T) {
	t.Parallel()

	const ns = "gameplane-games"
	ctx := context.Background()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	sub := "e2e-account-removal-" + suffix
	email := sub + "@e2e.example"

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	apiBase, idpBase := oidcHelmPorts(t)

	// --- An SSO user who owns a GameServer and shares it ------------------
	sso, firstID := oidcHelmSession(t, apiBase, idpBase, sub, email, []string{oidcHelmAdminGroup})
	t.Cleanup(func() {
		r, _, _ := admin.Delete("/users/" + strconv.FormatInt(firstID, 10))
		if r != nil {
			_ = r.Body.Close()
		}
	})

	tmplName := "e2e-acct-rm-tmpl-" + suffix
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E busybox (account removal)",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).Create(ctx, tmpl, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create template %s: %v", tmplName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})

	gsName := "e2e-acct-rm-gs-" + suffix
	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata": map[string]any{
			"name":      gsName,
			"namespace": ns,
			"annotations": map[string]any{
				"gameplane.local/owner-id": strconv.FormatInt(firstID, 10),
				"gameplane.local/owner":    email,
			},
		},
		"spec": map[string]any{"templateRef": map[string]any{"name": tmplName}},
	}}
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Create(ctx, gs, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create gameserver %s/%s: %v", ns, gsName, err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	resp, body, err := sso.Post("/servers/"+gsName+":shares", map[string]any{"canStart": false, "neverExpires": true})
	if err != nil {
		t.Fatalf("POST /servers/%s:shares: %v", gsName, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /servers/%s:shares: status=%d body=%s", gsName, resp.StatusCode, string(body))
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.Token == "" {
		t.Fatalf("decode share token: %v body=%s", err, string(body))
	}
	if status, b := publicShareGet(t, apiBase, created.Token); status != http.StatusOK {
		t.Fatalf("GET /shares/<token> before account removal: status=%d body=%s", status, string(b))
	}

	// --- Remove the account ------------------------------------------------
	resp, body, err = admin.Delete("/users/" + strconv.FormatInt(firstID, 10))
	if err != nil {
		t.Fatalf("DELETE /users/%d: %v", firstID, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /users/%d: status=%d body=%s", firstID, resp.StatusCode, string(body))
	}

	// The share link stops resolving, with the uniform not-found body.
	status, b := publicShareGet(t, apiBase, created.Token)
	if status != http.StatusNotFound {
		t.Errorf("GET /shares/<token> after account removal: status=%d want=%d body=%s", status, http.StatusNotFound, string(b))
	}
	if !bytes.Equal(b, []byte(`{"error":"not found"}`+"\n")) {
		t.Errorf("GET /shares/<token> after account removal: body=%q, want the uniform not-found body", string(b))
	}

	// --- The same IdP subject signs in again, as a fresh user --------------
	_, secondID := oidcHelmSession(t, apiBase, idpBase, sub, email, []string{"some-unmapped-group"})
	t.Cleanup(func() {
		r, _, _ := admin.Delete("/users/" + strconv.FormatInt(secondID, 10))
		if r != nil {
			_ = r.Body.Close()
		}
	})
	if secondID == firstID {
		t.Fatalf("sign-in after account removal returned the removed user id %d", firstID)
	}
}
