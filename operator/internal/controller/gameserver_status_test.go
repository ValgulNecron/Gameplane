package controller

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func TestDerivePhase(t *testing.T) {
	table := []struct {
		name     string
		suspend  bool
		ssExists bool
		ssReady  bool
		hbFresh  bool
		want     gameplanev1alpha1.GameServerPhase
	}{
		{"pending when no ss", false, false, false, false, gameplanev1alpha1.GameServerPhasePending},
		{"starting when ss not ready", false, true, false, false, gameplanev1alpha1.GameServerPhaseStarting},
		{"starting when ready but no heartbeat", false, true, true, false, gameplanev1alpha1.GameServerPhaseStarting},
		{"running when ready and fresh heartbeat", false, true, true, true, gameplanev1alpha1.GameServerPhaseRunning},
		{"suspended when suspend + ss gone", true, false, false, false, gameplanev1alpha1.GameServerPhaseSuspended},
		{"stopping when suspend + ss still ready", true, true, true, true, gameplanev1alpha1.GameServerPhaseStopping},
	}
	for _, tc := range table {
		t.Run(tc.name, func(t *testing.T) {
			gs := &gameplanev1alpha1.GameServer{
				Spec: gameplanev1alpha1.GameServerSpec{Suspend: tc.suspend},
			}
			got := derivePhase(gs, tc.ssExists, tc.ssReady, tc.hbFresh, idleAwake)
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestHeartbeatFresh(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{}
	if heartbeatFresh(gs) {
		t.Error("no heartbeat should be stale")
	}
	now := metav1.Now()
	gs.Status.Agent = &gameplanev1alpha1.AgentStatus{LastHeartbeat: &now}
	if !heartbeatFresh(gs) {
		t.Error("heartbeat now should be fresh")
	}
	old := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	gs.Status.Agent.LastHeartbeat = &old
	if heartbeatFresh(gs) {
		t.Error("heartbeat 10m ago should be stale")
	}
}

func TestValidatePlayitEndpoint(t *testing.T) {
	tests := []struct {
		name              string
		endpoint          *gameplanev1alpha1.GameServerEndpoint
		advertisedPorts   []string
		expectValid       bool
		expectMessagePart string
	}{
		{
			name: "valid ipv4",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "203.0.113.1",
				Port: 25565,
			},
			advertisedPorts: []string{"java"},
			expectValid:     true,
		},
		{
			name: "valid ipv6",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "2001:db8::1",
				Port: 25565,
			},
			advertisedPorts: []string{"java"},
			expectValid:     true,
		},
		{
			name: "valid dns name",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "game.example.com",
				Port: 25565,
			},
			advertisedPorts: []string{"java"},
			expectValid:     true,
		},
		{
			name: "valid dns name with hyphens",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "my-game-server.example.com",
				Port: 25565,
			},
			advertisedPorts: []string{"java"},
			expectValid:     true,
		},
		{
			name: "empty host",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "hostname",
		},
		{
			name: "control character in host",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "game\x00server.com",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "control",
		},
		{
			name: "embedded scheme http",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "http://game.example.com",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "scheme",
		},
		{
			name: "embedded scheme https",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "https://game.example.com",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "scheme",
		},
		{
			name: "port 0",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "203.0.113.1",
				Port: 0,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "port",
		},
		{
			name: "port 65536",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "203.0.113.1",
				Port: 65536,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "port",
		},
		{
			name: "port name not advertised",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "rcon",
				Host: "203.0.113.1",
				Port: 25575,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "advertised",
		},
		{
			name: "very long hostname",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: strings.Repeat("a", 254),
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "too long",
		},
		{
			name: "whitespace in host",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "game server.com",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "whitespace",
		},
		{
			name: "tab character in host",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "game\tserver.com",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "whitespace",
		},
		{
			name: "minimum valid port",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "203.0.113.1",
				Port: 1,
			},
			advertisedPorts: []string{"java"},
			expectValid:     true,
		},
		{
			name: "maximum valid port",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "203.0.113.1",
				Port: 65535,
			},
			advertisedPorts: []string{"java"},
			expectValid:     true,
		},
		{
			name: "private ipv4 rejected",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "10.0.0.1",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "private",
		},
		{
			name: "loopback ipv4 rejected",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "127.0.0.1",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "internal",
		},
		{
			name: "cluster-local dns rejected",
			endpoint: &gameplanev1alpha1.GameServerEndpoint{
				Name: "java",
				Host: "myserver.default.svc.cluster.local",
				Port: 25565,
			},
			advertisedPorts:   []string{"java"},
			expectValid:       false,
			expectMessagePart: "cluster-local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, msg := validatePlayitEndpoint(tt.endpoint, tt.advertisedPorts)
			if valid != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, valid)
			}
			if !tt.expectValid && !strings.Contains(msg, tt.expectMessagePart) {
				t.Errorf("expected error containing %q, got %q", tt.expectMessagePart, msg)
			}
		})
	}
}

func TestValidatePlayitEndpoints(t *testing.T) {
	tests := []struct {
		name            string
		endpoints       []gameplanev1alpha1.GameServerEndpoint
		advertisedPorts []string
		expectValid     []string // expected port names in valid result
		expectErrorPart string
	}{
		{
			name: "single valid endpoint",
			endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Name: "java", Host: "203.0.113.1", Port: 25565},
			},
			advertisedPorts: []string{"java"},
			expectValid:     []string{"java"},
			expectErrorPart: "",
		},
		{
			name: "multi-port happy path",
			endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Name: "java", Host: "203.0.113.1", Port: 25565},
				{Name: "bedrock", Host: "203.0.113.1", Port: 19133},
			},
			advertisedPorts: []string{"java", "bedrock"},
			expectValid:     []string{"java", "bedrock"},
			expectErrorPart: "",
		},
		{
			name: "duplicate port names",
			endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Name: "java", Host: "203.0.113.1", Port: 25565},
				{Name: "java", Host: "203.0.113.1", Port: 25566},
			},
			advertisedPorts: []string{"java"},
			expectValid:     []string{"java"},
			expectErrorPart: "duplicate",
		},
		{
			name: "invalid entry filtered out",
			endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Name: "java", Host: "203.0.113.1", Port: 25565},
				{Name: "query", Host: "10.0.0.1", Port: 25577},
			},
			advertisedPorts: []string{"java", "query"},
			expectValid:     []string{"java"},
			expectErrorPart: "private",
		},
		{
			name: "port name not advertised",
			endpoints: []gameplanev1alpha1.GameServerEndpoint{
				{Name: "java", Host: "203.0.113.1", Port: 25565},
				{Name: "rcon", Host: "203.0.113.1", Port: 25575},
			},
			advertisedPorts: []string{"java"},
			expectValid:     []string{"java"},
			expectErrorPart: "advertised",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errMsg := validatePlayitEndpoints(tt.endpoints, tt.advertisedPorts)
			if len(valid) != len(tt.expectValid) {
				t.Errorf("expected %d valid endpoints, got %d", len(tt.expectValid), len(valid))
			}
			for i, name := range tt.expectValid {
				if i >= len(valid) || valid[i].Name != name {
					t.Errorf("expected valid[%d].Name=%q", i, name)
				}
			}
			if tt.expectErrorPart != "" && !strings.Contains(errMsg, tt.expectErrorPart) {
				t.Errorf("expected error containing %q, got %q", tt.expectErrorPart, errMsg)
			}
			if tt.expectErrorPart == "" && errMsg != "" {
				t.Errorf("expected no error, got %q", errMsg)
			}
		})
	}
}

func TestGetAdvertisedPortNames(t *testing.T) {
	tmpl := &gameplanev1alpha1.GameTemplate{
		Spec: gameplanev1alpha1.GameTemplateSpec{
			Ports: []gameplanev1alpha1.GamePort{
				{
					Name:      "java",
					Advertise: true,
				},
				{
					Name:      "rcon",
					Advertise: false,
				},
				{
					Name:      "query",
					Advertise: true,
				},
			},
		},
	}

	got := getAdvertisedPortNames(tmpl)
	expected := []string{"java", "query"}

	if len(got) != len(expected) {
		t.Errorf("expected %d advertised ports, got %d", len(expected), len(got))
		return
	}

	for i, name := range expected {
		if got[i] != name {
			t.Errorf("expected port %d to be %q, got %q", i, name, got[i])
		}
	}
}

func TestGetAdvertisedPortNamesEmpty(t *testing.T) {
	tmpl := &gameplanev1alpha1.GameTemplate{
		Spec: gameplanev1alpha1.GameTemplateSpec{
			Ports: []gameplanev1alpha1.GamePort{
				{
					Name:      "java",
					Advertise: false,
				},
			},
		},
	}

	got := getAdvertisedPortNames(tmpl)
	if len(got) != 0 {
		t.Errorf("expected 0 advertised ports, got %d", len(got))
	}
}

// condByType indexes a condition list for the address-assignment tests below.
func condByType(conds []metav1.Condition, condType string) (metav1.Condition, bool) {
	for _, c := range conds {
		if c.Type == condType {
			return c, true
		}
	}
	return metav1.Condition{}, false
}

// lbService builds a LoadBalancer game Service carrying one port, optionally
// with an address already published by the cluster's address manager.
func lbService(ingressIP string) *corev1.Service {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeLoadBalancer,
			ClusterIP: "10.96.0.10",
			Ports: []corev1.ServicePort{
				{Name: "game", Port: 25565, Protocol: corev1.ProtocolTCP},
			},
		},
	}
	if ingressIP != "" {
		svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: ingressIP}}
	}
	return svc
}

// gsWithAddressRequest builds a GameServer asking for a pool and/or address
// under the given expose mode.
func gsWithAddressRequest(expose, pool, address string) *gameplanev1alpha1.GameServer {
	return &gameplanev1alpha1.GameServer{
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Expose:      expose,
				AddressPool: pool,
				Address:     address,
			},
		},
	}
}

func TestEndpointsFromServicePool(t *testing.T) {
	cases := []struct {
		name     string
		svc      *corev1.Service
		plan     addressPlan
		wantHost string
		wantPool string
	}{
		{
			// The whole point of the field: a translated request that the
			// address manager has answered stamps the pool it was asked for.
			name:     "translated request with an assigned address stamps the pool",
			svc:      lbService("203.0.113.7"),
			plan:     addressPlan{Manager: addressManagerMetalLB, Pool: "games", Outcome: addressPlanTranslated},
			wantHost: "203.0.113.7",
			wantPool: "games",
		},
		{
			// Backward compatibility: an untouched server's endpoints must
			// look exactly as they did before the field existed.
			name:     "no request leaves the pool empty",
			svc:      lbService("203.0.113.7"),
			plan:     addressPlan{Manager: addressManagerMetalLB, Outcome: addressPlanNotRequested},
			wantHost: "203.0.113.7",
			wantPool: "",
		},
		{
			name:     "address-only request leaves the pool empty",
			svc:      lbService("203.0.113.7"),
			plan:     addressPlan{Manager: addressManagerMetalLB, Address: "203.0.113.7", Outcome: addressPlanTranslated},
			wantHost: "203.0.113.7",
			wantPool: "",
		},
		{
			// The ClusterIP fallback shown while assignment is pending was
			// not drawn from any pool, so it must not claim one.
			name:     "pending assignment falls back to the cluster IP with no pool",
			svc:      lbService(""),
			plan:     addressPlan{Manager: addressManagerCilium, Pool: "games", Outcome: addressPlanTranslated},
			wantHost: "10.96.0.10",
			wantPool: "",
		},
		{
			name:     "no address manager configured claims no pool",
			svc:      lbService("203.0.113.7"),
			plan:     addressPlan{Pool: "games", Outcome: addressPlanNoAddressManagerConfigured},
			wantHost: "203.0.113.7",
			wantPool: "",
		},
		{
			name:     "request ignored for the expose mode claims no pool",
			svc:      lbService("203.0.113.7"),
			plan:     addressPlan{Manager: addressManagerMetalLB, Pool: "games", Outcome: addressPlanIgnoredForExposureMode},
			wantHost: "203.0.113.7",
			wantPool: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eps := endpointsFromService(tc.svc, tc.plan)
			if len(eps) != 1 {
				t.Fatalf("got %d endpoints, want 1", len(eps))
			}
			if eps[0].Host != tc.wantHost {
				t.Errorf("Host = %q, want %q", eps[0].Host, tc.wantHost)
			}
			if eps[0].Pool != tc.wantPool {
				t.Errorf("Pool = %q, want %q", eps[0].Pool, tc.wantPool)
			}
		})
	}
}

func TestEndpointsFromServicePoolStampsEveryPort(t *testing.T) {
	svc := lbService("203.0.113.7")
	svc.Spec.Ports = append(svc.Spec.Ports,
		corev1.ServicePort{Name: "query", Port: 25565, Protocol: corev1.ProtocolUDP})
	plan := addressPlan{Manager: addressManagerCilium, Pool: "games", Outcome: addressPlanTranslated}

	eps := endpointsFromService(svc, plan)
	if len(eps) != 2 {
		t.Fatalf("got %d endpoints, want 2", len(eps))
	}
	for _, ep := range eps {
		if ep.Pool != "games" {
			t.Errorf("endpoint %q: Pool = %q, want %q", ep.Name, ep.Pool, "games")
		}
	}
}

func TestAddressAssignmentCondition(t *testing.T) {
	cases := []struct {
		name       string
		gs         *gameplanev1alpha1.GameServer
		plan       addressPlan
		svc        *corev1.Service
		wantStatus metav1.ConditionStatus
		wantReason string
		wantInMsg  string
	}{
		{
			name:       "assigned from a requested pool",
			gs:         gsWithAddressRequest("LoadBalancer", "games", ""),
			plan:       addressPlan{Manager: addressManagerMetalLB, Pool: "games", Outcome: addressPlanTranslated},
			svc:        lbService("203.0.113.7"),
			wantStatus: metav1.ConditionTrue,
			wantReason: "Assigned",
			wantInMsg:  "from pool 'games'",
		},
		{
			name:       "assigned for an address-only request",
			gs:         gsWithAddressRequest("LoadBalancer", "", "203.0.113.7"),
			plan:       addressPlan{Manager: addressManagerCilium, Address: "203.0.113.7", Outcome: addressPlanTranslated},
			svc:        lbService("203.0.113.7"),
			wantStatus: metav1.ConditionTrue,
			wantReason: "Assigned",
			wantInMsg:  "203.0.113.7",
		},
		{
			name:       "pending while the address manager has not answered",
			gs:         gsWithAddressRequest("LoadBalancer", "games", ""),
			plan:       addressPlan{Manager: addressManagerMetalLB, Pool: "games", Outcome: addressPlanTranslated},
			svc:        lbService(""),
			wantStatus: metav1.ConditionFalse,
			wantReason: "AssignmentPending",
			wantInMsg:  "pool 'games'",
		},
		{
			name:       "waiting for the Service to exist",
			gs:         gsWithAddressRequest("LoadBalancer", "games", ""),
			plan:       addressPlan{Manager: addressManagerMetalLB, Pool: "games", Outcome: addressPlanTranslated},
			svc:        nil,
			wantStatus: metav1.ConditionFalse,
			wantReason: "ServiceNotReady",
			wantInMsg:  "waiting for the LoadBalancer Service",
		},
		{
			name:       "ignored on a NodePort server",
			gs:         gsWithAddressRequest("NodePort", "games", ""),
			plan:       addressPlan{Manager: addressManagerMetalLB, Pool: "games", Outcome: addressPlanIgnoredForExposureMode},
			svc:        nil,
			wantStatus: metav1.ConditionFalse,
			wantReason: "IgnoredForExposureMode",
			wantInMsg:  "'NodePort'",
		},
		{
			// An unset Expose is ClusterIP; the message must name that
			// rather than an empty string.
			name:       "ignored on a default-expose server names ClusterIP",
			gs:         gsWithAddressRequest("", "games", ""),
			plan:       addressPlan{Manager: addressManagerMetalLB, Pool: "games", Outcome: addressPlanIgnoredForExposureMode},
			svc:        nil,
			wantStatus: metav1.ConditionFalse,
			wantReason: "IgnoredForExposureMode",
			wantInMsg:  "'ClusterIP'",
		},
		{
			// Flavor "none": the Service is untouched, but the request must
			// still be reported so it never looks silently honored.
			name:       "no address manager configured",
			gs:         gsWithAddressRequest("LoadBalancer", "games", "203.0.113.7"),
			plan:       addressPlan{Pool: "games", Address: "203.0.113.7", Outcome: addressPlanNoAddressManagerConfigured},
			svc:        lbService("198.51.100.4"),
			wantStatus: metav1.ConditionFalse,
			wantReason: "NoAddressManagerConfigured",
			wantInMsg:  "address '203.0.113.7' from pool 'games'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conds := addressAssignmentCondition(nil, tc.gs, tc.plan, tc.svc, addressFailureReason{}, "")
			got, ok := condByType(conds, gameplanev1alpha1.GameServerConditionAddressAssignment)
			if !ok {
				t.Fatalf("no %s condition", gameplanev1alpha1.GameServerConditionAddressAssignment)
			}
			if got.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if got.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", got.Reason, tc.wantReason)
			}
			if !strings.Contains(got.Message, tc.wantInMsg) {
				t.Errorf("Message = %q, want it to contain %q", got.Message, tc.wantInMsg)
			}
		})
	}
}

// TestAddressAssignmentConditionAbsentWhenNotRequested is the backward-
// compatibility guard: every GameServer that predates this feature must keep
// exactly the conditions it had, so a server that asked for nothing carries no
// AddressAssignment condition at all — not a False one.
func TestAddressAssignmentConditionAbsentWhenNotRequested(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{}
	plan := planAddressPreference(gs, addressManagerMetalLB)

	conds := computeConditions(gs, gameplanev1alpha1.GameServerPhaseRunning, nil, idleAwake, plan, lbService("203.0.113.7"), addressFailureReason{}, "")
	if _, ok := condByType(conds, gameplanev1alpha1.GameServerConditionAddressAssignment); ok {
		t.Error("a server that requested no pool or address must carry no AddressAssignment condition")
	}
	// The conditions it does own are untouched.
	for _, want := range []string{"Ready", "Progressing", "Healthy"} {
		if _, ok := condByType(conds, want); !ok {
			t.Errorf("missing %s condition", want)
		}
	}
}

// TestAddressAssignmentConditionClearedOnRequestRemoval covers the other half
// of the same rule: clearing the spec fields drops the condition on the next
// reconcile rather than leaving a stale report behind.
func TestAddressAssignmentConditionClearedOnRequestRemoval(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{}
	gs.Status.Conditions = []metav1.Condition{
		{
			Type:               gameplanev1alpha1.GameServerConditionAddressAssignment,
			Status:             metav1.ConditionTrue,
			Reason:             "Assigned",
			LastTransitionTime: metav1.Now(),
		},
	}
	plan := planAddressPreference(gs, addressManagerMetalLB)

	conds := computeConditions(gs, gameplanev1alpha1.GameServerPhaseRunning, nil, idleAwake, plan, lbService("203.0.113.7"), addressFailureReason{}, "")
	if _, ok := condByType(conds, gameplanev1alpha1.GameServerConditionAddressAssignment); ok {
		t.Error("clearing the request must remove the stale AddressAssignment condition")
	}
}

func TestAddressRequestSummary(t *testing.T) {
	cases := []struct {
		name string
		plan addressPlan
		want string
	}{
		{"pool only", addressPlan{Pool: "games"}, "pool 'games'"},
		{"address only", addressPlan{Address: "203.0.113.7"}, "address '203.0.113.7'"},
		{
			"both", addressPlan{Pool: "games", Address: "203.0.113.7"},
			"address '203.0.113.7' from pool 'games'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := addressRequestSummary(tc.plan); got != tc.want {
				t.Errorf("addressRequestSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadBalancerAddress(t *testing.T) {
	cases := []struct {
		name string
		svc  *corev1.Service
		want string
	}{
		{"nil service", nil, ""},
		{"not a load balancer", &corev1.Service{
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.96.0.10"},
		}, ""},
		{"no ingress yet", lbService(""), ""},
		{"ingress ip", lbService("203.0.113.7"), "203.0.113.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := loadBalancerAddress(tc.svc); got != tc.want {
				t.Errorf("loadBalancerAddress() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A cloud load balancer that publishes both wants players resolving the name.
func TestLoadBalancerAddressPrefersHostname(t *testing.T) {
	svc := lbService("203.0.113.7")
	svc.Status.LoadBalancer.Ingress[0].Hostname = "lb.example.com"
	if got := loadBalancerAddress(svc); got != "lb.example.com" {
		t.Errorf("loadBalancerAddress() = %q, want the hostname", got)
	}
}

func TestAddressAssignmentCondition_PoolNotFound(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Expose:      "LoadBalancer",
				AddressPool: "nonexistent",
			},
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	plan := addressPlan{
		Manager: "metallb",
		Pool:    "nonexistent",
		Outcome: addressPlanTranslated,
	}
	eventFailure := addressFailureReason{
		reason:  "PoolNotFound",
		message: "Pool not found: nonexistent pool",
	}
	conds := addressAssignmentCondition(nil, gs, plan, nil, eventFailure, "")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if conds[0].Reason != "PoolNotFound" {
		t.Errorf("expected reason PoolNotFound, got %q", conds[0].Reason)
	}
	if conds[0].Status != metav1.ConditionFalse {
		t.Errorf("expected status False, got %q", conds[0].Status)
	}
}

func TestAddressAssignmentCondition_PoolExhausted(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Expose:      "LoadBalancer",
				AddressPool: "public",
			},
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	plan := addressPlan{
		Manager: "metallb",
		Pool:    "public",
		Outcome: addressPlanTranslated,
	}
	eventFailure := addressFailureReason{
		reason:  "PoolExhausted",
		message: "Pool exhausted: public pool has no addresses available",
	}
	conds := addressAssignmentCondition(nil, gs, plan, nil, eventFailure, "")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if conds[0].Reason != "PoolExhausted" {
		t.Errorf("expected reason PoolExhausted, got %q", conds[0].Reason)
	}
}

func TestAddressAssignmentCondition_AddressInUse(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Expose:  "LoadBalancer",
				Address: "203.0.113.1",
			},
		},
		ObjectMeta: metav1.ObjectMeta{Name: "server-a", Namespace: "default"},
	}
	plan := addressPlan{
		Manager: "metallb",
		Address: "203.0.113.1",
		Outcome: addressPlanTranslated,
	}
	conds := addressAssignmentCondition(nil, gs, plan, nil, addressFailureReason{}, "server-b")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if conds[0].Reason != "AddressInUse" {
		t.Errorf("expected reason AddressInUse, got %q", conds[0].Reason)
	}
	if !strings.Contains(conds[0].Message, "server-b") {
		t.Errorf("expected message to mention conflicting server, got %q", conds[0].Message)
	}
}

func TestAddressAssignmentCondition_IgnoredForExposureMode(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Expose:      "ClusterIP",
				AddressPool: "public",
			},
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	plan := addressPlan{
		Manager: "metallb",
		Pool:    "public",
		Outcome: addressPlanIgnoredForExposureMode,
	}
	conds := addressAssignmentCondition(nil, gs, plan, nil, addressFailureReason{}, "")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if conds[0].Reason != "IgnoredForExposureMode" {
		t.Errorf("expected reason IgnoredForExposureMode, got %q", conds[0].Reason)
	}
}

func TestAddressAssignmentCondition_NoAddressManagerConfigured(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Expose:      "LoadBalancer",
				AddressPool: "public",
			},
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	plan := addressPlan{
		Manager: "none",
		Pool:    "public",
		Outcome: addressPlanNoAddressManagerConfigured,
	}
	conds := addressAssignmentCondition(nil, gs, plan, nil, addressFailureReason{}, "")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if conds[0].Reason != "NoAddressManagerConfigured" {
		t.Errorf("expected reason NoAddressManagerConfigured, got %q", conds[0].Reason)
	}
}

func TestAddressAssignmentCondition_Assigned(t *testing.T) {
	gs := &gameplanev1alpha1.GameServer{
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Expose:      "LoadBalancer",
				AddressPool: "public",
			},
		},
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	plan := addressPlan{
		Manager: "metallb",
		Pool:    "public",
		Outcome: addressPlanTranslated,
	}
	svc := lbService("203.0.113.5")
	conds := addressAssignmentCondition(nil, gs, plan, svc, addressFailureReason{}, "")
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if conds[0].Reason != "Assigned" {
		t.Errorf("expected reason Assigned, got %q", conds[0].Reason)
	}
	if conds[0].Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %q", conds[0].Status)
	}
}

// warnEvent builds a Warning Service event attributed to the given reporting
// component, mirroring the shape MetalLB-style managers emit.
func warnEvent(component, reason, message string) corev1.Event {
	return corev1.Event{
		Type:    corev1.EventTypeWarning,
		Reason:  reason,
		Message: message,
		Source:  corev1.EventSource{Component: component},
	}
}

// TestExtractAddressFailureFromEvents pins the two properties the matcher must
// hold: it never panics or invents a reason for events it does not understand,
// and it recognises each of the three failure shapes when they come from
// something that looks like an address manager.
func TestExtractAddressFailureFromEvents(t *testing.T) {
	cases := []struct {
		name       string
		events     []corev1.Event
		wantReason string
		wantInMsg  string
	}{
		{name: "nil events"},
		{name: "empty slice", events: []corev1.Event{}},
		{
			name: "normal event from the address manager is ignored",
			events: []corev1.Event{{
				Type:    corev1.EventTypeNormal,
				Reason:  "PoolNotFound",
				Message: "pool not found",
				Source:  corev1.EventSource{Component: "metallb-controller"},
			}},
		},
		{
			name:   "unrecognised warning from another component",
			events: []corev1.Event{warnEvent("kubelet", "FailedMount", "volume is already in use by another pod")},
		},
		{
			name:   "warning from an unrelated controller must not report a pool failure",
			events: []corev1.Event{warnEvent("ingress-controller", "PoolExhausted", "backend pool exhausted")},
		},
		{
			name:   "warning with no source at all",
			events: []corev1.Event{warnEvent("", "PoolNotFound", "pool not found: games")},
		},
		{
			name:   "address-manager warning with no recognised text",
			events: []corev1.Event{warnEvent("metallb-speaker", "SomethingElse", "unrelated speaker problem")},
		},
		{
			name:       "pool not found by reason",
			events:     []corev1.Event{warnEvent("metallb-controller", "PoolNotFound", "no pool named games")},
			wantReason: "PoolNotFound",
			wantInMsg:  "no pool named games",
		},
		{
			name:       "pool not found by message",
			events:     []corev1.Event{warnEvent("metallb-controller", "AllocationFailed", "pool not found: games")},
			wantReason: "PoolNotFound",
			wantInMsg:  "pool not found: games",
		},
		{
			name:       "pool exhausted by message",
			events:     []corev1.Event{warnEvent("metallb-controller", "AllocationFailed", "pool games is exhausted")},
			wantReason: "PoolExhausted",
			wantInMsg:  "pool games is exhausted",
		},
		{
			name:       "address in use by reason",
			events:     []corev1.Event{warnEvent("cilium-operator", "AddressInUse", "203.0.113.7 taken")},
			wantReason: "AddressInUse",
			wantInMsg:  "203.0.113.7 taken",
		},
		{
			name: "unrecognised warning ahead of a real one does not mask it",
			events: []corev1.Event{
				warnEvent("kubelet", "BackOff", "restarting failed container"),
				warnEvent("metallb-controller", "AllocationFailed", "address already in use"),
			},
			wantReason: "AddressInUse",
			wantInMsg:  "address already in use",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractAddressFailureFromEvents(tc.events)
			if got.reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", got.reason, tc.wantReason)
			}
			if tc.wantReason == "" {
				if got.message != "" {
					t.Errorf("zero value expected, got message %q", got.message)
				}
				return
			}
			if !strings.Contains(got.message, tc.wantInMsg) {
				t.Errorf("message %q does not contain %q", got.message, tc.wantInMsg)
			}
		})
	}
}
