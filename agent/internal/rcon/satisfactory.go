// satisfactory.go implements Satisfactory Dedicated Server's HTTPS API as
// an RCON client. Unlike every other protocol in this package, this isn't
// a socket console at all — it's a JSON function-call API: every request
// is a POST to one endpoint with a `function` field in the body selecting
// the operation. Exec maps onto the RunCommand function, which runs a
// free-text console command exactly like the other protocols' Exec.
//
// Protocol reference: https://github.com/satisfactory-oas/spec
//
// Wire format:
//
//	Endpoint: POST https://<host>:<port>/api/v1, Content-Type: application/json
//	          (default port 7777, same as the game). The spec documents
//	          paths as /api/v1/?function=X purely to key one OpenAPI
//	          operation per function; every request schema requires
//	          `function` in the JSON body, so every call is a POST to the
//	          bare /api/v1 endpoint.
//
//	Login:    {"function":"PasswordLogin","data":{"minimumPrivilegeLevel":"Administrator","password":"<pw>"}}
//	          -> {"data":{"authenticationToken":"<token>"}}
//	          Every later call carries Authorization: Bearer <token>.
//
//	Exec:     {"function":"RunCommand","data":{"command":"<cmd>"}}
//
//	Error:    {"errorCode":"string","errorMessage":"string","errorData":{}}
//	          errorCode is required on any error response, at any HTTP
//	          status — including 200 (the spec documents "200 Ok - Error"
//	          as a valid response for several operations), so a 2xx status
//	          alone never proves success.
//
// # The RunCommand success-response ambiguity
//
// The OpenAPI spec contradicts itself about what a successful RunCommand
// returns: the runCommand operation's response table declares only
// "204 No Content - Success", with no body. A separate schema,
// runCommandResponse — {"data":{"commandResult":"string"}} — exists in
// the spec but is never referenced from the operation's own responses, so
// it reads as an orphaned/aspirational schema rather than the documented
// contract. The operation's description ("returns it's output to the
// Console") is also ambiguous: it may mean the *server's* own console,
// not a value handed back to the caller. Rather than guess which
// dedicated-server build a given deployment runs, this client accepts
// either shape as success:
//
//   - HTTP 204: success, no output. Returns ("", nil) — this is NOT an
//     error, and callers must not treat empty output as a failure.
//   - HTTP 200 with {"data":{"commandResult":"..."}}: returns commandResult.
//   - HTTP 200 with {"errorCode":...}: returns an error, never the raw
//     body as if it were command output (see the "200 Ok - Error" note
//     above).
//
// # Auth
//
// A 401/403 on RunCommand means the cached token was rejected (expired,
// revoked, or the server restarted and forgot it) — Exec re-logs-in
// exactly ONCE and retries the same command. A 401/403 that persists past
// that single retry means the password itself is wrong, not just the
// token, and is reported as ErrAuth without looping further.
//
// # TLS
//
// The dedicated server generates a self-signed certificate by default and
// offers no way to supply a CA to verify it against. This client dials the
// game container over pod-local loopback (127.0.0.1) — RCON clients in this
// package never leave the pod, see the package doc comment on rcon.go — so
// accepting that self-signed cert doesn't expose us to a network-path MITM:
// nothing routes this connection off-host.
//
// That safety property is enforced, not just asserted: NewSatisfactory only
// sets InsecureSkipVerify when the host actually resolves to loopback (see
// isLoopbackHost). Point it at any other host and the cert is verified
// normally — which, with no trust anchor for the self-signed cert, fails the
// dial loudly instead of trusting whatever answers. The skip is also scoped
// to THIS client's own http.Transport (built fresh in NewSatisfactory, never
// http.DefaultTransport or http.DefaultClient) so no other HTTP caller in
// the agent inherits it.
package rcon

import (
	"bytes"
	"context"
	"crypto/tls"
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
	defaultSatisfactoryPort = 7777

	// defaultSatisfactoryDialTimeout bounds the underlying TCP connect.
	// A struct field (not just this const) so tests can shrink it —
	// mirrors websocket.go's dialTimeout/battleye.go's dialTimeout.
	defaultSatisfactoryDialTimeout = 5 * time.Second

	// defaultSatisfactoryRequestTimeout bounds one whole HTTP round trip
	// (connect + TLS handshake + request + response), via the request's
	// context. A struct field so tests can shrink it.
	defaultSatisfactoryRequestTimeout = 10 * time.Second

	// defaultSatisfactoryAuthFailureCooldown bounds how long ensureLocked
	// (via loginLocked) refuses to re-POST a known-bad password. Mirrors
	// websocket.go's/battleye.go's identically-named field and rationale:
	// without it, every heartbeat/players poller tick would re-attempt
	// login with the same rejected password.
	defaultSatisfactoryAuthFailureCooldown = 15 * time.Second

	// satisfactoryMaxResponseBytes bounds how much of one HTTP response
	// body this client will read, mirroring the read caps the other
	// clients in this package apply (webSocketReadLimit, maxTelnetReply)
	// so a misbehaving or malicious server can't grow an unbounded buffer.
	satisfactoryMaxResponseBytes = 1 << 20 // 1 MiB
)

// satisfactoryRequest is the envelope every call in this API sends: a
// function name plus its (function-specific) data payload.
type satisfactoryRequest struct {
	Function string `json:"function"`
	Data     any    `json:"data,omitempty"`
}

// satisfactoryLoginData is PasswordLogin's request payload.
type satisfactoryLoginData struct {
	MinimumPrivilegeLevel string `json:"minimumPrivilegeLevel"`
	Password              string `json:"password"`
}

// satisfactoryCommandData is RunCommand's request payload.
type satisfactoryCommandData struct {
	Command string `json:"command"`
}

// satisfactoryEnvelope is the shape of every response this client reads:
// either a success payload under "data", or an error under "errorCode"/
// "errorMessage". Both PasswordLogin and RunCommand share this envelope,
// just with different fields populated inside Data.
type satisfactoryEnvelope struct {
	Data *struct {
		AuthenticationToken string `json:"authenticationToken"`
		CommandResult       string `json:"commandResult"`
	} `json:"data,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// Satisfactory is a lazy, token-caching client for Satisfactory Dedicated
// Server's HTTPS API. It's safe for use from multiple goroutines; all ops
// are serialized on a single mutex (there's no persistent connection to
// serialize access to — every call is one independent HTTPS request —
// but the token/lastAuthFailure state needs the same protection every
// other client in this package gives its connection).
type Satisfactory struct {
	baseURL string
	passFn  PassFn

	httpClient *http.Client

	// dialTimeout/requestTimeout/authFailureCooldown are struct fields
	// (not package consts) so tests can shrink them — mirrors
	// websocket.go's and battleye.go's identically-named fields.
	dialTimeout         time.Duration
	requestTimeout      time.Duration
	authFailureCooldown time.Duration

	mu              sync.Mutex
	token           string
	lastAuthFailure time.Time
}

// NewSatisfactory builds a Satisfactory HTTPS API client. Login happens
// lazily on the first Exec.
func NewSatisfactory(host string, port int, pass PassFn) *Satisfactory {
	if port == 0 {
		port = defaultSatisfactoryPort
	}
	c := &Satisfactory{
		baseURL:             fmt.Sprintf("https://%s/api/v1", net.JoinHostPort(host, fmt.Sprint(port))),
		passFn:              pass,
		dialTimeout:         defaultSatisfactoryDialTimeout,
		requestTimeout:      defaultSatisfactoryRequestTimeout,
		authFailureCooldown: defaultSatisfactoryAuthFailureCooldown,
	}
	// The dedicated server presents a self-signed cert, so verification has
	// to be skipped — but only because this connection never leaves the pod.
	// Rather than assert that in a comment and skip unconditionally, enforce
	// it: verification is bypassed ONLY when the host is genuinely loopback.
	// If a template ever points this client at a non-loopback host, the cert
	// IS verified (and, lacking a trust anchor for the self-signed cert, the
	// dial fails loudly) instead of silently trusting whatever answers — and
	// gosec's G402 data-flow sees a guarded skip, not a blanket one.
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if isLoopbackHost(host) {
		tlsCfg.InsecureSkipVerify = true
	}
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Read c.dialTimeout on every dial (not captured once at
				// construction) so a test that shrinks it after
				// NewSatisfactory still takes effect.
				d := net.Dialer{Timeout: c.dialTimeout}
				return d.DialContext(ctx, network, addr)
			},
			TLSClientConfig: tlsCfg,
		},
	}
	return c
}

// isLoopbackHost reports whether host names the local machine, so that
// skipping TLS verification for the server's self-signed cert cannot expose
// the client to an off-host MITM. A bare "localhost" counts; anything else
// must parse to a loopback IP.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Close releases any idle connections held by this client. Satisfactory's
// API is stateless per-request (no persistent session socket like the
// other clients in this package), so there's nothing else to tear down.
func (c *Satisfactory) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// Exec runs one console command via RunCommand and returns its output.
func (c *Satisfactory) Exec(cmd string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token == "" {
		if err := c.loginLocked(); err != nil {
			return "", err
		}
	}

	result, status, err := c.runCommandLocked(cmd)
	if err != nil {
		return "", err
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		// The cached token was rejected. Re-login exactly ONCE and retry
		// the same command — never loop on this: a server that rejects
		// the retry too means the password is wrong, not just the token.
		c.token = ""
		if err := c.loginLocked(); err != nil {
			return "", err
		}
		result, status, err = c.runCommandLocked(cmd)
		if err != nil {
			return "", err
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			c.lastAuthFailure = time.Now()
			return "", fmt.Errorf("satisfactory rcon exec %q: %w (rejected again after re-login)", cmd, ErrAuth)
		}
	}

	return result, nil
}

// loginLocked calls PasswordLogin and caches the returned token. Must be
// called with c.mu held.
func (c *Satisfactory) loginLocked() error {
	if cooldown := c.authFailureCooldown; !c.lastAuthFailure.IsZero() && cooldown > 0 && time.Since(c.lastAuthFailure) < cooldown {
		return fmt.Errorf("satisfactory rcon: %w (cached, retrying login after cooldown)", ErrAuth)
	}

	pw, err := c.passFn()
	if err != nil {
		return fmt.Errorf("satisfactory rcon: resolve password: %w", err)
	}

	reqBody, err := json.Marshal(satisfactoryRequest{
		Function: "PasswordLogin",
		Data: satisfactoryLoginData{
			MinimumPrivilegeLevel: "Administrator",
			Password:              pw,
		},
	})
	if err != nil {
		return fmt.Errorf("satisfactory rcon: marshal login request: %w", err)
	}

	status, body, err := c.doRequestLocked(reqBody, "")
	if err != nil {
		return fmt.Errorf("satisfactory rcon: login: %w", err)
	}

	var env satisfactoryEnvelope
	if len(body) > 0 {
		if uerr := json.Unmarshal(body, &env); uerr != nil {
			return fmt.Errorf("satisfactory rcon: login: unmarshal response: %w", uerr)
		}
	}

	// A rejected password/insufficient privilege is the only documented
	// failure mode for PasswordLogin, so any error envelope or 401/403
	// here is treated as ErrAuth and starts the cooldown. A non-2xx
	// status WITHOUT an error envelope (e.g. a bare 500) is a server
	// problem, not proof the password is wrong, so it's reported as a
	// plain error and deliberately does NOT arm the cooldown.
	if status == http.StatusUnauthorized || status == http.StatusForbidden || env.ErrorCode != "" {
		c.lastAuthFailure = time.Now()
		if env.ErrorCode != "" {
			return fmt.Errorf("satisfactory rcon: login: %w: %s: %s", ErrAuth, env.ErrorCode, env.ErrorMessage)
		}
		return fmt.Errorf("satisfactory rcon: login: %w (http %d)", ErrAuth, status)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("satisfactory rcon: login: unexpected status %d", status)
	}
	if env.Data == nil || env.Data.AuthenticationToken == "" {
		return fmt.Errorf("satisfactory rcon: login: response had no authenticationToken")
	}

	c.token = env.Data.AuthenticationToken
	c.lastAuthFailure = time.Time{}
	return nil
}

// runCommandLocked calls RunCommand once and classifies the response. It
// deliberately does NOT treat a 401/403 as an error return — Exec decides
// whether to re-login and retry — so status is always meaningful on a nil
// error. Must be called with c.mu held.
func (c *Satisfactory) runCommandLocked(cmd string) (result string, status int, err error) {
	reqBody, err := json.Marshal(satisfactoryRequest{
		Function: "RunCommand",
		Data:     satisfactoryCommandData{Command: cmd},
	})
	if err != nil {
		return "", 0, fmt.Errorf("satisfactory rcon exec %q: marshal request: %w", cmd, err)
	}

	status, body, err := c.doRequestLocked(reqBody, c.token)
	if err != nil {
		return "", 0, fmt.Errorf("satisfactory rcon exec %q: %w", cmd, err)
	}

	switch status {
	case http.StatusNoContent:
		// See the "RunCommand success-response ambiguity" note in the
		// file doc comment: 204 is a documented success, not an error and
		// not "no reply yet".
		return "", status, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		// Body format on a 401/403 isn't specified by the spec; Exec
		// decides whether to re-login, so don't fail trying to parse it.
		return "", status, nil
	}

	var env satisfactoryEnvelope
	if len(body) > 0 {
		if uerr := json.Unmarshal(body, &env); uerr != nil {
			return "", status, fmt.Errorf("satisfactory rcon exec %q: unmarshal response: %w", cmd, uerr)
		}
	}

	if env.ErrorCode != "" {
		// The spec documents "200 Ok - Error": a 2xx status never proves
		// success by itself, so the error envelope must be checked before
		// trusting Data — otherwise an error body gets returned as if it
		// were command output.
		return "", status, fmt.Errorf("satisfactory rcon exec %q: %s: %s", cmd, env.ErrorCode, env.ErrorMessage)
	}
	if status < 200 || status >= 300 {
		return "", status, fmt.Errorf("satisfactory rcon exec %q: unexpected status %d", cmd, status)
	}
	if env.Data != nil {
		return env.Data.CommandResult, status, nil
	}
	return "", status, nil
}

// doRequestLocked POSTs body to the API endpoint, attaching an
// Authorization header when token is non-empty, and returns the raw
// status and response body. Must be called with c.mu held (it uses
// c.httpClient/c.baseURL/c.requestTimeout, none of which change after
// construction, but keeping it under the same lock as every other method
// in this file keeps the locking discipline uniform and trivially
// correct).
func (c *Satisfactory) doRequestLocked(body []byte, token string) (int, []byte, error) {
	d := c.requestTimeout
	if d <= 0 {
		d = defaultSatisfactoryRequestTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, satisfactoryMaxResponseBytes))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response body: %w", err)
	}
	return resp.StatusCode, respBody, nil
}
