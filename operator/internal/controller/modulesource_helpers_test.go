package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

func TestRefreshInterval(t *testing.T) {
	t.Run("zero falls back to default 1h", func(t *testing.T) {
		src := &kestrelv1alpha1.ModuleSource{}
		if got := refreshInterval(src); got != defaultRefreshInterval {
			t.Fatalf("got %v want %v", got, defaultRefreshInterval)
		}
	})

	t.Run("below the floor clamps to minimum", func(t *testing.T) {
		src := &kestrelv1alpha1.ModuleSource{
			Spec: kestrelv1alpha1.ModuleSourceSpec{
				RefreshInterval: metav1.Duration{Duration: 10 * time.Second},
			},
		}
		if got := refreshInterval(src); got != minRefreshInterval {
			t.Fatalf("got %v want %v", got, minRefreshInterval)
		}
	})

	t.Run("normal value passes through", func(t *testing.T) {
		src := &kestrelv1alpha1.ModuleSource{
			Spec: kestrelv1alpha1.ModuleSourceSpec{
				RefreshInterval: metav1.Duration{Duration: 5 * time.Minute},
			},
		}
		if got := refreshInterval(src); got != 5*time.Minute {
			t.Fatalf("got %v want 5m", got)
		}
	})
}

func TestModuleSourceReconciler_NewClient_DefaultPath(t *testing.T) {
	r := &ModuleSourceReconciler{}
	c := r.newClient(nil, true)
	if c == nil {
		t.Fatal("expected non-nil client from default path")
	}
}
