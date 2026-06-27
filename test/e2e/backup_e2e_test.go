//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestBackup_OperatorMaterializesJob proves the Backup reconciler
// translates a Backup CR into a Kubernetes Job in the same namespace,
// wired up with the restic env vars from the user's RepoRef Secret
// and a /data mount onto the GameServer's PVC.
//
// The test does NOT wait for the backup to complete — that depends on
// the restic image pulling, restic init succeeding against the
// in-cluster REST server, and a valid backup transmission. All of
// that is exercised by the operator's envtest suite. Here we only
// assert the contract the API layer cares about: "applying a Backup
// CR makes a Job appear".
func TestBackup_OperatorMaterializesJob(t *testing.T) {
	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-backup-busybox-tmpl"
	gs := "e2e-backup-target"

	// 1. Restic infrastructure (restic-server + game-namespace
	// NetworkPolicy widening + the credentials Secret). All resources
	// land in their respective namespaces and survive across tests; we
	// only delete them on cleanup of the *last* test run, which is
	// effectively never — the kind cluster gets torn down between CI
	// runs anyway.
	envInstance.ApplyYAML(t, "restic-server.yaml")
	envInstance.ApplyYAML(t, "backup-restic-secret.yaml")

	// 2. Target GameServer. The operator's StatefulSet creates the
	// `<gs>-data` PVC, which kind's default storage class binds as soon
	// as a pod claims it.
	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	// 3. Apply Backup CR.
	bkName := "e2e-backup"
	bk := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "Backup",
		"metadata":   map[string]any{"name": bkName, "namespace": ns},
		"spec": map[string]any{
			"serverRef": map[string]any{"name": gs},
			"repoRef":   map[string]any{"name": "e2e-restic-creds", "key": "repo"},
			"strategy":  "restic-snapshot",
			"quiesce":   false, // skip the agent quiesce dance — busybox has no game protocol
		},
	}}
	if _, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).
		Create(ctx, bk, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create backup: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupGVR).Namespace(ns).
			Delete(context.Background(), bkName, metav1.DeleteOptions{})
	})

	// 4. Operator must create a Job in the same namespace. Job naming
	// is operator-internal (typically derived from the Backup name); we
	// don't assume an exact name — instead we list Jobs in the namespace
	// and look for one with an ownerReference back to our Backup.
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		jobs, err := envInstance.K8s.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, "list jobs: " + err.Error()
		}
		for _, j := range jobs.Items {
			for _, owner := range j.OwnerReferences {
				if owner.Kind == "Backup" && owner.Name == bkName {
					// Sanity-check that the Job pod template references
					// the right Secret (so a future refactor that
					// silently drops RESTIC_PASSWORD trips this test).
					for _, c := range j.Spec.Template.Spec.Containers {
						for _, env := range c.Env {
							if env.Name == "RESTIC_PASSWORD" && env.ValueFrom != nil &&
								env.ValueFrom.SecretKeyRef != nil &&
								env.ValueFrom.SecretKeyRef.Name == "e2e-restic-creds" {
								return true, ""
							}
						}
					}
					return false, "job " + j.Name + " owned by Backup but RESTIC_PASSWORD env not wired"
				}
			}
		}
		return false, "no Job owned by Backup " + bkName + " yet"
	})
}

// TestBackupSchedule_CreatesBackupCR proves the BackupSchedule
// controller emits Backup CRs over time. We use a one-minute cron
// rather than `@every 10s` because BackupScheduleSpec.Schedule has a
// MinLength=9 validator (5-field cron) — `@every 10s` is rejected by
// the API server before the controller ever sees it.
func TestBackupSchedule_CreatesBackupCR(t *testing.T) {
	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-bksched-busybox-tmpl"
	gs := "e2e-bksched-target"

	envInstance.ApplyYAML(t, "restic-server.yaml")
	envInstance.ApplyYAML(t, "backup-restic-secret.yaml")

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	schedName := "e2e-bksched"
	sched := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "BackupSchedule",
		"metadata":   map[string]any{"name": schedName, "namespace": ns},
		"spec": map[string]any{
			"serverRef": map[string]any{"name": gs},
			"schedule":  "* * * * *", // every minute — minimum cadence the validator accepts
			"repoRef":   map[string]any{"name": "e2e-restic-creds", "key": "repo"},
			"strategy":  "restic-snapshot",
			"quiesce":   false,
		},
	}}
	if _, err := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
		Create(ctx, sched, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create backupschedule: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
			Delete(context.Background(), schedName, metav1.DeleteOptions{})
	})

	// Wait up to 2 minutes for a Backup CR owned by the schedule to
	// appear. The cron grants the scheduler a window of up to 60s; on
	// top of that, controller-runtime's reconcile cadence adds a few
	// seconds of slack.
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
		// Surface the schedule's status to make a hung schedule easier
		// to debug from CI logs.
		got, gerr := envInstance.Dyn.Resource(backupScheduleGVR).Namespace(ns).
			Get(ctx, schedName, metav1.GetOptions{})
		if gerr == nil {
			s, _, _ := unstructured.NestedString(got.Object, "status", "lastScheduleTime")
			next, _, _ := unstructured.NestedString(got.Object, "status", "nextScheduleTime")
			return false, "no scheduled Backup yet (lastScheduleTime=" + s +
				", nextScheduleTime=" + next + ")"
		}
		return false, "no scheduled Backup yet"
	})

	// Sanity-check that the Backup we found references the right
	// server and repo — guards against a regression where the
	// scheduler drops fields on the way through.
	bks, err := envInstance.Dyn.Resource(backupGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list backups (sanity): %v", err)
	}
	var found *unstructured.Unstructured
	for i := range bks.Items {
		for _, owner := range bks.Items[i].GetOwnerReferences() {
			if owner.Kind == "BackupSchedule" && owner.Name == schedName {
				found = &bks.Items[i]
				break
			}
		}
	}
	if found == nil {
		t.Fatal("scheduled Backup vanished between Eventually and sanity check")
	}
	srvRef, _, _ := unstructured.NestedString(found.Object, "spec", "serverRef", "name")
	if srvRef != gs {
		t.Errorf("scheduled Backup serverRef=%q, want %q", srvRef, gs)
	}
	repoName, _, _ := unstructured.NestedString(found.Object, "spec", "repoRef", "name")
	if !strings.Contains(repoName, "restic") {
		t.Errorf("scheduled Backup repoRef=%q, expected to contain 'restic'", repoName)
	}
}
