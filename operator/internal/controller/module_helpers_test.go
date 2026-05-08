package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
	"github.com/kestrel-gg/kestrel/operator/internal/oci"
)

func TestByCatalogName(t *testing.T) {
	entries := []kestrelv1alpha1.ModuleEntry{
		{Name: "alpha"},
		{Name: "beta"},
	}
	if got := byCatalogName(entries, "beta"); got == nil || got.Name != "beta" {
		t.Fatalf("got %+v", got)
	}
	if got := byCatalogName(entries, "missing"); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	if got := byCatalogName(nil, "x"); got != nil {
		t.Fatal("nil entries should return nil")
	}
}

func TestOwnedBy(t *testing.T) {
	owner := &kestrelv1alpha1.Module{
		ObjectMeta: metav1.ObjectMeta{
			UID:  types.UID("uid-1"),
			Name: "alpha",
		},
	}
	t.Run("matching ref", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Module", UID: "uid-1"},
				},
			},
		}
		if !ownedBy(obj, owner) {
			t.Fatal("expected match")
		}
	})
	t.Run("wrong UID", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Module", UID: "uid-other"},
				},
			},
		}
		if ownedBy(obj, owner) {
			t.Fatal("uid mismatch should not match")
		}
	})
	t.Run("wrong Kind", func(t *testing.T) {
		obj := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Other", UID: "uid-1"},
				},
			},
		}
		if ownedBy(obj, owner) {
			t.Fatal("Kind mismatch should not match")
		}
	})
	t.Run("no owner refs", func(t *testing.T) {
		obj := &corev1.ConfigMap{}
		if ownedBy(obj, owner) {
			t.Fatal("empty refs should not match")
		}
	})
}

func TestModuleReconciler_NewClient_DefaultPath(t *testing.T) {
	// When NewClient hook is unset, newClient must build a real oci client.
	r := &ModuleReconciler{}
	c := r.newClient(nil, false)
	if c == nil {
		t.Fatal("expected non-nil client from default path")
	}
	_ = oci.CredentialFunc(nil) // silence unused-import in case of refactor
}
