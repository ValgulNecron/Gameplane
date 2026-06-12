package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
	"github.com/kestrel-gg/kestrel/operator/internal/modsrc"
)

// defaultRefreshInterval is used when ModuleSource.spec.refreshInterval
// is unset or zero. It also caps the requeue interval when set very low
// (a 1-second refresh would hammer the source).
const (
	defaultRefreshInterval = time.Hour
	minRefreshInterval     = time.Minute
)

// ModuleSourceReconciler indexes the store behind each ModuleSource and
// surfaces the catalog into status.modules. It does not install
// anything — install is driven separately by Module CRs.
type ModuleSourceReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string // namespace where credential Secrets live (operator ns)

	// FetchOptions carries operator-level fetcher config (CLI flags).
	FetchOptions modsrc.Options

	// NewFetcher lets envtest swap in an in-process Fetcher. nil → the
	// real per-source-type fetcher from modsrc.ForSource.
	NewFetcher func(ctx context.Context, src *kestrelv1alpha1.ModuleSource) (modsrc.Fetcher, error)
}

// +kubebuilder:rbac:groups=kestrel.gg,resources=modulesources,verbs=get;list;watch
// +kubebuilder:rbac:groups=kestrel.gg,resources=modulesources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ModuleSourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var src kestrelv1alpha1.ModuleSource
	if err := r.Get(ctx, req.NamespacedName, &src); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	fetcher, err := r.fetcherFor(ctx, &src)
	if err != nil {
		return r.fail(ctx, &src, fmt.Errorf("configure source: %w", err))
	}

	entries, warnings, err := fetcher.Index(ctx)
	for _, w := range warnings {
		logger.Info("module index warning", "source", src.Name, "warning", w)
	}

	now := metav1.Now()
	src.Status.LastSync = &now
	src.Status.ObservedGeneration = src.Generation

	// Total index failure: the source is unreachable. Don't publish a
	// catalog of empty stubs as if it were healthy — report the failure
	// and back off.
	if err != nil {
		logger.Error(err, "indexing source", "source", src.Name)
		src.Status.Modules = nil
		src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
			Type:               kestrelv1alpha1.ModuleSourceConditionSynced,
			Status:             metav1.ConditionFalse,
			Reason:             "IndexFailed",
			Message:            err.Error(),
			ObservedGeneration: src.Generation,
		})
		src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
			Type:               kestrelv1alpha1.ModuleSourceConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "SourceUnreachable",
			ObservedGeneration: src.Generation,
		})
		if uerr := r.Status().Update(ctx, &src); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: minRefreshInterval}, nil
	}

	src.Status.Modules = entries
	src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleSourceConditionSynced,
		Status:             metav1.ConditionTrue,
		Reason:             "Indexed",
		Message:            fmt.Sprintf("indexed %d of %d module(s)", len(entries)-len(warnings), len(entries)),
		ObservedGeneration: src.Generation,
	})
	src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleSourceConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "CatalogPopulated",
		ObservedGeneration: src.Generation,
	})

	if err := r.Status().Update(ctx, &src); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: refreshInterval(&src)}, nil
}

func (r *ModuleSourceReconciler) fail(ctx context.Context, src *kestrelv1alpha1.ModuleSource, err error) (ctrl.Result, error) {
	src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleSourceConditionSynced,
		Status:             metav1.ConditionFalse,
		Reason:             "IndexFailed",
		Message:            err.Error(),
		ObservedGeneration: src.Generation,
	})
	if uerr := r.Status().Update(ctx, src); uerr != nil {
		return ctrl.Result{}, uerr
	}
	// Back off on failure: shorter than the configured interval to recover
	// quickly, but never under the 1-minute floor.
	return ctrl.Result{RequeueAfter: minRefreshInterval}, nil
}

func (r *ModuleSourceReconciler) fetcherFor(ctx context.Context, src *kestrelv1alpha1.ModuleSource) (modsrc.Fetcher, error) {
	if r.NewFetcher != nil {
		return r.NewFetcher(ctx, src)
	}
	return modsrc.ForSource(ctx, r.Client, r.Namespace, src, r.FetchOptions)
}

func refreshInterval(src *kestrelv1alpha1.ModuleSource) time.Duration {
	d := src.Spec.RefreshInterval.Duration
	if d <= 0 {
		return defaultRefreshInterval
	}
	if d < minRefreshInterval {
		return minRefreshInterval
	}
	return d
}

func (r *ModuleSourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kestrelv1alpha1.ModuleSource{}).
		Complete(r)
}
