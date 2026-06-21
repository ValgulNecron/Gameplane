package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// annoRestoreCreatedBy marks a GameServer that a volume-snapshot Restore
// provisioned, so a re-running restore re-finds its own server instead of
// mistaking it for a pre-existing name collision. The new server is
// intentionally NOT owner-referenced by the Restore (it must outlive it),
// so this annotation is how the controller tracks it.
const annoRestoreCreatedBy = "restore.kestrel.gg/created-by"

// reconcileVolumeSnapshotRestore restores a volume-snapshot Backup by
// standing up a brand-new GameServer whose data PVC is seeded from the CSI
// snapshot (GameServer.spec.storage.dataSource → reconcilePVC). The original
// server is never touched. The new server's spec is copied from the original
// (which must still exist to source the spec).
func (r *RestoreReconciler) reconcileVolumeSnapshotRestore(
	ctx context.Context, rs *kestrelv1alpha1.Restore, src *kestrelv1alpha1.Backup,
) (ctrl.Result, error) {
	// The snapshot must have actually bound or the new PVC can't be seeded.
	if src.Status.VolumeSnapshotContentName == "" {
		return r.fail(ctx, rs, fmt.Sprintf(
			"source backup %q has no bound VolumeSnapshot", rs.Spec.BackupRef.Name))
	}

	newKey := types.NamespacedName{Name: rs.Spec.ServerRef.Name, Namespace: rs.Namespace}
	var newGS kestrelv1alpha1.GameServer
	err := r.Get(ctx, newKey, &newGS)
	switch {
	case err == nil:
		// A server with the target name exists. If we created it, wait for
		// it to come up; otherwise it's a collision — volume-snapshot
		// restores must create a fresh server, never overwrite one.
		if newGS.Annotations[annoRestoreCreatedBy] != rs.Name {
			return r.fail(ctx, rs, fmt.Sprintf(
				"target server %q already exists; volume-snapshot restores create a new server",
				rs.Spec.ServerRef.Name))
		}
		return r.awaitRestoredServer(ctx, rs, &newGS)
	case !apierrors.IsNotFound(err):
		return ctrl.Result{}, err
	}

	// Target doesn't exist yet — build it from the original server's spec.
	var orig kestrelv1alpha1.GameServer
	if err := r.Get(ctx, types.NamespacedName{Name: src.Spec.ServerRef.Name, Namespace: rs.Namespace}, &orig); err != nil {
		if apierrors.IsNotFound(err) {
			return r.fail(ctx, rs, fmt.Sprintf(
				"original server %q not found; cannot derive spec for a volume-snapshot restore",
				src.Spec.ServerRef.Name))
		}
		return ctrl.Result{}, err
	}

	created := &kestrelv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rs.Spec.ServerRef.Name,
			Namespace:   rs.Namespace,
			Annotations: map[string]string{annoRestoreCreatedBy: rs.Name},
		},
	}
	orig.Spec.DeepCopyInto(&created.Spec)
	created.Spec.Suspend = false
	if created.Spec.Storage == nil {
		created.Spec.Storage = &kestrelv1alpha1.GameStorageSpec{}
	}
	created.Spec.Storage.DataSource = &kestrelv1alpha1.GameDataSource{
		Kind: "VolumeSnapshot",
		Name: rs.Status.SnapshotID,
	}
	if err := r.Create(ctx, created); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Lost a create race; the next pass finds it via the annotation.
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	if rs.Status.StartTime == nil {
		now := metav1.Now()
		rs.Status.StartTime = &now
		if err := r.Status().Update(ctx, rs); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// awaitRestoredServer drives the Restore to a terminal phase based on the
// newly-provisioned server: Running → Succeeded, Failed → Failed, otherwise
// keep polling while it starts up.
func (r *RestoreReconciler) awaitRestoredServer(
	ctx context.Context, rs *kestrelv1alpha1.Restore, gs *kestrelv1alpha1.GameServer,
) (ctrl.Result, error) {
	switch gs.Status.Phase {
	case kestrelv1alpha1.GameServerPhaseRunning:
		now := metav1.Now()
		rs.Status.Phase = kestrelv1alpha1.RestorePhaseSucceeded
		if rs.Status.CompletionTime == nil {
			rs.Status.CompletionTime = &now
		}
		rs.Status.Conditions = upsertCondition(rs.Status.Conditions, metav1.Condition{
			Type:               "Completed",
			Status:             metav1.ConditionTrue,
			Reason:             "Succeeded",
			ObservedGeneration: rs.Generation,
		})
		if err := r.Status().Update(ctx, rs); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	case kestrelv1alpha1.GameServerPhaseFailed:
		return r.fail(ctx, rs, fmt.Sprintf("restored server %q failed to start", gs.Name))
	default:
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
}
