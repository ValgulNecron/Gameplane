//go:build envtest

package controller

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// withNetworkCaptureReconciler wires a NetworkCaptureReconciler using the
// shared manager, matching the withXReconciler convention in
// helpers_envtest_test.go. stub lets each test control the sidecar's
// reported behavior without a real capture-sidecar HTTPS endpoint.
func withNetworkCaptureReconciler(stub SidecarCaptureClient) setupReconciler {
	return func(mgr manager.Manager) error {
		return (&NetworkCaptureReconciler{
			Client:              mgr.GetClient(),
			Scheme:              mgr.GetScheme(),
			SidecarClient:       stub,
			CaptureEnabled:      true,
			CaptureSidecarImage: "ghcr.io/valgulnecron/gameplane/capture-sidecar:test",
		}).SetupWithManager(mgr)
	}
}

// buildCapturePod constructs the minimal Pod fixture the reconciler needs
// for ephemeral-container injection: an "app.kubernetes.io/instance" label
// (read by mapPodToNetworkCaptures) and the "captures" emptyDir volume that
// gameserver_controller.go provisions unconditionally on every real game
// pod template — matching the real name, not a fixture-only stand-in.
func buildCapturePod(ns, gsName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-0", gsName),
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/instance": gsName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "game",
					Image: "test-image:latest",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "captures",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "agent-tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: gsName + "-agent-tls",
						},
					},
				},
			},
		},
	}
}

// buildCaptureGameServer constructs a minimal GameServer with capture enabled.
func buildCaptureGameServer(ns, name string) *gameplanev1alpha1.GameServer {
	return &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: gameplanev1alpha1.GameServerSpec{
			TemplateRef: gameplanev1alpha1.GameTemplateRef{Name: "test-template"},
			Capture: &gameplanev1alpha1.CaptureConfiguration{
				Enabled:          true,
				RetentionSeconds: ptrTo(int32(86400)),
			},
		},
		Status: gameplanev1alpha1.GameServerStatus{
			Phase: gameplanev1alpha1.GameServerPhaseRunning,
		},
	}
}

// buildNetworkCapture constructs a Pending NetworkCapture targeting gsName.
func buildNetworkCapture(ns, name, gsName, filter string) *gameplanev1alpha1.NetworkCapture {
	return &gameplanev1alpha1.NetworkCapture{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: gameplanev1alpha1.NetworkCaptureSpec{
			ServerRef:   corev1.LocalObjectReference{Name: gsName},
			Filter:      ptrTo(filter),
			MaxDuration: &metav1.Duration{Duration: 5 * time.Minute},
			MaxSize:     resource.NewQuantity(5368709120, resource.BinarySI),
		},
		Status: gameplanev1alpha1.NetworkCaptureStatus{
			Phase: gameplanev1alpha1.CapturePhasePending,
		},
	}
}

func getNetworkCapture(t *testing.T, ns, name string) *gameplanev1alpha1.NetworkCapture {
	t.Helper()
	var nc gameplanev1alpha1.NetworkCapture
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: name}, &nc); err != nil {
		t.Fatalf("get network capture %s/%s: %v", ns, name, err)
	}
	return &nc
}

func TestNetworkCapture_PendingToRunningToCompleted(t *testing.T) {
	ns := newNamespace(t)

	var mu sync.Mutex
	started := false
	stub := &StubSidecarClient{
		startCaptureFn: func(_ context.Context, _, _, _ string, _ *string, _, _ int64) error {
			mu.Lock()
			started = true
			mu.Unlock()
			return nil
		},
		getCaptureStatusFn: func(_ context.Context, _, _, _ string) (string, int64, int64, string, error) {
			mu.Lock()
			defer mu.Unlock()
			if !started {
				return "", 0, 0, "", fmt.Errorf("capture not found")
			}
			return "completed", 100, 5242880, "capture finished", nil
		},
	}

	startMgr(t, ns, withNetworkCaptureReconciler(stub))

	gs := buildCaptureGameServer(ns, "test-server")
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildCapturePod(ns, "test-server")); err != nil {
		t.Fatalf("create pod: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildNetworkCapture(ns, "cap-test-001", "test-server", "tcp port 25565")); err != nil {
		t.Fatalf("create network capture: %v", err)
	}

	eventually(t, func() (bool, string) {
		nc := getNetworkCapture(t, ns, "cap-test-001")
		if nc.Status.Phase != gameplanev1alpha1.CapturePhaseCompleted {
			return false, fmt.Sprintf("phase = %s, want Completed", nc.Status.Phase)
		}
		if nc.Status.StartTime == nil {
			return false, "StartTime not set"
		}
		if nc.Status.CompletionTime == nil {
			return false, "CompletionTime not set"
		}
		if nc.Status.BytesWritten == nil || nc.Status.BytesWritten.Value() != 5242880 {
			return false, fmt.Sprintf("BytesWritten = %v, want 5242880", nc.Status.BytesWritten)
		}
		if len(nc.OwnerReferences) == 0 {
			return false, "OwnerReferences not backfilled"
		}
		return true, ""
	})

	// Ephemeral container must reference the real "captures" volume, not a
	// fixture-only name, and must use the reconciler's configured image.
	var pod corev1.Pod
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: ns, Name: "test-server-0"}, &pod); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if len(pod.Spec.EphemeralContainers) != 1 {
		t.Fatalf("EphemeralContainers = %d, want 1", len(pod.Spec.EphemeralContainers))
	}
	ec := pod.Spec.EphemeralContainers[0]
	if ec.Image != "ghcr.io/valgulnecron/gameplane/capture-sidecar:test" {
		t.Errorf("ephemeral container image = %q, want the configured CaptureSidecarImage", ec.Image)
	}
	foundVolume := false
	for _, vm := range ec.VolumeMounts {
		if vm.Name == "captures" {
			foundVolume = true
		}
		if vm.Name == "capture-data" {
			t.Errorf("ephemeral container mounts nonexistent volume %q", vm.Name)
		}
	}
	if !foundVolume {
		t.Errorf("ephemeral container does not mount the real \"captures\" volume")
	}
}

func TestNetworkCapture_ConcurrencyRejection(t *testing.T) {
	ns := newNamespace(t)

	var mu sync.Mutex
	started := false
	stub := &StubSidecarClient{
		startCaptureFn: func(_ context.Context, _, _, _ string, _ *string, _, _ int64) error {
			mu.Lock()
			started = true
			mu.Unlock()
			return nil
		},
		getCaptureStatusFn: func(_ context.Context, _, _, _ string) (string, int64, int64, string, error) {
			mu.Lock()
			defer mu.Unlock()
			if !started {
				return "", 0, 0, "", fmt.Errorf("capture not found")
			}
			return "running", 50, 2621440, "capture running", nil
		},
	}

	startMgr(t, ns, withNetworkCaptureReconciler(stub))

	gs := buildCaptureGameServer(ns, "test-server-conc")
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildCapturePod(ns, "test-server-conc")); err != nil {
		t.Fatalf("create pod: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildNetworkCapture(ns, "cap-conc-001", "test-server-conc", "tcp port 25565")); err != nil {
		t.Fatalf("create first network capture: %v", err)
	}

	eventually(t, func() (bool, string) {
		nc := getNetworkCapture(t, ns, "cap-conc-001")
		if nc.Status.Phase != gameplanev1alpha1.CapturePhaseRunning {
			return false, fmt.Sprintf("phase = %s, want Running", nc.Status.Phase)
		}
		return true, ""
	})

	if err := k8sClient.Create(context.Background(), buildNetworkCapture(ns, "cap-conc-002", "test-server-conc", "tcp port 25566")); err != nil {
		t.Fatalf("create second network capture: %v", err)
	}

	eventually(t, func() (bool, string) {
		nc := getNetworkCapture(t, ns, "cap-conc-002")
		if nc.Status.Phase != gameplanev1alpha1.CapturePhaseFailed {
			return false, fmt.Sprintf("phase = %s, want Failed", nc.Status.Phase)
		}
		if nc.Status.Message == "" {
			return false, "no failure message recorded"
		}
		return true, ""
	})

	// The first capture must remain Running — rejection of the second must
	// not disturb it.
	first := getNetworkCapture(t, ns, "cap-conc-001")
	if first.Status.Phase != gameplanev1alpha1.CapturePhaseRunning {
		t.Errorf("first capture phase = %s, want it to remain Running", first.Status.Phase)
	}
}

// TestNetworkCapture_UserStopCallsSidecar verifies that a user-requested
// stop (status patched to Completed with the API's "stopped by user
// request" message, mirroring api/internal/kube/capture.go's
// StopNetworkCapture) actually tells the sidecar to stop capturing exactly
// once, instead of leaving it to keep writing packets until it hits its own
// max-duration/max-size limit.
func TestNetworkCapture_UserStopCallsSidecar(t *testing.T) {
	ns := newNamespace(t)

	var mu sync.Mutex
	started := false
	stopCalls := 0
	stub := &StubSidecarClient{
		startCaptureFn: func(_ context.Context, _, _, _ string, _ *string, _, _ int64) error {
			mu.Lock()
			started = true
			mu.Unlock()
			return nil
		},
		stopCaptureFn: func(_ context.Context, _, _, _ string) error {
			mu.Lock()
			stopCalls++
			mu.Unlock()
			return nil
		},
		getCaptureStatusFn: func(_ context.Context, _, _, _ string) (string, int64, int64, string, error) {
			mu.Lock()
			defer mu.Unlock()
			if !started {
				return "", 0, 0, "", fmt.Errorf("capture not found")
			}
			return "running", 10, 1024, "capture running", nil
		},
	}

	startMgr(t, ns, withNetworkCaptureReconciler(stub))

	gs := buildCaptureGameServer(ns, "test-server-stop")
	if err := k8sClient.Create(context.Background(), gs); err != nil {
		t.Fatalf("create gameserver: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildCapturePod(ns, "test-server-stop")); err != nil {
		t.Fatalf("create pod: %v", err)
	}
	if err := k8sClient.Create(context.Background(), buildNetworkCapture(ns, "cap-stop-001", "test-server-stop", "tcp port 25565")); err != nil {
		t.Fatalf("create network capture: %v", err)
	}

	eventually(t, func() (bool, string) {
		nc := getNetworkCapture(t, ns, "cap-stop-001")
		if nc.Status.Phase != gameplanev1alpha1.CapturePhaseRunning {
			return false, fmt.Sprintf("phase = %s, want Running", nc.Status.Phase)
		}
		return true, ""
	})

	// Emulate the API's StopNetworkCapture: patch status directly to
	// Completed with its exact message, bypassing the reconciler.
	nc := getNetworkCapture(t, ns, "cap-stop-001")
	now := metav1.NewTime(time.Now().UTC())
	nc.Status.Phase = gameplanev1alpha1.CapturePhaseCompleted
	nc.Status.CompletionTime = &now
	nc.Status.Message = "stopped by user request"
	if err := k8sClient.Status().Update(context.Background(), nc); err != nil {
		t.Fatalf("patch capture to user-stopped: %v", err)
	}

	eventually(t, func() (bool, string) {
		mu.Lock()
		calls := stopCalls
		mu.Unlock()
		if calls < 1 {
			return false, "sidecar StopCapture not called yet"
		}
		return true, ""
	})

	// Give the reconciler a further moment to settle, then confirm it fired
	// exactly once even though the terminal phase keeps getting reconciled
	// (e.g. via the Pod watch).
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	calls := stopCalls
	mu.Unlock()
	if calls != 1 {
		t.Errorf("sidecar StopCapture called %d times, want exactly 1", calls)
	}
}

// StubSidecarClient implements SidecarCaptureClient for testing.
type StubSidecarClient struct {
	startCaptureFn     func(ctx context.Context, ns, server, id string, filter *string, dur, size int64) error
	stopCaptureFn      func(ctx context.Context, ns, server, id string) error
	getCaptureStatusFn func(ctx context.Context, ns, server, id string) (string, int64, int64, string, error)
}

func (s *StubSidecarClient) StartCapture(ctx context.Context, namespace, serverName, captureID string, filter *string, maxDurationSeconds, maxSizeBytes int64) error {
	if s.startCaptureFn != nil {
		return s.startCaptureFn(ctx, namespace, serverName, captureID, filter, maxDurationSeconds, maxSizeBytes)
	}
	return nil
}

func (s *StubSidecarClient) StopCapture(ctx context.Context, namespace, serverName, captureID string) error {
	if s.stopCaptureFn != nil {
		return s.stopCaptureFn(ctx, namespace, serverName, captureID)
	}
	return nil
}

func (s *StubSidecarClient) GetCaptureStatus(ctx context.Context, namespace, serverName, captureID string) (string, int64, int64, string, error) {
	if s.getCaptureStatusFn != nil {
		return s.getCaptureStatusFn(ctx, namespace, serverName, captureID)
	}
	return "running", 0, 0, "", nil
}
