package controller

import (
	"strings"
	"testing"
	"time"

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
