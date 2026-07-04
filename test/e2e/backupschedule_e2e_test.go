//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// TestBackupSchedule_SuspendStopsScheduling proves spec.suspend short-
// circuits the cron path: with suspend=true no Backup CRs are emitted,
// and flipping back to false resumes scheduling within one cron window.
//
// We can't observe the operator's "should have fired but skipped" decision
// directly, so the contract is enforced at the cluster-resource level: no
// new Backups owned by the schedule appear during the suspended window.
func TestBackupSchedule_SuspendStopsScheduling(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-bksched-suspend-tmpl"
	gs := "e2e-bksched-suspend-target"
	schedName := "e2e-bksched-suspend"

	ensureResticRepo(t)

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	// Create the schedule already suspended so we can take a baseline
	// count of zero before unsuspending.
	sched := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "BackupSchedule",
		"metadata":   map[string]any{"name": schedName, "namespace": ns},
		"spec": map[string]any{
			"serverRef": map[string]any{"name": gs},
			"schedule":  "* * * * *",
			"repoRef":   map[string]any{"name": "e2e-restic-creds", "key": "repo"},
			"strategy":  "restic-snapshot",
			"quiesce":   false,
			"suspend":   true,
		},
	}}
	if _, err := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
		Create(ctx, sched, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create suspended backupschedule: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
			Delete(context.Background(), schedName, metav1.DeleteOptions{})
	})

	// Watch for ~75s. A non-suspended schedule with `* * * * *` would
	// have fired once. Any owned Backup appearing in this window is a
	// regression in the suspend gate.
	deadline := time.Now().Add(75 * time.Second)
	for time.Now().Before(deadline) {
		bks, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list backups: %v", err)
		}
		for _, item := range bks.Items {
			for _, owner := range item.GetOwnerReferences() {
				if owner.Kind == "BackupSchedule" && owner.Name == schedName {
					t.Fatalf("suspended schedule still emitted Backup %s", item.GetName())
				}
			}
		}
		time.Sleep(5 * time.Second)
	}

	// Flip suspend to false. Within the next cron window (≤60s) plus
	// some reconcile slack, a Backup must appear.
	patch := []byte(`{"spec":{"suspend":false}}`)
	if _, err := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
		Patch(ctx, schedName, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch suspend=false: %v", err)
	}
	envInstance.Eventually(t, 2*time.Minute+30*time.Second, func() (bool, string) {
		bks, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "list backups: " + err.Error()
		}
		for _, item := range bks.Items {
			for _, owner := range item.GetOwnerReferences() {
				if owner.Kind == "BackupSchedule" && owner.Name == schedName {
					return true, ""
				}
			}
		}
		return false, "no Backup CR yet after unsuspend"
	})
}

// TestBackupSchedule_RetentionTrimsPast — with keepLast=1, the schedule
// must keep at most one Backup CR after multiple cron firings have
// accumulated. The reconciler GCs older Backups owned by the schedule;
// retention is the dashboard contract that prevents unbounded growth.
//
// Slowest test in the new suite: needs ≥2 cron windows (~2.5 min) plus
// retention reconcile slack. The 6-min timeout absorbs both with margin.
func TestBackupSchedule_RetentionTrimsPast(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-bksched-retention-tmpl"
	gs := "e2e-bksched-retention-target"
	schedName := "e2e-bksched-retention"

	ensureResticRepo(t)

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	sched := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "BackupSchedule",
		"metadata":   map[string]any{"name": schedName, "namespace": ns},
		"spec": map[string]any{
			"serverRef":         map[string]any{"name": gs},
			"schedule":          "* * * * *",
			"repoRef":           map[string]any{"name": "e2e-restic-creds", "key": "repo"},
			"strategy":          "restic-snapshot",
			"quiesce":           false,
			"concurrencyPolicy": "Allow",
			"retention":         map[string]any{"keepLast": int64(1)},
		},
	}}
	if _, err := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
		Create(ctx, sched, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create retention backupschedule: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
			Delete(context.Background(), schedName, metav1.DeleteOptions{})
	})

	// Wait for ≥2 cron firings. The first may take up to 60s of cron
	// alignment + reconcile slack; the second adds another 60s. Allowing
	// 3 minutes here gives the controller a comfortable cushion before
	// retention is asserted.
	envInstance.Eventually(t, 3*time.Minute, func() (bool, string) {
		bks, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "list backups: " + err.Error()
		}
		got := 0
		for _, item := range bks.Items {
			for _, owner := range item.GetOwnerReferences() {
				if owner.Kind == "BackupSchedule" && owner.Name == schedName {
					got++
					break
				}
			}
		}
		if got >= 2 {
			return true, ""
		}
		return false, "fewer than 2 owned Backups produced (got " + itoa(got) + ")"
	})

	// Retention reconcile is asynchronous — the controller may emit a
	// new Backup and then trim the oldest on a follow-up tick. The
	// helper counts only Succeeded backups (in-flight ones are expected
	// with a one-minute cron and are never trimmed); if more than
	// keepLast succeeded backups persist, retention is broken. Two cron
	// windows of slack lets the second backup finish and get trimmed.
	waitBackupCount(t, ns, schedName, 1, 3*time.Minute)
}

// TestBackupSchedule_ConcurrencyForbid — concurrencyPolicy=Forbid must
// prevent a new Backup from starting while a previous one is still in
// flight. We force overlap by using a slow strategy (restic-snapshot
// against the in-cluster restic-server takes >5s end-to-end on kind),
// then count active Backups while one is mid-run.
//
// We're not asserting "exactly one ever exists" — Backup CRs persist
// after they finish. We assert "at most one is non-terminal at a time",
// which is the operator's actual contract.
func TestBackupSchedule_ConcurrencyForbid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-bksched-concurrency-tmpl"
	gs := "e2e-bksched-concurrency-target"
	schedName := "e2e-bksched-concurrency"

	ensureResticRepo(t)

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	sched := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "BackupSchedule",
		"metadata":   map[string]any{"name": schedName, "namespace": ns},
		"spec": map[string]any{
			"serverRef":         map[string]any{"name": gs},
			"schedule":          "* * * * *",
			"repoRef":           map[string]any{"name": "e2e-restic-creds", "key": "repo"},
			"strategy":          "restic-snapshot",
			"quiesce":           false,
			"concurrencyPolicy": "Forbid",
		},
	}}
	if _, err := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
		Create(ctx, sched, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create concurrency backupschedule: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
			Delete(context.Background(), schedName, metav1.DeleteOptions{})
	})

	// Wait for the first owned Backup to land.
	envInstance.Eventually(t, 2*time.Minute+30*time.Second, func() (bool, string) {
		bks, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "list backups: " + err.Error()
		}
		for _, item := range bks.Items {
			for _, owner := range item.GetOwnerReferences() {
				if owner.Kind == "BackupSchedule" && owner.Name == schedName {
					return true, ""
				}
			}
		}
		return false, "no scheduled Backup yet"
	})

	// Sample over the next 90s; if a second Backup CR appears while the
	// first is still in a non-terminal phase (Pending/Running), Forbid
	// has been violated.
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		bks, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			List(ctx, metav1.ListOptions{})
		if err != nil {
			t.Fatalf("list backups: %v", err)
		}
		nonTerminal := 0
		var names []string
		for _, item := range bks.Items {
			isOwned := false
			for _, owner := range item.GetOwnerReferences() {
				if owner.Kind == "BackupSchedule" && owner.Name == schedName {
					isOwned = true
					break
				}
			}
			if !isOwned {
				continue
			}
			phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
			if phase != "Succeeded" && phase != "Failed" {
				nonTerminal++
				names = append(names, item.GetName()+"="+phase)
			}
		}
		if nonTerminal > 1 {
			t.Fatalf("Forbid violated: %d non-terminal Backups in flight: %v", nonTerminal, names)
		}
		time.Sleep(3 * time.Second)
	}
}
