package kube

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// resyncPeriod is the shared informer factory's safety net against missed
// watch events.
const resyncPeriod = 10 * time.Minute

// WatchClusters starts an informer on the Cluster CRD and populates the
// registry as clusters are added, updated, or deleted. The "local" cluster
// is ignored (it is always provided by the control plane). Errors loading
// remote kubeconfigs are logged but do not crash — the registry continues
// to serve what it has.
func WatchClusters(ctx context.Context, home *Client, reg *Registry, ns string) {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(home.Dynamic, resyncPeriod)

	if _, err := factory.ForResource(GVRCluster).Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			name := u.GetName()
			// Skip the default cluster (always provided by the control plane).
			if name == reg.DefaultID() {
				slog.Debug("cluster watch: skipping local cluster")
				return
			}
			if err := loadCluster(ctx, home, reg, ns, name); err != nil {
				slog.Warn("cluster watch: failed to load cluster", "cluster", name, "err", err)
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			u, ok := newObj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			name := u.GetName()
			// Skip the default cluster.
			if name == reg.DefaultID() {
				return
			}
			if err := loadCluster(ctx, home, reg, ns, name); err != nil {
				slog.Warn("cluster watch: failed to update cluster", "cluster", name, "err", err)
			}
		},
		DeleteFunc: func(obj any) {
			u, ok := obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
			name := u.GetName()
			// Never remove the default cluster.
			if name == reg.DefaultID() {
				return
			}
			reg.Remove(name)
			slog.Debug("cluster watch: removed cluster", "cluster", name)
		},
	}); err != nil {
		slog.Warn("cluster watch: register handler failed", "err", err)
		return
	}

	factory.Start(ctx.Done())
	// Block until the initial list lands in the caches.
	if !cache.WaitForCacheSync(ctx.Done(), factory.ForResource(GVRCluster).Informer().HasSynced) {
		slog.Warn("cluster watch: cache sync failed")
		return
	}
	slog.Debug("cluster watch: started")
}

// loadCluster reads a Cluster CRD, extracts the kubeconfig Secret reference,
// loads the secret, creates a client, and registers it in the registry.
func loadCluster(ctx context.Context, home *Client, reg *Registry, ns, name string) error {
	u, err := home.Dynamic.Resource(GVRCluster).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get cluster CRD: %w", err)
	}

	// Extract spec.kubeconfigSecret from the unstructured Cluster.
	kcSpec, ok, err := unstructured.NestedMap(u.Object, "spec", "kubeconfigSecret")
	if err != nil {
		return fmt.Errorf("read spec.kubeconfigSecret: %w", err)
	}
	if !ok {
		return fmt.Errorf("spec.kubeconfigSecret not found")
	}

	secretName, ok := kcSpec["name"].(string)
	if !ok || secretName == "" {
		return fmt.Errorf("spec.kubeconfigSecret.name is missing or not a string")
	}

	secretKey, ok := kcSpec["key"].(string)
	if !ok {
		secretKey = "kubeconfig"
	}

	c, err := ClientFromSecret(ctx, home, ns, secretName, secretKey)
	if err != nil {
		return fmt.Errorf("load client from secret: %w", err)
	}

	reg.Set(name, c)
	slog.Debug("cluster watch: loaded cluster", "cluster", name)
	return nil
}
