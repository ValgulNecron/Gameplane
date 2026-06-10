package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

func TestComputeConditions(t *testing.T) {
	cases := []struct {
		phase  kestrelv1alpha1.GameServerPhase
		ready  metav1.ConditionStatus
		health metav1.ConditionStatus
	}{
		{kestrelv1alpha1.GameServerPhaseRunning, metav1.ConditionTrue, metav1.ConditionTrue},
		{kestrelv1alpha1.GameServerPhaseStarting, metav1.ConditionFalse, metav1.ConditionFalse},
		{kestrelv1alpha1.GameServerPhaseStopping, metav1.ConditionFalse, metav1.ConditionFalse},
		{kestrelv1alpha1.GameServerPhaseSuspended, metav1.ConditionFalse, metav1.ConditionFalse},
		{kestrelv1alpha1.GameServerPhaseFailed, metav1.ConditionFalse, metav1.ConditionFalse},
		{kestrelv1alpha1.GameServerPhase("unknown"), metav1.ConditionUnknown, metav1.ConditionUnknown},
	}
	for _, tc := range cases {
		t.Run(string(tc.phase), func(t *testing.T) {
			gs := &kestrelv1alpha1.GameServer{}
			conds := computeConditions(gs, tc.phase)
			byType := map[string]metav1.Condition{}
			for _, c := range conds {
				byType[c.Type] = c
			}
			if byType["Ready"].Status != tc.ready {
				t.Errorf("Ready=%v want %v", byType["Ready"].Status, tc.ready)
			}
			if byType["Healthy"].Status != tc.health {
				t.Errorf("Healthy=%v want %v", byType["Healthy"].Status, tc.health)
			}
		})
	}
}

func TestEndpointsFromService_ClusterIP(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.0.0.5",
			Ports: []corev1.ServicePort{
				{Name: "game", Port: 25565, Protocol: corev1.ProtocolTCP},
				{Name: "rcon", Port: 25575, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	got := endpointsFromService(svc)
	if len(got) != 2 || got[0].Host != "10.0.0.5" || got[0].Port != 25565 {
		t.Fatalf("got %+v", got)
	}
}

func TestEndpointsFromService_NodePort(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{Name: "game", Port: 25565, NodePort: 30001, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	got := endpointsFromService(svc)
	if got[0].Port != 30001 {
		t.Fatalf("expected NodePort %d, got %d", 30001, got[0].Port)
	}
}

func TestEndpointsFromService_LoadBalancer(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "game", Port: 25565, Protocol: corev1.ProtocolTCP},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "203.0.113.1"},
				},
			},
		},
	}
	got := endpointsFromService(svc)
	if got[0].Host != "203.0.113.1" {
		t.Fatalf("got host=%q", got[0].Host)
	}
}

func TestEndpointsFromService_LoadBalancerHostname(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Name: "g", Port: 1}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{Hostname: "elb.example.com"}},
			},
		},
	}
	got := endpointsFromService(svc)
	if got[0].Host != "elb.example.com" {
		t.Fatalf("got %q", got[0].Host)
	}
}
