// Package ws proxies WebSocket connections from the dashboard to the
// agent running in a game pod. Auth to the agent is via mTLS (the API
// loads its client cert from a Secret mounted by the Helm chart).
package ws

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// Mount attaches the WS/file proxy routes under /ws and /servers/:name/files.
func Mount(r chi.Router, k *kube.Client, caBundle, clientCert, clientKey string) {
	tlsCfg, err := agentTLSConfig(caBundle, clientCert, clientKey)
	if err != nil {
		// Allow startup without mTLS in dev — every request 503s until
		// the chart's mTLS hook populates the Secrets.
		tlsCfg = nil
	}
	client := &http.Client{
		Timeout:   0,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}

	p := &proxy{k: k, tls: tlsCfg, http: client}
	r.Get("/ws/servers/{name}/console", p.wsProxy("/console"))
	r.Get("/ws/servers/{name}/logs", p.wsProxy("/logs/tail"))
	r.Get("/servers/{name}/logs/download", p.httpProxy("/logs/download"))
	// PTY console attaches via the Kubernetes API (not the agent), so it
	// doesn't need mTLS material — it uses the API's existing in-cluster
	// kubeconfig. Mounted unconditionally.
	mountAttach(r, k)
	// Startup logs stream the game container's stdout via the pod-log API
	// (also no agent mTLS needed), so download/config output is visible
	// before the game's own log file exists. Mounted unconditionally.
	mountPodLogs(r, k)
	r.Route("/servers/{name}/files", func(r chi.Router) {
		r.Get("/list", p.httpProxy("/files/list"))
		r.Get("/read", p.httpProxy("/files/read"))
		r.Get("/download", p.httpProxy("/files/download"))
		r.Post("/write", p.httpProxy("/files/write"))
		r.Post("/upload", p.httpProxy("/files/upload"))
		r.Post("/mkdir", p.httpProxy("/files/mkdir"))
		r.Delete("/delete", p.httpProxy("/files/delete"))
	})
	r.Route("/servers/{name}/players", func(r chi.Router) {
		r.Get("/", p.httpProxy("/players"))
		r.Get("/banned", p.httpProxy("/players/banned"))
		r.Post("/kick", p.httpProxy("/players/kick"))
		r.Post("/ban", p.httpProxy("/players/ban"))
		r.Post("/unban", p.httpProxy("/players/unban"))
		r.Get("/whitelist", p.httpProxy("/players/whitelist"))
		r.Post("/whitelist/add", p.httpProxy("/players/whitelist/add"))
		r.Post("/whitelist/remove", p.httpProxy("/players/whitelist/remove"))
	})
	// Module-declared operator actions and live status metrics. RBAC
	// (api/internal/rbac) gates these by the same method+segment rules as
	// the rest of /servers: the action run is a POST → operator+, while
	// the status read is a GET → viewer+.
	r.Post("/servers/{name}/actions/run", p.httpProxy("/actions/run"))
	r.Get("/servers/{name}/status", p.httpProxy("/status"))
	// Mod/plugin management. Listing is a GET → viewer+; install (POST)
	// and remove (DELETE) are mutations → operator+, by the same rbac
	// method+segment rules as the rest of /servers. Upload gets its own
	// body cap matching the largest module install policy (the agent still
	// enforces the module's real per-file limit).
	r.Get("/servers/{name}/mods", p.httpProxy("/mods"))
	r.Post("/servers/{name}/mods/install", p.httpProxy("/mods/install"))
	r.Post("/servers/{name}/mods/upload", p.httpProxyLimit("/mods/upload", 512<<20))
	r.Delete("/servers/{name}/mods", p.httpProxy("/mods"))
}

type proxy struct {
	k    *kube.Client
	tls  *tls.Config
	http *http.Client
}

// agentHost returns the in-cluster DNS + port of the agent sidecar.
// Kept as a method so tests can override it.
func (p *proxy) agentHost(name, namespace string) string {
	// The operator maintains a dedicated ClusterIP Service named
	// <gs>-agent for the sidecar (the game's own Service may be
	// NodePort/LoadBalancer and per-pod DNS only resolves under
	// headless Services). Agent listens on :8090.
	return agentHostFor(name, namespace)
}

func (p *proxy) wsProxy(agentPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if p.tls == nil {
			http.Error(w, "agent mTLS not configured", http.StatusServiceUnavailable)
			return
		}
		name := chi.URLParam(req, "name")
		ns, err := scope.Resolve(req)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		host := p.agentHost(name, ns)
		upstream := "wss://" + host + agentPath

		downConn, err := websocket.Accept(w, req, nil)
		if err != nil {
			return
		}
		defer downConn.Close(websocket.StatusNormalClosure, "")

		upConn, upResp, err := websocket.Dial(req.Context(), upstream, &websocket.DialOptions{
			HTTPClient: p.http,
		})
		if upResp != nil && upResp.Body != nil {
			_ = upResp.Body.Close()
		}
		if err != nil {
			// The upgrade to the browser already succeeded (Console.tsx
			// printed "— connected —"), so the close reason below isn't
			// visible to it. Send a structured error frame first, matching
			// the agent's {kind,body} envelope, so the console shows a real
			// message instead of a bare disconnect.
			_ = downConn.Write(req.Context(), websocket.MessageText,
				[]byte(`{"kind":"err","body":"agent unreachable"}`))
			_ = downConn.Close(websocket.StatusBadGateway, "agent dial failed")
			return
		}
		defer upConn.Close(websocket.StatusNormalClosure, "")

		// Bidirectional copy.
		errCh := make(chan error, 2)
		go func() { errCh <- copyWS(req, downConn, upConn) }()
		go func() { errCh <- copyWS(req, upConn, downConn) }()
		<-errCh
	}
}

func copyWS(req *http.Request, src, dst *websocket.Conn) error {
	for {
		typ, data, err := src.Read(req.Context())
		if err != nil {
			return err
		}
		if err := dst.Write(req.Context(), typ, data); err != nil {
			return err
		}
	}
}

// httpProxy forwards a request to the agent with the default 64 MiB body
// cap — enough for modest uploads through the file-browser proxy while
// blocking unbounded spam.
func (p *proxy) httpProxy(agentPath string) http.HandlerFunc {
	return p.httpProxyLimit(agentPath, 64<<20)
}

// httpProxyLimit is httpProxy with an explicit request-body cap, for the
// few routes that legitimately carry more (mod uploads).
func (p *proxy) httpProxyLimit(agentPath string, maxBody int64) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if p.tls == nil {
			http.Error(w, "agent mTLS not configured", http.StatusServiceUnavailable)
			return
		}
		name := chi.URLParam(req, "name")
		ns, err := scope.Resolve(req)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		host := p.agentHost(name, ns)
		upstream := "https://" + host + agentPath
		if req.URL.RawQuery != "" {
			upstream += "?" + req.URL.RawQuery
		}

		upReq, err := http.NewRequestWithContext(
			req.Context(), req.Method, upstream,
			http.MaxBytesReader(w, req.Body, maxBody),
		)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		// Forward only headers the agent actually consumes. Never proxy
		// Cookie, Authorization, X-Gameplane-CSRF — those are the user's
		// session material and the agent doesn't need them (mTLS is what
		// it authenticates on). Leaking them to agent logs or a
		// compromised sidecar would hand over live session tokens.
		copyProxyHeaders(upReq.Header, req.Header)
		resp, err := p.http.Do(upReq)
		if err != nil {
			writeUpstreamErr(w, req, err)
			return
		}
		defer resp.Body.Close()
		copyResponseHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

// writeUpstreamErr maps transport-level failures on the API→agent leg
// to gateway statuses so the dashboard can tell "agent down" (502/504)
// apart from API bugs (500). The dashboard's error handling and
// TestAPI_AgentUnreachable both rely on this distinction.
func writeUpstreamErr(w http.ResponseWriter, req *http.Request, err error) {
	status := http.StatusBadGateway
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		status = http.StatusGatewayTimeout
	}
	slog.Error("agent proxy upstream error",
		"method", req.Method, "path", req.URL.Path, "status", status, "err", err)
	http.Error(w, "agent unreachable", status)
}

// proxyHeaderAllowlist is the set of request headers forwarded from the
// dashboard-facing API to the in-cluster agent. Anything not in this
// set is dropped — most importantly Cookie, Authorization, and the CSRF
// header, which are user-session material the agent doesn't need.
var proxyHeaderAllowlist = map[string]bool{
	"Accept":              true,
	"Accept-Encoding":     true,
	"Content-Type":        true,
	"Content-Length":      true,
	"Content-Encoding":    true,
	"Content-Disposition": true,
	"Range":               true,
	"If-None-Match":       true,
	"If-Modified-Since":   true,
}

// responseHeaderDenylist strips hop-by-hop and auth-adjacent response
// headers from the agent before passing the body through to the browser.
var responseHeaderDenylist = map[string]bool{
	"Set-Cookie":         true,
	"Www-Authenticate":   true,
	"Proxy-Authenticate": true,
	"Connection":         true,
	"Keep-Alive":         true,
	"Transfer-Encoding":  true,
	"Upgrade":            true,
}

func copyProxyHeaders(dst, src http.Header) {
	for k, v := range src {
		if !proxyHeaderAllowlist[http.CanonicalHeaderKey(k)] {
			continue
		}
		dst[http.CanonicalHeaderKey(k)] = v
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for k, v := range src {
		if responseHeaderDenylist[http.CanonicalHeaderKey(k)] {
			continue
		}
		dst[http.CanonicalHeaderKey(k)] = v
	}
}

func agentTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	if caFile == "" || certFile == "" || keyFile == "" {
		return nil, errors.New("mTLS material missing")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("empty CA bundle")
	}
	// ServerName is left unset on purpose: the dialer derives it from
	// the wss:// URL's host, which is the pod FQDN and the authority
	// that the agent's server cert is issued for.
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
