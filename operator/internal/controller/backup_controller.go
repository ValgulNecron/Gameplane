package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
	"github.com/kestrel-gg/kestrel/operator/internal/agent"
)

// Annotations the BackupReconciler maintains on each Backup to track
// quiesce state across reconcile passes.
const (
	annoQuiesceAttempted = "backup.kestrel.gg/quiesce-attempted"
	annoQuiescedAt       = "backup.kestrel.gg/quiesced-at"
	annoUnquiescedAt     = "backup.kestrel.gg/unquiesced-at"
)

// AgentQuiescer is the slice of *agent.Client the BackupReconciler
// uses. Defined here as an interface so envtest can swap in a fake
// without standing up an HTTPS listener.
type AgentQuiescer interface {
	Quiesce(ctx context.Context, namespace, server string) error
	Unquiesce(ctx context.Context, namespace, server string) error
}

// BackupLogReader fetches the restic container's logs for a completed
// Backup so the reconciler can extract the trailing JSON summary.
// Production uses a Pods.GetLogs implementation; envtest plugs in a
// fake that returns a canned restic-output string.
type BackupLogReader interface {
	BackupLogs(ctx context.Context, namespace, backupName string) (io.ReadCloser, error)
}

// BackupReconciler drives a Backup to completion by creating a one-shot
// Job that runs restic against the GameServer's data volume.
type BackupReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Clientset kubernetes.Interface
	// AgentClient may be nil for installs that haven't configured
	// operator → agent mTLS yet; the reconciler treats that as "no
	// quiesce capability" and proceeds with raw backups.
	AgentClient AgentQuiescer
	// LogReader, if set, overrides the default kubernetes-Clientset
	// based log reader. Used by tests.
	LogReader BackupLogReader
}

// +kubebuilder:rbac:groups=kestrel.gg,resources=backups,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kestrel.gg,resources=backups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *BackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var b kestrelv1alpha1.Backup
	if err := r.Get(ctx, req.NamespacedName, &b); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if b.Status.Phase == kestrelv1alpha1.BackupPhaseSucceeded ||
		b.Status.Phase == kestrelv1alpha1.BackupPhaseFailed {
		return ctrl.Result{}, nil
	}

	// Volume-snapshot strategy is declared in the API but not yet
	// implemented. Short-circuit so the user gets a clean signal
	// rather than a perpetually-Pending Backup.
	if b.Spec.Strategy == "volume-snapshot" {
		return r.fail(ctx, &b, "volume-snapshot strategy not yet implemented")
	}

	if err := r.maybeQuiesce(ctx, &b); err != nil {
		return ctrl.Result{}, err
	}

	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: b.Name, Namespace: b.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, job, func() error {
		if job.CreationTimestamp.IsZero() {
			job.Spec.Template.Spec = r.buildBackupPodSpec(&b)
			job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
		}
		return controllerutil.SetControllerReference(&b, job, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	return r.mirrorJobStatus(ctx, &b, job)
}

// maybeQuiesce calls the agent's /quiesce endpoint once per Backup.
// The quiesce-attempted annotation makes the call idempotent across
// requeues; ErrUnsupported is recorded as "attempted but unsupported"
// so the matching unquiesce step is skipped on terminal phase.
func (r *BackupReconciler) maybeQuiesce(ctx context.Context, b *kestrelv1alpha1.Backup) error {
	if !b.Spec.Quiesce {
		return nil
	}
	if r.AgentClient == nil {
		return nil
	}
	if _, ok := b.Annotations[annoQuiesceAttempted]; ok {
		return nil
	}

	err := r.AgentClient.Quiesce(ctx, b.Namespace, b.Spec.ServerRef.Name)
	switch {
	case err == nil:
		now := time.Now().UTC().Format(time.RFC3339)
		patchBackupAnnotations(b, map[string]string{
			annoQuiesceAttempted: "true",
			annoQuiescedAt:       now,
		})
	case errors.Is(err, agent.ErrUnsupported):
		// Game has no quiesce equivalent. Mark attempted and proceed —
		// games like Valheim write to disk consistently anyway.
		patchBackupAnnotations(b, map[string]string{
			annoQuiesceAttempted: "unsupported",
		})
	default:
		// Hard failure (network, RCON down). Surface as Failed: a
		// backup taken against an unflushed Minecraft world can be
		// inconsistent, so it's safer to fail loudly than to proceed.
		_, ferr := r.fail(ctx, b, fmt.Sprintf("quiesce failed: %v", err))
		if ferr != nil {
			return ferr
		}
		return err
	}
	return r.Update(ctx, b)
}

// patchBackupAnnotations merges in the given key/value pairs without
// dropping pre-existing annotations.
func patchBackupAnnotations(b *kestrelv1alpha1.Backup, kv map[string]string) {
	if b.Annotations == nil {
		b.Annotations = map[string]string{}
	}
	for k, v := range kv {
		b.Annotations[k] = v
	}
}

// mirrorJobStatus copies phase + timestamps from the Job into the
// Backup's status, scrapes the restic JSON summary out of pod logs on
// success, and triggers the matching unquiesce when the Backup
// reaches a terminal phase.
func (r *BackupReconciler) mirrorJobStatus(
	ctx context.Context, b *kestrelv1alpha1.Backup, job *batchv1.Job,
) (ctrl.Result, error) {
	phase := b.Status.Phase
	switch {
	case job.Status.Succeeded > 0:
		phase = kestrelv1alpha1.BackupPhaseSucceeded
	case job.Status.Failed > 0:
		phase = kestrelv1alpha1.BackupPhaseFailed
	case job.Status.Active > 0:
		phase = kestrelv1alpha1.BackupPhaseRunning
	default:
		if phase == "" {
			phase = kestrelv1alpha1.BackupPhasePending
		}
	}

	dirty := false
	if b.Status.Phase != phase {
		b.Status.Phase = phase
		dirty = true
	}
	if b.Status.ObservedGeneration != b.Generation {
		b.Status.ObservedGeneration = b.Generation
		dirty = true
	}
	if job.Status.StartTime != nil && b.Status.StartTime == nil {
		b.Status.StartTime = job.Status.StartTime
		dirty = true
	}
	if job.Status.CompletionTime != nil && b.Status.CompletionTime == nil {
		b.Status.CompletionTime = job.Status.CompletionTime
		dirty = true
	}

	// On first observation of Succeeded, scrape the restic summary
	// for snapshot id + size. Restore depends on a non-empty
	// snapshotID so this isn't optional — but a parse error must
	// not flip the Backup to Failed (the data was actually written),
	// so we surface the error in Message and let the user retry.
	if phase == kestrelv1alpha1.BackupPhaseSucceeded && b.Status.SnapshotID == "" {
		if summary, err := r.readResticSummary(ctx, b); err == nil {
			b.Status.SnapshotID = summary.SnapshotID
			if summary.TotalBytesProcessed > 0 {
				q := resource.MustParse(strconv.FormatInt(summary.TotalBytesProcessed, 10))
				b.Status.Size = &q
			}
			dirty = true
		} else if !errors.Is(err, ErrNoResticSummary) {
			if b.Status.Message != err.Error() {
				b.Status.Message = err.Error()
				dirty = true
			}
		}
	}

	cond := metav1.Condition{
		Type:               "Completed",
		ObservedGeneration: b.Generation,
	}
	switch phase {
	case kestrelv1alpha1.BackupPhaseSucceeded:
		cond.Status = metav1.ConditionTrue
		cond.Reason = "Succeeded"
	case kestrelv1alpha1.BackupPhaseFailed:
		cond.Status = metav1.ConditionFalse
		cond.Reason = "JobFailed"
		cond.Message = "backup job reported Failed"
		if b.Status.Message == "" {
			b.Status.Message = "backup job reported Failed"
			dirty = true
		}
	default:
		cond.Status = metav1.ConditionFalse
		cond.Reason = string(phase)
	}
	newConds := upsertCondition(b.Status.Conditions, cond)
	if !sameConditions(b.Status.Conditions, newConds) {
		b.Status.Conditions = newConds
		dirty = true
	}

	if dirty {
		if err := r.Status().Update(ctx, b); err != nil {
			return ctrl.Result{}, err
		}
	}

	if phase == kestrelv1alpha1.BackupPhaseSucceeded || phase == kestrelv1alpha1.BackupPhaseFailed {
		if err := r.maybeUnquiesce(ctx, b); err != nil {
			// Best-effort: log via ctrl logger and return; the next
			// requeue (driven by external events) won't bring us
			// back here because we're already terminal — accept
			// auto-save staying off rather than blocking the user's
			// view of a "Succeeded" backup. The unquiesced-at
			// annotation will be missing, signalling a manual
			// follow-up is needed.
			ctrllog := ctrl.LoggerFrom(ctx)
			ctrllog.Error(err, "unquiesce failed (best-effort)", "backup", b.Name)
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

// maybeUnquiesce reverses the earlier quiesce. It is only called once
// the Backup is in a terminal phase, and it skips when:
//   - quiesce was never attempted (no annotation),
//   - quiesce was attempted but the game didn't support it,
//   - unquiesce already succeeded on a prior pass.
func (r *BackupReconciler) maybeUnquiesce(ctx context.Context, b *kestrelv1alpha1.Backup) error {
	if r.AgentClient == nil {
		return nil
	}
	state := b.Annotations[annoQuiesceAttempted]
	if state == "" || state == "unsupported" {
		return nil
	}
	if _, ok := b.Annotations[annoUnquiescedAt]; ok {
		return nil
	}
	if err := r.AgentClient.Unquiesce(ctx, b.Namespace, b.Spec.ServerRef.Name); err != nil {
		return err
	}
	patchBackupAnnotations(b, map[string]string{
		annoUnquiescedAt: time.Now().UTC().Format(time.RFC3339),
	})
	return r.Update(ctx, b)
}

// readResticSummary fetches the restic container's logs and parses the
// trailing JSON summary line. Caller must be holding a Backup whose
// matching Job is in Succeeded.
func (r *BackupReconciler) readResticSummary(ctx context.Context, b *kestrelv1alpha1.Backup) (*ResticSummary, error) {
	reader := r.LogReader
	if reader == nil {
		if r.Clientset == nil {
			return nil, errors.New("no log reader configured")
		}
		reader = &clientsetLogReader{cs: r.Clientset}
	}
	rc, err := reader.BackupLogs(ctx, b.Namespace, b.Name)
	if err != nil {
		return nil, fmt.Errorf("read restic logs: %w", err)
	}
	defer rc.Close()
	return ParseResticSummary(rc)
}

// fail writes a terminal Failed phase + message + condition to the
// Backup. Mirrors RestoreReconciler.fail so the two controllers
// produce identical user-visible signals.
func (r *BackupReconciler) fail(ctx context.Context, b *kestrelv1alpha1.Backup, msg string) (ctrl.Result, error) {
	now := metav1.Now()
	b.Status.Phase = kestrelv1alpha1.BackupPhaseFailed
	b.Status.ObservedGeneration = b.Generation
	if b.Status.Message != msg {
		b.Status.Message = msg
	}
	if b.Status.CompletionTime == nil {
		b.Status.CompletionTime = &now
	}
	b.Status.Conditions = upsertCondition(b.Status.Conditions, metav1.Condition{
		Type:               "Completed",
		Status:             metav1.ConditionFalse,
		Reason:             "Failed",
		Message:            msg,
		ObservedGeneration: b.Generation,
	})
	if err := r.Status().Update(ctx, b); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *BackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kestrelv1alpha1.Backup{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// buildBackupPodSpec produces the pod for a single restic backup.
//
// Layout:
//   - initContainer "restic-init": idempotently prepares the repo so a
//     first-ever Backup against an empty bucket succeeds. `restic cat
//     config` returns 0 if the repo exists; otherwise we run `restic
//     init` to create it.
//   - container "restic": runs the actual `restic backup --json` with
//     spec.tags appended to the default "kestrel" tag.
//
// Both containers share the same env (repo + password from the user's
// Secret) and a tmpfs-backed cache to keep the rootfs read-only.
func (r *BackupReconciler) buildBackupPodSpec(b *kestrelv1alpha1.Backup) corev1.PodSpec {
	nonRoot := true
	roRootFS := true
	noPrivEsc := false
	uid := int64(65532)

	env := []corev1.EnvVar{
		{Name: "RESTIC_REPOSITORY", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: b.Spec.RepoRef.Name},
				Key:                  "repo",
			},
		}},
		{Name: "RESTIC_PASSWORD", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: b.Spec.RepoRef.Name},
				Key:                  "password",
			},
		}},
		{Name: "XDG_CACHE_HOME", Value: "/tmp/restic-cache"},
	}

	containerSec := &corev1.SecurityContext{
		RunAsNonRoot:             &nonRoot,
		RunAsUser:                &uid,
		ReadOnlyRootFilesystem:   &roRootFS,
		AllowPrivilegeEscalation: &noPrivEsc,
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}

	args := []string{"backup", "/data", "--json", "--tag", "kestrel"}
	for _, t := range b.Spec.Tags {
		args = append(args, "--tag", t)
	}

	return corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:   &nonRoot,
			RunAsUser:      &uid,
			RunAsGroup:     &uid,
			FSGroup:        &uid,
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
		InitContainers: []corev1.Container{{
			Name:    "restic-init",
			Image:   "restic/restic:0.17.1",
			Command: []string{"/bin/sh", "-c"},
			// `cat config` is the canonical "does the repo exist" probe;
			// `init` is idempotent against an empty target.
			Args: []string{"restic cat config >/dev/null 2>&1 || restic init"},
			Env:  env,
			VolumeMounts: []corev1.VolumeMount{
				{Name: "cache", MountPath: "/tmp"},
			},
			SecurityContext: containerSec,
		}},
		Containers: []corev1.Container{{
			Name:  "restic",
			Image: "restic/restic:0.17.1",
			Args:  args,
			Env:   env,
			VolumeMounts: []corev1.VolumeMount{
				{Name: "data", MountPath: "/data", ReadOnly: true},
				{Name: "cache", MountPath: "/tmp"},
			},
			SecurityContext: containerSec,
		}},
		Volumes: []corev1.Volume{
			{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: b.Spec.ServerRef.Name + "-data",
						ReadOnly:  true,
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

// --- production log reader -----------------------------------------------

// clientsetLogReader fetches the logs of the most-recently-completed
// pod for a Backup's Job. The Job controller stamps every pod with a
// `job-name=<jobName>` label, which is what we filter on.
type clientsetLogReader struct {
	cs kubernetes.Interface
}

func (l *clientsetLogReader) BackupLogs(ctx context.Context, namespace, backupName string) (io.ReadCloser, error) {
	pods, err := l.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + backupName,
	})
	if err != nil {
		return nil, err
	}
	var pick *corev1.Pod
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodSucceeded {
			continue
		}
		if pick == nil || p.CreationTimestamp.After(pick.CreationTimestamp.Time) {
			pick = p
		}
	}
	if pick == nil {
		return nil, errors.New("no succeeded pod yet for backup job")
	}
	req := l.cs.CoreV1().Pods(namespace).GetLogs(pick.Name, &corev1.PodLogOptions{Container: "restic"})
	return req.Stream(ctx)
}

// Compile-time guards: BackupReconciler integrates with the constants
// declared above; tests rely on them not drifting silently.
var (
	_ = types.NamespacedName{}
	_ = apierrors.IsNotFound
)
