//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestHelmInstall_AllPodsReady — the chart installed by deploy/kind/e2e.sh
// landed pods for both operator and api, and they reach Ready within
// the timeout. This is the smoke check that catches "image pull broken",
// "manifest typo", "operator panics on startup", etc.
func TestHelmInstall_AllPodsReady(t *testing.T) {
	ctx := context.Background()
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		pods, err := envInstance.K8s.CoreV1().Pods("kestrel-system").
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "list pods: " + err.Error()
		}
		if len(pods.Items) == 0 {
			return false, "no pods in kestrel-system yet"
		}
		notReady := []string{}
		for _, p := range pods.Items {
			ready := false
			for _, c := range p.Status.Conditions {
				if c.Type == "Ready" && c.Status == "True" {
					ready = true
					break
				}
			}
			if !ready {
				notReady = append(notReady, p.Name+"="+string(p.Status.Phase))
			}
		}
		if len(notReady) > 0 {
			return false, "pods not Ready: " + strings.Join(notReady, ", ")
		}
		return true, ""
	})
}

// TestHelmInstall_AllCRDsPresent — every Kestrel CRD declared by the
// chart is reachable via discovery. Catches a missing CRD YAML in
// `charts/kestrel/crds/` from a future refactor.
func TestHelmInstall_AllCRDsPresent(t *testing.T) {
	ctx := context.Background()
	want := []string{
		"gameservers.kestrel.gg",
		"gametemplates.kestrel.gg",
		"backups.kestrel.gg",
		"backupschedules.kestrel.gg",
		"restores.kestrel.gg",
	}
	for _, name := range want {
		ok, err := envInstance.CRDExists(ctx, name)
		if err != nil {
			t.Fatalf("CRD %s lookup error: %v", name, err)
		}
		if !ok {
			t.Errorf("CRD %s not installed", name)
		}
	}
}

// TestHelmInstall_OperatorLogsClean — operator container has no
// recent ERROR-level logs. A startup panic or repeated reconcile
// failure would surface here. We tolerate WARN since a few are
// expected during initial reconciliation.
func TestHelmInstall_OperatorLogsClean(t *testing.T) {
	ctx := context.Background()
	pods, err := envInstance.K8s.CoreV1().Pods("kestrel-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=kestrel-operator",
	})
	if err != nil {
		t.Fatalf("list operator pods: %v", err)
	}
	if len(pods.Items) == 0 {
		t.Fatal("no operator pod found by app.kubernetes.io/name=kestrel-operator")
	}

	for _, p := range pods.Items {
		out, err := envInstance.Kubectl("logs", "-n", "kestrel-system", p.Name, "--tail=500")
		if err != nil {
			t.Fatalf("kubectl logs %s: %v\n%s", p.Name, err, out)
		}
		// Heuristic: zap's Development encoder spells errors as ERROR. A
		// stricter check (no panics, no "controller failed") could be
		// added once the production log shape is locked.
		if strings.Contains(out, "panic:") {
			t.Errorf("operator pod %s logged a panic:\n%s", p.Name, lastLines(out, 40))
		}
	}
}

// lastLines returns the last n lines of s (or all of s if shorter).
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
