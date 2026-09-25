//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// TestGameServer_WipeReportsFailureOnPermissionDenied covers F-054: a wipe
// Job that cannot empty the volume (a subdirectory owned by a different
// uid, mode 0755, so the wipe Job's fixed uid 65532 lacks write access to
// unlink files in it) must surface as a failure — a DataWipe=False
// condition on the GameServer — never as a silently acked success.
func TestGameServer_WipeReportsFailureOnPermissionDenied(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmpl := "e2e-wipe-busybox"
	gs := "e2e-wipe-unwritable"

	applyBusyboxTemplate(t, tmpl)
	applyBusyboxGameServer(t, ns, gs, tmpl)

	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		_, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "ss not yet: " + err.Error()
		}
		return true, ""
	})
	waitPVCBound(t, ns, gs+"-data", 90*time.Second)

	// Suspend so the wipe Job (and our prep Job) can mount the RWO volume
	// without the game pod holding it.
	suspendPatch := []byte(`{"spec":{"suspend":true}}`)
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, suspendPatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch suspend=true: %v", err)
	}
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get ss: " + err.Error()
		}
		if ss.Spec.Replicas != nil && *ss.Spec.Replicas == 0 {
			return true, ""
		}
		return false, fmt.Sprintf("expected replicas=0, got %v", ss.Spec.Replicas)
	})

	// Prep Job: create /data/locked owned by uid 1000, mode 0755. The wipe
	// Job runs as uid 65532 (not in this group), so it can list/stat the
	// file but cannot unlink it — the EACCES this test reproduces.
	prepName := gs + "-wipe-prep"
	var (
		nonRoot   = true
		noPrivEsc = false
		prepUID   = int64(1000)
		backoff0  = int32(0)
	)
	prepJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: prepName, Namespace: ns},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff0,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   &nonRoot,
						RunAsUser:      &prepUID,
						RunAsGroup:     &prepUID,
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
					Containers: []corev1.Container{{
						Name:         "prep",
						Image:        "busybox:1.36",
						Command:      []string{"/bin/sh", "-c"},
						Args:         []string{"mkdir -p /data/locked && touch /data/locked/keep.txt && chmod 0755 /data/locked"},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &nonRoot,
							RunAsUser:                &prepUID,
							AllowPrivilegeEscalation: &noPrivEsc,
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: gs + "-data"},
						},
					}},
				},
			},
		},
	}
	bg := metav1.DeletePropagationBackground
	t.Cleanup(func() {
		_ = envInstance.K8s.BatchV1().Jobs(ns).Delete(context.Background(), prepName, metav1.DeleteOptions{PropagationPolicy: &bg})
	})
	if _, err := envInstance.K8s.BatchV1().Jobs(ns).Create(ctx, prepJob, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create prep job: %v", err)
	}
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		j, err := envInstance.K8s.BatchV1().Jobs(ns).Get(ctx, prepName, metav1.GetOptions{})
		if err != nil {
			return false, "get prep job: " + err.Error()
		}
		if j.Status.Succeeded > 0 {
			return true, ""
		}
		return false, fmt.Sprintf("prep job not done: succeeded=%d failed=%d", j.Status.Succeeded, j.Status.Failed)
	})

	// Request the wipe.
	token := "e2e-wipe-token-1"
	wipePatch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{"gameplane.local/wipe-data-requested":%q}}}`, token))
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Patch(ctx, gs, types.MergePatchType, wipePatch, metav1.PatchOptions{}); err != nil {
		t.Fatalf("patch wipe request: %v", err)
	}

	// The wipe Job (BackoffLimit=2) must fail rather than be acked as done.
	envInstance.Eventually(t, 120*time.Second, func() (bool, string) {
		got, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).Get(ctx, gs, metav1.GetOptions{})
		if err != nil {
			return false, "get gameserver: " + err.Error()
		}
		cond := findCondition(got.Object, "DataWipe")
		if cond == nil {
			return false, "DataWipe condition not found"
		}
		if cond["status"] != "False" {
			return false, fmt.Sprintf("DataWipe status=%v", cond["status"])
		}
		if cond["reason"] != "JobFailed" {
			return false, fmt.Sprintf("DataWipe reason=%v, want JobFailed", cond["reason"])
		}
		done, _, _ := unstructured.NestedString(got.Object, "metadata", "annotations", "gameplane.local/wipe-data-completed")
		if done == token {
			return false, "wipe was acked as completed despite the Job failing"
		}
		return true, ""
	})
}
