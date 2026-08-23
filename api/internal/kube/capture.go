package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredpkg "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// GetGameServer fetches a GameServer by namespace and name, converted to
// the typed CRD so callers can read spec.capture/status.capture without
// hand-walking unstructured fields. Unlike GetServer (whose doc comment
// claims a nil-on-not-found contract it doesn't actually implement), this
// propagates the real error — callers classify it with
// k8s.io/apimachinery/pkg/api/errors.IsNotFound.
func (c *Client) GetGameServer(ctx context.Context, ns, name string) (*gameplanev1alpha1.GameServer, error) {
	u, err := c.GetServer(ctx, ns, name)
	if err != nil {
		return nil, fmt.Errorf("get game server %s: %w", name, err)
	}
	gs := &gameplanev1alpha1.GameServer{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, gs); err != nil {
		return nil, fmt.Errorf("get game server %s: convert from unstructured: %w", name, err)
	}
	return gs, nil
}

// CreateNetworkCapture creates a new NetworkCapture CRD owned by the target
// GameServer. serverUID carries the GameServer's UID so the created object
// gets a real ownerReference — without it, a NetworkCapture would outlive
// the GameServer it was taken against and never be garbage-collected with
// it. The object returned reflects what the apiserver actually persisted
// (server-assigned name/creationTimestamp/status), not the unsubmitted
// local copy.
func (c *Client) CreateNetworkCapture(
	ctx context.Context,
	ns,
	captureID,
	serverName string,
	serverUID types.UID,
	filter *string,
	maxDuration *metav1.Duration,
	maxSize *resource.Quantity,
	ttlSecondsAfterFinished *int32,
) (*gameplanev1alpha1.NetworkCapture, error) {
	ownerRefs := []metav1.OwnerReference{
		{
			APIVersion: "gameplane.local/v1alpha1",
			Kind:       "GameServer",
			Name:       serverName,
			UID:        serverUID,
		},
	}
	nc := &gameplanev1alpha1.NetworkCapture{
		ObjectMeta: metav1.ObjectMeta{
			Name:            captureID,
			Namespace:       ns,
			OwnerReferences: ownerRefs,
		},
		Spec: gameplanev1alpha1.NetworkCaptureSpec{
			ServerRef: corev1.LocalObjectReference{
				Name: serverName,
			},
			Filter:                  filter,
			MaxDuration:             maxDuration,
			MaxSize:                 maxSize,
			TTLSecondsAfterFinished: ttlSecondsAfterFinished,
		},
	}

	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(nc)
	if err != nil {
		return nil, fmt.Errorf("create network capture %s: convert to unstructured: %w", captureID, err)
	}

	created, err := c.Dynamic.Resource(GVRNetworkCapture).Namespace(ns).
		Create(ctx, &unstructuredpkg.Unstructured{Object: raw}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create network capture %s: %w", captureID, err)
	}

	// The CRD declares subresources.status, so the apiserver strips any
	// "status" field from the create body above — a NetworkCapture is
	// always born with status.phase=="" from Create alone. Set it to
	// Pending here via a UpdateStatus call so callers (and the reconciler,
	// which gates on Status.Phase) see a real phase immediately rather
	// than waiting on the first reconcile to notice an empty phase.
	if err := unstructuredpkg.SetNestedField(created.Object, string(gameplanev1alpha1.CapturePhasePending), "status", "phase"); err != nil {
		return nil, fmt.Errorf("create network capture %s: set pending phase: %w", captureID, err)
	}
	updated, err := c.Dynamic.Resource(GVRNetworkCapture).Namespace(ns).
		UpdateStatus(ctx, created, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create network capture %s: set pending phase: %w", captureID, err)
	}

	result := &gameplanev1alpha1.NetworkCapture{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(updated.Object, result); err != nil {
		return nil, fmt.Errorf("create network capture %s: convert from unstructured: %w", captureID, err)
	}
	return result, nil
}

// GetNetworkCapture fetches a NetworkCapture by namespace and name.
// Returns nil if not found.
func (c *Client) GetNetworkCapture(ctx context.Context, ns, name string) (*gameplanev1alpha1.NetworkCapture, error) {
	u, err := c.Dynamic.Resource(GVRNetworkCapture).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get network capture %s: %w", name, err)
	}

	nc := &gameplanev1alpha1.NetworkCapture{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, nc); err != nil {
		return nil, fmt.Errorf("get network capture %s: convert from unstructured: %w", name, err)
	}
	return nc, nil
}

// ListNetworkCaptures lists all NetworkCaptures for a GameServer in a namespace.
func (c *Client) ListNetworkCaptures(ctx context.Context, ns, serverName string) ([]gameplanev1alpha1.NetworkCapture, error) {
	ul, err := c.Dynamic.Resource(GVRNetworkCapture).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list network captures for server %s: %w", serverName, err)
	}

	var captures []gameplanev1alpha1.NetworkCapture
	for _, u := range ul.Items {
		nc := &gameplanev1alpha1.NetworkCapture{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, nc); err != nil {
			return nil, fmt.Errorf("list network captures for server %s: convert from unstructured: %w", serverName, err)
		}
		if nc.Spec.ServerRef.Name == serverName {
			captures = append(captures, *nc)
		}
	}
	return captures, nil
}

// StopNetworkCapture marks a running NetworkCapture Completed by patching
// its status subresource directly. Per the accepted design (research.md,
// "Capture lifecycle": "User stops capture -> API patches NetworkCapture to
// phase=Completed"), there is no spec-level "stop requested" field on
// NetworkCaptureSpec for the operator to reconcile — the API is the one
// that writes the terminal phase here. The sidecar and the operator's
// retention reconciler observe the phase transition and tear down the
// capture process / start the TTL clock respectively.
func (c *Client) StopNetworkCapture(ctx context.Context, ns, name string) (*gameplanev1alpha1.NetworkCapture, error) {
	now := metav1.NewTime(time.Now().UTC())
	patch := map[string]any{
		"status": map[string]any{
			"phase":          string(gameplanev1alpha1.CapturePhaseCompleted),
			"completionTime": now.Format(time.RFC3339),
			"message":        "stopped by user request",
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("stop network capture %s: marshal patch: %w", name, err)
	}

	u, err := c.Dynamic.Resource(GVRNetworkCapture).Namespace(ns).
		Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	if err != nil {
		return nil, fmt.Errorf("stop network capture %s: %w", name, err)
	}

	nc := &gameplanev1alpha1.NetworkCapture{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, nc); err != nil {
		return nil, fmt.Errorf("stop network capture %s: convert from unstructured: %w", name, err)
	}
	return nc, nil
}

// DeleteNetworkCapture deletes a NetworkCapture by namespace and name.
func (c *Client) DeleteNetworkCapture(ctx context.Context, ns, name string) error {
	if err := c.Dynamic.Resource(GVRNetworkCapture).Namespace(ns).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete network capture %s: %w", name, err)
		}
	}
	return nil
}
