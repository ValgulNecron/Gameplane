package controller

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// RestoreReconciler drives a Restore through suspend → restic-restore Job
// → resume. The target GameServer is paused for the duration of the Job
// to serialize I/O against its data PVC.
type RestoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// ResticImage is the image for the restic restore Job. Set from an
	// operator flag so air-gapped installs can point it at a private
	// registry mirror. Empty falls back to DefaultResticImage.
	ResticImage string
}

// +kubebuilder:rbac:groups=gameplane.local,resources=restores,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=restores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=gameservers,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=backups,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *RestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var rs gameplanev1alpha1.Restore
	if err := r.Get(ctx, req.NamespacedName, &rs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if rs.Status.Phase == gameplanev1alpha1.RestorePhaseSucceeded ||
		rs.Status.Phase == gameplanev1alpha1.RestorePhaseFailed {
		return ctrl.Result{}, nil
	}

	// Pin the snapshotID at first observation so retention can't pull
	// the rug out from under us mid-restore.
	if rs.Status.SnapshotID == "" {
		var src gameplanev1alpha1.Backup
		if err := r.Get(ctx, types.NamespacedName{Name: rs.Spec.BackupRef.Name, Namespace: rs.Namespace}, &src); err != nil {
			if apierrors.IsNotFound(err) {
				return r.fail(ctx, &rs, fmt.Sprintf("source backup %q not found", rs.Spec.BackupRef.Name))
			}
			return ctrl.Result{}, err
		}
		if src.Status.Phase == gameplanev1alpha1.BackupPhaseFailed {
			// Terminal: a Failed backup will never grow a snapshotID, so
			// waiting would leave the Restore Pending forever.
			return r.fail(ctx, &rs, fmt.Sprintf(
				"source backup %q failed and has no usable snapshot: %s",
				rs.Spec.BackupRef.Name, src.Status.Message))
		}
		if src.Status.Phase != gameplanev1alpha1.BackupPhaseSucceeded || src.Status.SnapshotID == "" {
			// Source backup is not ready yet. Stay Pending.
			if rs.Status.Phase != gameplanev1alpha1.RestorePhasePending {
				rs.Status.Phase = gameplanev1alpha1.RestorePhasePending
				rs.Status.ObservedGeneration = rs.Generation
				if err := r.Status().Update(ctx, &rs); err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}
		rs.Status.SnapshotID = src.Status.SnapshotID
		if src.Spec.Strategy == "volume-snapshot" {
			// Volume-snapshot restores stand up a brand-new server rather
			// than suspending and overwriting an existing one, so they skip
			// the Suspending phase entirely.
			rs.Status.Phase = gameplanev1alpha1.RestorePhaseRunning
		} else {
			rs.Status.Phase = gameplanev1alpha1.RestorePhaseSuspending
		}
		rs.Status.ObservedGeneration = rs.Generation
		if err := r.Status().Update(ctx, &rs); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Re-resolve the source Backup so we know its strategy on every pass
	// (the pin block above only runs once). Volume-snapshot restores never
	// touch an existing server — they provision a new one seeded from the
	// CSI snapshot — so they branch off here, before the suspend/Job flow.
	var src gameplanev1alpha1.Backup
	if err := r.Get(ctx, types.NamespacedName{Name: rs.Spec.BackupRef.Name, Namespace: rs.Namespace}, &src); err != nil {
		if apierrors.IsNotFound(err) {
			return r.fail(ctx, &rs, fmt.Sprintf("source backup %q disappeared", rs.Spec.BackupRef.Name))
		}
		return ctrl.Result{}, err
	}
	if src.Spec.Strategy == "volume-snapshot" {
		return r.reconcileVolumeSnapshotRestore(ctx, &rs, &src)
	}

	var gs gameplanev1alpha1.GameServer
	gsKey := types.NamespacedName{Name: rs.Spec.ServerRef.Name, Namespace: rs.Namespace}
	if err := r.Get(ctx, gsKey, &gs); err != nil {
		if apierrors.IsNotFound(err) {
			return r.fail(ctx, &rs, fmt.Sprintf("target server %q not found", rs.Spec.ServerRef.Name))
		}
		return ctrl.Result{}, err
	}

	if rs.Status.Phase == gameplanev1alpha1.RestorePhaseSuspending {
		if !gs.Spec.Suspend {
			gs.Spec.Suspend = true
			if err := r.Update(ctx, &gs); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		if gs.Status.Phase != gameplanev1alpha1.GameServerPhaseSuspended &&
			gs.Status.Phase != gameplanev1alpha1.GameServerPhaseStopped {
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		rs.Status.Phase = gameplanev1alpha1.RestorePhaseRunning
		if err := r.Status().Update(ctx, &rs); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// src (the source Backup, with RepoRef for the Job env) was resolved
	// above before the volume-snapshot branch.
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "restore-" + rs.Name, Namespace: rs.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, job, func() error {
		if job.CreationTimestamp.IsZero() {
			job.Spec.Template.Spec = r.buildRestorePodSpec(&rs, &src)
			job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
		}
		return controllerutil.SetControllerReference(&rs, job, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	switch {
	case job.Status.Succeeded > 0:
		// Resume the GameServer.
		if gs.Spec.Suspend {
			gs.Spec.Suspend = false
			if err := r.Update(ctx, &gs); err != nil {
				return ctrl.Result{}, err
			}
		}
		now := metav1.Now()
		rs.Status.Phase = gameplanev1alpha1.RestorePhaseSucceeded
		if rs.Status.StartTime == nil && job.Status.StartTime != nil {
			rs.Status.StartTime = job.Status.StartTime
		}
		if rs.Status.CompletionTime == nil {
			if job.Status.CompletionTime != nil {
				rs.Status.CompletionTime = job.Status.CompletionTime
			} else {
				rs.Status.CompletionTime = &now
			}
		}
		rs.Status.Conditions = upsertCondition(rs.Status.Conditions, metav1.Condition{
			Type:               "Completed",
			Status:             metav1.ConditionTrue,
			Reason:             "Succeeded",
			ObservedGeneration: rs.Generation,
		})
		if err := r.Status().Update(ctx, &rs); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil

	case job.Status.Failed > 0:
		// Leave the server suspended; surface the failure.
		return r.fail(ctx, &rs, "restore job reported Failed")

	default:
		if rs.Status.StartTime == nil && job.Status.StartTime != nil {
			rs.Status.StartTime = job.Status.StartTime
			if err := r.Status().Update(ctx, &rs); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
}

func (r *RestoreReconciler) fail(ctx context.Context, rs *gameplanev1alpha1.Restore, msg string) (ctrl.Result, error) {
	now := metav1.Now()
	rs.Status.Phase = gameplanev1alpha1.RestorePhaseFailed
	rs.Status.Message = msg
	if rs.Status.CompletionTime == nil {
		rs.Status.CompletionTime = &now
	}
	rs.Status.Conditions = upsertCondition(rs.Status.Conditions, metav1.Condition{
		Type:               "Completed",
		Status:             metav1.ConditionFalse,
		Reason:             "Failed",
		Message:            msg,
		ObservedGeneration: rs.Generation,
	})
	if err := r.Status().Update(ctx, rs); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *RestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gameplanev1alpha1.Restore{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// buildRestorePodSpec mirrors backup_controller.buildBackupPodSpec but
// runs `restic restore <id>` and mounts the data PVC read-write.
func (r *RestoreReconciler) buildRestorePodSpec(
	rs *gameplanev1alpha1.Restore, src *gameplanev1alpha1.Backup,
) corev1.PodSpec {
	nonRoot := true
	roRootFS := true
	noPrivEsc := false
	uid := int64(65532)
	return corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:   &nonRoot,
			RunAsUser:      &uid,
			RunAsGroup:     &uid,
			FSGroup:        &uid,
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
		Containers: []corev1.Container{{
			Name:  "restic",
			Image: resticImageOrDefault(r.ResticImage),
			// --target / restores each snapshot entry at its original
			// absolute path. The companion backup runs `restic backup
			// /data`, so the snapshot tree is rooted at /data/...; with
			// --target /data the path doubles to /data/data/marker.txt.
			Args: []string{"restore", rs.Status.SnapshotID, "--target", "/"},
			Env: []corev1.EnvVar{
				{Name: "RESTIC_REPOSITORY", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: src.Spec.RepoRef.Name},
						Key:                  "repo",
					},
				}},
				{Name: "RESTIC_PASSWORD", ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: src.Spec.RepoRef.Name},
						Key:                  "password",
					},
				}},
				{Name: "XDG_CACHE_HOME", Value: "/tmp/restic-cache"},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: "/data"},
				{Name: "cache", MountPath: "/tmp"},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             &nonRoot,
				RunAsUser:                &uid,
				ReadOnlyRootFilesystem:   &roRootFS,
				AllowPrivilegeEscalation: &noPrivEsc,
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
		}},
		Volumes: []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: rs.Spec.ServerRef.Name + "-data",
					},
				},
			},
			{
				Name:         "cache",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
		},
	}
}
