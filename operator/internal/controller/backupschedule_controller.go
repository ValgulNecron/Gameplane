package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/robfig/cron/v3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// conditionRetentionTrimmed reports whether the most recent retention
// pass pruned old backups successfully. Status False (reason TrimFailed)
// means old backups may be accumulating because a List/Delete failed —
// the dashboard surfaces this so an operator can intervene, rather than
// the failure being swallowed into the controller log only.
const conditionRetentionTrimmed = "RetentionTrimmed"

// BackupScheduleReconciler computes the next firing time from
// Spec.Schedule and creates Backup objects when due. Retention trimming
// is enforced by deleting oldest Backups that fall outside keep-* rules.
type BackupScheduleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kestrel.gg,resources=backupschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kestrel.gg,resources=backupschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kestrel.gg,resources=backups,verbs=get;list;watch;create;delete

func (r *BackupScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var sched kestrelv1alpha1.BackupSchedule
	if err := r.Get(ctx, req.NamespacedName, &sched); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Snapshot the observed status so we can write back exactly once, and
	// only when something actually changed (avoids reconcile churn).
	before := sched.Status.DeepCopy()

	if sched.Spec.Suspend {
		sched.Status.NextScheduleTime = nil
		return ctrl.Result{}, r.persistStatus(ctx, before, &sched)
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	expr, err := parser.Parse(sched.Spec.Schedule)
	if err != nil {
		logger.Error(err, "invalid cron expression", "schedule", sched.Spec.Schedule)
		return ctrl.Result{}, nil
	}

	now := time.Now()
	prev := now.Add(-time.Hour)
	if sched.Status.LastScheduleTime != nil {
		prev = sched.Status.LastScheduleTime.Time
	}
	next := expr.Next(prev)

	if !next.After(now) {
		if err := r.fire(ctx, &sched, now); err != nil {
			return ctrl.Result{}, err
		}
		sched.Status.LastScheduleTime = &metav1.Time{Time: now}
		next = expr.Next(now)
	}

	// Retention trimming is best-effort: a transient List/Delete failure
	// must not wedge scheduling, so we record the outcome as a condition
	// instead of returning the error and retrying the whole reconcile.
	if err := r.trimBackups(ctx, &sched); err != nil {
		logger.Error(err, "retention trim")
		sched.Status.Conditions = upsertCondition(sched.Status.Conditions, metav1.Condition{
			Type:               conditionRetentionTrimmed,
			Status:             metav1.ConditionFalse,
			Reason:             "TrimFailed",
			Message:            err.Error(),
			ObservedGeneration: sched.Generation,
		})
	} else if sched.Spec.Retention != nil {
		sched.Status.Conditions = upsertCondition(sched.Status.Conditions, metav1.Condition{
			Type:               conditionRetentionTrimmed,
			Status:             metav1.ConditionTrue,
			Reason:             "TrimSucceeded",
			Message:            "retention policy applied",
			ObservedGeneration: sched.Generation,
		})
	}

	sched.Status.NextScheduleTime = &metav1.Time{Time: next}

	if err := r.persistStatus(ctx, before, &sched); err != nil {
		return ctrl.Result{}, err
	}

	requeueAfter := time.Until(next)
	if requeueAfter < time.Second {
		requeueAfter = time.Second
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *BackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kestrelv1alpha1.BackupSchedule{}).
		Owns(&kestrelv1alpha1.Backup{}).
		Complete(r)
}

func (r *BackupScheduleReconciler) fire(
	ctx context.Context, sched *kestrelv1alpha1.BackupSchedule, at time.Time,
) error {
	b := &kestrelv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", sched.Name, at.UTC().Format("20060102-150405")),
			Namespace: sched.Namespace,
			Labels:    map[string]string{"kestrel.gg/backup-schedule": sched.Name},
		},
		Spec: kestrelv1alpha1.BackupSpec{
			ServerRef: sched.Spec.ServerRef,
			RepoRef:   sched.Spec.RepoRef,
			Strategy:  sched.Spec.Strategy,
			Quiesce:   sched.Spec.Quiesce,
		},
	}
	if err := controllerutil.SetControllerReference(sched, b, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, b)
}

// persistStatus writes the schedule's status subresource only when it
// differs from the snapshot taken at the start of Reconcile. Centralising
// the write means a condition flip is persisted even on reconciles where
// NextScheduleTime is unchanged (e.g. a dormant or suspended schedule).
func (r *BackupScheduleReconciler) persistStatus(
	ctx context.Context,
	before *kestrelv1alpha1.BackupScheduleStatus,
	sched *kestrelv1alpha1.BackupSchedule,
) error {
	if scheduleStatusEqual(before, &sched.Status) {
		return nil
	}
	return r.Status().Update(ctx, sched)
}

// scheduleStatusEqual reports whether the two statuses are equivalent for
// the fields this controller manages.
func scheduleStatusEqual(a, b *kestrelv1alpha1.BackupScheduleStatus) bool {
	return metav1TimeEqual(a.LastScheduleTime, b.LastScheduleTime) &&
		metav1TimeEqual(a.NextScheduleTime, b.NextScheduleTime) &&
		sameConditions(a.Conditions, b.Conditions)
}

func metav1TimeEqual(a, b *metav1.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Time.Equal(b.Time)
	}
}
