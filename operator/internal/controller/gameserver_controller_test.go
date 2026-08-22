package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := gameplanev1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("gameplane scheme: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("core scheme: %v", err)
	}
	return s
}

// TestFindAddressConflict_NoConflict tests the case where no other server
// requests or holds the address.
func TestFindAddressConflict_NoConflict(t *testing.T) {
	s := testScheme(t)
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}
	other := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "beta", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.6", // Different address
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs, other).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	conflict, err := r.findAddressConflict(context.Background(), gs)
	if err != nil {
		t.Fatalf("findAddressConflict: %v", err)
	}
	if conflict != "" {
		t.Errorf("want no conflict, got %q", conflict)
	}
}

// TestFindAddressConflict_HoldingInStatus tests the case where another server
// already holds the requested address in its status.endpoints.
func TestFindAddressConflict_HoldingInStatus(t *testing.T) {
	s := testScheme(t)
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}
	other := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "beta", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
		Status: gameplanev1alpha1.GameServerStatus{
			Endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Host: "10.0.0.5", Port: 25565},
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs, other).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	conflict, err := r.findAddressConflict(context.Background(), gs)
	if err != nil {
		t.Fatalf("findAddressConflict: %v", err)
	}
	if conflict != "beta" {
		t.Errorf("want conflict 'beta', got %q", conflict)
	}
}

// TestFindAddressConflict_HoldingInStatusCrossNamespace tests the case where
// a server in a different namespace holds the address.
func TestFindAddressConflict_HoldingInStatusCrossNamespace(t *testing.T) {
	s := testScheme(t)
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}
	other := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "beta", Namespace: "other-ns"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
		Status: gameplanev1alpha1.GameServerStatus{
			Endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Host: "10.0.0.5", Port: 25565},
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs, other).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	conflict, err := r.findAddressConflict(context.Background(), gs)
	if err != nil {
		t.Fatalf("findAddressConflict: %v", err)
	}
	if conflict != "other-ns/beta" {
		t.Errorf("want conflict 'other-ns/beta', got %q", conflict)
	}
}

// TestFindAddressConflict_NoSelfConflict tests that a server doesn't report
// itself as a conflict even if it holds the address in status.
func TestFindAddressConflict_NoSelfConflict(t *testing.T) {
	s := testScheme(t)
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
		Status: gameplanev1alpha1.GameServerStatus{
			Endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Host: "10.0.0.5", Port: 25565},
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	conflict, err := r.findAddressConflict(context.Background(), gs)
	if err != nil {
		t.Fatalf("findAddressConflict: %v", err)
	}
	if conflict != "" {
		t.Errorf("want no self-conflict, got %q", conflict)
	}
}

// TestFindAddressConflict_TwoRequestingWithTiebreak tests that when two servers
// both request the same address, only the earlier-created one reports a conflict.
// The tiebreak is based on creation timestamp (and namespace/name if timestamps match).
func TestFindAddressConflict_TwoRequestingWithTiebreak(t *testing.T) {
	s := testScheme(t)

	// Create two servers requesting the same address.
	// Use creationTimestamps to control the tiebreak.
	older := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "older",
			Namespace:         "games",
			CreationTimestamp: metav1.NewTime(metav1.Now().Time.Add(-10 * time.Second)),
		},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}
	newer := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "newer",
			Namespace:         "games",
			CreationTimestamp: metav1.NewTime(metav1.Now().Time),
		},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(older, newer).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	// The newer server (created later) should report the older one as a conflict.
	conflict, err := r.findAddressConflict(context.Background(), newer)
	if err != nil {
		t.Fatalf("findAddressConflict for newer: %v", err)
	}
	if conflict != "older" {
		t.Errorf("newer: want conflict 'older', got %q", conflict)
	}

	// The older server (created first) should NOT report a conflict.
	conflict, err = r.findAddressConflict(context.Background(), older)
	if err != nil {
		t.Fatalf("findAddressConflict for older: %v", err)
	}
	if conflict != "" {
		t.Errorf("older: want no conflict, got %q", conflict)
	}
}

// TestFindAddressConflict_TwoRequestingWithNamespaceTiebreak tests the tiebreak
// when two servers have the same creation timestamp in different namespaces.
func TestFindAddressConflict_TwoRequestingWithNamespaceTiebreak(t *testing.T) {
	s := testScheme(t)

	now := metav1.NewTime(metav1.Now().Time)
	// Server in namespace "aaa" (sorts first)
	earlier := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			Namespace:         "aaa",
			CreationTimestamp: now,
		},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}
	// Server in namespace "bbb" (sorts later)
	later := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			Namespace:         "bbb",
			CreationTimestamp: now,
		},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(earlier, later).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	// The one in "bbb" should report "aaa" as a conflict (namespace sort).
	conflict, err := r.findAddressConflict(context.Background(), later)
	if err != nil {
		t.Fatalf("findAddressConflict for later: %v", err)
	}
	if conflict != "aaa/test" {
		t.Errorf("later: want conflict 'aaa/test', got %q", conflict)
	}

	// The one in "aaa" should report no conflict.
	conflict, err = r.findAddressConflict(context.Background(), earlier)
	if err != nil {
		t.Fatalf("findAddressConflict for earlier: %v", err)
	}
	if conflict != "" {
		t.Errorf("earlier: want no conflict, got %q", conflict)
	}
}

// TestFindAddressConflict_TwoRequestingWithNameTiebreak tests the tiebreak
// when two servers have the same creation timestamp and namespace.
func TestFindAddressConflict_TwoRequestingWithNameTiebreak(t *testing.T) {
	s := testScheme(t)

	now := metav1.NewTime(metav1.Now().Time)
	// Server named "alpha" (sorts first)
	alpha := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "alpha",
			Namespace:         "games",
			CreationTimestamp: now,
		},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}
	// Server named "beta" (sorts later)
	beta := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "beta",
			Namespace:         "games",
			CreationTimestamp: now,
		},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(alpha, beta).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	// The one named "beta" should report "alpha" as a conflict (name sort).
	conflict, err := r.findAddressConflict(context.Background(), beta)
	if err != nil {
		t.Fatalf("findAddressConflict for beta: %v", err)
	}
	if conflict != "alpha" {
		t.Errorf("beta: want conflict 'alpha', got %q", conflict)
	}

	// The one named "alpha" should report no conflict.
	conflict, err = r.findAddressConflict(context.Background(), alpha)
	if err != nil {
		t.Fatalf("findAddressConflict for alpha: %v", err)
	}
	if conflict != "" {
		t.Errorf("alpha: want no conflict, got %q", conflict)
	}
}

// TestFindAddressConflict_UnrelatedAddress tests that an unrelated address
// doesn't cause a conflict.
func TestFindAddressConflict_UnrelatedAddress(t *testing.T) {
	s := testScheme(t)
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.5",
			},
		},
	}
	other := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "beta", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Address: "10.0.0.6",
			},
		},
		Status: gameplanev1alpha1.GameServerStatus{
			Endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Host: "10.0.0.6", Port: 25565},
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs, other).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	conflict, err := r.findAddressConflict(context.Background(), gs)
	if err != nil {
		t.Fatalf("findAddressConflict: %v", err)
	}
	if conflict != "" {
		t.Errorf("want no conflict, got %q", conflict)
	}
}

// TestFindAddressConflict_NoAddressRequested tests that servers with no address
// requested return no conflict.
func TestFindAddressConflict_NoAddressRequested(t *testing.T) {
	s := testScheme(t)
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "games"},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				// No address requested
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(gs).Build()
	r := &GameServerReconciler{Client: cl, APIReader: cl, Scheme: s}

	conflict, err := r.findAddressConflict(context.Background(), gs)
	if err != nil {
		t.Fatalf("findAddressConflict: %v", err)
	}
	if conflict != "" {
		t.Errorf("want no conflict, got %q", conflict)
	}
}
