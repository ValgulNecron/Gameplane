package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// ClusterStatusReconciler performs periodic health checks on remote clusters.
type ClusterStatusReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Namespace string // namespace where kubeconfig Secrets live (control-plane namespace)
}

// +kubebuilder:rbac:groups=gameplane.local,resources=clusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gameplane.local,resources=clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ClusterStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cluster gameplanev1alpha1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reserved-name guard: "local" is reserved for the local operator cluster.
	if req.Name == "local" {
		cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseUnhealthy
		cluster.Status.Message = `cluster name "local" is reserved`
		cluster.Status.LastCheckTime = &metav1.Time{Time: time.Now()}
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
			Type:               gameplanev1alpha1.ClusterConditionHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "NameReserved",
			Message:            `cluster name "local" is reserved`,
			ObservedGeneration: cluster.Generation,
		})
		if err := r.Status().Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Read the kubeconfig Secret.
	secretRef := cluster.Spec.KubeconfigSecret
	secretKey := secretRef.Key
	if secretKey == "" {
		secretKey = "kubeconfig"
	}

	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: secretRef.Name}, &secret); err != nil {
		// Secret not found or other read error.
		now := metav1.Now()
		cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseUnhealthy
		cluster.Status.Message = fmt.Sprintf("kubeconfig secret %s/%s not found", r.Namespace, secretRef.Name)
		cluster.Status.LastCheckTime = &now
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
			Type:               gameplanev1alpha1.ClusterConditionHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "SecretNotFound",
			Message:            fmt.Sprintf("kubeconfig secret not found: %v", err),
			ObservedGeneration: cluster.Generation,
		})
		if err := r.Status().Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Verify the secret has the gameplane kubeconfig label.
	if secret.Labels[gameplanev1alpha1.LabelClusterKubeconfig] != "true" {
		now := metav1.Now()
		cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseUnhealthy
		cluster.Status.Message = fmt.Sprintf("kubeconfig secret %s/%s missing label gameplane.local/cluster-kubeconfig=true", r.Namespace, secretRef.Name)
		cluster.Status.LastCheckTime = &now
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
			Type:               gameplanev1alpha1.ClusterConditionHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "InvalidSecret",
			Message:            "kubeconfig secret missing required label",
			ObservedGeneration: cluster.Generation,
		})
		if err := r.Status().Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Read the kubeconfig data from the secret key.
	kubeconfigData, ok := secret.Data[secretKey]
	if !ok {
		now := metav1.Now()
		cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseUnhealthy
		cluster.Status.Message = fmt.Sprintf("kubeconfig secret %s/%s missing key %q", r.Namespace, secretRef.Name, secretKey)
		cluster.Status.LastCheckTime = &now
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
			Type:               gameplanev1alpha1.ClusterConditionHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "MissingKey",
			Message:            fmt.Sprintf("kubeconfig secret missing key %q", secretKey),
			ObservedGeneration: cluster.Generation,
		})
		if err := r.Status().Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Parse the kubeconfig.
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		now := metav1.Now()
		cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseUnhealthy
		cluster.Status.Message = fmt.Sprintf("failed to parse kubeconfig: %v", err)
		cluster.Status.LastCheckTime = &now
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
			Type:               gameplanev1alpha1.ClusterConditionHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "BadKubeconfig",
			Message:            fmt.Sprintf("failed to parse kubeconfig: %v", err),
			ObservedGeneration: cluster.Generation,
		})
		if err := r.Status().Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Set a timeout for discovery operations.
	restCfg.Timeout = 10 * time.Second

	// Get the server version to verify connectivity.
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		now := metav1.Now()
		cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseUnhealthy
		cluster.Status.Message = fmt.Sprintf("failed to create discovery client: %v", err)
		cluster.Status.LastCheckTime = &now
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
			Type:               gameplanev1alpha1.ClusterConditionHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "DiscoveryClientFailed",
			Message:            fmt.Sprintf("failed to create discovery client: %v", err),
			ObservedGeneration: cluster.Generation,
		})
		if err := r.Status().Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Get the server version.
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		now := metav1.Now()
		cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseUnhealthy
		cluster.Status.Message = fmt.Sprintf("cluster health check failed: %v", err)
		cluster.Status.LastCheckTime = &now
		cluster.Status.ObservedGeneration = cluster.Generation
		cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
			Type:               gameplanev1alpha1.ClusterConditionHealthy,
			Status:             metav1.ConditionFalse,
			Reason:             "HealthCheckFailed",
			Message:            fmt.Sprintf("cluster health check failed: %v", err),
			ObservedGeneration: cluster.Generation,
		})
		if err := r.Status().Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	// Cluster is healthy.
	now := metav1.Now()
	cluster.Status.Phase = gameplanev1alpha1.ClusterPhaseHealthy
	cluster.Status.Message = ""
	cluster.Status.ServerVersion = serverVersion.String()
	cluster.Status.LastCheckTime = &now
	cluster.Status.ObservedGeneration = cluster.Generation
	cluster.Status.Conditions = upsertCondition(cluster.Status.Conditions, metav1.Condition{
		Type:               gameplanev1alpha1.ClusterConditionHealthy,
		Status:             metav1.ConditionTrue,
		Reason:             "HealthCheckPassed",
		Message:            fmt.Sprintf("cluster is healthy, version %s", serverVersion.String()),
		ObservedGeneration: cluster.Generation,
	})

	if err := r.Status().Update(ctx, &cluster); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
}

func (r *ClusterStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gameplanev1alpha1.Cluster{}).
		Complete(r)
}
