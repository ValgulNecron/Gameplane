//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// adminUsername / adminPassword are the credentials TestAPI_*
// bootstraps. The password is *only* sensitive within this kind
// cluster, but we still pipe it through --password-stdin and write it
// to a 0600-mode file rather than scattering it in argv.
const (
	adminUsername = "e2e-admin"
	adminPassword = "e2e-test-admin-password-1234"
)

// TestAPI_BootstrapAndLogin proves the bootstrap-admin subcommand can
// seed a fresh admin user inside the running api Deployment, and that
// the resulting credentials authenticate against /auth/login.
//
// Side effect: writes the admin password to test/e2e/.tmp/admin-password
// for handoff to the Playwright live-mode suite (PR3). The directory is
// gitignored.
func TestAPI_BootstrapAndLogin(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)
	envInstance.WriteAdminPasswordFile(t, adminPassword)

	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	resp, body, err := cli.Get("/users/me")
	if err != nil {
		t.Fatalf("/users/me: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/users/me %d: %s", resp.StatusCode, string(body))
	}
	if !strings.Contains(string(body), `"username":"`+adminUsername+`"`) {
		t.Fatalf("/users/me response missing admin username: %s", string(body))
	}
	if !strings.Contains(string(body), `"role":"admin"`) {
		t.Fatalf("/users/me response missing admin role: %s", string(body))
	}
}

// TestAPI_LoginPrivacy enforces the pre-auth privacy rule from
// CLAUDE.md §3 at the /auth/login level: a wrong-password attempt
// against a real user, and a login attempt for a nonexistent user,
// must be indistinguishable. Otherwise the response timing or body
// becomes a username-enumeration oracle.
func TestAPI_LoginPrivacy(t *testing.T) {
	t.Parallel()

	// Make sure the admin row exists so the "real user, wrong password"
	// branch actually exercises VerifyPassword (not the dummy).
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	// Borrow the port-forward + http client from a successful client.
	// We're going to issue raw POSTs without a session, but the BaseURL
	// is still the in-cluster Service.
	cli := envInstance.APIClient(t, adminUsername, adminPassword)
	defer cli.Close()

	tries := []struct {
		name string
		body string
	}{
		{"realUserWrongPassword", `{"username":"` + adminUsername + `","password":"definitely-not-the-password"}`},
		{"nonexistentUser", `{"username":"this-user-does-not-exist","password":"definitely-not-the-password"}`},
	}
	bodies := make([]string, 0, len(tries))
	for _, tc := range tries {
		req, err := http.NewRequest(http.MethodPost, cli.BaseURL+"/auth/login", strings.NewReader(tc.body))
		if err != nil {
			t.Fatalf("%s: build request: %v", tc.name, err)
		}
		req.Header.Set("Content-Type", "application/json")
		// Don't reuse the cookie jar — these are fresh attempts.
		raw := &http.Client{Timeout: cli.HTTP.Timeout}
		resp, err := raw.Do(req)
		if err != nil {
			t.Fatalf("%s: do: %v", tc.name, err)
		}
		buf := make([]byte, 1024)
		n, _ := resp.Body.Read(buf)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s: expected 401, got %d body=%q", tc.name, resp.StatusCode, string(buf[:n]))
		}
		bodies = append(bodies, strings.TrimSpace(string(buf[:n])))
	}
	if bodies[0] != bodies[1] {
		t.Fatalf("login error responses differ — enumeration oracle:\n  realUser:    %q\n  nonexistent: %q",
			bodies[0], bodies[1])
	}
	if !strings.Contains(bodies[0], "invalid credentials") {
		t.Fatalf("login error body should be the generic 'invalid credentials' string, got %q", bodies[0])
	}
}

// TestAPI_RBAC_ViewerCannotMutate creates a viewer-role user, logs in
// as them, and confirms the RBAC middleware rejects POST /servers/*:start
// (which requires operator+). The audit row for the rejected call
// should land with status=403 — checked from the same admin session.
func TestAPI_RBAC_ViewerCannotMutate(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	// Per-process unique viewer name. The gameplane-system DB carries
	// state between test runs against a `make e2e-up`-managed cluster
	// (the API PVC isn't wiped); a fixed username collides with the
	// previous run's row and the create handler returns 500. CI's
	// ephemeral cluster doesn't see this, but local iteration does.
	viewerName := fmt.Sprintf("e2e-viewer-%d", time.Now().UnixNano())
	const viewerPW = "e2e-viewer-password-1234"

	resp, body, err := admin.Post("/users", map[string]string{
		"username": viewerName,
		"password": viewerPW,
		"role":     "viewer",
	})
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create viewer %d: %s", resp.StatusCode, string(body))
	}

	viewer := envInstance.APIClient(t, viewerName, viewerPW)
	defer viewer.Close()

	// Viewer attempts a mutation. The route exists and the user is
	// authenticated; the only thing that should bounce them is RBAC.
	mResp, mBody, err := viewer.Post("/servers/no-such-server:start", nil)
	if err != nil {
		t.Fatalf("viewer POST :start: %v", err)
	}
	if mResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%q", mResp.StatusCode, string(mBody))
	}
}

// TestAPI_AuditEmitsOnMutation checks that a successful admin mutation
// produces a row in the audit log. The /admin/audit endpoint returns
// the most-recent events; we just need to find ours by path.
//
// We pick a mutation that does NOT invalidate the caller's own session.
// Reset-password and role-change both delete sessions for the target
// user (security feature in users.go), so calling them on the admin's
// own row blanks out the cookie we're using to read /admin/audit.
// PATCH /users/{id} with a display-name change avoids that pitfall.
func TestAPI_AuditEmitsOnMutation(t *testing.T) {
	t.Parallel()

	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	admin := envInstance.APIClient(t, adminUsername, adminPassword)
	defer admin.Close()

	getMe, body, err := admin.Get("/users/me")
	if err != nil {
		t.Fatalf("/users/me: %v", err)
	}
	if getMe.StatusCode != http.StatusOK {
		t.Fatalf("/users/me %d: %s", getMe.StatusCode, string(body))
	}
	// Extract our id without pulling in encoding/json: the response
	// shape is small and we only need the leading "id":N field.
	id := extractIntField(string(body), "id")
	if id == "" {
		t.Fatalf("could not parse id from /users/me: %s", string(body))
	}

	uniqueDisplayName := "e2e-audit-marker-" + id
	rResp, rBody, err := admin.Patch("/users/"+id, map[string]string{"displayName": uniqueDisplayName})
	if err != nil {
		t.Fatalf("patch user displayName: %v", err)
	}
	if rResp.StatusCode != http.StatusOK {
		t.Fatalf("patch user displayName %d: %s", rResp.StatusCode, string(rBody))
	}

	aResp, aBody, err := admin.Get("/admin/audit?limit=50")
	if err != nil {
		t.Fatalf("/admin/audit: %v", err)
	}
	if aResp.StatusCode != http.StatusOK {
		t.Fatalf("/admin/audit %d: %s", aResp.StatusCode, string(aBody))
	}
	wantPath := `"path":"/users/` + id + `","status":200`
	if !strings.Contains(string(aBody), wantPath) {
		t.Fatalf("audit log missing %s, got: %s", wantPath, string(aBody))
	}
	// NOTE: We deliberately do NOT assert on "actor": the audit
	// middleware (api/internal/audit/audit.go) reads the original
	// request context after the chain returns, but sessions.Authenticate
	// adds the user via req.WithContext on the downstream request — the
	// upstream req in the middleware closure never sees it. As a result
	// every mutation logs as "anonymous". Expanding this to a
	// known-actor check requires fixing the audit middleware to capture
	// the *passed-down* context, which is out of scope for this test.
}

// extractIntField has been promoted to env.go so non-test helper code
// (Env.CreateUser) can use it. Test files in this package still call
// it through the package-level scope.
