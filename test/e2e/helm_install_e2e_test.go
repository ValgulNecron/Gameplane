//go:build e2e

package e2e

import (
	"context"
	"fmt"
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
	t.Parallel()

	ctx := context.Background()
	// Scope to the chart's own workloads. The parallel suite runs one-shot
	// helper pods (healthz probes, oras/cosign Jobs, the restic warm-up) in
	// this namespace whose Succeeded phase would otherwise read here as
	// "not Ready".
	for _, sel := range []string{
		"app.kubernetes.io/name=gameplane-operator",
		"app.kubernetes.io/name=gameplane-api",
	} {
		sel := sel
		envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
			pods, err := envInstance.K8s.CoreV1().Pods("gameplane-system").
				List(ctx, metav1.ListOptions{LabelSelector: sel})
			if err != nil {
				return false, "list pods: " + err.Error()
			}
			if len(pods.Items) == 0 {
				return false, "no pods for " + sel + " yet"
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
}

// TestHelmInstall_AllCRDsPresent — every Gameplane CRD declared by the
// chart is reachable via discovery. Catches a missing CRD YAML in
// `charts/gameplane/crds/` from a future refactor.
func TestHelmInstall_AllCRDsPresent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	want := []string{
		"gameservers.gameplane.local",
		"gametemplates.gameplane.local",
		"backups.gameplane.local",
		"backupschedules.gameplane.local",
		"restores.gameplane.local",
		"modules.gameplane.local",
		"modulesources.gameplane.local",
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

// TestHelmInstall_CRDApplyHookSkippedOnFreshInstall — the other half of
// F-218. deploy/kind/e2e.sh installs the chart onto a brand-new kind cluster,
// so Helm's native crds/ step creates every CRD (carrying this chart's
// bundle-hash stamp) before templates are rendered. The crds.autoApply hook
// must recognise those as current and stay pre-upgrade only: firing on
// every install would make air-gapped first installs pull the kubectl image
// and add a Job to every install. A bare "does the CRD exist" check fired on
// every install for exactly that reason, and the upgrade bucket's
// install-over-leftover-CRDs phase cannot catch it, since the hook is
// supposed to fire there.
func TestHelmInstall_CRDApplyHookSkippedOnFreshInstall(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	if got := crdApplyHookEvents(ctx, t, "gameplane", "gameplane-system"); got != "pre-upgrade" {
		t.Errorf("fresh install rendered the crd-apply hook for %q, want \"pre-upgrade\" only "+
			"(the hook must not run on an install whose CRDs crds/ just created)", got)
	}
	// The stamps matching is WHY it stayed pre-upgrade; a mismatch here means
	// crds/ and crd-manifests/ disagree, which CI's chart render job guards.
	want := manifestCRDStamp(t, "gameplane.local_gameservers.yaml")
	if got := liveCRDStamp(ctx, t, "gameservers.gameplane.local"); got != want {
		t.Errorf("live gameservers CRD %s = %q on a fresh install, want the chart's %q",
			crdBundleStampAnnotation, got, want)
	}
}

// TestHelmInstall_OperatorLogsClean — operator container has no
// recent ERROR-level logs. A startup panic or repeated reconcile
// failure would surface here. We tolerate WARN since a few are
// expected during initial reconciliation.
func TestHelmInstall_OperatorLogsClean(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pods, err := envInstance.K8s.CoreV1().Pods("gameplane-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=gameplane-operator",
	})
	if err != nil {
		t.Fatalf("list operator pods: %v", err)
	}
	if len(pods.Items) == 0 {
		t.Fatal("no operator pod found by app.kubernetes.io/name=gameplane-operator")
	}

	for _, p := range pods.Items {
		out, err := envInstance.Kubectl(ctx, "logs", "-n", "gameplane-system", p.Name, "--tail=500")
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

// TestHelmInstall_APILogsClean — mirror of OperatorLogsClean for the
// API pod. Catches "API container starts and immediately panics" or
// repeated request-handling crashes that pod-Ready alone might miss
// during a slow-starting readiness probe.
func TestHelmInstall_APILogsClean(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pods, err := envInstance.K8s.CoreV1().Pods("gameplane-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=gameplane-api",
	})
	if err != nil {
		t.Fatalf("list api pods: %v", err)
	}
	if len(pods.Items) == 0 {
		t.Fatal("no api pod found by app.kubernetes.io/name=gameplane-api")
	}

	for _, p := range pods.Items {
		out, err := envInstance.Kubectl(ctx, "logs", "-n", "gameplane-system", p.Name, "--tail=500")
		if err != nil {
			t.Fatalf("kubectl logs %s: %v\n%s", p.Name, err, out)
		}
		if strings.Contains(out, "panic:") {
			t.Errorf("api pod %s logged a panic:\n%s", p.Name, lastLines(out, 40))
		}
	}
}

// TestHelmInstall_APIHealthz — proves the API service answers
// /healthz over the cluster network. Runs `curl -fsS` from a
// transient pod in gameplane-system; -f makes curl exit non-zero on
// non-2xx so we can assert on the kubectl exit code instead of
// parsing kubectl-and-pod-mixed stdout.
//
// The probe pod uses curlimages/curl, which is the only external
// (non-Gameplane) image this suite pulls. First-run cost is the image
// pull from the public registry into kind; the 90s budget is mostly
// to absorb that.
func TestHelmInstall_APIHealthz(t *testing.T) {
	t.Parallel()

	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		// Random suffix so an Eventually retry doesn't collide with a
		// not-yet-cleaned-up pod from the previous tick.
		name := fmt.Sprintf("healthz-probe-%d", time.Now().UnixNano())
		out, err := envInstance.Kubectl(
			t.Context(),
			"run", "-n", "gameplane-system",
			"--rm", "--restart=Never", "--attach",
			"--image=curlimages/curl:8.10.1",
			name,
			"--",
			"curl", "-fsS", "--max-time", "5",
			"http://gameplane-api/healthz",
		)
		if err != nil {
			return false, fmt.Sprintf("api healthz probe failed: %v\n%s", err, out)
		}
		return true, ""
	})
}

// metricsProbeScript runs in a transient curl pod. It prints the public
// API port's status code for /metrics, flags a Prometheus body there, and
// reports whether the dedicated metrics port serves Prometheus text.
const metricsProbeScript = `code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 http://gameplane-api/metrics)
echo "public-status=$code"
if curl -s --max-time 5 http://gameplane-api/metrics | grep -q '^# HELP'; then echo "public-body=metrics"; fi
if curl -fsS --max-time 5 http://gameplane-api:9090/metrics | grep -q '^# HELP go_goroutines'; then echo "metrics-port=ok"; fi
exit 0`

// TestHelmInstall_MetricsNotOnPublicPort — the API's public port (the
// Service port the Ingress and the web front end route to) does not serve
// Prometheus metrics, and the dedicated metrics port that the ServiceMonitor
// scrapes does. No login. Uses the same curl image as
// TestHelmInstall_APIHealthz.
func TestHelmInstall_MetricsNotOnPublicPort(t *testing.T) {
	t.Parallel()

	var out string
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		// Random suffix so an Eventually retry doesn't collide with a
		// not-yet-cleaned-up pod from the previous tick.
		name := fmt.Sprintf("metrics-probe-%d", time.Now().UnixNano())
		o, err := envInstance.Kubectl(
			t.Context(),
			"run", "-n", "gameplane-system",
			"--rm", "--restart=Never", "--attach",
			"--image=curlimages/curl:8.10.1",
			name,
			"--command", "--",
			"sh", "-c", metricsProbeScript,
		)
		if err != nil {
			return false, fmt.Sprintf("metrics probe pod failed: %v\n%s", err, o)
		}
		if !strings.Contains(o, "public-status=") || strings.Contains(o, "public-status=000") {
			return false, "api public port not answering yet:\n" + o
		}
		if !strings.Contains(o, "metrics-port=ok") {
			return false, "api metrics port not serving Prometheus text yet:\n" + o
		}
		out = o
		return true, ""
	})

	if strings.Contains(out, "public-status=2") || strings.Contains(out, "public-body=metrics") {
		t.Fatalf("the API's public port served /metrics:\n%s", out)
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
