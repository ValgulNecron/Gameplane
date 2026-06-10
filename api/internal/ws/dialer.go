// Package ws proxies WebSocket connections from the dashboard to the
// agent running in a game pod. Auth to the agent is via mTLS (the API
// loads its client cert from a Secret mounted by the Helm chart).
package ws

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/kestrel-gg/kestrel/api/internal/httperr"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
	"github.com/kestrel-gg/kestrel/api/internal/scope"
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
	// PTY console attaches via the Kubernetes API (not the agent), so it
	// doesn't need mTLS material — it uses the API's existing in-cluster
	// kubeconfig. Mounted unconditionally.
	mountAttach(r, k)
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
	})
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
	return fmt.Sprintf("%s-agent.%s.svc.cluster.local:8090", name, namespace)
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

func (p *proxy) httpProxy(agentPath string) http.HandlerFunc {
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

		// Cap upstream body the same way we cap our own. 64 MiB is
		// chosen to allow modest uploads through the file-browser
		// proxy while blocking unbounded spam.
		upReq, err := http.NewRequestWithContext(
			req.Context(), req.Method, upstream,
			http.MaxBytesReader(w, req.Body, 64<<20),
		)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		// Forward only headers the agent actually consumes. Never proxy
		// Cookie, Authorization, X-Kestrel-CSRF — those are the user's
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
