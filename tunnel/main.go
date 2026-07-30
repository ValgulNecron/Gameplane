// Command tunnel is a relay client supervisor that runs as a pod and
// configures/starts/supervises a third-party relay process (frp, Tailscale, playit).
// The binary picks its behavior from the TUNNEL_TYPE env var and handles provider-specific
// config rendering, credential management, and child process supervision.
//
// Configuration and contract (see operator/internal/controller/gameserver_tunnel.go
// on the operator side, which is authoritative):
//
//   - Env: GAMESERVER_NAME, GAMESERVER_NAMESPACE, TUNNEL_TYPE (frp|tailscale|playit).
//   - frp: FRP_SERVER_ADDR, FRP_SERVER_PORT, BACKING_SERVICE_DNS, BACKING_SERVICE_PORT.
//   - tailscale: TAILSCALE_HOSTNAME, TAILSCALE_TAGS, BACKING_SERVICE_DNS, BACKING_SERVICE_PORTS.
//   - playit: PLAYIT_TUNNEL_NAME, BACKING_SERVICE_DNS, BACKING_SERVICE_PORTS.
//   - Credentials Secret is mounted read-only at /etc/gameplane/tunnel-auth.
//   - Credential key names: frp uses "token", tailscale uses "authKey", playit uses "secretKey".
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Version is overridden at build time via -ldflags (see Dockerfile).
var Version = "dev"

const (
	tunnelAuthMountDir = "/etc/gameplane/tunnel-auth"
)

func main() {
	relayBinary := flag.String("relay-binary", "", "path to the relay binary (defaults per provider)")
	flag.Parse()

	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Printf("tunnel %s starting for %s/%s (provider=%s)", Version, cfg.GameServerNamespace, cfg.GameServerName, cfg.TunnelType)

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down", sig)
		cancel()
	}()

	if err := run(ctx, cfg, *relayBinary); err != nil {
		log.Fatalf("tunnel exiting: %v", err)
	}
	cancel()
	signal.Stop(sigCh)
}

// Config holds the parsed configuration for the tunnel supervisor.
type Config struct {
	GameServerName      string
	GameServerNamespace string
	TunnelType          string // "frp", "tailscale", or "playit"

	// FRP-specific
	FrpServerAddr     string
	FrpServerPort     int
	BackingServiceDNS string
	BackingServicePort string // "name:port,..." format

	// Tailscale-specific
	TailscaleHostname string
	TailscaleTags    string // comma-separated
	BackingServicePorts string // "name:port,..." format

	// Playit-specific
	PlayitTunnelName string
	// BackingServicePorts shared with Tailscale
}

// loadConfig reads and validates the tunnel's configuration via getenv.
func loadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		GameServerName:      getenv("GAMESERVER_NAME"),
		GameServerNamespace: getenv("GAMESERVER_NAMESPACE"),
		TunnelType:          getenv("TUNNEL_TYPE"),
		BackingServiceDNS:   getenv("BACKING_SERVICE_DNS"),
	}

	if cfg.GameServerName == "" {
		return Config{}, errors.New("GAMESERVER_NAME is required")
	}
	if cfg.GameServerNamespace == "" {
		return Config{}, errors.New("GAMESERVER_NAMESPACE is required")
	}
	if cfg.TunnelType == "" {
		return Config{}, errors.New("TUNNEL_TYPE is required")
	}
	if cfg.BackingServiceDNS == "" {
		return Config{}, errors.New("BACKING_SERVICE_DNS is required")
	}

	switch cfg.TunnelType {
	case "frp":
		cfg.FrpServerAddr = getenv("FRP_SERVER_ADDR")
		if cfg.FrpServerAddr == "" {
			return Config{}, errors.New("FRP_SERVER_ADDR is required for frp provider")
		}

		portStr := getenv("FRP_SERVER_PORT")
		if portStr == "" {
			cfg.FrpServerPort = 7000
		} else {
			port, err := strconv.Atoi(portStr)
			if err != nil || port < 1 || port > 65535 {
				return Config{}, fmt.Errorf("invalid FRP_SERVER_PORT: %q", portStr)
			}
			cfg.FrpServerPort = port
		}

		cfg.BackingServicePort = getenv("BACKING_SERVICE_PORT")
		if cfg.BackingServicePort == "" {
			return Config{}, errors.New("BACKING_SERVICE_PORT is required for frp provider")
		}

	case "tailscale":
		cfg.TailscaleHostname = getenv("TAILSCALE_HOSTNAME")
		if cfg.TailscaleHostname == "" {
			return Config{}, errors.New("TAILSCALE_HOSTNAME is required for tailscale provider")
		}
		cfg.TailscaleTags = getenv("TAILSCALE_TAGS")

		cfg.BackingServicePorts = getenv("BACKING_SERVICE_PORTS")
		if cfg.BackingServicePorts == "" {
			return Config{}, errors.New("BACKING_SERVICE_PORTS is required for tailscale provider")
		}

	case "playit":
		cfg.PlayitTunnelName = getenv("PLAYIT_TUNNEL_NAME")
		if cfg.PlayitTunnelName == "" {
			return Config{}, errors.New("PLAYIT_TUNNEL_NAME is required for playit provider")
		}

		cfg.BackingServicePorts = getenv("BACKING_SERVICE_PORTS")
		if cfg.BackingServicePorts == "" {
			return Config{}, errors.New("BACKING_SERVICE_PORTS is required for playit provider")
		}

	default:
		return Config{}, fmt.Errorf("unsupported TUNNEL_TYPE: %q", cfg.TunnelType)
	}

	return cfg, nil
}

// run supervises the relay process until ctx is cancelled or the process
// exits with an unrecoverable error (e.g., bad credentials, missing config).
func run(ctx context.Context, cfg Config, relayBinaryOverride string) error {
	// Read the provider's credentials from the mounted Secret.
	creds, err := readCredentials(cfg)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}

	// Render the provider's config file.
	configFile, err := renderConfig(cfg, creds)
	if err != nil {
		return fmt.Errorf("render config: %w", err)
	}
	defer func() {
		if configFile != "" {
			_ = os.Remove(configFile)
		}
	}()

	// Determine the relay binary path.
	relayBinary := relayBinaryOverride
	if relayBinary == "" {
		relayBinary = relayBinaryDefault(cfg.TunnelType)
	}

	// Supervise the relay process with exponential backoff on exit.
	backoff := &exponentialBackoff{}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		cmd := buildCommand(cfg, relayBinary, configFile, creds)
		log.Printf("starting %s relay process", cfg.TunnelType)

		err := runCommand(ctx, cmd)
		if err != nil && ctx.Err() != nil {
			// Context was cancelled; shut down cleanly.
			return nil
		}

		if err != nil {
			log.Printf("relay process exited: %v", err)
			// Check if this is an unrecoverable error (e.g., bad config, missing credentials).
			if isUnrecoverable(cfg, err) {
				return fmt.Errorf("unrecoverable error: %w", err)
			}

			// Transient failure; sleep before restarting.
			delay := backoff.next()
			log.Printf("restarting relay in %v", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil
			}
			continue
		}
	}
}

// readCredentials reads the provider's credential from the mounted Secret directory.
func readCredentials(cfg Config) (string, error) {
	var keyName string
	switch cfg.TunnelType {
	case "frp":
		keyName = "token"
	case "tailscale":
		keyName = "authKey"
	case "playit":
		keyName = "secretKey"
	default:
		return "", fmt.Errorf("unknown tunnel type: %q", cfg.TunnelType)
	}

	path := fmt.Sprintf("%s/%s", tunnelAuthMountDir, keyName)
	data, err := os.ReadFile(path)
	if err != nil {
		// Don't log the file contents; log only the path.
		return "", fmt.Errorf("read credential from %s: %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// renderConfig generates the provider-specific config file and returns its path.
// The caller is responsible for cleaning it up.
//
// TODO: addAddressReporter is the extension point for playit address discovery.
// Once playit reports a discovered address (via gameservers/status or similar),
// a provider-specific reporter should be invoked here. The exact mechanism
// (interface, callback, status update) is TBD.
func renderConfig(cfg Config, credential string) (string, error) {
	switch cfg.TunnelType {
	case "frp":
		return renderFrpConfig(cfg, credential)
	case "tailscale":
		return renderTailscaleConfig(cfg, credential)
	case "playit":
		return renderPlayitConfig(cfg, credential)
	default:
		return "", fmt.Errorf("unknown tunnel type: %q", cfg.TunnelType)
	}
}

// renderFrpConfig generates an frpc config file.
func renderFrpConfig(cfg Config, token string) (string, error) {
	f, err := os.CreateTemp("", "frpc-*.toml")
	if err != nil {
		return "", fmt.Errorf("create frpc config file: %w", err)
	}
	defer f.Close()

	// Build the frpc config: server address/port, auth token, and per-port proxies.
	config := fmt.Sprintf(`
serverAddr = "%s"
serverPort = %d
auth.method = "token"
auth.token = "%s"
`, cfg.FrpServerAddr, cfg.FrpServerPort, escapeTomlString(token))

	// Parse BACKING_SERVICE_PORT format: "name:port,name:port,..."
	// and create a proxy section for each.
	for _, entry := range strings.Split(cfg.BackingServicePort, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid port mapping: %q", entry)
		}
		name := strings.TrimSpace(parts[0])
		port := strings.TrimSpace(parts[1])

		config += fmt.Sprintf(`
[[proxies]]
name = "%s"
type = "tcp"
localIP = "%s"
localPort = %s
remotePort = %s
`, name, cfg.BackingServiceDNS, port, port)
	}

	if _, err := f.WriteString(config); err != nil {
		return "", fmt.Errorf("write frpc config: %w", err)
	}

	return f.Name(), nil
}

// renderTailscaleConfig generates a Tailscale config/auth setup.
// For Tailscale, we write the auth key to a temporary file that tailscaled
// can read during initialization.
func renderTailscaleConfig(cfg Config, authKey string) (string, error) {
	f, err := os.CreateTemp("", "tailscale-auth-*")
	if err != nil {
		return "", fmt.Errorf("create tailscale auth file: %w", err)
	}
	defer f.Close()

	// Tailscale expects the auth key to be written to a file.
	// The tailscaled daemon reads this during startup and clears it afterward.
	if _, err := f.WriteString(authKey); err != nil {
		return "", fmt.Errorf("write tailscale auth key: %w", err)
	}

	return f.Name(), nil
}

// renderPlayitConfig generates a playit config file.
func renderPlayitConfig(cfg Config, secretKey string) (string, error) {
	f, err := os.CreateTemp("", "playit-*.toml")
	if err != nil {
		return "", fmt.Errorf("create playit config file: %w", err)
	}
	defer f.Close()

	// Build the playit config with the secret key and port mappings.
	config := fmt.Sprintf(`
secretKey = "%s"
tunnelName = "%s"
`, escapeTomlString(secretKey), escapeTomlString(cfg.PlayitTunnelName))

	// Parse BACKING_SERVICE_PORTS format: "name:port,name:port,..."
	for _, entry := range strings.Split(cfg.BackingServicePorts, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid port mapping: %q", entry)
		}
		name := strings.TrimSpace(parts[0])
		port := strings.TrimSpace(parts[1])

		config += fmt.Sprintf(`
[[portForward]]
name = "%s"
localPort = %s
`, escapeTomlString(name), port)
	}

	if _, err := f.WriteString(config); err != nil {
		return "", fmt.Errorf("write playit config: %w", err)
	}

	return f.Name(), nil
}

// escapeTomlString escapes special characters in a TOML string value.
func escapeTomlString(s string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
	).Replace(s)
}

// relayBinaryDefault returns the default path for a relay binary by provider.
func relayBinaryDefault(tunnelType string) string {
	switch tunnelType {
	case "frp":
		return "/usr/local/bin/frpc"
	case "tailscale":
		return "/usr/local/bin/tailscaled"
	case "playit":
		return "/usr/local/bin/playit"
	default:
		return ""
	}
}

// buildCommand constructs the command to run the relay process.
// For tailscale, the credential (auth key) must be passed via TS_AUTHKEY env var.
func buildCommand(cfg Config, relayBinary, configFile, credential string) *exec.Cmd {
	switch cfg.TunnelType {
	case "frp":
		return exec.Command(relayBinary, "-c", configFile)
	case "tailscale":
		// Tailscale uses tailscaled (daemon) with flags rather than a config file.
		// The pod's hardened securityContext (no NET_ADMIN, no /dev/net/tun device)
		// requires userspace networking mode via --tun=userspace-networking flag.
		cmd := exec.Command(relayBinary, "--tun=userspace-networking", "--state=/tmp/tailscale.state")
		// Pass the auth key to tailscaled via the TS_AUTHKEY environment variable.
		cmd.Env = append(os.Environ(), "TS_AUTHKEY="+credential)
		return cmd
	case "playit":
		return exec.Command(relayBinary, "-c", configFile)
	default:
		return nil
	}
}

// runCommand executes the relay command and waits for it to exit or ctx to cancel.
func runCommand(ctx context.Context, cmd *exec.Cmd) error {
	// Inherit stdio so relay logs flow through to the pod's stdout/stderr.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start relay: %w", err)
	}

	// Wait for either the process to exit or ctx to cancel.
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// Context cancelled; send SIGTERM to the child process and wait for it.
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
		<-done // Wait for the process to actually exit after SIGTERM.
		return ctx.Err()
	}
}

// isUnrecoverable reports whether an error is unrecoverable (e.g., bad config,
// missing credentials) vs. transient (e.g., network failure, relay unavailable).
// Heuristic: if the error looks like a permission/config issue, it's unrecoverable;
// otherwise assume it's transient and worth retrying.
func isUnrecoverable(_ Config, err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Exit code 126 or 127: permission denied or command not found (unrecoverable).
	if strings.Contains(errStr, "exit status 126") || strings.Contains(errStr, "exit status 127") {
		return true
	}
	// Permission denied reading credentials or config: unrecoverable.
	if strings.Contains(errStr, "permission denied") {
		return true
	}
	return false
}

// exponentialBackoff tracks restart delay with capped exponential backoff.
type exponentialBackoff struct {
	mu      sync.Mutex
	retries int
}

// next returns the next backoff duration and increments the retry counter.
func (b *exponentialBackoff) next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.retries++

	// Exponential backoff: 1s, 2s, 4s, 8s, 16s, capped at 5 minutes + jitter.
	base := 1 << uint(b.retries-1)
	if base > 300 {
		base = 300
	}
	// Add jitter: ±10% to avoid thundering herd on mass restart.
	jitter := time.Duration(rand.Intn(base/5 + 1))
	return time.Duration(base)*time.Second + jitter - time.Duration(base/10)*time.Second
}
