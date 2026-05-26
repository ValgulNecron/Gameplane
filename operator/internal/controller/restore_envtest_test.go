//go:build envtest

package controller

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// TestRestore_FailsWhenBackupMissing — Restore referencing a
// non-existent Backup transitions to Failed with a clear message.
func TestRestore_FailsWhenBackupMissing(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", "minecraft")); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildRestore(ns, "rs-1", "missing-backup", "smp")); err != nil {
		t.Fatalf("create restore: %v", err)
	}

	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		if r.Status.Phase != kestrelv1alpha1.RestorePhaseFailed {
			return false, describeRestoreStatus(r)
		}
		if r.Status.Message == "" {
			return false, "no failure message set"
		}
		return true, ""
	})
}

// TestRestore_StaysPendingUntilBackupSucceeded — While the source
// Backup is not yet in Succeeded, the Restore should sit in Pending
// and not advance to Suspending.
func TestRestore_StaysPendingUntilBackupSucceeded(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	if err := k8sClient.Create(context.Background(), buildResticRepoSecret(ns, "repo")); err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", "minecraft")); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	// Backup exists but Status is still empty (Phase=="").
	if err := k8sClient.Create(context.Background(), buildBackup(ns, "smp-001", "smp", "repo")); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildRestore(ns, "rs-1", "smp-001", "smp")); err != nil {
		t.Fatalf("create restore: %v", err)
	}

	// First wait until Restore observes the Pending phase. (It starts
	// with empty Status; the controller writes Pending.)
	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		return r.Status.Phase == kestrelv1alpha1.RestorePhasePending, describeRestoreStatus(r)
	})

	// Then assert it stays in Pending while the Backup isn't ready.
	consistently(t, 2*time.Second, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		if r.Status.Phase != kestrelv1alpha1.RestorePhasePending {
			return false, "advanced past Pending: " + describeRestoreStatus(r)
		}
		if r.Status.SnapshotID != "" {
			return false, "snapshotID pinned prematurely"
		}
		return true, ""
	})
}

// TestRestore_PinsSnapshotIDOnTransition — When the source Backup
// reaches Succeeded with a snapshotID, the Restore copies it into its
// own Status (so retention can't pull it out from under us mid-run).
func TestRestore_PinsSnapshotIDOnTransition(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	if err := k8sClient.Create(context.Background(), buildResticRepoSecret(ns, "repo")); err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", "minecraft")); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildBackup(ns, "smp-001", "smp", "repo")); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildRestore(ns, "rs-1", "smp-001", "smp")); err != nil {
		t.Fatalf("create restore: %v", err)
	}

	// Drive the Backup to Succeeded with a known snapshot ID.
	markBackupSucceeded(t, ns, "smp-001", "abc123def", "12Mi")

	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		if r.Status.SnapshotID != "abc123def" {
			return false, describeRestoreStatus(r)
		}
		// After pinning, the next reconcile should advance to Suspending
		// (or further — Suspending → Running is a single step away, so
		// we accept Suspending or Running here).
		switch r.Status.Phase {
		case kestrelv1alpha1.RestorePhaseSuspending, kestrelv1alpha1.RestorePhaseRunning:
			return true, ""
		default:
			return false, "phase = " + string(r.Status.Phase)
		}
	})
}

// TestRestore_SuspendsTargetGameServer — Once snapshotID is pinned,
// the controller patches the target GameServer's Spec.Suspend=true.
func TestRestore_SuspendsTargetGameServer(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	if err := k8sClient.Create(context.Background(), buildResticRepoSecret(ns, "repo")); err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", "minecraft")); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildBackup(ns, "smp-001", "smp", "repo")); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildRestore(ns, "rs-1", "smp-001", "smp")); err != nil {
		t.Fatalf("create restore: %v", err)
	}

	markBackupSucceeded(t, ns, "smp-001", "snap-1", "")

	eventually(t, func() (bool, string) {
		gs := getGameServer(t, ns, "smp")
		if !gs.Spec.Suspend {
			return false, "GameServer.Spec.Suspend still false"
		}
		return true, ""
	})
}

// TestRestore_AdvancesToRunningOnceSuspended — Restore waits in
// Suspending until GameServer.Status.Phase reports Suspended, then
// advances to Running and creates the restore Job.
func TestRestore_AdvancesToRunningOnceSuspended(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	if err := k8sClient.Create(context.Background(), buildResticRepoSecret(ns, "repo")); err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", "minecraft")); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildBackup(ns, "smp-001", "smp", "repo")); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildRestore(ns, "rs-1", "smp-001", "smp")); err != nil {
		t.Fatalf("create restore: %v", err)
	}

	markBackupSucceeded(t, ns, "smp-001", "snap-1", "")

	// Wait until Spec.Suspend has been flipped on.
	eventually(t, func() (bool, string) {
		return getGameServer(t, ns, "smp").Spec.Suspend, "spec.suspend not yet true"
	})

	// While GameServer.Status.Phase is anything other than Suspended/Stopped
	// the Restore must NOT create a Job.
	consistently(t, 1500*time.Millisecond, func() (bool, string) {
		if _, ok := getJob(t, ns, "restore-rs-1"); ok {
			return false, "restore Job created before GameServer reached Suspended"
		}
		return true, ""
	})

	// Bump GameServer status into Suspended, expect the Job to materialize.
	markGameServerPhase(t, ns, "smp", kestrelv1alpha1.GameServerPhaseSuspended)

	eventually(t, func() (bool, string) {
		j, ok := getJob(t, ns, "restore-rs-1")
		if !ok {
			return false, "restore Job not yet created"
		}
		// Sanity-check the job runs `restic restore <snapshot>`.
		if len(j.Spec.Template.Spec.Containers) == 0 {
			return false, "job has no containers"
		}
		args := j.Spec.Template.Spec.Containers[0].Args
		if len(args) < 2 || args[0] != "restore" || args[1] != "snap-1" {
			return false, "unexpected restic args: " + sprintArgs(args)
		}
		// --target / is load-bearing: the backup snapshots /data
		// absolutely, so restoring to /data would double-prefix to
		// /data/data/<file>. See restore_controller.go.
		var sawTargetRoot bool
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "--target" && args[i+1] == "/" {
				sawTargetRoot = true
				break
			}
		}
		if !sawTargetRoot {
			return false, "expected --target / in restic args, got: " + sprintArgs(args)
		}
		return true, ""
	})

	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		return r.Status.Phase == kestrelv1alpha1.RestorePhaseRunning, describeRestoreStatus(r)
	})
}

// TestRestore_ResumesGameServerOnSuccess — Job Succeeded ⇒ Restore
// transitions to Succeeded and target GameServer is un-suspended.
func TestRestore_ResumesGameServerOnSuccess(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	bootstrapRestoreToRunning(t, ns)

	patchJobStatus(t, ns, "restore-rs-1", func(s *batchv1.JobStatus) {
		now := metav1.Now()
		s.Succeeded = 1
		s.StartTime = &now
		s.CompletionTime = &now
	})

	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		if r.Status.Phase != kestrelv1alpha1.RestorePhaseSucceeded {
			return false, describeRestoreStatus(r)
		}
		gs := getGameServer(t, ns, "smp")
		if gs.Spec.Suspend {
			return false, "GameServer was not resumed (spec.suspend still true)"
		}
		if r.Status.CompletionTime == nil {
			return false, "no completionTime"
		}
		return true, ""
	})
}

// TestRestore_LeavesServerSuspendedOnFailure — Job Failed ⇒ Restore
// goes Failed and the GameServer stays suspended (operator
// intentionally does NOT auto-resume; humans should investigate).
func TestRestore_LeavesServerSuspendedOnFailure(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	bootstrapRestoreToRunning(t, ns)

	patchJobStatus(t, ns, "restore-rs-1", func(s *batchv1.JobStatus) {
		s.Failed = 1
	})

	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		if r.Status.Phase != kestrelv1alpha1.RestorePhaseFailed {
			return false, describeRestoreStatus(r)
		}
		if r.Status.Message == "" {
			return false, "no failure message"
		}
		gs := getGameServer(t, ns, "smp")
		if !gs.Spec.Suspend {
			return false, "GameServer was incorrectly resumed after restore failure"
		}
		return true, ""
	})
}

// TestRestore_FailsWhenServerMissing — Backup OK but referenced
// GameServer doesn't exist ⇒ Restore goes Failed.
func TestRestore_FailsWhenServerMissing(t *testing.T) {
	ns := newNamespace(t)
	startMgr(t, ns, withRestoreReconciler())

	if err := k8sClient.Create(context.Background(), buildResticRepoSecret(ns, "repo")); err != nil {
		t.Fatalf("create secret: %v", err)
	}
	// Note: no GameServer created.
	if err := k8sClient.Create(context.Background(), buildBackup(ns, "smp-001", "smp", "repo")); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildRestore(ns, "rs-1", "smp-001", "smp")); err != nil {
		t.Fatalf("create restore: %v", err)
	}
	markBackupSucceeded(t, ns, "smp-001", "snap-1", "")

	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		if r.Status.Phase != kestrelv1alpha1.RestorePhaseFailed {
			return false, describeRestoreStatus(r)
		}
		return true, ""
	})
}

// bootstrapRestoreToRunning sets up a namespace such that a Restore
// named "rs-1" against GameServer "smp" / Backup "smp-001" has
// reached Phase=Running and the restore Job named "restore-rs-1"
// exists in zero status.
func bootstrapRestoreToRunning(t *testing.T, ns string) {
	t.Helper()

	if err := k8sClient.Create(context.Background(), buildResticRepoSecret(ns, "repo")); err != nil {
		t.Fatalf("create secret: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildGameServer(ns, "smp", "minecraft")); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildBackup(ns, "smp-001", "smp", "repo")); err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildRestore(ns, "rs-1", "smp-001", "smp")); err != nil {
		t.Fatalf("create restore: %v", err)
	}

	markBackupSucceeded(t, ns, "smp-001", "snap-1", "")

	eventually(t, func() (bool, string) {
		return getGameServer(t, ns, "smp").Spec.Suspend, "waiting for spec.suspend"
	})
	markGameServerPhase(t, ns, "smp", kestrelv1alpha1.GameServerPhaseSuspended)

	eventually(t, func() (bool, string) {
		_, ok := getJob(t, ns, "restore-rs-1")
		return ok, "waiting for restore Job"
	})
	eventually(t, func() (bool, string) {
		r := getRestore(t, ns, "rs-1")
		return r.Status.Phase == kestrelv1alpha1.RestorePhaseRunning, describeRestoreStatus(r)
	})
}

func sprintArgs(a []string) string {
	out := "["
	for i, s := range a {
		if i > 0 {
			out += " "
		}
		out += s
	}
	return out + "]"
}
