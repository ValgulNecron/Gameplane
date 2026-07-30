package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// loadConfig tests
// -----------------------------------------------------------------------

func stubEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoadConfigFrp(t *testing.T) {
	cfg, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":       "my-server",
		"GAMESERVER_NAMESPACE":  "games",
		"TUNNEL_TYPE":           "frp",
		"FRP_SERVER_ADDR":       "frp.example.com",
		"FRP_SERVER_PORT":       "7000",
		"BACKING_SERVICE_DNS":   "my-server.games.svc",
		"BACKING_SERVICE_PORT":  "game:25565",
	}))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.GameServerName != "my-server" {
		t.Errorf("GameServerName = %q, want %q", cfg.GameServerName, "my-server")
	}
	if cfg.TunnelType != "frp" {
		t.Errorf("TunnelType = %q, want %q", cfg.TunnelType, "frp")
	}
	if cfg.FrpServerAddr != "frp.example.com" {
		t.Errorf("FrpServerAddr = %q, want %q", cfg.FrpServerAddr, "frp.example.com")
	}
	if cfg.FrpServerPort != 7000 {
		t.Errorf("FrpServerPort = %d, want %d", cfg.FrpServerPort, 7000)
	}
}

func TestLoadConfigFrpDefaultPort(t *testing.T) {
	cfg, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":      "my-server",
		"GAMESERVER_NAMESPACE": "games",
		"TUNNEL_TYPE":          "frp",
		"FRP_SERVER_ADDR":      "frp.example.com",
		"BACKING_SERVICE_DNS":  "my-server.games.svc",
		"BACKING_SERVICE_PORT": "game:25565",
	}))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.FrpServerPort != 7000 {
		t.Errorf("default FrpServerPort = %d, want %d", cfg.FrpServerPort, 7000)
	}
}

func TestLoadConfigTailscale(t *testing.T) {
	cfg, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":        "my-server",
		"GAMESERVER_NAMESPACE":   "games",
		"TUNNEL_TYPE":            "tailscale",
		"TAILSCALE_HOSTNAME":     "my-game",
		"TAILSCALE_TAGS":         "tag:gameplane,tag:game",
		"BACKING_SERVICE_DNS":    "my-server.games.svc",
		"BACKING_SERVICE_PORTS":  "game:25565",
	}))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.TunnelType != "tailscale" {
		t.Errorf("TunnelType = %q, want %q", cfg.TunnelType, "tailscale")
	}
	if cfg.TailscaleHostname != "my-game" {
		t.Errorf("TailscaleHostname = %q, want %q", cfg.TailscaleHostname, "my-game")
	}
	if cfg.TailscaleTags != "tag:gameplane,tag:game" {
		t.Errorf("TailscaleTags = %q, want %q", cfg.TailscaleTags, "tag:gameplane,tag:game")
	}
}

func TestLoadConfigPlayit(t *testing.T) {
	cfg, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":       "my-server",
		"GAMESERVER_NAMESPACE":  "games",
		"TUNNEL_TYPE":           "playit",
		"PLAYIT_TUNNEL_NAME":    "my-tunnel",
		"BACKING_SERVICE_DNS":   "my-server.games.svc",
		"BACKING_SERVICE_PORTS": "game:25565",
	}))
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.TunnelType != "playit" {
		t.Errorf("TunnelType = %q, want %q", cfg.TunnelType, "playit")
	}
	if cfg.PlayitTunnelName != "my-tunnel" {
		t.Errorf("PlayitTunnelName = %q, want %q", cfg.PlayitTunnelName, "my-tunnel")
	}
}

func TestLoadConfigMissingRequired(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "missing GAMESERVER_NAME",
			env: map[string]string{
				"GAMESERVER_NAMESPACE": "games",
				"TUNNEL_TYPE":          "frp",
			},
		},
		{
			name: "missing GAMESERVER_NAMESPACE",
			env: map[string]string{
				"GAMESERVER_NAME": "my-server",
				"TUNNEL_TYPE":     "frp",
			},
		},
		{
			name: "missing TUNNEL_TYPE",
			env: map[string]string{
				"GAMESERVER_NAME":      "my-server",
				"GAMESERVER_NAMESPACE": "games",
			},
		},
		{
			name: "missing BACKING_SERVICE_DNS",
			env: map[string]string{
				"GAMESERVER_NAME":      "my-server",
				"GAMESERVER_NAMESPACE": "games",
				"TUNNEL_TYPE":          "frp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(stubEnv(tt.env))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestLoadConfigFrpMissingAddress(t *testing.T) {
	_, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":      "my-server",
		"GAMESERVER_NAMESPACE": "games",
		"TUNNEL_TYPE":          "frp",
		"BACKING_SERVICE_DNS":  "my-server.games.svc",
	}))
	if err == nil {
		t.Error("expected error for missing FRP_SERVER_ADDR")
	}
}

func TestLoadConfigFrpMissingPortMapping(t *testing.T) {
	_, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":      "my-server",
		"GAMESERVER_NAMESPACE": "games",
		"TUNNEL_TYPE":          "frp",
		"FRP_SERVER_ADDR":      "frp.example.com",
		"BACKING_SERVICE_DNS":  "my-server.games.svc",
	}))
	if err == nil {
		t.Error("expected error for missing BACKING_SERVICE_PORT")
	}
}

func TestLoadConfigInvalidFrpPort(t *testing.T) {
	tests := []struct {
		name     string
		portVal  string
		wantErr  bool
	}{
		{"valid", "7000", false},
		{"out of range", "70000", true},
		{"non-numeric", "abc", true},
		{"zero", "0", true},
		{"negative", "-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(stubEnv(map[string]string{
				"GAMESERVER_NAME":      "my-server",
				"GAMESERVER_NAMESPACE": "games",
				"TUNNEL_TYPE":          "frp",
				"FRP_SERVER_ADDR":      "frp.example.com",
				"FRP_SERVER_PORT":      tt.portVal,
				"BACKING_SERVICE_DNS":  "my-server.games.svc",
				"BACKING_SERVICE_PORT": "game:25565",
			}))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigUnsupportedTunnelType(t *testing.T) {
	_, err := loadConfig(stubEnv(map[string]string{
		"GAMESERVER_NAME":      "my-server",
		"GAMESERVER_NAMESPACE": "games",
		"TUNNEL_TYPE":          "invalid-provider",
		"BACKING_SERVICE_DNS":  "my-server.games.svc",
	}))
	if err == nil {
		t.Error("expected error for unsupported TUNNEL_TYPE")
	}
}

// -----------------------------------------------------------------------
// renderConfig tests
// -----------------------------------------------------------------------

func TestRenderFrpConfig(t *testing.T) {
	cfg := Config{
		GameServerName:      "my-server",
		GameServerNamespace: "games",
		TunnelType:          "frp",
		FrpServerAddr:       "frp.example.com",
		FrpServerPort:       7000,
		BackingServiceDNS:   "my-server.games.svc",
		BackingServicePort:  "game:25565",
	}

	path, err := renderFrpConfig(cfg, "test-token")
	if err != nil {
		t.Fatalf("renderFrpConfig() error = %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "serverAddr = \"frp.example.com\"") {
		t.Errorf("config missing serverAddr")
	}
	if !strings.Contains(content, "serverPort = 7000") {
		t.Errorf("config missing serverPort")
	}
	if !strings.Contains(content, "auth.token = \"test-token\"") {
		t.Errorf("config missing auth token")
	}
	if !strings.Contains(content, "localIP = \"my-server.games.svc\"") {
		t.Errorf("config missing backing service DNS")
	}
}

func TestRenderFrpConfigMultiplePorts(t *testing.T) {
	cfg := Config{
		FrpServerAddr:      "frp.example.com",
		FrpServerPort:      7000,
		BackingServiceDNS:  "my-server.games.svc",
		BackingServicePort: "java:25565,bedrock:19133",
	}

	path, err := renderFrpConfig(cfg, "token")
	if err != nil {
		t.Fatalf("renderFrpConfig() error = %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "name = \"java\"") {
		t.Errorf("config missing java port mapping")
	}
	if !strings.Contains(content, "name = \"bedrock\"") {
		t.Errorf("config missing bedrock port mapping")
	}
}

func TestRenderTailscaleConfig(t *testing.T) {
	cfg := Config{
		TailscaleHostname: "my-game",
		TailscaleTags:     "tag:gameplane",
	}

	path, err := renderTailscaleConfig(cfg, "test-auth-key")
	if err != nil {
		t.Fatalf("renderTailscaleConfig() error = %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	if string(data) != "test-auth-key" {
		t.Errorf("tailscale auth file content = %q, want %q", string(data), "test-auth-key")
	}
}

func TestRenderPlayitConfig(t *testing.T) {
	cfg := Config{
		PlayitTunnelName:    "my-tunnel",
		BackingServicePorts: "game:25565,query:19133",
	}

	path, err := renderPlayitConfig(cfg, "test-secret-key")
	if err != nil {
		t.Fatalf("renderPlayitConfig() error = %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "secretKey = \"test-secret-key\"") {
		t.Errorf("config missing secretKey")
	}
	if !strings.Contains(content, "tunnelName = \"my-tunnel\"") {
		t.Errorf("config missing tunnelName")
	}
	if !strings.Contains(content, "name = \"game\"") {
		t.Errorf("config missing game port")
	}
	if !strings.Contains(content, "name = \"query\"") {
		t.Errorf("config missing query port")
	}
}

func TestRenderConfigDispatchesByType(t *testing.T) {
	tests := []struct {
		tunnelType string
		cfg        Config
	}{
		{
			"frp",
			Config{
				TunnelType:         "frp",
				FrpServerAddr:      "frp.example.com",
				BackingServiceDNS:  "svc.svc",
				BackingServicePort: "game:25565",
			},
		},
		{
			"tailscale",
			Config{
				TunnelType:          "tailscale",
				TailscaleHostname:   "my-game",
				BackingServicePorts: "game:25565",
			},
		},
		{
			"playit",
			Config{
				TunnelType:          "playit",
				PlayitTunnelName:    "my-tunnel",
				BackingServicePorts: "game:25565",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.tunnelType, func(t *testing.T) {
			path, err := renderConfig(tt.cfg, "cred")
			if err != nil {
				t.Errorf("renderConfig() error = %v", err)
				return
			}
			defer os.Remove(path)
			if path == "" {
				t.Error("renderConfig() returned empty path")
			}
		})
	}
}

// -----------------------------------------------------------------------
// readCredentials tests
// -----------------------------------------------------------------------

func TestReadCredentialsFromFile(t *testing.T) {
	tmpdir := t.TempDir()

	// Create a mock credentials directory
	credPath := filepath.Join(tmpdir, "authKey")
	if err := os.WriteFile(credPath, []byte("test-auth-key"), 0o600); err != nil {
		t.Fatalf("write test credential: %v", err)
	}

	// Temporarily override tunnelAuthMountDir (can't be done from the test,
	// so we test the logic directly by testing readCredentials behavior).
	// This test verifies the error path when the credential file is missing.
	cfg := Config{TunnelType: "tailscale"}
	_, err := readCredentials(cfg)
	if err == nil || !strings.Contains(err.Error(), "read credential") {
		t.Errorf("readCredentials() error = %v, want credential read error", err)
	}
}

func TestReadCredentialsKeyNames(t *testing.T) {
	tests := []struct {
		tunnelType string
		expectedKey string
	}{
		{"frp", "token"},
		{"tailscale", "authKey"},
		{"playit", "secretKey"},
	}

	for _, tt := range tests {
		t.Run(tt.tunnelType, func(t *testing.T) {
			cfg := Config{TunnelType: tt.tunnelType}
			// We expect this to fail (file not found in test), but verify
			// the error mentions the expected key name.
			_, err := readCredentials(cfg)
			if err == nil {
				t.Skip("test environment does not have mounted credentials")
			}
			if !strings.Contains(err.Error(), tt.expectedKey) {
				t.Errorf("error should mention key name %q, got %v", tt.expectedKey, err)
			}
		})
	}
}

// -----------------------------------------------------------------------
// escapeTomlString tests
// -----------------------------------------------------------------------

func TestEscapeTomlString(t *testing.T) {
	tests := []struct {
		input  string
		want   string
	}{
		{"simple", "simple"},
		{"with\"quote", "with\\\"quote"},
		{"with\\slash", "with\\\\slash"},
		{"with\nnewline", "with\\nnewline"},
		{"with\ttab", "with\\ttab"},
		{"combined\\\"both", "combined\\\\\\\"both"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeTomlString(tt.input)
			if got != tt.want {
				t.Errorf("escapeTomlString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------
// relayBinaryDefault tests
// -----------------------------------------------------------------------

func TestRelayBinaryDefault(t *testing.T) {
	tests := []struct {
		tunnelType string
		want       string
	}{
		{"frp", "/usr/local/bin/frpc"},
		{"tailscale", "/usr/local/bin/tailscaled"},
		{"playit", "/usr/local/bin/playit"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.tunnelType, func(t *testing.T) {
			got := relayBinaryDefault(tt.tunnelType)
			if got != tt.want {
				t.Errorf("relayBinaryDefault(%q) = %q, want %q", tt.tunnelType, got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------
// exponentialBackoff tests
// -----------------------------------------------------------------------

func TestExponentialBackoff(t *testing.T) {
	b := &exponentialBackoff{}

	// First retry: 1s (base 1).
	d1 := b.next()
	if d1 < 900*time.Millisecond || d1 > 1100*time.Millisecond {
		t.Errorf("first backoff = %v, want ~1s", d1)
	}

	// Second retry: ~2s (base 2).
	d2 := b.next()
	if d2 < 1800*time.Millisecond || d2 > 2200*time.Millisecond {
		t.Errorf("second backoff = %v, want ~2s", d2)
	}

	// Backoff should be increasing.
	if d2 <= d1 {
		t.Errorf("second backoff %v should be > first %v", d2, d1)
	}
}

func TestExponentialBackoffCap(t *testing.T) {
	b := &exponentialBackoff{}

	// Advance to near the cap: 256 = 2^8, next would be 512.
	for i := 0; i < 10; i++ {
		b.next()
	}

	// Check that we're capped at 5 minutes.
	d := b.next()
	if d > 5*time.Minute+200*time.Millisecond {
		t.Errorf("backoff %v exceeds 5-minute cap", d)
	}
}

// -----------------------------------------------------------------------
// isUnrecoverable tests
// -----------------------------------------------------------------------

func TestIsUnrecoverable(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{"nil", nil, false},
		{"permission denied", errors.New("permission denied"), true},
		{"exit 126", errors.New("exit status 126"), true},
		{"exit 127", errors.New("exit status 127"), true},
		{"network error", errors.New("dial: connection refused"), false},
		{"generic", errors.New("something went wrong"), false},
	}

	cfg := Config{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnrecoverable(cfg, tt.err)
			if got != tt.want {
				t.Errorf("isUnrecoverable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// -----------------------------------------------------------------------
// run/supervision tests
// -----------------------------------------------------------------------

func TestRunContextCancellation(t *testing.T) {
	cfg := Config{
		GameServerName:      "test",
		GameServerNamespace: "games",
		TunnelType:          "frp",
		FrpServerAddr:       "localhost",
		FrpServerPort:       7000,
		BackingServiceDNS:   "test.games.svc",
		BackingServicePort:  "game:25565",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// With a cancelled context, run should return nil (clean shutdown).
	err := run(ctx, cfg, "nonexistent-binary")
	if err != nil {
		t.Errorf("run with cancelled context = %v, want nil", err)
	}
}

func TestRunUnrecoverableError(t *testing.T) {
	cfg := Config{
		GameServerName:      "test",
		GameServerNamespace: "games",
		TunnelType:          "frp",
		FrpServerAddr:       "localhost",
		FrpServerPort:       7000,
		BackingServiceDNS:   "test.games.svc",
		BackingServicePort:  "game:25565",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Pointing to a binary that doesn't exist (exit 127: command not found).
	err := run(ctx, cfg, "/nonexistent/path/binary")
	if err == nil {
		t.Error("run with nonexistent binary should return an error")
	}
}
