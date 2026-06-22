package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kestrelv1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// GameTemplateReconciler mostly exists to maintain status.inUseCount —
// templates are static config and don't create child objects.
type GameTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kestrel.gg,resources=gametemplates,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kestrel.gg,resources=gametemplates/status,verbs=get;update;patch

func (r *GameTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var tmpl kestrelv1alpha1.GameTemplate
	if err := r.Get(ctx, req.NamespacedName, &tmpl); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var servers kestrelv1alpha1.GameServerList
	if err := r.List(ctx, &servers); err != nil {
		return ctrl.Result{}, err
	}
	var count int32
	for i := range servers.Items {
		if servers.Items[i].Spec.TemplateRef.Name == tmpl.Name {
			count++
		}
	}
	if tmpl.Status.InUseCount == count && tmpl.Status.ObservedGeneration == tmpl.Generation {
		return ctrl.Result{}, nil
	}
	tmpl.Status.InUseCount = count
	tmpl.Status.ObservedGeneration = tmpl.Generation
	return ctrl.Result{}, r.Status().Update(ctx, &tmpl)
}

func (r *GameTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kestrelv1alpha1.GameTemplate{}).
		Watches(&kestrelv1alpha1.GameServer{}, enqueueTemplateForServer()).
		Complete(r)
}
