package controller

import (
	"context"
	"fmt"
	"path"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
	"github.com/kestrel-gg/kestrel/operator/internal/oci"
)

// defaultRefreshInterval is used when ModuleSource.spec.refreshInterval
// is unset or zero. It also caps the requeue interval when set very low
// (a 1-second refresh would hammer the registry).
const (
	defaultRefreshInterval = time.Hour
	minRefreshInterval     = time.Minute
)

// ModuleSourceReconciler indexes one or more OCI registries listed in a
// ModuleSource and surfaces the catalog into status.modules. It does
// not install anything — install is driven separately by Module CRs.
type ModuleSourceReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string // namespace where pull-secret Secrets live (operator ns)

	// NewClient lets envtest swap in an in-process Client. nil → real OCI client.
	NewClient func(creds oci.CredentialFunc, insecure bool) ociClient
}

// ociClient is the subset of *oci.Client the reconciler needs. Lets us
// inject a fake in tests.
type ociClient interface {
	ListTags(ctx context.Context, ref string) ([]string, error)
	Pull(ctx context.Context, ref, reference string) (*oci.Bundle, error)
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

	creds, err := oci.CredentialFromSecret(ctx, r.Client, r.Namespace, src.Spec.PullSecretRef)
	if err != nil {
		return r.fail(ctx, &src, fmt.Errorf("resolve credentials: %w", err))
	}

	cli := r.newClient(creds, src.Spec.Insecure)

	entries := make([]kestrelv1alpha1.ModuleEntry, 0, len(src.Spec.Modules))
	for _, m := range src.Spec.Modules {
		ref := path.Join(src.Spec.URL, m.Name)
		entry, err := r.indexModule(ctx, cli, m.Name, ref)
		if err != nil {
			logger.Error(err, "indexing module", "module", m.Name)
			// Surface partial progress; one bad module shouldn't blank the
			// whole catalog. Keep the entry with no versions so the UI
			// shows the failure inline.
			entries = append(entries, kestrelv1alpha1.ModuleEntry{
				Name:      m.Name,
				Reference: ref,
			})
			continue
		}
		entries = append(entries, entry)
	}

	now := metav1.Now()
	src.Status.LastSync = &now
	src.Status.ObservedGeneration = src.Generation
	src.Status.Modules = entries
	src.Status.Conditions = upsertCondition(src.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleSourceConditionSynced,
		Status:             metav1.ConditionTrue,
		Reason:             "Indexed",
		Message:            fmt.Sprintf("indexed %d module(s)", len(entries)),
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

// indexModule lists tags for a module and probes its latest version to
// pick up display metadata.
func (r *ModuleSourceReconciler) indexModule(ctx context.Context, cli ociClient, name, ref string) (kestrelv1alpha1.ModuleEntry, error) {
	tags, err := cli.ListTags(ctx, ref)
	if err != nil {
		return kestrelv1alpha1.ModuleEntry{}, fmt.Errorf("list tags: %w", err)
	}
	entry := kestrelv1alpha1.ModuleEntry{
		Name:      name,
		Reference: ref,
		Versions:  tags,
	}
	if len(tags) == 0 {
		return entry, fmt.Errorf("no semver tags found at %s", ref)
	}
	entry.LatestVersion = tags[0]

	bundle, err := cli.Pull(ctx, ref, entry.LatestVersion)
	if err != nil {
		// Tags found but pull failed — surface tags but no metadata.
		return entry, fmt.Errorf("pull metadata for %s:%s: %w", ref, entry.LatestVersion, err)
	}
	if bundle.Metadata.Name != "" && bundle.Metadata.Name != name {
		return entry, fmt.Errorf("bundle metadata name %q != source ref name %q", bundle.Metadata.Name, name)
	}
	entry.DisplayName = bundle.Metadata.DisplayName
	entry.Summary = bundle.Metadata.Summary
	entry.Game = bundle.Metadata.Game
	entry.Icon = bundle.Metadata.Icon
	// Stable order on output so unchanged inputs produce no status churn.
	sort.Strings(entry.Versions)
	sort.Slice(entry.Versions, func(i, j int) bool {
		return semverDescending(entry.Versions[i], entry.Versions[j])
	})
	return entry, nil
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

func (r *ModuleSourceReconciler) newClient(creds oci.CredentialFunc, insecure bool) ociClient {
	if r.NewClient != nil {
		return r.NewClient(creds, insecure)
	}
	return oci.New(creds, insecure)
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
