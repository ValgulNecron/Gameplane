package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// TestReconcile_RetriesUnquiesceOnTerminalBackup covers F-044: a Backup that
// has already gone terminal (phase persisted, snapshot id present) must keep
// retrying a previously-failed unquiesce instead of the top-of-Reconcile
// terminal guard abandoning it forever.
func TestReconcile_RetriesUnquiesceOnTerminalBackup(t *testing.T) {
	b := quiescedBackup()
	b.Status.Phase = gameplanev1alpha1.BackupPhaseSucceeded
	b.Status.SnapshotID = "snap-x"

	q := &scrapeQuiescer{unquiesceErr: errors.New("agent unreachable")}
	r := newScrapeReconciler(t, b, nil, time.Hour, q)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: b.Namespace, Name: b.Name}}

	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatalf("expected a requeue to retry the failed unquiesce, got %+v", res)
	}
	if q.unquiesced != 0 {
		t.Fatalf("unquiesce recorded success while the agent call was failing")
	}

	// The agent recovers. A fresh Reconcile of the same (still terminal, per
	// its persisted status.phase) Backup must retry rather than short-circuit.
	q.unquiesceErr = nil
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if q.unquiesced != 1 {
		t.Fatalf("Unquiesce called %d times after recovery, want 1", q.unquiesced)
	}

	var got gameplanev1alpha1.Backup
	if err := r.Get(context.Background(), req.NamespacedName, &got); err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if _, ok := got.Annotations[annoUnquiescedAt]; !ok {
		t.Error("unquiesced-at annotation missing after a successful retry")
	}
	if got.Status.Phase != gameplanev1alpha1.BackupPhaseSucceeded {
		t.Errorf("phase = %q, want Succeeded (unchanged)", got.Status.Phase)
	}
}

// TestReconcile_AddsFinalizerForQuiescedBackup covers the create-time half of
// F-048: a Backup with spec.quiesce=true must carry BackupFinalizer before it
// can be deleted, so the operator gets a chance to release the world.
func TestReconcile_AddsFinalizerForQuiescedBackup(t *testing.T) {
	b := scrapeBackup()
	b.Spec.Quiesce = true
	r := newScrapeReconciler(t, b, nil, time.Hour, nil)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: b.Namespace, Name: b.Name}}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got gameplanev1alpha1.Backup
	if err := r.Get(context.Background(), req.NamespacedName, &got); err != nil {
		t.Fatalf("get backup: %v", err)
	}
	found := false
	for _, f := range got.Finalizers {
		if f == gameplanev1alpha1.BackupFinalizer {
			found = true
		}
	}
	if !found {
		t.Errorf("finalizers = %v, want %q present", got.Finalizers, gameplanev1alpha1.BackupFinalizer)
	}
}

// TestReconcile_DeleteReleasesQuiescedWorldBeforeFinalizerClears covers the
// delete-time half of F-048: deleting a Backup that quiesced the game must
// send the unquiesce before the finalizer (and therefore the object) clears,
// and must keep the finalizer (and retry) while the agent is unreachable.
func TestReconcile_DeleteReleasesQuiescedWorldBeforeFinalizerClears(t *testing.T) {
	b := quiescedBackup()
	b.Finalizers = []string{gameplanev1alpha1.BackupFinalizer}
	now := metav1.Now()
	b.DeletionTimestamp = &now

	q := &scrapeQuiescer{unquiesceErr: errors.New("agent unreachable")}
	r := newScrapeReconciler(t, b, nil, time.Hour, q)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: b.Namespace, Name: b.Name}}

	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatalf("expected a requeue while the unquiesce is failing, got %+v", res)
	}
	var mid gameplanev1alpha1.Backup
	if err := r.Get(context.Background(), req.NamespacedName, &mid); err != nil {
		t.Fatalf("get backup: %v", err)
	}
	stillHasFinalizer := false
	for _, f := range mid.Finalizers {
		if f == gameplanev1alpha1.BackupFinalizer {
			stillHasFinalizer = true
		}
	}
	if !stillHasFinalizer {
		t.Fatal("finalizer removed before the unquiesce succeeded")
	}

	q.unquiesceErr = nil
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if q.unquiesced != 1 {
		t.Errorf("Unquiesce called %d times, want 1", q.unquiesced)
	}
	var got gameplanev1alpha1.Backup
	err = r.Get(context.Background(), req.NamespacedName, &got)
	if err == nil {
		for _, f := range got.Finalizers {
			if f == gameplanev1alpha1.BackupFinalizer {
				t.Error("finalizer still present after a successful unquiesce")
			}
		}
	}
	// A fake client without a delete-on-zero-finalizers reaper keeps the
	// object around once its finalizer list is empty; either the object is
	// gone (real apiserver behavior) or its finalizer list is empty is
	// acceptable here.
}

// TestReconcile_DeleteWithoutQuiesceClearsFinalizerImmediately covers the
// no-op path: a Backup that never quiesced (or wasn't asked to) must not be
// blocked on an unquiesce it never owed.
func TestReconcile_DeleteWithoutQuiesceClearsFinalizerImmediately(t *testing.T) {
	b := scrapeBackup()
	b.Finalizers = []string{gameplanev1alpha1.BackupFinalizer}
	now := metav1.Now()
	b.DeletionTimestamp = &now

	q := &scrapeQuiescer{}
	r := newScrapeReconciler(t, b, nil, time.Hour, q)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: b.Namespace, Name: b.Name}}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if q.unquiesced != 0 {
		t.Error("Unquiesce called for a Backup that never quiesced")
	}
}
