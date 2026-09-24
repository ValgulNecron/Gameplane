package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

const (
	// WipeRequestedAnnotation carries the token of a requested data wipe
	// (set by the API). WipeCompletedAnnotation echoes it back once the
	// wipe Job has succeeded, so the same request never re-runs.
	WipeRequestedAnnotation = "gameplane.local/wipe-data-requested"
	WipeCompletedAnnotation = "gameplane.local/wipe-data-completed"
	wipeTokenLabel          = "gameplane.local/wipe-token"

	// wipeScript empties the directory given as $1 and verifies it is empty
	// afterwards; see the comment at its use in createWipeJob for why the
	// verification step is required.
	wipeScript = `find "$1" -mindepth 1 -delete
left=$(ls -A "$1") || exit 1
if [ -n "$left" ]; then
  echo "data wipe incomplete; entries remain under $1:" >&2
  printf '%s\n' "$left" >&2
  exit 1
fi`
)

// reconcileWipe runs a one-shot Job that empties the GameServer's data PVC
// when a wipe has been requested and the server is suspended (so the game
// pod isn't holding the ReadWriteOnce volume). It's idempotent: the request
// is acked once the Job succeeds and the same token never re-runs.
func (r *GameServerReconciler) reconcileWipe(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
) error {
	req := gs.Annotations[WipeRequestedAnnotation]
	done := gs.Annotations[WipeCompletedAnnotation]
	jobName := gs.Name + "-wipe"

	if req == "" || req == done {
		// Nothing pending — clean up any finished wipe Job left behind.
		return r.deleteWipeJob(ctx, gs.Namespace, jobName)
	}

	// Only wipe while suspended; otherwise the game pod still mounts the
	// volume. The API sets suspend=true when requesting a wipe.
	if !gs.Spec.Suspend {
		log.FromContext(ctx).Info("data wipe requested but server not suspended; waiting", "server", gs.Name)
		return nil
	}

	var job batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: gs.Namespace}, &job)
	switch {
	case apierrors.IsNotFound(err):
		return r.createWipeJob(ctx, gs, tmpl, jobName, req)
	case err != nil:
		return err
	}

	// A leftover Job from a previous request — replace it.
	if job.Labels[wipeTokenLabel] != req {
		return r.deleteWipeJob(ctx, gs.Namespace, jobName)
	}
	// The current request finished successfully — ack and clean up.
	if job.Status.Succeeded > 0 {
		if err := r.ackWipe(ctx, gs, req); err != nil {
			return err
		}
		return r.deleteWipeJob(ctx, gs.Namespace, jobName)
	}
	// The Job exhausted its retries without succeeding (e.g. a permission
	// error the wipe container's uid can't get past) — report the failure
	// on the GameServer instead of leaving the request silently pending.
	// Do not ack and do not delete the Job, so its pod logs stay available
	// for the operator to inspect; a new request (a different token) will
	// still replace it via the "leftover Job" branch above.
	if jobPermanentlyFailed(&job) {
		return r.setWipeFailed(ctx, gs)
	}
	return nil
}

func (r *GameServerReconciler) createWipeJob(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
	name, token string,
) error {
	mountPath := effectiveMountPath(tmpl)
	uid := int64(65532) // same non-root UID the game + backup jobs use
	nonRoot := true
	noPrivEsc := false
	backoff := int32(2)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: gs.Namespace,
			Labels: map[string]string{
				wipeTokenLabel:                 token,
				"app.kubernetes.io/managed-by": "gameplane",
				"app.kubernetes.io/name":       gs.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   &nonRoot,
						RunAsUser:      &uid,
						RunAsGroup:     &uid,
						FSGroup:        &uid,
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
					Containers: []corev1.Container{{
						Name: "wipe",
						// Same shell image as the config-init container, so an
						// air-gapped install only has to mirror one.
						Image:   configInitImageOrDefault(r.ConfigInitImage),
						Command: []string{"/bin/sh", "-c"},
						// Remove all contents (including dotfiles) but keep the
						// mount point itself. `find -delete` doesn't error on an
						// empty directory (unlike the glob patterns this replaced,
						// which errored on "no match" and had to swallow that with
						// `2>/dev/null; true` — which also swallowed a real EACCES
						// from a subdirectory this uid can't write into).
						//
						// find's exit status alone is NOT trusted: BusyBox find
						// (the default image) prints a failed unlink/rmdir from
						// -delete to stderr but still exits 0. So the script then
						// checks the volume is actually empty and exits non-zero
						// (failing the Job) if anything is left, or if the listing
						// itself fails. The mount path is passed as $1 rather than
						// spliced into the script.
						Args:         []string{wipeScript, "wipe", mountPath},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: mountPath}},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &nonRoot,
							RunAsUser:                &uid,
							AllowPrivilegeEscalation: &noPrivEsc,
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: gs.Name + "-data",
							},
						},
					}},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(gs, job, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (r *GameServerReconciler) ackWipe(ctx context.Context, gs *gameplanev1alpha1.GameServer, token string) error {
	patch := client.MergeFrom(gs.DeepCopy())
	if gs.Annotations == nil {
		gs.Annotations = map[string]string{}
	}
	gs.Annotations[WipeCompletedAnnotation] = token
	return r.Patch(ctx, gs, patch)
}

func (r *GameServerReconciler) deleteWipeJob(ctx context.Context, ns, name string) error {
	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &job); err != nil {
		return client.IgnoreNotFound(err)
	}
	policy := metav1.DeletePropagationBackground
	return client.IgnoreNotFound(r.Delete(ctx, &job, &client.DeleteOptions{PropagationPolicy: &policy}))
}

// jobPermanentlyFailed reports whether job has given up (batch/v1 sets
// JobConditionFailed True once BackoffLimit is exhausted).
func jobPermanentlyFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// setWipeFailed upserts the DataWipe=False/JobFailed condition on the
// GameServer so a wipe that didn't actually empty the volume is visible
// instead of silently acked (F-054).
func (r *GameServerReconciler) setWipeFailed(ctx context.Context, gs *gameplanev1alpha1.GameServer) error {
	base := gs.DeepCopy()
	gs.Status.Conditions = upsertCondition(gs.Status.Conditions, metav1.Condition{
		Type:               gameplanev1alpha1.GameServerConditionDataWipe,
		Status:             metav1.ConditionFalse,
		Reason:             "JobFailed",
		Message:            "data wipe job did not complete; the volume may not be fully cleared",
		ObservedGeneration: gs.Generation,
	})
	return r.Status().Patch(ctx, gs, client.MergeFrom(base))
}
