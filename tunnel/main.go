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
//   - playit: PLAYIT_TUNNEL_NAME, BACKING_SERVICE_DNS, BACKING_SERVICE_PORTS. Validated for the
//     operator contract (and as a label for the future playit address-discovery reporter, see the
//     TODO on renderConfig below), but NOT passed to playitd: playitd has no local config file for
//     port forwards -- those are managed against the account tied to the secret key via the
//     playit.gg dashboard/API, not by this supervisor.
//   - Credentials Secret is mounted read-only at /etc/gameplane/tunnel-auth.
//   - Credential key names: frp uses "token", tailscale uses "authKey", playit uses "secretKey".
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

// credentialsDir is the directory holding the mounted Secret's credential
// files. It defaults to the real mount point; it is a package-level
// variable, rather than a constant, solely so tests can repoint it at a
// t.TempDir() and exercise readCredentials' real success/error paths
// without depending on the container filesystem. Production code never
// changes it from tunnelAuthMountDir.
var credentialsDir = tunnelAuthMountDir

// allowedRelayBinaries is the closed set of relay binary paths the
// supervisor is permitted to exec, keyed by TUNNEL_TYPE. This defends
// against a poisoned -relay-binary flag (or future env-driven override)
// causing the supervisor to exec an arbitrary path.
// Note on "playit": the daemon binary is playitd (confirmed against
// playit-cloud/playit-agent's own Dockerfile, which installs it at exactly
// this path and execs it in the foreground as "playitd --secret ...
// --platform-docker"). playit-cli (historically installed here as "playit")
// is a *service manager* around playitd -- it has no flag to run a tunnel
// in the foreground under this file's exec+Wait supervision model, so it is
// not a viable relayBinary for this supervisor.
var allowedRelayBinaries = map[string]string{
	"frp":       "/usr/local/bin/frpc",
	"tailscale": "/usr/local/bin/tailscaled",
	"playit":    "/usr/local/bin/playitd",
}

// allowedCredentialKeys is the closed set of credential file names the
// supervisor will read, keyed by TUNNEL_TYPE.
var allowedCredentialKeys = map[string]string{
	"frp":       "token",
	"tailscale": "authKey",
	"playit":    "secretKey",
}

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
	FrpServerAddr      string
	FrpServerPort      int
	BackingServiceDNS  string
	BackingServicePort string // "name:port,..." format

	// Tailscale-specific
	TailscaleHostname   string
	TailscaleTags       string // comma-separated
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
	relayBinary, err = validateRelayBinary(cfg.TunnelType, relayBinary)
	if err != nil {
		return fmt.Errorf("relay binary: %w", err)
	}

	// Supervise the relay process with exponential backoff on exit.
	backoff := &exponentialBackoff{}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		cmd := buildCommand(ctx, cfg, relayBinary, configFile)
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
	keyName, ok := allowedCredentialKeys[cfg.TunnelType]
	if !ok {
		return "", fmt.Errorf("unknown tunnel type: %q", cfg.TunnelType)
	}

	dir := filepath.Clean(credentialsDir)
	path := filepath.Join(dir, keyName)

	// Defense in depth: keyName is drawn from the closed set above, but
	// confirm the resolved path did not escape the mount directory before
	// opening it, so a variable can never be used to read an arbitrary file.
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("credential path %q escapes mount directory %q", path, dir)
	}

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
		return renderTailscaleConfig(cfg.TailscaleHostname, credential)
	case "playit":
		return renderPlayitConfig(credential)
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
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close frpc config file: %w", err)
	}

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

	if err := os.WriteFile(path, []byte(config), 0o600); err != nil {
		return "", fmt.Errorf("write frpc config: %w", err)
	}

	return path, nil
}

// tailscaledConfig mirrors (the subset we use of) tailscaled's own
// ConfigVAlpha struct (ipn/conf.go in tailscale/tailscale), consumed via
// `tailscaled --config=<path>`. This is tailscaled's documented declarative
// config file (https://tailscale.com/docs/reference/tailscaled/tailescaled-config-file);
// "alpha0" is the only version value it currently accepts, and the docs
// note the schema may still change. It is what lets a bare `tailscaled`
// process (no containerboot, no separate `tailscale up`) authenticate and
// set its hostname in a single exec: containerboot's TS_AUTHKEY env var and
// `tailscale up --hostname=` are containerboot/CLI conveniences that build
// on top of this same mechanism, but neither is read by tailscaled itself,
// which is why the previous TS_AUTHKEY env var and --hostname flag here
// were both no-ops (the latter would have made tailscaled reject the flag
// and exit immediately).
type tailscaledConfig struct {
	Version  string `json:"version"`
	AuthKey  string `json:"authKey,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// renderTailscaleConfig generates tailscaled's declarative config file
// (see tailscaledConfig) containing the auth key and, if set, the hostname.
func renderTailscaleConfig(hostname, authKey string) (string, error) {
	cfg := tailscaledConfig{
		Version:  "alpha0",
		AuthKey:  authKey,
		Hostname: hostname,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal tailscaled config: %w", err)
	}

	f, err := os.CreateTemp("", "tailscaled-config-*.json")
	if err != nil {
		return "", fmt.Errorf("create tailscaled config file: %w", err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close tailscaled config file: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write tailscaled config: %w", err)
	}

	return path, nil
}

// renderPlayitConfig writes the secret key to a file for playitd's
// --secret-path flag.
//
// playitd (confirmed against playit-cloud/playit-agent's own source: the
// playitd CLI in packages/playitd/src/bin/playitd.rs takes only --secret,
// --secret-path, --socket-path, --log-path, --platform-docker, and
// --version-overrides) has no local config file for port forwards or a
// tunnel name -- those are configured against the account tied to this
// secret key via the playit.gg dashboard/API, not by this supervisor. The
// PlayitTunnelName and BackingServicePorts fields on Config are
// intentionally not used here; see the package doc comment.
//
// --secret-path (rather than passing the key inline via --secret) is used
// so the secret never appears in the process's argv, which is readable by
// anything that can see /proc/<pid>/cmdline in the pod.
func renderPlayitConfig(secretKey string) (string, error) {
	f, err := os.CreateTemp("", "playit-secret-*")
	if err != nil {
		return "", fmt.Errorf("create playit secret file: %w", err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close playit secret file: %w", err)
	}

	if err := os.WriteFile(path, []byte(secretKey), 0o600); err != nil {
		return "", fmt.Errorf("write playit secret: %w", err)
	}

	return path, nil
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
	return allowedRelayBinaries[tunnelType]
}

// validateRelayBinary confirms path is the expected, allow-listed relay
// binary for tunnelType, rejecting anything else. This is what stands
// between a poisoned -relay-binary flag and an arbitrary exec: since the
// supervisor only ever needs to run one of three known binaries, there is
// no legitimate case where the resolved path should be anything else.
func validateRelayBinary(tunnelType, path string) (string, error) {
	want, ok := allowedRelayBinaries[tunnelType]
	if !ok {
		return "", fmt.Errorf("unknown tunnel type: %q", tunnelType)
	}
	clean := filepath.Clean(path)
	if clean != want {
		return "", fmt.Errorf("relay binary %q is not the allowed path for provider %q (want %q)", path, tunnelType, want)
	}
	return clean, nil
}

// buildCommand constructs the command to run the relay process. All three
// providers take their secret via the rendered configFile (see
// renderConfig) rather than a raw credential argument here -- frpc's TOML
// embeds the token, tailscaled's declarative config JSON embeds the auth
// key and hostname, and playitd's --secret-path points straight at the
// secret file -- so buildCommand itself never needs the raw credential.
//
// The command is tied to ctx so cancellation stops the child (see
// runCommand, which overrides the default Cancel behavior to signal
// SIGTERM rather than SIGKILL).
func buildCommand(ctx context.Context, cfg Config, relayBinary, configFile string) *exec.Cmd {
	switch cfg.TunnelType {
	case "frp":
		// Confirmed against fatedier/frp's cmd/frpc/sub/root.go: "-c"/"--config"
		// is frpc's real config-file flag.
		return exec.CommandContext(ctx, relayBinary, "-c", configFile)
	case "tailscale":
		// Tailscale uses tailscaled (daemon) with flags rather than a config file.
		// The pod's hardened securityContext (no NET_ADMIN, no /dev/net/tun device)
		// requires userspace networking mode via --tun=userspace-networking flag.
		// --config points at the declarative config file (see tailscaledConfig)
		// carrying the auth key and hostname. All three flags confirmed against
		// tailscale/tailscale's cmd/tailscaled/tailscaled.go flag definitions.
		return exec.CommandContext(ctx, relayBinary,
			"--tun=userspace-networking",
			"--state=/tmp/tailscale.state",
			"--config="+configFile,
		)
	case "playit":
		// Confirmed against playit-cloud/playit-agent's packages/playitd/src/bin/playitd.rs
		// and its own Dockerfile/docker/entrypoint.sh, which exec exactly this
		// (playitd --secret ... --platform-docker) as the foreground process in
		// their official container image.
		return exec.CommandContext(ctx, relayBinary, "--secret-path", configFile, "--platform-docker")
	default:
		return nil
	}
}

// runCommand executes the relay command and waits for it to exit or ctx to cancel.
//
// cmd must have been built with exec.CommandContext against the same ctx
// (see buildCommand). By default that arranges to SIGKILL the child the
// instant ctx is done; that's too abrupt for a relay process, so this
// overrides Cancel to send SIGTERM instead and gives it WaitDelay to exit
// cleanly before exec falls back to a hard kill.
func runCommand(ctx context.Context, cmd *exec.Cmd) error {
	// Inherit stdio so relay logs flow through to the pod's stdout/stderr.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = 10 * time.Second

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start relay: %w", err)
	}

	err := cmd.Wait()
	if ctx.Err() != nil {
		// Context was cancelled; cmd.Cancel already signalled the child and
		// exec.Cmd waited (up to WaitDelay) for it to exit.
		return ctx.Err()
	}
	return err
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

	// Exponential backoff: 1s, 2s, 4s, 8s, 16s, capped at 5 minutes.
	//
	// No jitter: jitter exists to spread out a thundering herd of many
	// replicas reconnecting to the same endpoint at once. This supervisor
	// runs as a single-replica Deployment per GameServer talking to its own
	// dedicated relay connection, so there's no herd here to spread out,
	// and adding one would only make restart timing less predictable to
	// operators reading logs.
	base := 1 << uint(b.retries-1)
	if base > 300 {
		base = 300
	}
	return time.Duration(base) * time.Second
}
