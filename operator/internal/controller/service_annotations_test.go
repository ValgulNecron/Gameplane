package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// TestApplyManagedServiceAnnotations verifies the operator converges
// spec.networking.serviceAnnotations onto the Service: applying desired keys,
// pruning keys removed from spec, and never clobbering annotations written by
// other controllers.
func TestApplyManagedServiceAnnotations(t *testing.T) {
	svc := &corev1.Service{}

	applyManagedServiceAnnotations(svc, map[string]string{"a": "1", "b": "2"})
	if svc.Annotations["a"] != "1" || svc.Annotations["b"] != "2" {
		t.Fatalf("apply: %v", svc.Annotations)
	}
	if svc.Annotations[managedServiceAnnotationsKey] != "a,b" {
		t.Fatalf("sentinel = %q, want a,b", svc.Annotations[managedServiceAnnotationsKey])
	}

	// An annotation written by another controller (e.g. cloud LB).
	svc.Annotations["external/foo"] = "bar"

	// Drop "b" from spec: "a" kept, "b" pruned, "external/foo" preserved.
	applyManagedServiceAnnotations(svc, map[string]string{"a": "1"})
	if svc.Annotations["a"] != "1" {
		t.Errorf("a dropped: %v", svc.Annotations)
	}
	if _, ok := svc.Annotations["b"]; ok {
		t.Errorf("b not pruned: %v", svc.Annotations)
	}
	if svc.Annotations["external/foo"] != "bar" {
		t.Errorf("external annotation clobbered: %v", svc.Annotations)
	}
	if svc.Annotations[managedServiceAnnotationsKey] != "a" {
		t.Errorf("sentinel = %q, want a", svc.Annotations[managedServiceAnnotationsKey])
	}

	// Clear all managed annotations: sentinel removed, external preserved.
	applyManagedServiceAnnotations(svc, nil)
	if _, ok := svc.Annotations["a"]; ok {
		t.Errorf("a not pruned on clear: %v", svc.Annotations)
	}
	if _, ok := svc.Annotations[managedServiceAnnotationsKey]; ok {
		t.Errorf("sentinel not removed: %v", svc.Annotations)
	}
	if svc.Annotations["external/foo"] != "bar" {
		t.Errorf("external annotation clobbered on clear: %v", svc.Annotations)
	}
}
