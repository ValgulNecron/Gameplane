//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestGameServer_IngressNetworkPolicyShapeAndCascade: the operator
// reconciles a per-GameServer ingress NetworkPolicy (`<gs>-game-ingress`,
// see gameIngressPolicyName/reconcileNetworkPolicy in
// operator/internal/controller/gameserver_controller.go) admitting player
// traffic to the GameTemplate's `advertise: true` container ports only —
// RCON and the agent port must stay closed.
//
// This asserts the object's SHAPE only: podSelector, policyTypes, the
// configured source CIDR, and that the advertised port is present while
// the non-advertised one is not. It deliberately does NOT assert that
// traffic is actually blocked or admitted — kind's default CNI does not
// enforce NetworkPolicy, so a test relying on enforcement would pass or
// fail for reasons unrelated to the operator's reconciliation logic.
//
// It also asserts cascade-delete: the policy is owned by the GameServer
// (OwnerReference, like the StatefulSet/Service/PVC), so it must be GC'd
// when the GameServer is deleted.
func TestGameServer_IngressNetworkPolicyShapeAndCascade(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ns := "gameplane-games"
	tmplName := fmt.Sprintf("e2e-netpol-tmpl-%d", time.Now().UnixNano())
	gsName := fmt.Sprintf("e2e-netpol-gs-%d", time.Now().UnixNano())
	policyName := gsName + "-game-ingress"

	// A template with one advertised port (the game port) and one
	// non-advertised port (RCON) — the operator must open only the
	// former. None of the suite's existing template helpers declare a
	// non-advertised port, so this test builds its own rather than
	// reusing applyBusyboxTemplate.
	tmpl := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gameplane.local/v1alpha1",
		"kind":       "GameTemplate",
		"metadata":   map[string]any{"name": tmplName},
		"spec": map[string]any{
			"displayName": "E2E netpol busybox (" + tmplName + ")",
			"game":        "busybox",
			"version":     "1",
			"image":       "busybox:1.36",
			"command":     []any{"sh", "-c", "sleep 100000"},
			"ports": []any{
				map[string]any{"name": "game", "containerPort": int64(25566), "advertise": true, "protocol": "TCP"},
				map[string]any{"name": "rcon", "containerPort": int64(25575), "advertise": false, "protocol": "TCP"},
			},
		},
	}}
	if _, err := envInstance.Dyn.Resource(gameTemplateGVR).
		Create(ctx, tmpl, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatalf("create template: %v", err)
	}
	t.Cleanup(func() {
		_ = envInstance.Dyn.Resource(gameTemplateGVR).
			Delete(context.Background(), tmplName, metav1.DeleteOptions{})
	})

	applyBusyboxGameServer(t, ns, gsName, tmplName)

	var np *networkingv1.NetworkPolicy
	envInstance.Eventually(t, 60*time.Second, func() (bool, string) {
		got, err := envInstance.K8s.NetworkingV1().NetworkPolicies(ns).Get(ctx, policyName, metav1.GetOptions{})
		if err != nil {
			return false, "get networkpolicy: " + err.Error()
		}
		np = got
		return true, ""
	})

	// podSelector must match this GameServer's pods only (name+instance
	// labels, same pair the Service and StatefulSet use).
	wantSelector := map[string]string{
		"app.kubernetes.io/name":     "gameplane-game",
		"app.kubernetes.io/instance": gsName,
	}
	if len(np.Spec.PodSelector.MatchLabels) != len(wantSelector) {
		t.Errorf("podSelector.matchLabels = %v, want exactly %v", np.Spec.PodSelector.MatchLabels, wantSelector)
	}
	for k, v := range wantSelector {
		if got := np.Spec.PodSelector.MatchLabels[k]; got != v {
			t.Errorf("podSelector.matchLabels[%s] = %q, want %q", k, got, v)
		}
	}

	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress {
		t.Errorf("policyTypes = %v, want [Ingress]", np.Spec.PolicyTypes)
	}

	if len(np.Spec.Ingress) != 1 {
		t.Fatalf("ingress rules = %d, want 1", len(np.Spec.Ingress))
	}
	rule := np.Spec.Ingress[0]

	// Source CIDR: the chart's default networkPolicies.gameIngress.fromCIDRs
	// is [0.0.0.0/0] and nothing in deploy/kind/e2e.sh overrides it.
	foundCIDR := false
	for _, from := range rule.From {
		if from.IPBlock != nil && from.IPBlock.CIDR == "0.0.0.0/0" {
			foundCIDR = true
		}
	}
	if !foundCIDR {
		t.Errorf("ingress[0].from missing ipBlock 0.0.0.0/0: %+v", rule.From)
	}

	// Ports: exactly the advertised game port; the non-advertised RCON
	// port (and the agent's 8090) must never appear.
	if len(rule.Ports) != 1 {
		t.Fatalf("ingress[0].ports has %d entries, want exactly 1 (the advertised game port): %+v",
			len(rule.Ports), rule.Ports)
	}
	gamePort := rule.Ports[0]
	if gamePort.Port == nil || gamePort.Port.IntValue() != 25566 {
		t.Errorf("ingress[0].ports[0].port = %v, want 25566", gamePort.Port)
	}
	if gamePort.Protocol == nil || *gamePort.Protocol != corev1.ProtocolTCP {
		t.Errorf("ingress[0].ports[0].protocol = %v, want TCP", gamePort.Protocol)
	}
	for _, p := range rule.Ports {
		if p.Port != nil && p.Port.IntValue() == 25575 {
			t.Errorf("non-advertised RCON port 25575 must not appear in the NetworkPolicy ports: %+v", rule.Ports)
		}
	}

	// Cascade-delete: removing the GameServer must GC the NetworkPolicy —
	// it's owned via OwnerReference exactly like the StatefulSet/Service/PVC
	// (see TestGameServer_CascadingDelete for the equivalent StatefulSet
	// assertion).
	if err := envInstance.Dyn.Resource(gameServerGVR).Namespace(ns).
		Delete(ctx, gsName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("delete gameserver: %v", err)
	}
	envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
		_, err := envInstance.K8s.NetworkingV1().NetworkPolicies(ns).Get(ctx, policyName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, ""
		}
		if err != nil {
			return false, "get networkpolicy: " + err.Error()
		}
		return false, "networkpolicy still present after GameServer delete"
	})
}
