//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// TestUpgrade_FromPreviousRelease covers the single most dangerous operation a
// user performs and the only one CI never exercised: `helm upgrade`.
//
// The cluster this runs against was installed by deploy/kind/upgrade.sh from
// the PUBLISHED chart and PUBLISHED GHCR images of the previous release — not
// from the working tree. The test seeds real state into it, upgrades to the
// working-tree chart, and asserts nothing was lost.
//
// It is deliberately NOT t.Parallel(): it upgrades the whole Helm release out
// from under everything in the namespace, so it gets its own cluster and its
// own CI job (the same reason the multicluster bucket has one).
//
// The four things an upgrade can silently break, and what proves each:
//
//  1. CRDs. Helm installs crds/ on first install and NEVER updates them on
//     upgrade. The chart works around that with a pre-upgrade hook
//     (crds.autoApply), which until now had zero e2e coverage of the path it
//     was written for. Asserted by diffing the live CRD schema before/after.
//  2. Workloads. A running GameServer and its data must survive.
//  3. The database. Migrations run against a populated SQLite PVC on startup;
//     a user created before the upgrade must still be able to log in after.
//  4. The new components must actually come up clean.
func TestUpgrade_FromPreviousRelease(t *testing.T) {
	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-upgrade-busybox"
	gs := "e2e-upgrade-target"

	const (
		adminUser = "upgradeadmin"
		adminPass = "upgrade-admin-pw-1"
		marker    = "gameplane-upgrade-marker-payload"
	)

	// ---- 1. seed state into the OLD release ------------------------------

	// Bootstrap the admin BEFORE upgrading. Logging in with these same
	// credentials afterwards is what proves the database and its migrations
	// survived: the user row only exists in the pre-upgrade SQLite file.
	envInstance.BootstrapAdmin(t, adminUser, adminPass)

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)
	waitGameContainerReady(t, ns, gs+"-0", 2*time.Minute)

	// `sync` so the bytes are on the PVC's backing store before the pod is
	// restarted by the upgrade, not sitting in the page cache.
	if out, err := envInstance.KubectlExec(t, ns, "pod/"+gs+"-0",
		"sh", "-c", "echo -n "+marker+" > /data/marker.txt && sync"); err != nil {
		t.Fatalf("write marker: %v\n%s", err, out)
	}

	// ---- 2. work out what the upgrade is supposed to change --------------

	// Rather than hardcoding a field name that goes stale, compute the drift:
	// which properties does the working-tree CRD declare that the installed
	// (old-release) CRD does not? Those are exactly what the pre-upgrade hook
	// has to apply, and asserting on them cannot rot.
	const crdName = "gametemplates.gameplane.local"
	liveBefore := crdSpecProperties(t, crdName)
	wantAfter := manifestSpecProperties(t, "gameplane.local_gametemplates.yaml")

	var newProps []string
	for p := range wantAfter {
		if _, ok := liveBefore[p]; !ok {
			newProps = append(newProps, p)
		}
	}
	if len(newProps) == 0 {
		// Not a failure, but the CRD half of this test would be vacuous, and a
		// silently-vacuous assertion is worse than none. Say so loudly.
		t.Logf("NOTE: the working-tree GameTemplate CRD declares no spec property "+
			"the installed release lacks (%d properties, identical set) — the CRD "+
			"assertion below can only prove the hook did not REMOVE anything.",
			len(wantAfter))
	} else {
		t.Logf("upgrade must add %d GameTemplate spec propert(ies) to the live CRD: %s",
			len(newProps), strings.Join(newProps, ", "))
	}

	// ---- 3. upgrade to the working tree ----------------------------------

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	tag := envInstance.Tag
	if tag == "" {
		tag = "e2e"
	}
	upgrade := exec.CommandContext(ctx, "helm", "upgrade", "gameplane",
		filepath.Join(repoRoot, "charts", "gameplane"),
		"--namespace", "gameplane-system",
		"--set", "image.registry=gameplane-test",
		"--set", "image.tag="+tag,
		"--set", "ingress.enabled=false",
		"--set", "web.enabled=false",
		"--set", "operator.agentImage=gameplane-test/agent:"+tag,
		"--set", "api.resources.limits.memory=1Gi",
		"--set", "operator.leaderElect=false",
		"--set", "defaultModuleSource.enabled=false",
		"--wait", "--timeout", "6m",
	)
	upgrade.Env = append(os.Environ(), "KUBECONFIG="+os.Getenv("KUBECONFIG"))
	if out, err := upgrade.CombinedOutput(); err != nil {
		t.Fatalf("helm upgrade to the working-tree chart failed: %v\n%s", err, out)
	}

	// ---- 4. the CRDs were actually updated -------------------------------

	// This is the hook's literal contract. Without it Helm leaves the old
	// release's CRD schema in place and any new field silently vanishes on
	// apply — the failure mode the hook exists to prevent.
	liveAfter := crdSpecProperties(t, crdName)
	for _, p := range newProps {
		if _, ok := liveAfter[p]; !ok {
			t.Errorf("GameTemplate CRD is missing spec property %q after upgrade — the "+
				"crds.autoApply pre-upgrade hook did not apply the new schema", p)
		}
	}
	for p := range liveBefore {
		if _, ok := liveAfter[p]; !ok {
			t.Errorf("GameTemplate CRD LOST spec property %q across the upgrade", p)
		}
	}

	// ---- 5. the workload and its data survived ---------------------------

	got, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("GameServer %s/%s is gone after the upgrade: %v", ns, gs, err)
	}
	if suspend, _, _ := unstructured.NestedBool(got.Object, "spec", "suspend"); suspend {
		t.Errorf("GameServer was suspended by the upgrade; spec.suspend should be untouched")
	}

	waitPVCBound(t, ns, gs+"-data", 2*time.Minute)
	waitGameContainerReady(t, ns, gs+"-0", 4*time.Minute)

	// Retry the read rather than exec'ing once. An upgrade re-renders the
	// StatefulSet, so the pod keeps its name but is legitimately replaced —
	// a Ready observation can be of the outgoing pod, and the exec then
	// lands mid-restart ("container not found", "no running task found").
	// That is the workload behaving correctly, not a regression, so the
	// assertion tolerates the churn and waits for the volume to be readable.
	// What is being proven is that the bytes survived, not that they were
	// readable at one exact instant.
	envInstance.Eventually(t, 3*time.Minute, func() (bool, string) {
		out, err := envInstance.KubectlExec(t, ns, "pod/"+gs+"-0", "cat", "/data/marker.txt")
		if err != nil {
			return false, "exec: " + err.Error() + " out=" + out
		}
		// kubectl mixes its "Defaulted container ..." stderr preamble into
		// CombinedOutput, so check for the marker as a substring.
		if !strings.Contains(out, marker) {
			return false, "marker not in output yet: " + out
		}
		return true, ""
	})

	// ---- 6. the database and its migrations survived ---------------------

	// Deliberately after the upgrade: the API pod was replaced, so this both
	// re-establishes a port-forward against the NEW pod and proves the new
	// binary's migrations ran against the old release's populated SQLite file
	// without dropping the user rows.
	client := envInstance.APIClient(t, adminUser, adminPass)
	defer client.Close()

	resp, body, err := client.Get("/servers/" + gs + "?namespace=" + ns)
	if err != nil {
		t.Fatalf("read the pre-upgrade server through the upgraded API: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("GET /servers/%s after upgrade: status %d body %s", gs, resp.StatusCode, body)
	}
	if !strings.Contains(string(body), gs) {
		t.Errorf("upgraded API does not return the pre-upgrade GameServer %q; body=%s", gs, body)
	}
}

// waitGameContainerReady polls until the named pod's "game" container reports
// Ready. Shared by the seed and post-upgrade phases.
func waitGameContainerReady(t *testing.T, ns, pod string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	envInstance.Eventually(t, timeout, func() (bool, string) {
		p, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			return false, "get pod: " + err.Error()
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name == "game" && cs.Ready {
				return true, ""
			}
		}
		return false, "game container not ready yet"
	})
}

// crdSpecProperties returns the top-level property names under the CRD's
// spec schema, as the apiserver currently serves it.
func crdSpecProperties(t *testing.T, crdName string) map[string]struct{} {
	t.Helper()
	crd, err := envInstance.Dyn.Resource(crdGVR).Get(context.Background(), crdName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get CRD %s: %v", crdName, err)
	}
	versions, _, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil || len(versions) == 0 {
		t.Fatalf("CRD %s has no spec.versions: %v", crdName, err)
	}
	v, ok := versions[0].(map[string]any)
	if !ok {
		t.Fatalf("CRD %s spec.versions[0] is not an object", crdName)
	}
	props, _, err := unstructured.NestedMap(v, "schema", "openAPIV3Schema", "properties", "spec", "properties")
	if err != nil {
		t.Fatalf("CRD %s: read spec properties: %v", crdName, err)
	}
	return keySet(props)
}

// manifestSpecProperties reads the same property set out of the working-tree
// chart's crd-manifests/ copy — the exact files the pre-upgrade hook applies
// (CI already guards crd-manifests/ against crds/ drift).
func manifestSpecProperties(t *testing.T, file string) map[string]struct{} {
	t.Helper()
	path := filepath.Join("..", "..", "charts", "gameplane", "crd-manifests", file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	obj := &unstructured.Unstructured{Object: doc}
	versions, _, err := unstructured.NestedSlice(obj.Object, "spec", "versions")
	if err != nil || len(versions) == 0 {
		t.Fatalf("%s has no spec.versions: %v", path, err)
	}
	v, ok := versions[0].(map[string]any)
	if !ok {
		t.Fatalf("%s spec.versions[0] is not an object", path)
	}
	props, _, err := unstructured.NestedMap(v, "schema", "openAPIV3Schema", "properties", "spec", "properties")
	if err != nil {
		t.Fatalf("%s: read spec properties: %v", path, err)
	}
	return keySet(props)
}

func keySet(m map[string]any) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}
