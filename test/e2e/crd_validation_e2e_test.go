//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestCRD_Validation_* assert that the CRD OpenAPI schemas under
// operator/config/crd/ reject obviously-invalid specs at the
// kube-apiserver admission layer — no controller reconciliation is
// involved. We use kubectl apply -f - rather than the dynamic client
// because admission errors flow through kubectl's usual stderr surface,
// which is easier to assert against than to fish out of a typed client
// error.

func TestCRD_Validation_GameServerWithoutTemplate(t *testing.T) {
	yaml := `apiVersion: gameplane.gg/v1alpha1
kind: GameServer
metadata:
  name: e2e-validation-gs-no-template
  namespace: gameplane-games
spec:
  templateRef:
    name: ""
`
	expectAdmissionRejection(t, yaml, []string{"templateRef", "Required value", "MinLength", "name"})
}

func TestCRD_Validation_BackupScheduleBadCron(t *testing.T) {
	yaml := `apiVersion: gameplane.gg/v1alpha1
kind: BackupSchedule
metadata:
  name: e2e-validation-bksched-badcron
  namespace: gameplane-games
spec:
  serverRef:
    name: any-server
  schedule: "not-a-cron"
  repoRef:
    name: e2e-restic-creds
    key: repo
`
	expectAdmissionRejection(t, yaml, []string{"schedule", "MinLength", "Invalid", "cron"})
}

func TestCRD_Validation_BackupRequiresServerRef(t *testing.T) {
	yaml := `apiVersion: gameplane.gg/v1alpha1
kind: Backup
metadata:
  name: e2e-validation-bk-no-serverref
  namespace: gameplane-games
spec:
  repoRef:
    name: e2e-restic-creds
    key: repo
`
	expectAdmissionRejection(t, yaml, []string{"serverRef", "Required value", "missing"})
}

func TestCRD_Validation_GameTemplateRequiresImage(t *testing.T) {
	yaml := `apiVersion: gameplane.gg/v1alpha1
kind: GameTemplate
metadata:
  name: e2e-validation-tmpl-no-image
spec:
  displayName: "no image"
  game: "busybox"
  version: "1"
`
	expectAdmissionRejection(t, yaml, []string{"image", "Required value", "MinLength"})
}

// expectAdmissionRejection runs `kubectl apply -f -` with the given
// YAML and asserts the apply errors out. The matchAny list is a set of
// substrings; the assertion passes when at least one of them appears in
// the kubectl output, so the test stays robust against minor wording
// differences across kube-apiserver versions while still proving the
// rejection was about the right field.
func expectAdmissionRejection(t *testing.T, yaml string, matchAny []string) {
	t.Helper()
	out, err := envInstance.KubectlWithStdin(yaml, "apply", "-f", "-")
	if err == nil {
		// Best-effort delete to keep the cluster clean for subsequent
		// tests; the apply we just made created a real CR.
		_, _ = envInstance.KubectlWithStdin(yaml, "delete", "-f", "-", "--ignore-not-found")
		t.Fatalf("CRD schema accepted a spec it must reject — the validation regressed in operator/api/v1alpha1/*_types.go (or the chart CRDs are stale; helm never upgrades crds/, recreate the cluster after CRD changes).\nyaml:\n%s\nkubectl output:\n%s",
			yaml, out)
		return
	}
	lower := strings.ToLower(out)
	for _, m := range matchAny {
		if strings.Contains(lower, strings.ToLower(m)) {
			return
		}
	}
	t.Fatalf("admission rejection did not mention any of %v\nkubectl output:\n%s", matchAny, out)
}
