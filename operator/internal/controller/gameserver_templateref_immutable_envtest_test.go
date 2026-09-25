//go:build envtest

package controller

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// TestGameServer_TemplateRefIsImmutable — F-047: repointing spec.templateRef
// at another GameTemplate leaves the StatefulSet wedged with only an
// operator-log error, because the resulting selector/label diff is a
// Forbidden update at the apiserver. The CRD now rejects the change itself
// at admission (CEL "self == oldSelf"), so the user gets a clear error
// instead of a silently-wedged server.
func TestGameServer_TemplateRefIsImmutable(t *testing.T) {
	ns := newNamespace(t)
	// No reconciler needed: this is pure API-server admission behavior.
	startMgr(t, ns)

	tmplA := buildGameTemplate(uniqueName("minecraft-a"))
	if err := k8sClient.Create(context.Background(), tmplA); err != nil {
		t.Fatalf("create template a: %v", err)
	}
	deleteCleanup(t, tmplA)

	tmplB := buildGameTemplate(uniqueName("minecraft-b"))
	if err := k8sClient.Create(context.Background(), tmplB); err != nil {
		t.Fatalf("create template b: %v", err)
	}
	deleteCleanup(t, tmplB)

	gs := buildGameServer(ns, "smp", tmplA.Name)
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}

	if err := k8sClient.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: "smp"}, gs); err != nil {
		t.Fatalf("re-get gameserver: %v", err)
	}
	gs.Spec.TemplateRef.Name = tmplB.Name
	err := k8sClient.Update(context.Background(), gs)
	if err == nil {
		t.Fatal("expected update to be rejected, but templateRef was changed")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected an Invalid (admission) error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "templateRef is immutable") {
		t.Fatalf("expected the CEL message in the error, got: %v", err)
	}

	// The spec was not changed server-side.
	if err := k8sClient.Get(context.Background(),
		types.NamespacedName{Namespace: ns, Name: "smp"}, gs); err != nil {
		t.Fatalf("re-get gameserver: %v", err)
	}
	if gs.Spec.TemplateRef.Name != tmplA.Name {
		t.Fatalf("templateRef = %q, want unchanged %q", gs.Spec.TemplateRef.Name, tmplA.Name)
	}
}
