package controller

import (
	"context"
	"fmt"

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
						Name:    "wipe",
						Image:   "busybox:1.36",
						Command: []string{"/bin/sh", "-c"},
						// Remove all contents (including dotfiles) but keep the
						// mount point itself.
						Args: []string{fmt.Sprintf(
							"rm -rf %[1]s/..?* %[1]s/.[!.]* %[1]s/* 2>/dev/null; true", mountPath)},
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
