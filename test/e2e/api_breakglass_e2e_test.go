//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// TestAPI_BootstrapAdminForceEndsExistingSessions: a forced reset through
// the bootstrap-admin break-glass command ends the reset account's
// existing sessions, the same way the dashboard password reset does.
//
// It uses its own throwaway account, never e2e-admin (resetting e2e-admin
// would end the sessions other tests in the job hold). Budget: zero
// e2e-admin logins; one local login as the throwaway account (a fresh
// per-username bucket, one slot of the job's shared per-IP budget), plus
// two kubectl exec calls.
func TestAPI_BootstrapAdminForceEndsExistingSessions(t *testing.T) {
	t.Parallel()

	const (
		username      = "e2e-breakglass-reset"
		firstPassword = "e2e-breakglass-first-password-1"
		resetPassword = "e2e-breakglass-reset-password-2"
	)

	// Let the shared e2e-admin bootstrap (once per process) finish first,
	// so this test's bootstrap-admin execs never overlap with it.
	envInstance.BootstrapAdmin(t, adminUsername, adminPassword)

	bootstrap := func(password string) {
		t.Helper()
		out, err := envInstance.KubectlWithStdin(t.Context(), password+"\n",
			"exec", "-i", "-n", "gameplane-system", "deploy/gameplane-api", "--",
			"/api", "bootstrap-admin",
			"--username="+username,
			"--password-stdin",
			"--force",
		)
		if err != nil {
			t.Fatalf("bootstrap-admin: %v\n%s", err, out)
		}
	}

	// --force on the first call too, so a rerun against a reused cluster
	// resets the account instead of failing on "already exists".
	bootstrap(firstPassword)

	cli := envInstance.APIClient(t, username, firstPassword)
	defer cli.Close()

	resp, body, err := cli.Get("/users/me")
	if err != nil {
		t.Fatalf("baseline /users/me: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("baseline /users/me: status=%d body=%s", resp.StatusCode, string(body))
	}

	bootstrap(resetPassword)

	resp, body, err = cli.Get("/users/me")
	if err != nil {
		t.Fatalf("post-reset /users/me: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-reset /users/me: status=%d body=%s, want 401 (session ended by the reset)",
			resp.StatusCode, string(body))
	}
}
