// Package rcon implements remote console clients for dedicated game servers.
// rest.go implements a generic HTTP/JSON REST client for games exposing web
// or REST administrative console APIs (e.g. FiveM's txAdmin, Farming Simulator 25).
package rcon

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultRESTDialTimeout         = 5 * time.Second
	defaultRESTRequestTimeout      = 10 * time.Second
	defaultRESTAuthFailureCooldown = 15 * time.Second
	restMaxResponseBytes           = 1 << 20 // 1 MiB bounded read
)

// RESTAdapter handles endpoint-specific request formatting and response parsing
// across different game REST API implementations.
type RESTAdapter interface {
	Name() string
	BuildRequest(ctx context.Context, baseURL, cmd, credential string) (*http.Request, error)
	ParseResponse(resp *http.Response, body []byte) (string, error)
}

// RESTOption configures a REST client.
type RESTOption func(*REST)

// WithAdapter configures an explicit RESTAdapter.
func WithAdapter(adapter RESTAdapter) RESTOption {
	return func(r *REST) {
		if adapter != nil {
			r.adapter = adapter
		}
	}
}

// WithGame configures the game name to select the appropriate adapter.
func WithGame(game string) RESTOption {
	return func(r *REST) {
		lower := strings.ToLower(strings.TrimSpace(game))
		switch lower {
		case "fivem":
			r.adapter = &TxAdminAdapter{}
		case "farming-simulator-25", "fs25":
			r.adapter = &FarmingSimulatorAdapter{}
		}
	}
}

// WithCustomPath overrides the default request path on the adapter.
func WithCustomPath(path string) RESTOption {
	return func(r *REST) {
		r.customPath = path
	}
}

// WithScheme configures http vs https scheme.
func WithScheme(scheme string) RESTOption {
	return func(r *REST) {
		r.scheme = scheme
	}
}

// TxAdminAdapter adapts the generic REST client to FiveM txAdmin's HTTP API.
type TxAdminAdapter struct {
	CustomPath string
}

// Name returns the identifier of the txadmin adapter.
func (a *TxAdminAdapter) Name() string { return "txadmin" }

// BuildRequest constructs the HTTP request for FiveM txAdmin commands.
func (a *TxAdminAdapter) BuildRequest(ctx context.Context, baseURL, cmd, credential string) (*http.Request, error) {
	path := "/fxserver/commands"
	if a.CustomPath != "" {
		path = a.CustomPath
	}
	url := strings.TrimRight(baseURL, "/") + path

	payload := map[string]string{
		"action":    "console",
		"parameter": cmd,
		"command":   cmd,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("txadmin json marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if credential != "" {
		req.Header.Set("X-TxAdmin-Token", credential)
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	return req, nil
}

// ParseResponse parses the txAdmin HTTP response payload.
func (a *TxAdminAdapter) ParseResponse(resp *http.Response, body []byte) (string, error) {
	if resp.StatusCode == http.StatusNoContent || len(body) == 0 {
		return "", nil
	}

	var envelope struct {
		Status  string `json:"status"`
		Output  string `json:"output"`
		Result  string `json:"result"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Output != "" {
			return envelope.Output, nil
		}
		if envelope.Result != "" {
			return envelope.Result, nil
		}
		if envelope.Message != "" {
			return envelope.Message, nil
		}
		if str, ok := envelope.Data.(string); ok && str != "" {
			return str, nil
		}
	}
	return strings.TrimSpace(string(body)), nil
}

// FarmingSimulatorAdapter adapts the REST client to Farming Simulator 25's web admin API.
type FarmingSimulatorAdapter struct {
	CustomPath string
}

// Name returns the identifier of the farming simulator adapter.
func (a *FarmingSimulatorAdapter) Name() string { return "farming-simulator-25" }

// BuildRequest constructs the HTTP request for Farming Simulator 25 web admin commands.
func (a *FarmingSimulatorAdapter) BuildRequest(ctx context.Context, baseURL, cmd, credential string) (*http.Request, error) {
	path := "/api/console"
	if a.CustomPath != "" {
		path = a.CustomPath
	}
	url := strings.TrimRight(baseURL, "/") + path

	payload := map[string]string{
		"command": cmd,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("farming simulator json marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if credential != "" {
		creds := credential
		if !strings.Contains(creds, ":") {
			creds = "admin:" + creds
		}
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	}
	return req, nil
}

// ParseResponse parses the Farming Simulator 25 HTTP response payload.
func (a *FarmingSimulatorAdapter) ParseResponse(resp *http.Response, body []byte) (string, error) {
	if resp.StatusCode == http.StatusNoContent || len(body) == 0 {
		return "", nil
	}

	var envelope struct {
		Output  string `json:"output"`
		Result  string `json:"result"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Output != "" {
			return envelope.Output, nil
		}
		if envelope.Result != "" {
			return envelope.Result, nil
		}
		if envelope.Message != "" {
			return envelope.Message, nil
		}
		if str, ok := envelope.Data.(string); ok && str != "" {
			return str, nil
		}
	}
	return strings.TrimSpace(string(body)), nil
}

// DefaultRESTAdapter provides generic JSON/text command execution over HTTP.
type DefaultRESTAdapter struct {
	CustomPath string
}

// Name returns the identifier of the default REST adapter.
func (a *DefaultRESTAdapter) Name() string { return "generic" }

// BuildRequest constructs the HTTP request for generic REST commands.
func (a *DefaultRESTAdapter) BuildRequest(ctx context.Context, baseURL, cmd, credential string) (*http.Request, error) {
	path := "/api/command"
	if a.CustomPath != "" {
		path = a.CustomPath
	}
	url := strings.TrimRight(baseURL, "/") + path

	payload := map[string]string{
		"command": cmd,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("generic rest json marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if credential != "" {
		if strings.Contains(credential, ":") {
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credential)))
		} else {
			req.Header.Set("Authorization", "Bearer "+credential)
			req.Header.Set("X-TxAdmin-Token", credential)
		}
	}
	return req, nil
}

// ParseResponse parses the generic REST HTTP response payload.
func (a *DefaultRESTAdapter) ParseResponse(resp *http.Response, body []byte) (string, error) {
	if resp.StatusCode == http.StatusNoContent || len(body) == 0 {
		return "", nil
	}

	var envelope struct {
		Output        string `json:"output"`
		Result        string `json:"result"`
		Message       string `json:"message"`
		CommandResult string `json:"commandResult"`
		Data          any    `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Output != "" {
			return envelope.Output, nil
		}
		if envelope.Result != "" {
			return envelope.Result, nil
		}
		if envelope.Message != "" {
			return envelope.Message, nil
		}
		if envelope.CommandResult != "" {
			return envelope.CommandResult, nil
		}
		if str, ok := envelope.Data.(string); ok && str != "" {
			return str, nil
		}
	}
	return strings.TrimSpace(string(body)), nil
}

// REST is a generic HTTP REST console client.
type REST struct {
	host       string
	port       int
	scheme     string
	customPath string
	passFn     PassFn
	adapter    RESTAdapter

	httpClient *http.Client

	dialTimeout         time.Duration
	requestTimeout      time.Duration
	authFailureCooldown time.Duration

	mu              sync.Mutex
	lastAuthFailure time.Time
}

// NewREST constructs a REST client targeting the specified host and port.
func NewREST(host string, port int, pass PassFn, opts ...RESTOption) *REST {
	scheme := "http"
	if port == 443 {
		scheme = "https"
	}

	var adapter RESTAdapter
	switch port {
	case 40120:
		adapter = &TxAdminAdapter{}
	case 8080:
		adapter = &FarmingSimulatorAdapter{}
	default:
		adapter = &DefaultRESTAdapter{}
	}

	c := &REST{
		host:                host,
		port:                port,
		scheme:              scheme,
		passFn:              pass,
		adapter:             adapter,
		dialTimeout:         defaultRESTDialTimeout,
		requestTimeout:      defaultRESTRequestTimeout,
		authFailureCooldown: defaultRESTAuthFailureCooldown,
	}

	for _, opt := range opts {
		opt(c)
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if isLoopbackHost(host) {
		tlsCfg.InsecureSkipVerify = true
	}

	c.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{Timeout: c.dialTimeout}
				return d.DialContext(ctx, network, addr)
			},
			TLSClientConfig: tlsCfg,
		},
	}

	return c
}

// Close releases any idle connections held by the HTTP client.
func (c *REST) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// Exec executes a remote console command via HTTP REST.
func (c *REST) Exec(cmd string) (string, error) {
	c.mu.Lock()
	if !c.lastAuthFailure.IsZero() && time.Since(c.lastAuthFailure) < c.authFailureCooldown {
		c.mu.Unlock()
		return "", fmt.Errorf("rest rcon: %w (cooldown active)", ErrAuth)
	}
	c.mu.Unlock()

	var credential string
	if c.passFn != nil {
		var err error
		credential, err = c.passFn()
		if err != nil {
			return "", fmt.Errorf("rest rcon resolve password: %w", err)
		}
	}

	baseURL := fmt.Sprintf("%s://%s", c.scheme, net.JoinHostPort(c.host, fmt.Sprint(c.port)))

	ctx, cancel := context.WithTimeout(context.Background(), c.requestTimeout)
	defer cancel()

	req, err := c.adapter.BuildRequest(ctx, baseURL, cmd, credential)
	if err != nil {
		return "", fmt.Errorf("rest rcon build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("rest rcon request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.mu.Lock()
		c.lastAuthFailure = time.Now()
		c.mu.Unlock()
		return "", fmt.Errorf("rest rcon exec %q: %w (status %d)", cmd, ErrAuth, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, restMaxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("rest rcon read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("rest rcon exec %q: unexpected status %d: %s", cmd, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return c.adapter.ParseResponse(resp, body)
}
