package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
	"github.com/ValgulNecron/gameplane/operator/internal/modsrc"
	"github.com/ValgulNecron/gameplane/operator/internal/verify"
)

// ModuleReconciler materializes Module CRs into GameTemplate CRs. The
// produced GameTemplate carries an OwnerReference back to the Module so
// uninstall = `kubectl delete module <name>` and the K8s GC reaps the
// template.
type ModuleReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string

	// OperatorVersion is this operator's build version, compared against a
	// bundle's gameplaneMinVersion to refuse modules that need a newer
	// operator. Empty or "dev" disables the check.
	OperatorVersion string

	// FetchOptions carries operator-level fetcher config (CLI flags).
	FetchOptions modsrc.Options

	// NewFetcher is overridden in tests with an in-process fake. nil →
	// the real per-source-type fetcher from modsrc.ForSource.
	NewFetcher func(ctx context.Context, src *kestrelv1alpha1.ModuleSource) (modsrc.Fetcher, error)

	// NewVerifier is overridden in tests with an in-process fake. nil →
	// the real cosign verifier from verify.Build.
	NewVerifier func(ctx context.Context, src *kestrelv1alpha1.ModuleSource) (verify.Verifier, error)
}

// +kubebuilder:rbac:groups=gameplane.gg,resources=modules,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gameplane.gg,resources=modules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gameplane.gg,resources=modules/finalizers,verbs=update
// +kubebuilder:rbac:groups=gameplane.gg,resources=gametemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gameplane.gg,resources=gameservers,verbs=get;list;watch

func (r *ModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var mod kestrelv1alpha1.Module
	if err := r.Get(ctx, req.NamespacedName, &mod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !mod.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &mod)
	}
	if !controllerutil.ContainsFinalizer(&mod, kestrelv1alpha1.ModuleFinalizer) {
		controllerutil.AddFinalizer(&mod, kestrelv1alpha1.ModuleFinalizer)
		if err := r.Update(ctx, &mod); err != nil {
			return ctrl.Result{}, err
		}
		// Re-queue from the watch event after the finalizer add.
		return ctrl.Result{}, nil
	}

	// Resolve which version to install.
	src, err := r.getSource(ctx, mod.Spec.Source.Name)
	if err != nil {
		return r.markFailed(ctx, &mod, "SourceNotFound", err)
	}
	entry := byCatalogName(src.Status.Modules, mod.Spec.Name)
	if entry == nil {
		return r.markPending(ctx, &mod, "WaitingForCatalog",
			fmt.Errorf("source %q has not yet indexed module %q", src.Name, mod.Spec.Name))
	}
	desiredVersion := mod.Spec.Version
	if desiredVersion == "" {
		desiredVersion = entry.LatestVersion
	}
	if desiredVersion == "" {
		return r.markPending(ctx, &mod, "NoVersionAvailable",
			fmt.Errorf("source %q has no available versions for module %q", src.Name, mod.Spec.Name))
	}
	if !slices.Contains(entry.Versions, desiredVersion) {
		return r.markFailed(ctx, &mod, "VersionUnavailable",
			fmt.Errorf("version %q not in catalog for %q (available: %v)",
				desiredVersion, mod.Spec.Name, entry.Versions))
	}

	if mod.Status.AppliedVersion == desiredVersion && mod.Status.AppliedTemplate == mod.Name &&
		mod.Status.Phase == kestrelv1alpha1.ModulePhaseReady &&
		(entry.Digest == "" || mod.Status.AppliedDigest == entry.Digest) {
		// Already converged. Non-OCI sources publish a single version
		// stream, so the digest comparison is what catches content
		// changes hiding behind an unchanged version string.
		return ctrl.Result{}, nil
	}

	// Pull bundle.
	fetcher, err := r.fetcherFor(ctx, src)
	if err != nil {
		return r.markFailed(ctx, &mod, "SourceConfig", err)
	}

	if err := r.markPullingTransition(ctx, &mod, desiredVersion); err != nil {
		return ctrl.Result{}, err
	}
	bundle, err := fetcher.Pull(ctx, mod.Spec.Name, desiredVersion)
	if err != nil {
		return r.markFailed(ctx, &mod, "PullFailed", err)
	}

	// Verify the bundle's signature before trusting any of its content —
	// including the metadata read below. Nop when the source declares no
	// verify policy.
	verifier, err := r.verifierFor(ctx, src)
	if err != nil {
		return r.markFailed(ctx, &mod, "VerifyConfig", err)
	}
	if err := verifier.Verify(ctx, entry.Reference, bundle.Digest); err != nil {
		return r.markFailed(ctx, &mod, "SignatureInvalid", err)
	}

	// Honor a content pin: refuse a bundle whose digest doesn't match the
	// one the user pinned (catches a tag moved to new content).
	if mod.Spec.Digest != "" && bundle.Digest != mod.Spec.Digest {
		return r.markFailed(ctx, &mod, "DigestMismatch",
			fmt.Errorf("pinned digest %s but resolved bundle is %s", mod.Spec.Digest, bundle.Digest))
	}

	// Refuse a bundle that needs a newer operator than this one — the
	// reconciler can't honor capabilities it doesn't understand. Leaving the
	// previously-applied GameTemplate untouched here is intentional.
	if r.operatorTooOld(bundle.Metadata.GameplaneMinVersion) {
		return r.markFailed(ctx, &mod, "IncompatibleOperator",
			fmt.Errorf("module %q requires Kestrel >= %s but this operator is %s",
				mod.Spec.Name, bundle.Metadata.GameplaneMinVersion, r.OperatorVersion))
	}

	// Materialize a GameTemplate.
	if err := r.applyTemplate(ctx, &mod, bundle, desiredVersion, src.Name); err != nil {
		return r.markFailed(ctx, &mod, "ApplyTemplate", err)
	}

	// Update Status to Ready. Record the version being replaced as the
	// rollback target before overwriting it (only when it actually changes).
	if mod.Status.AppliedVersion != "" && mod.Status.AppliedVersion != desiredVersion {
		mod.Status.PreviousVersion = mod.Status.AppliedVersion
		mod.Status.PreviousDigest = mod.Status.AppliedDigest
	}
	mod.Status.Phase = kestrelv1alpha1.ModulePhaseReady
	mod.Status.AppliedVersion = desiredVersion
	mod.Status.AppliedDigest = bundle.Digest
	mod.Status.AppliedTemplate = mod.Name
	mod.Status.LastError = ""
	mod.Status.ObservedGeneration = mod.Generation
	mod.Status.Conditions = upsertCondition(mod.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Applied",
		Message:            fmt.Sprintf("GameTemplate %q at %s", mod.Name, desiredVersion),
		ObservedGeneration: mod.Generation,
	})
	mod.Status.Conditions = upsertCondition(mod.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleConditionPulling,
		Status:             metav1.ConditionFalse,
		Reason:             "Applied",
		ObservedGeneration: mod.Generation,
	})
	if err := r.Status().Update(ctx, &mod); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ModuleReconciler) applyTemplate(ctx context.Context, mod *kestrelv1alpha1.Module, bundle *modsrc.Bundle, version, sourceName string) error {
	parsed := &kestrelv1alpha1.GameTemplate{}
	if err := yaml.Unmarshal(bundle.TemplateYAML, parsed); err != nil {
		return fmt.Errorf("parse template.yaml: %w", err)
	}
	desired := &kestrelv1alpha1.GameTemplate{}
	desired.Name = mod.Name
	desired.Spec = parsed.Spec

	// Stamp module-management labels/annotations on the GameTemplate
	// metadata so the API can distinguish managed vs. manual templates.
	if desired.Labels == nil {
		desired.Labels = map[string]string{}
	}
	desired.Labels[kestrelv1alpha1.LabelManagedBy] = kestrelv1alpha1.ManagedByModule
	desired.Labels[kestrelv1alpha1.LabelModuleName] = mod.Spec.Name
	desired.Labels[kestrelv1alpha1.LabelModuleVersion] = version
	desired.Labels[kestrelv1alpha1.LabelModuleSource] = sourceName
	if desired.Annotations == nil {
		desired.Annotations = map[string]string{}
	}
	desired.Annotations[kestrelv1alpha1.LabelModuleDigest] = bundle.Digest

	// Set the OwnerReference so deleting the Module GCs the template.
	if err := controllerutil.SetControllerReference(mod, desired, r.Scheme); err != nil {
		return fmt.Errorf("set owner ref: %w", err)
	}

	// Server-side apply or create-or-update via SSA-like pattern. We do
	// a Get + decide create/update because some tests assert on the
	// resulting object's fields and we want predictable behavior.
	var existing kestrelv1alpha1.GameTemplate
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name}, &existing)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	// Verify ownership before clobbering — refuse to mutate a template
	// that wasn't created by this Module (e.g. pre-existing manual
	// install with the same name).
	if existing.Labels[kestrelv1alpha1.LabelManagedBy] != kestrelv1alpha1.ManagedByModule {
		return fmt.Errorf("template %q exists and is not module-managed", desired.Name)
	}
	if !ownedBy(&existing, mod) {
		return fmt.Errorf("template %q is owned by a different Module", desired.Name)
	}
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	existing.Annotations = mergeAnnotations(existing.Annotations, desired.Annotations)
	return r.Update(ctx, &existing)
}

// finalize handles the deletion path. We refuse to release the
// finalizer (and therefore allow GC of the GameTemplate) while any
// GameServer references it — uninstall would orphan running pods.
func (r *ModuleReconciler) finalize(ctx context.Context, mod *kestrelv1alpha1.Module) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(mod, kestrelv1alpha1.ModuleFinalizer) {
		return ctrl.Result{}, nil
	}
	tmplName := mod.Status.AppliedTemplate
	if tmplName == "" {
		tmplName = mod.Name
	}
	var servers kestrelv1alpha1.GameServerList
	if err := r.List(ctx, &servers); err != nil {
		return ctrl.Result{}, err
	}
	var inUse []string
	for i := range servers.Items {
		if servers.Items[i].Spec.TemplateRef.Name == tmplName {
			inUse = append(inUse, servers.Items[i].Namespace+"/"+servers.Items[i].Name)
		}
	}
	if len(inUse) > 0 {
		// Surface a clear blocker on status; the API translates this
		// into a 409 on DELETE /modules/{name} so the UI can render a
		// useful error.
		mod.Status.Phase = kestrelv1alpha1.ModulePhaseFailed
		mod.Status.LastError = fmt.Sprintf("GameTemplate %q is still in use by: %v", tmplName, inUse)
		mod.Status.Conditions = upsertCondition(mod.Status.Conditions, metav1.Condition{
			Type:               kestrelv1alpha1.ModuleConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "InUse",
			Message:            mod.Status.LastError,
			ObservedGeneration: mod.Generation,
		})
		if err := r.Status().Update(ctx, mod); err != nil {
			return ctrl.Result{}, err
		}
		// Don't release the finalizer; requeue so we re-check after the
		// user removes the GameServers.
		return ctrl.Result{Requeue: true}, nil
	}

	// Delete the materialized GameTemplate. SetControllerReference would
	// also let the K8s GC do this, but we don't rely on the GC ordering
	// because the Module disappears first.
	tmpl := &kestrelv1alpha1.GameTemplate{}
	if err := r.Get(ctx, types.NamespacedName{Name: tmplName}, tmpl); err == nil {
		if ownedBy(tmpl, mod) {
			if err := r.Delete(ctx, tmpl); err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("delete owned template: %w", err)
			}
		}
	}
	controllerutil.RemoveFinalizer(mod, kestrelv1alpha1.ModuleFinalizer)
	if err := r.Update(ctx, mod); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ModuleReconciler) getSource(ctx context.Context, name string) (*kestrelv1alpha1.ModuleSource, error) {
	var src kestrelv1alpha1.ModuleSource
	if err := r.Get(ctx, types.NamespacedName{Name: name}, &src); err != nil {
		return nil, err
	}
	return &src, nil
}

func (r *ModuleReconciler) markPending(ctx context.Context, mod *kestrelv1alpha1.Module, reason string, err error) (ctrl.Result, error) {
	mod.Status.Phase = kestrelv1alpha1.ModulePhasePending
	mod.Status.LastError = err.Error()
	mod.Status.ObservedGeneration = mod.Generation
	mod.Status.Conditions = upsertCondition(mod.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: mod.Generation,
	})
	if uerr := r.Status().Update(ctx, mod); uerr != nil {
		return ctrl.Result{}, uerr
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *ModuleReconciler) markFailed(ctx context.Context, mod *kestrelv1alpha1.Module, reason string, err error) (ctrl.Result, error) {
	mod.Status.Phase = kestrelv1alpha1.ModulePhaseFailed
	mod.Status.LastError = err.Error()
	mod.Status.ObservedGeneration = mod.Generation
	mod.Status.Conditions = upsertCondition(mod.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            err.Error(),
		ObservedGeneration: mod.Generation,
	})
	// A failure terminates the pull attempt. markPullingTransition sets
	// Pulling=True before the steps that fail into here (pull, verify,
	// digest, apply), so clear it — otherwise the dashboard shows a module
	// stuck "Pulling" forever alongside "Failed".
	mod.Status.Conditions = upsertCondition(mod.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleConditionPulling,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		ObservedGeneration: mod.Generation,
	})
	if uerr := r.Status().Update(ctx, mod); uerr != nil {
		return ctrl.Result{}, uerr
	}
	return ctrl.Result{}, err
}

func (r *ModuleReconciler) markPullingTransition(ctx context.Context, mod *kestrelv1alpha1.Module, version string) error {
	if mod.Status.Phase == kestrelv1alpha1.ModulePhasePulling {
		return nil
	}
	mod.Status.Phase = kestrelv1alpha1.ModulePhasePulling
	mod.Status.Conditions = upsertCondition(mod.Status.Conditions, metav1.Condition{
		Type:               kestrelv1alpha1.ModuleConditionPulling,
		Status:             metav1.ConditionTrue,
		Reason:             "Pulling",
		Message:            "pulling " + version,
		ObservedGeneration: mod.Generation,
	})
	return r.Status().Update(ctx, mod)
}

func (r *ModuleReconciler) fetcherFor(ctx context.Context, src *kestrelv1alpha1.ModuleSource) (modsrc.Fetcher, error) {
	if r.NewFetcher != nil {
		return r.NewFetcher(ctx, src)
	}
	return modsrc.ForSource(ctx, r.Client, r.Namespace, src, r.FetchOptions)
}

func (r *ModuleReconciler) verifierFor(ctx context.Context, src *kestrelv1alpha1.ModuleSource) (verify.Verifier, error) {
	if r.NewVerifier != nil {
		return r.NewVerifier(ctx, src)
	}
	return verify.Build(ctx, r.Client, r.Namespace, src)
}

// operatorTooOld reports whether minVersion (a bundle's gameplaneMinVersion)
// is newer than this operator. It is conservative: an empty requirement, a
// "dev"/empty operator build, or either value failing to parse as semver all
// skip the gate so local and pre-release clusters keep working.
func (r *ModuleReconciler) operatorTooOld(minVersion string) bool {
	if minVersion == "" || r.OperatorVersion == "" || r.OperatorVersion == "dev" {
		return false
	}
	have := "v" + strings.TrimPrefix(r.OperatorVersion, "v")
	want := "v" + strings.TrimPrefix(minVersion, "v")
	if !semver.IsValid(have) || !semver.IsValid(want) {
		return false
	}
	return semver.Compare(have, want) < 0
}

func byCatalogName(entries []kestrelv1alpha1.ModuleEntry, name string) *kestrelv1alpha1.ModuleEntry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func ownedBy(o client.Object, owner *kestrelv1alpha1.Module) bool {
	for _, ref := range o.GetOwnerReferences() {
		if ref.UID == owner.UID && ref.Kind == "Module" {
			return true
		}
	}
	return false
}

func mergeAnnotations(into, from map[string]string) map[string]string {
	if into == nil {
		into = map[string]string{}
	}
	for k, v := range from {
		into[k] = v
	}
	return into
}

func (r *ModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kestrelv1alpha1.Module{}).
		Owns(&kestrelv1alpha1.GameTemplate{}).
		Watches(&kestrelv1alpha1.ModuleSource{}, enqueueModulesForSource(r.Client)).
		Complete(r)
}

// enqueueModulesForSource maps a ModuleSource change to a reconcile of
// every Module that references it — so once the source's catalog
// indexes, pending Modules can resolve their version.
func enqueueModulesForSource(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		src, ok := obj.(*kestrelv1alpha1.ModuleSource)
		if !ok {
			return nil
		}
		var mods kestrelv1alpha1.ModuleList
		if err := c.List(ctx, &mods); err != nil {
			return nil
		}
		var reqs []reconcile.Request
		for i := range mods.Items {
			if mods.Items[i].Spec.Source.Name == src.Name {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: mods.Items[i].Name},
				})
			}
		}
		return reqs
	})
}
