package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/robfig/cron/v3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

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
	if sched.Spec.Suspend {
		return ctrl.Result{}, r.updateNextTime(ctx, &sched, nil)
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
	if err := r.trimBackups(ctx, &sched); err != nil {
		logger.Error(err, "retention trim")
	}
	if err := r.updateNextTime(ctx, &sched, &next); err != nil {
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
			RepoRef:   &sched.Spec.RepoRef,
			Strategy:  sched.Spec.Strategy,
			Quiesce:   sched.Spec.Quiesce,
		},
	}
	if err := controllerutil.SetControllerReference(sched, b, r.Scheme); err != nil {
		return err
	}
	return r.Create(ctx, b)
}

func (r *BackupScheduleReconciler) updateNextTime(
	ctx context.Context, sched *kestrelv1alpha1.BackupSchedule, next *time.Time,
) error {
	var newNext *metav1.Time
	if next != nil {
		newNext = &metav1.Time{Time: *next}
	}
	if metav1TimeEqual(sched.Status.NextScheduleTime, newNext) {
		return nil
	}
	sched.Status.NextScheduleTime = newNext
	return r.Status().Update(ctx, sched)
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

// silence unused import in early scaffold
var _ = corev1.EventSource{}
