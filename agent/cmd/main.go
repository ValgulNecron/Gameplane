package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ValgulNecron/gameplane/agent/internal/actions"
	"github.com/ValgulNecron/gameplane/agent/internal/auth"
	"github.com/ValgulNecron/gameplane/agent/internal/caps"
	"github.com/ValgulNecron/gameplane/agent/internal/console"
	"github.com/ValgulNecron/gameplane/agent/internal/files"
	"github.com/ValgulNecron/gameplane/agent/internal/heartbeat"
	"github.com/ValgulNecron/gameplane/agent/internal/lifecycle"
	"github.com/ValgulNecron/gameplane/agent/internal/logs"
	"github.com/ValgulNecron/gameplane/agent/internal/mods"
	"github.com/ValgulNecron/gameplane/agent/internal/players"
	"github.com/ValgulNecron/gameplane/agent/internal/quiesce"
	"github.com/ValgulNecron/gameplane/agent/internal/rcon"
	"github.com/ValgulNecron/gameplane/agent/internal/status"
	"github.com/ValgulNecron/gameplane/agent/internal/usage"
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

func main() {
	var (
		addr         string
		dataRoot     string
		rconHost     string
		rconPort     int
		rconPassFile string
		rconEnabled  bool
		gameLogPath  string
		certFile     string
		keyFile      string
		clientCAFile string
		apiTokenFile string
		serverName   string
		templateName string
		gameName     string
		capsJSON     string
	)
	flag.StringVar(&addr, "addr", ":8090", "HTTP listen address")
	flag.StringVar(&dataRoot, "data-root", "/data", "path under which file ops are rooted")
	flag.StringVar(&rconHost, "rcon-host", "127.0.0.1", "RCON host (loopback in the pod)")
	flag.IntVar(&rconPort, "rcon-port", 25575, "RCON port")
	flag.StringVar(&rconPassFile, "rcon-password-file", "", "path to file holding the RCON password")
	flag.BoolVar(&rconEnabled, "rcon-enabled", envOr("GAMEPLANE_RCON_ENABLED", "true") != "false",
		"whether the game exposes RCON; when false, RCON-backed endpoints degrade instead of dialing")
	flag.StringVar(&gameLogPath, "game-log-path", "", "path to the game container's log file (for /logs/tail)")
	flag.StringVar(&certFile, "tls-cert", "", "server TLS cert (PEM). Enables HTTPS + requires client cert")
	flag.StringVar(&keyFile, "tls-key", "", "server TLS key (PEM)")
	flag.StringVar(&clientCAFile, "tls-client-ca", "", "CA that signs API client certs")
	flag.StringVar(&apiTokenFile, "api-token-file", "", "fallback shared-secret auth (used when TLS is not configured)")
	flag.StringVar(&serverName, "server-name", envOr("GAMEPLANE_SERVER_NAME", ""), "owning GameServer name")
	flag.StringVar(&templateName, "template", envOr("GAMEPLANE_TEMPLATE", ""), "GameTemplate name")
	flag.StringVar(&gameName, "game", envOr("GAMEPLANE_GAME", ""), "game identifier")
	flag.StringVar(&capsJSON, "capabilities", envOr("GAMEPLANE_CAPABILITIES", ""),
		"declared game capabilities (JSON, from GameTemplate spec.capabilities)")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	authCheck, err := auth.New(auth.Config{
		ClientCAFile: clientCAFile,
		TokenFile:    apiTokenFile,
	})
	if err != nil {
		logger.Error("auth setup", "err", err)
		os.Exit(1)
	}

	// Declared capabilities drive players/quiesce. A malformed blob
	// degrades to "nothing declared" (built-in fallbacks for known
	// games) rather than crashing the sidecar.
	capSpec, err := caps.Parse(capsJSON)
	if err != nil {
		logger.Warn("capabilities ignored", "err", err)
		capSpec = nil
	}
	var playerActions *caps.PlayerActions
	var quiesceSpec *caps.Quiesce
	var lifecycleSpec *caps.Lifecycle
	var actionSpecs []caps.ServerAction
	var statusSpec *caps.Status
	var modsSpec *caps.Mods
	if capSpec != nil {
		playerActions = capSpec.Players
		quiesceSpec = capSpec.Quiesce
		lifecycleSpec = capSpec.Lifecycle
		actionSpecs = capSpec.Actions
		statusSpec = capSpec.Status
		modsSpec = capSpec.Mods
	}

	var rconClient interface {
		Exec(cmd string) (string, error)
	} = rcon.New(rconHost, rconPort, rcon.PasswordFromFile(rconPassFile))
	if !rconEnabled {
		// The game declares no RCON (e.g. consoleMode pty/none). Dialing
		// 127.0.0.1:25575 would just fail on every request; the Disabled
		// client makes that explicit so handlers degrade cleanly.
		rconClient = rcon.Disabled{}
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/metrics", promhttp.Handler())

	r.Group(func(protected chi.Router) {
		protected.Use(authCheck.Middleware)
		files.Mount(protected, dataRoot)
		logs.Mount(protected, gameLogPath)
		console.Mount(protected, rconClient)
		players.Mount(protected, rconClient, gameName, playerActions)
		quiesce.Mount(protected, rconClient, gameName, quiesceSpec)
		lifecycle.Mount(protected, rconClient, gameName, lifecycleSpec)
		actions.Mount(protected, rconClient, gameName, actionSpecs)
		status.Mount(protected, rconClient, statusSpec)
		mods.Mount(protected, dataRoot, modsSpec)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if certFile != "" && keyFile != "" {
		tlsCfg, err := auth.ServerTLS(certFile, keyFile, clientCAFile)
		if err != nil {
			logger.Error("tls setup", "err", err)
			os.Exit(1)
		}
		srv.TLSConfig = tlsCfg
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Resource usage: in proc mode (the operator shares the pod's PID
	// namespace and sets GAMEPLANE_USAGE_PROC) the agent reports the game
	// process's CPU/memory from /proc and uses the operator-supplied limits
	// as the denominator; otherwise it falls back to its own cgroup.
	go heartbeat.Run(ctx, heartbeat.Config{
		ServerName: serverName,
		Template:   templateName,
		Game:       gameName,
		Version:    Version,
		RCON:       rconClient,
		Interval:   20 * time.Second,
		Usage: usage.New(usage.Config{
			DataDir:            dataRoot,
			ProcMode:           envOr("GAMEPLANE_USAGE_PROC", "") == "1",
			CPULimitMillicores: envInt("GAMEPLANE_CPU_LIMIT_MILLICORES"),
			MemLimitBytes:      envInt("GAMEPLANE_MEM_LIMIT_BYTES"),
		}),
	})

	go func() {
		logger.Info("agent listening", "addr", addr, "tls", srv.TLSConfig != nil, "version", Version)
		var err error
		if srv.TLSConfig != nil {
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// envInt parses an integer env var, returning 0 when unset or unparseable.
func envInt(key string) int64 {
	if v, err := strconv.ParseInt(os.Getenv(key), 10, 64); err == nil {
		return v
	}
	return 0
}
