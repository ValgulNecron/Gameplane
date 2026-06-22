package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
	"github.com/ValgulNecron/gameplane/operator/internal/modsrc"
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

// +kubebuilder:rbac:groups=gameplane.gg,resources=modulesources,verbs=get;list;watch
// +kubebuilder:rbac:groups=gameplane.gg,resources=modulesources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

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

	src.Status.ObservedGeneration = src.Generation

	// Total index failure: the source is unreachable. Preserve the
	// last-good catalog so a transient outage doesn't make already-installed
	// modules unresolvable — mark the sync stale instead of blanking it.
	// LastSync is deliberately NOT bumped here, so it always means "time of
	// the last successful index."
	if err != nil {
		logger.Error(err, "indexing source", "source", src.Name)
		src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
			Type:               kestrelv1alpha1.ModuleSourceConditionSynced,
			Status:             metav1.ConditionFalse,
			Reason:             "IndexFailed",
			Message:            err.Error(),
			ObservedGeneration: src.Generation,
		})
		// Keep Ready=True while a previously-indexed catalog is still being
		// served; only a source that has never indexed reports Ready=False.
		readyStatus := metav1.ConditionFalse
		readyReason := "SourceUnreachable"
		readyMsg := err.Error()
		if len(src.Status.Modules) > 0 {
			readyStatus = metav1.ConditionTrue
			readyReason = "ServingStaleCatalog"
			readyMsg = fmt.Sprintf("serving %d cached module(s); last index failed: %v",
				len(src.Status.Modules), err)
		}
		src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
			Type:               kestrelv1alpha1.ModuleSourceConditionReady,
			Status:             readyStatus,
			Reason:             readyReason,
			Message:            readyMsg,
			ObservedGeneration: src.Generation,
		})
		if uerr := r.Status().Update(ctx, &src); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: minRefreshInterval}, nil
	}

	now := metav1.Now()
	src.Status.LastSync = &now
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
		Watches(&corev1.ConfigMap{}, enqueueUploadSourcesForConfigMap(r.Client)).
		Complete(r)
}

// enqueueUploadSourcesForConfigMap re-indexes every upload-type source
// when a labeled bundle ConfigMap changes, so uploads land in the
// catalog immediately instead of on the next refresh tick.
func enqueueUploadSourcesForConfigMap(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetLabels()[kestrelv1alpha1.LabelModuleUpload] != "true" {
			return nil
		}
		var sources kestrelv1alpha1.ModuleSourceList
		if err := c.List(ctx, &sources); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range sources.Items {
			if sources.Items[i].Spec.Type == kestrelv1alpha1.ModuleSourceTypeUpload {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: sources.Items[i].Name},
				})
			}
		}
		return reqs
	})
}
