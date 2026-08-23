package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/ValgulNecron/gameplane/api/internal/audit"
	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/rbac"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// These are unit tests for the enable/disable handlers only (T050); they
// run against a fake dynamic client, never envtest — the start/stop/list/
// get/download happy paths already have envtest coverage in
// capture_envtest_test.go.

// fakeCaptureClient builds a kube.Client wired to a fake dynamic client
// that knows about both GameServers and NetworkCaptures.
// fakeKubeClient (modules_fake_test.go) doesn't register the
// NetworkCapture GVR, and captureDisable's stop-active-captures path
// needs it.
func fakeCaptureClient(objs ...runtime.Object) *kube.Client {
	scm := runtime.NewScheme()
	gvkr := map[schema.GroupVersionResource]string{
		kube.GVRs["servers"]:   "GameServerList",
		kube.GVRNetworkCapture: "NetworkCaptureList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scm, gvkr, objs...)
	return &kube.Client{Dynamic: dyn, Typed: kubefake.NewClientset()}
}

// newCaptureServerObj builds a GameServer fixture with spec.capture.enabled
// set as requested.
func newCaptureServerObj(name string, captureEnabled bool) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameServer",
		"metadata":   map[string]any{"name": name, "namespace": scope.DefaultNamespace},
		"spec": map[string]any{
			"templateRef": map[string]any{"name": "minecraft"},
			"capture":     map[string]any{"enabled": captureEnabled},
		},
	}}
}

// newCaptureNetworkCapture builds a NetworkCapture fixture in the given
// phase, referencing serverName.
func newCaptureNetworkCapture(name, serverName, phase string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "NetworkCapture",
		"metadata":   map[string]any{"name": name, "namespace": scope.DefaultNamespace},
		"spec":       map[string]any{"serverRef": map[string]any{"name": serverName}},
		"status":     map[string]any{"phase": phase},
	}}
}

func newCaptureAuditor(t *testing.T) *audit.Auditor {
	t.Helper()
	return audit.New(newTestStore(t))
}

func mountCaptureTestRouter(k *kube.Client, cfg CaptureConfig, auditor *audit.Auditor) *chi.Mux {
	r := chi.NewRouter()
	reg := kube.NewRegistry(scope.DefaultCluster)
	reg.Set(scope.DefaultCluster, k)
	MountCapture(r, reg, auditor, cfg, "", "", "")
	return r
}

func decodeCaptureToggle(t *testing.T, rr *httptest.ResponseRecorder) captureToggleResp {
	t.Helper()
	var resp captureToggleResp
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rr.Body)
	}
	return resp
}

// TestCaptureEnable_Success verifies the happy path: 200, the canonical
// status.capture body shape, and that spec.capture.enabled was actually
// patched to true.
func TestCaptureEnable_Success(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("alpha", false))
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: true}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/alpha:capture-enable", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want 200; body=%s", rr.Code, rr.Body)
	}

	resp := decodeCaptureToggle(t, rr)
	if resp.Name != "alpha" {
		t.Errorf("name = %q, want alpha", resp.Name)
	}
	if resp.Status.Capture.Ready {
		t.Error("ready should be false immediately after enable")
	}
	if resp.Status.Capture.ActiveCapture != nil {
		t.Errorf("activeCapture = %v, want nil", *resp.Status.Capture.ActiveCapture)
	}
	if resp.Status.Capture.LastCaptureTime != nil {
		t.Errorf("lastCaptureTime = %v, want nil", *resp.Status.Capture.LastCaptureTime)
	}
	if resp.Status.Capture.SidecarRestarts != 0 {
		t.Errorf("sidecarRestarts = %d, want 0", resp.Status.Capture.SidecarRestarts)
	}

	obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(scope.DefaultNamespace).Get(t.Context(), "alpha", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	enabled, _, _ := unstructured.NestedBool(obj.Object, "spec", "capture", "enabled")
	if !enabled {
		t.Error("spec.capture.enabled should be true after enable")
	}
}

// TestCaptureEnable_AlreadyEnabledIsIdempotent verifies that re-enabling
// an already-enabled server is not an error — the contract's precondition
// list for enable carries no "already enabled" condition, unlike disable.
func TestCaptureEnable_AlreadyEnabledIsIdempotent(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("idem", true))
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: true}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/idem:capture-enable", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("re-enable status = %d, want 200; body=%s", rr.Code, rr.Body)
	}
}

// TestCaptureEnable_Terminating verifies a GameServer with a
// deletionTimestamp set 409s rather than being patched.
func TestCaptureEnable_Terminating(t *testing.T) {
	srv := newCaptureServerObj("dying", false)
	now := metav1.Now()
	srv.SetDeletionTimestamp(&now)
	k := fakeCaptureClient(srv)
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: true}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/dying:capture-enable", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("enable on terminating server status = %d, want 409; body=%s", rr.Code, rr.Body)
	}
}

// TestCaptureEnable_FeatureDisabled verifies the cluster-wide 501 gate.
// httperr.WriteCode never echoes a caller-composed message for a >=500
// status (see its doc comment), so the body here is the generic
// http.StatusText, not the handler's own message string — matching the
// same shape as the existing clusterOps.enabled 501 precedent
// (cluster_actions_test.go's TestClusterActions_DisabledReturns501).
func TestCaptureEnable_FeatureDisabled(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("beta", false))
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: false}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/beta:capture-enable", nil)
	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("enable with feature disabled status = %d, want 501; body=%s", rr.Code, rr.Body)
	}
}

// TestCaptureEnable_NotFound verifies capture-enable on a nonexistent
// server 404s.
func TestCaptureEnable_NotFound(t *testing.T) {
	k := fakeCaptureClient()
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: true}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/ghost:capture-enable", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("enable on missing server status = %d, want 404; body=%s", rr.Code, rr.Body)
	}
}

// TestCaptureDisable_Success verifies the happy path: 200, ready forced
// false, activeCapture forced null (even though status.capture was
// pre-seeded with stale Running-looking values), spec.capture.enabled
// patched to false, and the still-Running NetworkCapture stopped.
func TestCaptureDisable_Success(t *testing.T) {
	srv := newCaptureServerObj("gamma", true)
	if err := unstructured.SetNestedField(srv.Object, true, "status", "capture", "ready"); err != nil {
		t.Fatalf("seed status.capture.ready: %v", err)
	}
	if err := unstructured.SetNestedField(srv.Object, "cap-stale", "status", "capture", "activeCapture"); err != nil {
		t.Fatalf("seed status.capture.activeCapture: %v", err)
	}
	nc := newCaptureNetworkCapture("cap-stale", "gamma", "Running")

	k := fakeCaptureClient(srv, nc)
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: true}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/gamma:capture-disable", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want 200; body=%s", rr.Code, rr.Body)
	}

	resp := decodeCaptureToggle(t, rr)
	if resp.Status.Capture.Ready {
		t.Error("ready should be forced false on disable")
	}
	if resp.Status.Capture.ActiveCapture != nil {
		t.Errorf("activeCapture = %v, want nil (disable forces it, even though status.capture was pre-seeded)", *resp.Status.Capture.ActiveCapture)
	}

	obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(scope.DefaultNamespace).Get(t.Context(), "gamma", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	enabled, _, _ := unstructured.NestedBool(obj.Object, "spec", "capture", "enabled")
	if enabled {
		t.Error("spec.capture.enabled should be false after disable")
	}

	ncAfter, err := k.Dynamic.Resource(kube.GVRNetworkCapture).
		Namespace(scope.DefaultNamespace).Get(t.Context(), "cap-stale", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get network capture: %v", err)
	}
	phase, _, _ := unstructured.NestedString(ncAfter.Object, "status", "phase")
	if phase != "Completed" {
		t.Errorf("active capture phase = %q, want Completed after disable stopped it", phase)
	}
}

// TestCaptureDisable_AlreadyDisabled verifies disabling a server that
// never had capture enabled 409s (unlike enable, disable's precondition
// list does have an "already disabled" condition).
func TestCaptureDisable_AlreadyDisabled(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("delta", false))
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: true}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/delta:capture-disable", nil)
	if rr.Code != http.StatusConflict {
		t.Fatalf("disable already-disabled status = %d, want 409; body=%s", rr.Code, rr.Body)
	}
}

// TestCaptureDisable_NotFound verifies capture-disable on a nonexistent
// server 404s.
func TestCaptureDisable_NotFound(t *testing.T) {
	k := fakeCaptureClient()
	r := mountCaptureTestRouter(k, CaptureConfig{FeatureEnabled: true}, newCaptureAuditor(t))

	rr := do(t, r, http.MethodPost, "/servers/ghost:capture-disable", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("disable on missing server status = %d, want 404; body=%s", rr.Code, rr.Body)
	}
}

// TestCaptureEnableDisable_RBAC_OperatorForbidden proves the RBAC
// ordering doesn't regress: an OPERATOR-role token holds servers:write
// but must NOT be able to reach capture-enable/disable, which require
// captures:manage (rbac.go's rule table, ordered before the servers:write
// catch-all). This is the specific token that would have caught the
// original hole — not merely "some non-admin token" (rbac_test.go's own
// comment on the equivalent rule-table test makes the same point).
func TestCaptureEnableDisable_RBAC_OperatorForbidden(t *testing.T) {
	k := fakeCaptureClient(newCaptureServerObj("epsilon", false))
	reg := kube.NewRegistry(scope.DefaultCluster)
	reg.Set(scope.DefaultCluster, k)

	r := chi.NewRouter()
	r.Use(rbac.Middleware(reg))
	MountCapture(r, reg, newCaptureAuditor(t), CaptureConfig{FeatureEnabled: true}, "", "", "")

	operator := &auth.User{
		ID:       7,
		Username: "op",
		Role:     rbac.RoleOperator,
		Perms: map[string]map[string]map[string]struct{}{
			"*": {"*": {"servers:write": {}}},
		},
	}

	for _, path := range []string{"/servers/epsilon:capture-enable", "/servers/epsilon:capture-disable"} {
		req := httptest.NewRequestWithContext(auth.WithUser(t.Context(), operator), http.MethodPost, path, nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("operator %s status = %d, want 403 (servers:write must not substitute for captures:manage); body=%s", path, rr.Code, rr.Body)
		}
	}
}
