package kube

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func newTestCluster(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "gameplane.local/v1alpha1",
			"kind":       "Cluster",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"displayName": "Test Cluster " + name,
				"kubeconfigSecret": map[string]any{
					"name": "cluster-" + name + "-kubeconfig",
					"key":  "kubeconfig",
				},
			},
		},
	}
}

func kubeconfig() []byte {
	return []byte(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com:6443
  name: test
contexts:
- context:
    cluster: test
    user: admin
  name: test
current-context: test
users:
- name: admin
  user:
    token: fake-token`)
}

func TestWatchClusters_LoadsRemoteCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set up fake clients with Cluster CRD support.
	scm := runtime.NewScheme()
	_ = scheme.AddToScheme(scm)

	gvkr := map[schema.GroupVersionResource]string{
		GVRCluster: "ClusterList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scm, gvkr, newTestCluster("remote"))
	typed := kubefake.NewClientset()

	// Create the kubeconfig Secret.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-remote-kubeconfig",
			Namespace: "gameplane-system",
			Labels: map[string]string{
				ClusterKubeconfigLabel: "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": kubeconfig(),
		},
	}
	_, _ = typed.CoreV1().Secrets("gameplane-system").Create(ctx, secret, metav1.CreateOptions{})

	home := &Client{Dynamic: dyn, Typed: typed}
	reg := NewRegistry("local")
	reg.Set("local", home) // The local cluster must exist.

	// Run the watcher.
	go WatchClusters(ctx, home, reg, "gameplane-system")

	// Poll until the remote cluster is registered or timeout.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := reg.Get("remote"); ok {
			return // Success!
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("watcher did not load remote cluster")
}

func TestWatchClusters_IgnoresLocalCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scm := runtime.NewScheme()
	_ = scheme.AddToScheme(scm)

	// Create a fake Cluster with name "local" (the default).
	gvkr := map[schema.GroupVersionResource]string{
		GVRCluster: "ClusterList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scm, gvkr, newTestCluster("local"))
	typed := kubefake.NewClientset()

	home := &Client{Dynamic: dyn, Typed: typed}
	reg := NewRegistry("local")
	reg.Set("local", home)

	// Store the original local client to verify it's not replaced.
	originalLocal := reg.Default()

	go WatchClusters(ctx, home, reg, "gameplane-system")

	// Wait a bit for the watcher to process.
	time.Sleep(1 * time.Second)

	// The local client should remain unchanged.
	if reg.Default() != originalLocal {
		t.Fatal("watcher replaced the local cluster client")
	}
}

func TestWatchClusters_DeletesCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scm := runtime.NewScheme()
	_ = scheme.AddToScheme(scm)

	gvkr := map[schema.GroupVersionResource]string{
		GVRCluster: "ClusterList",
	}
	// Start with a remote cluster registered.
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scm, gvkr, newTestCluster("remote"))
	typed := kubefake.NewClientset()

	// Create the kubeconfig Secret.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-remote-kubeconfig",
			Namespace: "gameplane-system",
			Labels: map[string]string{
				ClusterKubeconfigLabel: "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": kubeconfig(),
		},
	}
	_, _ = typed.CoreV1().Secrets("gameplane-system").Create(ctx, secret, metav1.CreateOptions{})

	home := &Client{Dynamic: dyn, Typed: typed}
	reg := NewRegistry("local")
	reg.Set("local", home)

	go WatchClusters(ctx, home, reg, "gameplane-system")

	// Wait for the cluster to be loaded.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := reg.Get("remote"); ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Now delete it from the dynamic client.
	_ = dyn.Resource(GVRCluster).Delete(ctx, "remote", metav1.DeleteOptions{})

	// Poll until it's removed.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := reg.Get("remote"); !ok {
			return // Success!
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("watcher did not delete remote cluster")
}
