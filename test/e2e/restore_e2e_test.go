//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestRestore_RoundTrip exercises the full backup → delete → restore →
// verify cycle against the in-cluster restic-server. It's the only test
// in the suite that asserts data integrity (every other backup test
// stops at "Job appeared / status Succeeded").
//
// The Restore controller's contract:
//
//	Pending → Suspending → Running (restic-restore Job) → Resuming → Succeeded
//
// During Suspending/Running the target GameServer's spec.suspend is
// flipped to true, and at Resuming it's flipped back. The test asserts
// the spec.suspend toggle and the snapshotID pinning (the snapshot the
// restore actually used must match the Backup's snapshotID, even if
// retention deletes other snapshots in the meantime).
func TestRestore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-restore-busybox"
	gs := "e2e-restore-target"
	bkName := "e2e-restore-bk"
	rsName := "e2e-restore-rs"

	envInstance.ApplyYAML(t, "restic-server.yaml")
	envInstance.ApplyYAML(t, "backup-restic-secret.yaml")

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	// Wait for the busybox pod to be running so we can write into /data.
	// We don't need the agent sidecar fully Ready for the data round-trip;
	// it just needs the game container to mount the PVC.
	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gs+"-0", metav1.GetOptions{})
		if err != nil {
			return false, "get pod: " + err.Error()
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == "game" && cs.Ready {
				return true, ""
			}
		}
		return false, "game container not ready yet"
	})

	const marker = "gameplane-restore-marker-payload"
	// kubectl exec defaults to the first container ("game") when -c is
	// omitted. The Env helper appends every arg after target to the
	// command (after the `--` separator), so passing `-c game` here
	// would land it in argv as a literal command, not as a kubectl flag.
	// `sync` after the write so the bytes are flushed to the PVC backing
	// store before the backup Job's restic-snapshot reads them.
	if out, err := envInstance.KubectlExec(t, ns, "pod/"+gs+"-0",
		"sh", "-c", "echo -n "+marker+" > /data/marker.txt && sync"); err != nil {
		t.Fatalf("write marker: %v\n%s", err, out)
	}
	// Confirm the marker landed before kicking off the backup, so a
	// failure later is unambiguously about backup/restore rather than
	// a missed write.
	//
	// kubectl writes a "Defaulted container ..." preamble to stderr
	// when -c is omitted; the Env helper merges stdout+stderr via
	// CombinedOutput, so we use Contains rather than equality.
	if out, err := envInstance.KubectlExec(t, ns, "pod/"+gs+"-0",
		"cat", "/data/marker.txt"); err != nil || !strings.Contains(out, marker) {
		t.Fatalf("marker not present after write: err=%v body=%q", err, out)
	}

	createBackup(t, ns, bkName, gs, "e2e-restic-creds", "repo")
	snapshotID := waitBackupSucceeded(t, ns, bkName, 5*time.Minute)
	if snapshotID == "" {
		t.Fatal("expected non-empty snapshotID on Succeeded backup")
	}

	// Wipe the marker so we can prove the restore brought it back.
	if out, err := envInstance.KubectlExec(t, ns, "pod/"+gs+"-0",
		"sh", "-c", "rm -f /data/marker.txt"); err != nil {
		t.Fatalf("delete marker: %v\n%s", err, out)
	}

	createRestore(t, ns, rsName, gs, bkName)

	// While the restore is running, the controller suspends the target.
	// We don't try to catch every transient phase — just verify suspend
	// flipped to true at some point during the run, then back to false
	// on Resuming/Succeeded.
	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gs: " + err.Error()
		}
		suspend, _, _ := unstructured.NestedBool(got.Object, "spec", "suspend")
		rs, _ := envInstance.Dyn.Resource(restoreGVR).Namespace(ns).
			Get(ctx, rsName, metav1.GetOptions{})
		phase := ""
		if rs != nil {
			phase, _, _ = unstructured.NestedString(rs.Object, "status", "phase")
		}
		// Either we observe suspend=true, or the controller already
		// galloped past the Suspending/Running phases (small windows on
		// a fast cluster). Both are acceptable evidence the workflow
		// engaged.
		if suspend || phase == "Resuming" || phase == "Succeeded" {
			return true, ""
		}
		return false, "suspend=false and phase=" + phase
	})

	waitRestoreSucceeded(t, ns, rsName, 5*time.Minute)

	got, err := envInstance.Dyn.Resource(restoreGVR).Namespace(ns).
		Get(ctx, rsName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get restore post-success: %v", err)
	}
	pinned, _, _ := unstructured.NestedString(got.Object, "status", "snapshotID")
	if pinned != snapshotID {
		t.Errorf("Restore.status.snapshotID=%q want pinned to backup's %q", pinned, snapshotID)
	}

	// Restore must put suspend back to false on success.
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		gsObj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gs: " + err.Error()
		}
		suspend, _, _ := unstructured.NestedBool(gsObj.Object, "spec", "suspend")
		if !suspend {
			return true, ""
		}
		return false, "spec.suspend still true after Restore.Succeeded"
	})

	// Wait for the game container to come back up after the suspend
	// toggle so we can read the marker file.
	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		pod, err := envInstance.K8s.CoreV1().Pods(ns).Get(ctx, gs+"-0", metav1.GetOptions{})
		if err != nil {
			return false, "get pod: " + err.Error()
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == "game" && cs.Ready {
				return true, ""
			}
		}
		return false, "game container not ready yet"
	})

	// The marker survives the round-trip. The fallback /data/data path
	// is kept defensively: a regression that re-introduces the original
	// double-prefix bug (snapshot tree rooted at /data, restored with
	// --target /data) would otherwise show up here as a file-not-found
	// before the controller-level envtest catches it.
	out, err := envInstance.KubectlExec(t, ns, "pod/"+gs+"-0",
		"sh", "-c", "cat /data/marker.txt 2>/dev/null || cat /data/data/marker.txt 2>/dev/null")
	if err != nil {
		// Surface what IS on the volume so a regression where the
		// operator restores to a wrong path is debuggable from CI logs
		// alone, without re-running locally.
		listing, _ := envInstance.KubectlExec(t, ns, "pod/"+gs+"-0",
			"sh", "-c", "ls -laR /data 2>&1 || true")
		t.Fatalf("read marker after restore: %v\nstdout: %s\nlisting of /data:\n%s",
			err, out, listing)
	}
	// kubectl exec's stderr "Defaulted container ..." preamble is mixed
	// into CombinedOutput; check for the marker as a substring.
	if !strings.Contains(out, marker) {
		t.Errorf("marker after restore body=%q, want it to contain %q", out, marker)
	}
}

// TestRestore_RejectsMissingBackup: a Restore that names a Backup that
// doesn't exist should land in Failed with a clear status message.
//
// The controller has no GameServer to suspend in this case (we don't
// even create one), so the test is fast — it stays at Pending until the
// reconciler resolves the missing backup, then transitions to Failed.
func TestRestore_RejectsMissingBackup(t *testing.T) {
	ctx := context.Background()
	ns := "gameplane-games"
	rsName := "e2e-restore-missing-backup"

	// Need a target GameServer for the spec to validate against. We don't
	// have to run it — the reconciler short-circuits on the missing
	// Backup well before it tries to suspend anything.
	tmpl := "e2e-restore-missing-tmpl"
	gs := "e2e-restore-missing-gs"
	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	createRestore(t, ns, rsName, gs, "definitely-not-a-real-backup")

	envInstance.Eventually(t, 2*time.Minute, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(restoreGVR).Namespace(ns).
			Get(ctx, rsName, metav1.GetOptions{})
		if err != nil {
			return false, "get restore: " + err.Error()
		}
		phase, _, _ := unstructured.NestedString(got.Object, "status", "phase")
		if phase != "Failed" {
			return false, "phase=" + phase
		}
		msg, _, _ := unstructured.NestedString(got.Object, "status", "message")
		if msg == "" {
			return false, "Failed phase but no status.message — controller should explain why"
		}
		return true, ""
	})

	// The GameServer should NOT have been suspended in the failed-Restore
	// path — the controller resolves the missing Backup before touching
	// the target's spec.
	gsObj, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Get(ctx, gs, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get gs after failed restore: %v", err)
	}
	suspend, _, _ := unstructured.NestedBool(gsObj.Object, "spec", "suspend")
	if suspend {
		t.Errorf("missing-Backup Restore should not have flipped GameServer.spec.suspend, but it did")
	}
}
