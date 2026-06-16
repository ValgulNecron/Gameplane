package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kestrel-gg/kestrel/api/internal/audit"
	"github.com/kestrel-gg/kestrel/api/internal/auth"
	"github.com/kestrel-gg/kestrel/api/internal/db"
	"github.com/kestrel-gg/kestrel/api/internal/handlers"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
	"github.com/kestrel-gg/kestrel/api/internal/rbac"
	"github.com/kestrel-gg/kestrel/api/internal/telemetry"
	"github.com/kestrel-gg/kestrel/api/internal/ws"
)

var Version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Subcommand dispatch. The legacy entrypoint (no arg / leading flag)
	// remains the serve path so existing manifests don't break.
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "serve":
			args = args[1:]
		case "bootstrap-admin":
			if err := bootstrapAdmin(ctx, args[1:], os.Stdin, os.Stderr); err != nil {
				logger.Error("bootstrap-admin", "err", err)
				os.Exit(1)
			}
			return
		default:
			logger.Error("unknown subcommand", "name", args[0])
			os.Exit(2)
		}
	}

	var cfg config
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfg.bindFlags(fs)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	store, err := db.Open(ctx, cfg.dbDriver, cfg.dbDSN)
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		logger.Error("migrate", "err", err)
		os.Exit(1)
	}

	restCfg, err := ctrl.GetConfig()
	if err != nil {
		logger.Error("get kubeconfig", "err", err)
		os.Exit(1)
	}
	k8s, err := kube.New(restCfg)
	if err != nil {
		logger.Error("kube client", "err", err)
		os.Exit(1)
	}

	sessions := auth.NewSessionStore(store)
	sessions.StartGC(ctx, time.Hour)
	localAuth := auth.NewLocal(store)
	oidcAuth, err := auth.NewOIDC(ctx, cfg.oidcIssuer, cfg.oidcClientID, cfg.oidcClientSecret, cfg.oidcRedirectURL)
	if err != nil && cfg.oidcIssuer != "" {
		logger.Warn("oidc disabled", "err", err)
	}

	auditor := audit.New(store)

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, middleware.RealIP)
	r.Use(secureHeaders)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(bodyLimit(1 << 20)) // 1 MiB default; upload proxy raises its own ceiling
	r.Use(audit.Middleware(auditor))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	r.Handle("/metrics", promhttp.Handler())

	// Public auth routes
	r.Route("/auth", func(r chi.Router) {
		// Pre-auth: which login methods are enabled (local always; OIDC
		// only when configured). No version/host/count — login-privacy.
		r.Get("/providers", handlers.AuthProvidersHandler(oidcAuth != nil, cfg.oidcDisplayName))
		// Rate-limit /login specifically — argon2id is ~200 ms of
		// single-core CPU per attempt, so an unlimited path invites a
		// trivial DoS.
		r.With(auth.LoginLimiter.Middleware).Post("/login", localAuth.HandleLogin(sessions))
		r.Post("/logout", sessions.HandleLogout())
		if oidcAuth != nil {
			r.Get("/oidc/start", oidcAuth.HandleStart())
			// Callback triggers an IdP token exchange + DB write per hit;
			// cap it so a misbehaving client (or a redirect loop) can't
			// flood either.
			r.With(auth.OIDCCallbackLimiter.Middleware).Get("/oidc/callback", oidcAuth.HandleCallback(sessions))
		}
	})

	// Protected API
	r.Group(func(p chi.Router) {
		p.Use(sessions.Authenticate)
		// A broad per-IP cap on writes keeps one authenticated client from
		// pegging the DB or k8s API. Reads go through unlimited (the main
		// bottleneck there is the dashboard's own polling).
		p.Use(mutationRateLimit)
		p.Use(rbac.Middleware())

		handlers.MountResources(p, k8s)
		handlers.MountPodEvents(p, k8s)
		handlers.MountLifecycle(p, k8s)
		handlers.MountOwnership(p, k8s, store)
		handlers.MountUsers(p, store, sessions)
		handlers.MountRoles(p, store)
		handlers.MountAudit(p, auditor)
		handlers.MountConfig(p, store)
		handlers.MountCluster(p, k8s, store, Version)
		handlers.MountClusterActions(p, k8s, cfg.clusterOps)
		handlers.MountEvents(p, k8s)
		handlers.MountDestinations(p, k8s)
		handlers.MountModules(p, k8s, cfg.namespace)
		ws.Mount(p, k8s, cfg.agentCABundle, cfg.agentClientCert, cfg.agentClientKey)
	})

	// Opt-in, off-by-default anonymous usage telemetry. No-op unless an
	// endpoint is configured AND the admin enabled the sendMetrics toggle.
	go telemetry.New(store, k8s, cfg.telemetryEndpoint, Version, 24*time.Hour).Run(ctx)

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		// 64 KiB header ceiling blocks request-smuggling probes and
		// cheap header-flood DoS. Legitimate clients never need more.
		MaxHeaderBytes: 64 << 10,
	}

	go func() {
		logger.Info("api listening", "addr", cfg.addr, "version", Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}

type config struct {
	addr     string
	dbDriver string
	dbDSN    string

	oidcIssuer       string
	oidcClientID     string
	oidcClientSecret string
	oidcRedirectURL  string
	oidcDisplayName  string

	telemetryEndpoint string
	clusterOps        bool

	agentCABundle   string
	agentClientCert string
	agentClientKey  string

	namespace string
}

func (c *config) bindFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.addr, "addr", ":8000", "HTTP listen address")
	fs.StringVar(&c.dbDriver, "db-driver", envOr("KESTREL_DB_DRIVER", "sqlite"), "sqlite or postgres")
	fs.StringVar(&c.dbDSN, "db-dsn", envOr("KESTREL_DB_DSN", "file:/data/kestrel.db?_pragma=journal_mode(WAL)"), "DSN")
	fs.StringVar(&c.oidcIssuer, "oidc-issuer", envOr("KESTREL_OIDC_ISSUER", ""), "OIDC issuer URL")
	fs.StringVar(&c.oidcClientID, "oidc-client-id", envOr("KESTREL_OIDC_CLIENT_ID", ""), "OIDC client id")
	fs.StringVar(&c.oidcClientSecret, "oidc-client-secret", envOr("KESTREL_OIDC_CLIENT_SECRET", ""), "OIDC client secret")
	fs.StringVar(&c.oidcRedirectURL, "oidc-redirect-url", envOr("KESTREL_OIDC_REDIRECT_URL", ""), "OIDC redirect URL")
	fs.StringVar(&c.oidcDisplayName, "oidc-display-name", envOr("KESTREL_OIDC_DISPLAY_NAME", "Single sign-on"), "label for the OIDC login button (no hostname — shown pre-auth)")
	fs.StringVar(&c.telemetryEndpoint, "telemetry-endpoint", envOr("KESTREL_TELEMETRY_ENDPOINT", ""), "URL to POST anonymous usage metrics to (empty = telemetry off)")
	fs.BoolVar(&c.clusterOps, "cluster-ops", envOr("KESTREL_CLUSTER_OPS", "") == "true", "enable credential-minting cluster ops (Add node, Download kubeconfig)")
	fs.StringVar(&c.agentCABundle, "agent-ca-bundle", envOr("KESTREL_AGENT_CA", ""), "CA bundle validating agent server certs")
	fs.StringVar(&c.agentClientCert, "agent-client-cert", envOr("KESTREL_AGENT_CLIENT_CERT", ""), "client cert presented to agents")
	fs.StringVar(&c.agentClientKey, "agent-client-key", envOr("KESTREL_AGENT_CLIENT_KEY", ""), "client key presented to agents")
	fs.StringVar(&c.namespace, "namespace", envOr("KESTREL_NAMESPACE", "kestrel-system"),
		"namespace the control plane runs in (module upload ConfigMaps are stored here)")
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// bodyLimit wraps every request body in MaxBytesReader so a decoder
// that forgets its own cap still can't exhaust memory. The upload proxy
// path (/servers/*/files/upload) is exempt because it streams multi-MiB
// files to the agent — MaxBytesReader is *authoritative*: once wrapped
// at 1 MiB, re-wrapping at 64 MiB downstream doesn't raise the ceiling.
func bodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Body != nil && !isUploadPath(req.URL.Path) {
				req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
			}
			next.ServeHTTP(w, req)
		})
	}
}

// isUploadPath matches the routes that legitimately accept large bodies.
// Keep the list tight — anything else should be capped by bodyLimit.
func isUploadPath(path string) bool {
	// /servers/{name}/files/upload
	return strings.HasSuffix(path, "/files/upload") && strings.HasPrefix(path, "/servers/")
}

// secureHeaders sets hardening response headers on every API reply. The
// API only returns JSON, never HTML, so the CSP is locked down to
// "nothing renders" — if a bug ever caused an HTML reply to slip out,
// the browser would refuse to execute scripts or open iframes.
//
// Frame-ancestors 'none' is the modern replacement for X-Frame-Options;
// we set both for older browsers. HSTS is set with a long max-age plus
// includeSubDomains so the dashboard origin and any sub-origins stick
// to HTTPS on repeat visits.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h := w.Header()
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		next.ServeHTTP(w, req)
	})
}

// mutationRateLimit applies MutationLimiter to POST/PUT/PATCH/DELETE only.
// Authenticated reads are unrestricted (the dashboard polls at 5s on
// several tabs, which would burn through any reasonable bucket).
func mutationRateLimit(next http.Handler) http.Handler {
	mw := auth.MutationLimiter.Middleware(next)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			mw.ServeHTTP(w, req)
		default:
			next.ServeHTTP(w, req)
		}
	})
}
