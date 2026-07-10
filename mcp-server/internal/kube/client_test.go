package kube

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// gvrToListKindMap gives the fake dynamic client an explicit GVR->ListKind
// mapping for every registered CRD, so List() doesn't depend on the fake
// client's pluralization guesser.
func gvrToListKindMap() map[schema.GroupVersionResource]string {
	m := make(map[schema.GroupVersionResource]string, len(CRDKinds))
	for kind, k := range CRDKinds {
		m[k.GVR] = kind + "List"
	}
	return m
}

func newUnstructuredCRD(kind, ns, name string, status map[string]any) *unstructured.Unstructured {
	metadata := map[string]any{"name": name}
	if ns != "" {
		metadata["namespace"] = ns
	}
	obj := map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       kind,
		"metadata":   metadata,
	}
	if status != nil {
		obj["status"] = status
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestListCRD(t *testing.T) {
	ctx := context.Background()
	gs := newUnstructuredCRD("GameServer", "games", "my-server", map[string]any{"phase": "Running"})
	c := &Client{Scheme: NewScheme()}
	c.dynamic = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(c.Scheme, gvrToListKindMap(), gs)

	list, err := c.ListCRD(ctx, "GameServer", "games", "")
	if err != nil {
		t.Fatalf("ListCRD: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(list.Items))
	}
	if list.Items[0].GetName() != "my-server" {
		t.Errorf("want name my-server, got %q", list.Items[0].GetName())
	}
}

func TestListCRDUnknownKind(t *testing.T) {
	c := &Client{Scheme: NewScheme()}
	c.dynamic = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(c.Scheme, gvrToListKindMap())

	_, err := c.ListCRD(context.Background(), "NotAKind", "", "")
	if !errors.Is(err, errUnknownKind) {
		t.Fatalf("want errUnknownKind, got %v", err)
	}
}

func TestGetCRD(t *testing.T) {
	ctx := context.Background()
	gs := newUnstructuredCRD("GameServer", "games", "my-server", map[string]any{"phase": "Running"})
	c := &Client{Scheme: NewScheme()}
	c.dynamic = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(c.Scheme, gvrToListKindMap(), gs)

	obj, err := c.GetCRD(ctx, "GameServer", "games", "my-server")
	if err != nil {
		t.Fatalf("GetCRD: %v", err)
	}
	if obj.GetName() != "my-server" {
		t.Errorf("want name my-server, got %q", obj.GetName())
	}

	if _, err := c.GetCRD(ctx, "GameServer", "", "my-server"); err == nil {
		t.Error("want error when namespace omitted for a namespaced kind")
	}
}

func TestGetCRDClusterScoped(t *testing.T) {
	ctx := context.Background()
	tmpl := newUnstructuredCRD("GameTemplate", "", "minecraft-java", nil)
	c := &Client{Scheme: NewScheme()}
	c.dynamic = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(c.Scheme, gvrToListKindMap(), tmpl)

	obj, err := c.GetCRD(ctx, "GameTemplate", "", "minecraft-java")
	if err != nil {
		t.Fatalf("GetCRD cluster-scoped: %v", err)
	}
	if obj.GetName() != "minecraft-java" {
		t.Errorf("want name minecraft-java, got %q", obj.GetName())
	}
}

func TestListPodsAndGetPod(t *testing.T) {
	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server-0", Namespace: "games"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	c := &Client{typed: k8sfake.NewSimpleClientset(pod), Scheme: NewScheme()}

	list, err := c.ListPods(ctx, "games", "")
	if err != nil {
		t.Fatalf("ListPods: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("want 1 pod, got %d", len(list.Items))
	}

	got, err := c.GetPod(ctx, "games", "my-server-0")
	if err != nil {
		t.Fatalf("GetPod: %v", err)
	}
	if got.Status.Phase != corev1.PodRunning {
		t.Errorf("want phase Running, got %q", got.Status.Phase)
	}

	if _, err := c.GetPod(ctx, "games", "does-not-exist"); err == nil {
		t.Error("want error for missing pod")
	}
}

func TestListEvents(t *testing.T) {
	ctx := context.Background()
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "ev1", Namespace: "games"},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod", Name: "my-server-0",
		},
		Reason:  "BackOff",
		Message: "back-off restarting failed container",
	}
	c := &Client{typed: k8sfake.NewSimpleClientset(ev), Scheme: NewScheme()}

	list, err := c.ListEvents(ctx, "games", "")
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("want 1 event, got %d", len(list.Items))
	}
	if list.Items[0].Reason != "BackOff" {
		t.Errorf("want reason BackOff, got %q", list.Items[0].Reason)
	}
}

func TestPodLogs(t *testing.T) {
	ctx := context.Background()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "my-server-0", Namespace: "games"}}
	c := &Client{typed: k8sfake.NewSimpleClientset(pod), Scheme: NewScheme()}

	logs, err := c.PodLogs(ctx, "games", "my-server-0", "", 0, false)
	if err != nil {
		t.Fatalf("PodLogs: %v", err)
	}
	if !strings.Contains(logs, "fake logs") {
		t.Errorf("want fake clientset's canned log text, got %q", logs)
	}
}

func TestNewScheme(t *testing.T) {
	scheme := NewScheme()
	for kind := range CRDKinds {
		gvk := gameplaneGroupVersion.WithKind(kind)
		if !scheme.Recognizes(gvk) {
			t.Errorf("scheme does not recognize %s", gvk)
		}
		listGVK := gameplaneGroupVersion.WithKind(kind + "List")
		if !scheme.Recognizes(listGVK) {
			t.Errorf("scheme does not recognize %s", listGVK)
		}
	}
}
