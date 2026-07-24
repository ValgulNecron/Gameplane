package main

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestParsePortsConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		want    int // number of ports
		wantErr bool
	}{
		{
			name:   "empty",
			config: "",
			want:   0,
		},
		{
			name:   "single port",
			config: "25565:TCP:minecraft",
			want:   1,
		},
		{
			name:   "multiple ports",
			config: "25565:TCP:minecraft,19133:UDP:generic",
			want:   2,
		},
		{
			name:   "with spaces",
			config: "25565: TCP : minecraft , 19133: UDP : generic",
			want:   2,
		},
		{
			name:   "leading comma",
			config: ",25565:TCP:minecraft",
			want:   1,
		},
		{
			name:   "trailing comma",
			config: "25565:TCP:minecraft,",
			want:   1,
		},
		{
			name:   "leading and trailing",
			config: ",25565:TCP:minecraft,",
			want:   1,
		},
		{
			name:   "none protocol",
			config: "25565:TCP:none",
			want:   1,
		},
		{
			name:    "invalid port number",
			config:  "abc:TCP:minecraft",
			wantErr: true,
		},
		{
			name:    "invalid protocol",
			config:  "25565:INVALID:minecraft",
			wantErr: true,
		},
		{
			name:    "invalid wake protocol",
			config:  "25565:TCP:invalid",
			wantErr: true,
		},
		{
			name:    "wrong format",
			config:  "25565:TCP",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePortsConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePortsConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != tt.want {
				t.Errorf("parsePortsConfig() got %d ports, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParsePortsConfigValues(t *testing.T) {
	config := "25565:TCP:minecraft,19133:UDP:generic"
	ports, err := parsePortsConfig(config)
	if err != nil {
		t.Fatalf("parsePortsConfig() error = %v", err)
	}

	if len(ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports))
	}

	if ports[0].ContainerPort != 25565 {
		t.Errorf("port 0 ContainerPort = %d, want 25565", ports[0].ContainerPort)
	}
	if ports[0].Protocol != "TCP" {
		t.Errorf("port 0 Protocol = %s, want TCP", ports[0].Protocol)
	}
	if ports[0].WakeProtocol != "minecraft" {
		t.Errorf("port 0 WakeProtocol = %s, want minecraft", ports[0].WakeProtocol)
	}

	if ports[1].ContainerPort != 19133 {
		t.Errorf("port 1 ContainerPort = %d, want 19133", ports[1].ContainerPort)
	}
	if ports[1].Protocol != "UDP" {
		t.Errorf("port 1 Protocol = %s, want UDP", ports[1].Protocol)
	}
	if ports[1].WakeProtocol != "generic" {
		t.Errorf("port 1 WakeProtocol = %s, want generic", ports[1].WakeProtocol)
	}
}

func TestUDPHeuristic(t *testing.T) {
	t.Run("threshold_met", func(t *testing.T) {
		h := NewUDPHeuristic(3, 10*time.Second)
		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}

		// First packet: should not wake
		if h.ShouldWake(addr) {
			t.Error("first packet should not wake")
		}

		// Second packet: should not wake
		if h.ShouldWake(addr) {
			t.Error("second packet should not wake")
		}

		// Third packet: should wake
		if !h.ShouldWake(addr) {
			t.Error("third packet should wake")
		}

		// Immediate fourth packet: should not wake (cooldown)
		if h.ShouldWake(addr) {
			t.Error("fourth packet should not wake (cooldown)")
		}
	})

	t.Run("window_expiry", func(t *testing.T) {
		h := NewUDPHeuristic(3, 100*time.Millisecond)
		addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}

		// Send 2 packets
		h.ShouldWake(addr)
		h.ShouldWake(addr)

		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)

		// Packets should be cleaned; this is the first "new" packet
		if h.ShouldWake(addr) {
			t.Error("should not wake after window expiry")
		}

		// Next packet
		if h.ShouldWake(addr) {
			t.Error("should not wake on second packet after reset")
		}

		// Third packet should wake
		if !h.ShouldWake(addr) {
			t.Error("third packet should wake")
		}
	})

	t.Run("different_addresses", func(t *testing.T) {
		h := NewUDPHeuristic(2, 10*time.Second)
		addr1 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
		addr2 := &net.UDPAddr{IP: net.ParseIP("127.0.0.2"), Port: 12346}

		// addr1: 2 packets
		h.ShouldWake(addr1)
		if !h.ShouldWake(addr1) {
			t.Error("addr1 second packet should wake")
		}

		// addr2: 1 packet
		if h.ShouldWake(addr2) {
			t.Error("addr2 first packet should not wake")
		}
	})
}

func TestWakeRequester(t *testing.T) {
	mockClient := &GameServerClient{
		name:      "test-server",
		namespace: "default",
	}
	wr := &WakeRequester{
		client:       mockClient,
		lastWakeTime: make(map[string]time.Time),
		mu:           &sync.Mutex{},
		minInterval:  100 * time.Millisecond,
	}

	ctx := context.Background()

	// First wake should succeed (or return no error at least)
	err := wr.RequestWake(ctx)
	if err != nil {
		t.Errorf("first RequestWake error = %v", err)
	}

	// Immediate second wake should be rate-limited (no patch, no error)
	err = wr.RequestWake(ctx)
	if err != nil {
		t.Errorf("second RequestWake error = %v", err)
	}

	// Wait for cooldown
	time.Sleep(150 * time.Millisecond)

	// Third wake should succeed
	err = wr.RequestWake(ctx)
	if err != nil {
		t.Errorf("third RequestWake error = %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	h := net.Listen("tcp", ":0")
	defer h.Close()

	port := PortConfig{
		ContainerPort: 25565,
		Protocol:      "TCP",
		WakeProtocol:  "generic",
	}

	mockClient := &GameServerClient{
		name:      "test",
		namespace: "default",
	}
	waker := &WakeRequester{
		client:       mockClient,
		lastWakeTime: make(map[string]time.Time),
		mu:           &sync.Mutex{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listener should exit when context is cancelled
	errChan := make(chan error, 1)
	go handleTCPListener(ctx, h, port, waker, errChan)

	cancel()
	time.Sleep(100 * time.Millisecond)

	// Should not have written to errChan when cancelled
	select {
	case <-errChan:
		t.Error("listener should not report error on context cancellation")
	default:
	}
}

func BenchmarkParsePortsConfig(b *testing.B) {
	config := "25565:TCP:minecraft,19133:UDP:generic,25575:TCP:none,27015:UDP:terraria"
	for i := 0; i < b.N; i++ {
		parsePortsConfig(config)
	}
}

func BenchmarkUDPHeuristic(b *testing.B) {
	h := NewUDPHeuristic(3, 10*time.Second)
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.ShouldWake(addr)
	}
}
