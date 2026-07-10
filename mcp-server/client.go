package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// gameplaneGroupVersion is the Gameplane CRD API group, mirroring
// operator/api/v1alpha1.GroupVersion. Redeclared here (rather than imported)
// so this module stays standalone, like its audit-syslog-bridge and
// telemetry-receiver siblings: no dependency on the operator module's Go
// types, no coupling to their release cadence.
var gameplaneGroupVersion = schema.GroupVersion{Group: "gameplane.local", Version: "v1alpha1"}

// errUnknownKind is returned when a tool is asked to operate on a kind name
// outside the fixed set of Gameplane CRDs this server knows how to read.
var errUnknownKind = errors.New("unknown Gameplane resource kind")

// crdKind describes one of the 7 Gameplane CRDs this server can list/get.
type crdKind struct {
	gvr        schema.GroupVersionResource
	namespaced bool
}

// crdKinds is the fixed, read-only registry of Gameplane CRDs. Keyed by the
// CamelCase Kind name a caller passes in (e.g. "GameServer"), so tool
// arguments read the same as `kubectl get <kind>`. Scope (namespaced vs
// cluster-scoped) mirrors the +kubebuilder:resource:scope markers in
// operator/api/v1alpha1/*_types.go and the RBAC comments in
// charts/gameplane/templates/operator.yaml.
var crdKinds = map[string]crdKind{
	"GameServer":     {gvr: gameplaneGroupVersion.WithResource("gameservers"), namespaced: true},
	"GameTemplate":   {gvr: gameplaneGroupVersion.WithResource("gametemplates"), namespaced: false},
	"Backup":         {gvr: gameplaneGroupVersion.WithResource("backups"), namespaced: true},
	"BackupSchedule": {gvr: gameplaneGroupVersion.WithResource("backupschedules"), namespaced: true},
	"Restore":        {gvr: gameplaneGroupVersion.WithResource("restores"), namespaced: true},
	"Module":         {gvr: gameplaneGroupVersion.WithResource("modules"), namespaced: false},
	"ModuleSource":   {gvr: gameplaneGroupVersion.WithResource("modulesources"), namespaced: false},
}

// maxLogBytes bounds a single pod-logs response so a runaway container can't
// stream an unbounded amount of text back through the tool result.
const maxLogBytes = 256 << 10

// defaultTailLines is applied when a get_pod_logs call doesn't specify one.
const defaultTailLines = 200

// maxTailLines caps an explicit request so a caller can't ask for the entire
// log history of a long-lived pod.
const maxTailLines = 5000

// newScheme builds a runtime.Scheme that knows the corev1 types (for the
// typed Pod/Event reads) plus the 7 Gameplane CRD kinds, registered against
// unstructured.Unstructured/UnstructuredList. Tools in this server use the
// dynamic client directly and don't strictly need a scheme to do so, but
// building one here keeps the process's view of "what a Gameplane resource
// is" explicit and gives future typed-decode paths (or fake-client tests) a
// scheme to work against, without importing operator/api/v1alpha1's
// generated types (see the comment on gameplaneGroupVersion).
func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	// Best-effort: corev1 registration into client-go's own scheme package
	// cannot fail for a fixed set of built-in types.
	_ = clientgoscheme.AddToScheme(scheme)

	for kind := range crdKinds {
		gvk := gameplaneGroupVersion.WithKind(kind)
		if !scheme.Recognizes(gvk) {
			scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		}
		listGVK := gameplaneGroupVersion.WithKind(kind + "List")
		if !scheme.Recognizes(listGVK) {
			scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})
		}
	}
	metav1.AddToGroupVersion(scheme, gameplaneGroupVersion)
	return scheme
}

// Client is a strictly read-only view of a Kubernetes cluster: it exposes
// List/Get-shaped methods only. There is deliberately no Create/Update/
// Delete/Patch/Apply method anywhere on this type — that is the structural
// half of this server's read-only guarantee (main_test.go asserts the other
// half: no tool is registered that could call one, even if this type grew
// one later).
type Client struct {
	Typed   kubernetes.Interface
	Dynamic dynamic.Interface
	Scheme  *runtime.Scheme
}

// newClient builds a Client from a rest.Config, e.g. the one returned by
// ctrl.GetConfig() (in-cluster config, falling back to KUBECONFIG/
// ~/.kube/config for local runs against a dev cluster).
func newClient(cfg *rest.Config) (*Client, error) {
	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build typed clientset: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build dynamic client: %w", err)
	}
	return &Client{Typed: typed, Dynamic: dyn, Scheme: newScheme()}, nil
}

// resourceInterface returns the dynamic client scoped to kind's GVR (and to
// namespace, when kind is namespaced and namespace is non-empty).
func (c *Client) resourceInterface(kind, namespace string) (dynamic.ResourceInterface, crdKind, error) {
	k, ok := crdKinds[kind]
	if !ok {
		return nil, crdKind{}, fmt.Errorf("%w: %q (known: %s)", errUnknownKind, kind, knownKindsList())
	}
	ri := c.Dynamic.Resource(k.gvr)
	if k.namespaced && namespace != "" {
		return ri.Namespace(namespace), k, nil
	}
	return ri, k, nil
}

func knownKindsList() string {
	out := ""
	for kind := range crdKinds {
		if out != "" {
			out += ", "
		}
		out += kind
	}
	return out
}

// ListCRD lists objects of a Gameplane CRD kind. namespace is ignored for
// cluster-scoped kinds; an empty namespace on a namespaced kind lists across
// all namespaces. labelSelector is passed through verbatim to the API server.
func (c *Client) ListCRD(ctx context.Context, kind, namespace, labelSelector string) (*unstructured.UnstructuredList, error) {
	ri, _, err := c.resourceInterface(kind, namespace)
	if err != nil {
		return nil, err
	}
	list, err := ri.List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", kind, err)
	}
	return list, nil
}

// GetCRD fetches a single Gameplane CRD object by name. namespace is
// required for namespaced kinds and ignored for cluster-scoped ones.
func (c *Client) GetCRD(ctx context.Context, kind, namespace, name string) (*unstructured.Unstructured, error) {
	k, ok := crdKinds[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %q (known: %s)", errUnknownKind, kind, knownKindsList())
	}
	if k.namespaced && namespace == "" {
		return nil, fmt.Errorf("%s is namespace-scoped: namespace is required", kind)
	}
	ri, _, err := c.resourceInterface(kind, namespace)
	if err != nil {
		return nil, err
	}
	obj, err := ri.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s %s/%s: %w", kind, namespace, name, err)
	}
	return obj, nil
}

// ListPods lists core Pods in namespace (empty namespace lists across all
// namespaces), optionally filtered by a label selector.
func (c *Client) ListPods(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
	list, err := c.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods in %q: %w", namespace, err)
	}
	return list, nil
}

// GetPod fetches a single Pod by namespace and name.
func (c *Client) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	pod, err := c.Typed.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod %s/%s: %w", namespace, name, err)
	}
	return pod, nil
}

// ListEvents lists core Events in namespace (empty namespace lists across
// all namespaces), optionally filtered by a field selector (e.g.
// "involvedObject.name=my-server,involvedObject.kind=GameServer").
func (c *Client) ListEvents(ctx context.Context, namespace, fieldSelector string) (*corev1.EventList, error) {
	list, err := c.Typed.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return nil, fmt.Errorf("list events in %q: %w", namespace, err)
	}
	return list, nil
}

// PodLogs fetches (a bounded tail of) a pod's logs. tailLines <= 0 applies
// defaultTailLines; a request above maxTailLines is capped rather than
// rejected, so an overly broad ask degrades gracefully instead of failing.
func (c *Client) PodLogs(ctx context.Context, namespace, pod, container string, tailLines int64, previous bool) (string, error) {
	tail := tailLines
	switch {
	case tail <= 0:
		tail = defaultTailLines
	case tail > maxTailLines:
		tail = maxTailLines
	}
	opts := &corev1.PodLogOptions{
		Container: container,
		Previous:  previous,
		TailLines: &tail,
	}
	stream, err := c.Typed.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("stream logs for pod %s/%s: %w", namespace, pod, err)
	}
	defer func() { _ = stream.Close() }()

	data, err := io.ReadAll(io.LimitReader(stream, maxLogBytes))
	if err != nil {
		return "", fmt.Errorf("read logs for pod %s/%s: %w", namespace, pod, err)
	}
	return string(data), nil
}
