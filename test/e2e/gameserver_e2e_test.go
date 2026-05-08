//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// gameTemplateGVR / gameServerGVR — typed clients aren't generated for
// the test/e2e module; we use the dynamic client.
var (
	gameTemplateGVR = schema.GroupVersionResource{Group: "kestrel.gg", Version: "v1alpha1", Resource: "gametemplates"}
	gameServerGVR   = schema.GroupVersionResource{Group: "kestrel.gg", Version: "v1alpha1", Resource: "gameservers"}
)

// TestGameServer_OperatorMaterializesChildren — apply a tiny template
// + a GameServer that references it. The operator must produce a
// StatefulSet, Service, and PVC. We do NOT wait for pods to reach
// Ready — that requires a real game image and the kind node may not
// be able to pull large external images. The test asserts the operator
// shaped the right cluster objects, which is the contract that
// matters at the operator layer.
func TestGameServer_OperatorMaterializesChildren(t *testing.T) {
	ctx := context.Background()
	ns := "kestrel-games"

	// Use a busybox-based "fake game" so the operator can construct a
	// pod spec that won't fail to render. Image is never actually
	// pulled here — we don't wait for the pod.
	tmplName := "e2e-busybox"
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kestrel.gg/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E busybox",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "noop", "containerPort": int64(12345), "advertise": true, "protocol": "TCP"},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create template: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})

	gsName := "e2e-test-srv"
	gs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kestrel.gg/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": gsName, "namespace": ns},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": tmplName},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Create(ctx, gs, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create gameserver: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
			Delete(context.Background(), gsName, metav1.DeleteOptions{})
	})

	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		// StatefulSet
		if _, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gsName, metav1.GetOptions{}); err != nil {
			return false, "ss: " + err.Error()
		}
		// Service
		if _, err := envInstance.K8s.CoreV1().Services(ns).Get(ctx, gsName, metav1.GetOptions{}); err != nil {
			return false, "svc: " + err.Error()
		}
		// PVC named <gs>-data
		if _, err := envInstance.K8s.CoreV1().PersistentVolumeClaims(ns).Get(ctx, gsName+"-data", metav1.GetOptions{}); err != nil {
			return false, "pvc: " + err.Error()
		}
		return true, ""
	})

	// Sanity-check the StatefulSet's pod template has the agent sidecar.
	ss, err := envInstance.K8s.AppsV1().StatefulSets(ns).Get(ctx, gsName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get statefulset: %v", err)
	}
	names := []string{}
	for _, c := range ss.Spec.Template.Spec.Containers {
		names = append(names, c.Name)
	}
	if !contains(names, "agent") {
		t.Errorf("sidecar missing — container names: %s", strings.Join(names, ","))
	}
	if !contains(names, "game") {
		t.Errorf("game container missing — container names: %s", strings.Join(names, ","))
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
