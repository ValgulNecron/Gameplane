// palworld.go implements Palworld Dedicated Server's REST admin API as an
// RCON client. Like satisfactory.go, this isn't a socket console at all —
// it's a plain-HTTP JSON API with one endpoint per operation, secured with
// per-request HTTP Basic auth rather than a socket handshake or a cached
// session token.
//
// # Why REST instead of RCON
//
// Palworld's source RCON support is officially deprecated ("scheduled to
// stop functioning in an upcoming update" per docs.palworldgame.com) in
// favor of this REST API, so this client is the migration target — new
// GameTemplates should declare rcon.protocol: palworld, not source, for
// this game.
//
// Protocol reference: https://docs.palworldgame.com/category/rest-api/
//
// Wire format:
//
//	Base:  http://<host>:<port>/v1/api/... — plain HTTP, NOT HTTPS (unlike
//	       satisfactory.go's API, this one has no TLS at all). Default port
//	       8212.
//	Auth:  HTTP Basic, username literally "admin", password the server's
//	       AdminPassword: Authorization: Basic base64("admin:<password>").
//	       Sent on EVERY request — there is no token or session to cache,
//	       unlike satisfactory.go's bearer token.
//	Errors: status codes only. A 401 means bad auth; there is no
//	       structured error body to parse (unlike satisfactory.go's
//	       {"errorCode":...} envelope), so classification here is by
//	       status code alone.
//
// Endpoints, all under /v1/api/:
//
//	GET  info      -> {"version","servername","description","worldguid"}
//	GET  players   -> {"players":[{"name","accountName","playerId","userId","ip","ping",...}]}
//	GET  metrics   -> {"serverfps","currentplayernum","maxplayernum","uptime","days",...}
//	GET  settings  -> server settings object
//	POST announce  {"message":"<text>"}
//	POST save      (no body)
//	POST shutdown  {"waittime":<int>,"message":"<text>"}
//	POST stop      (no body)
//	POST kick      {"userid":"<id>","message":"<text>"}
//	POST ban       {"userid":"<id>","message":"<text>"}
//	POST unban     {"userid":"<id>"}
//
// Note the casing asymmetry: request bodies use "userid" (all lowercase);
// responses use "userId"/"playerId" (camelCase). That's the real wire
// contract, not a typo to "fix".
//
// # Exec's verb dispatch
//
// Palworld's REST API has no free-text console, so Exec parses cmd into a
// verb plus the remainder of the line and dispatches to the matching
// endpoint — see the switch in Exec for the exact mapping. announce and
// shutdown deliberately take the FULL remainder of the line as their
// message: the deprecated RCON protocol this replaces truncated messages
// at the first space, and the REST body must not repeat that limitation.
// GET endpoints return the raw response body verbatim (so a regex-based
// capability, e.g. the players parser, can read it exactly like it reads
// Rust's playerlist output); POST endpoints that succeed with no
// meaningful body return "".
//
// # Auth-failure handling
//
// A 401 is the only signal this API gives for a rejected password (see
// "Errors" above) — Exec wraps it in ErrAuth (shared across every client in
// this package; declared in websocket.go) and arms authFailureCooldown so a
// poller calling Exec on a fixed timer (heartbeat, players) doesn't re-POST
// a known-bad password every tick. Any other failure — connection refused,
// a 400/5xx, a body read error — is a request or server problem, not proof
// of a bad credential, and must NOT be classified as ErrAuth or arm the
// cooldown; doing so would freeze every poller on what might be a one-off
// transient failure. This exact confusion (transient error misread as bad
// password) has bitten three earlier clients in this package.
package rcon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultPalworldPort = 8212

	// defaultPalworldDialTimeout bounds the underlying TCP connect. A
	// struct field (not just this const) so tests can shrink it — mirrors
	// satisfactory.go's dialTimeout.
	defaultPalworldDialTimeout = 5 * time.Second

	// defaultPalworldRequestTimeout bounds one whole HTTP round trip
	// (connect + request + response), via the request's context. A
	// struct field so tests can shrink it.
	defaultPalworldRequestTimeout = 10 * time.Second

	// defaultPalworldAuthFailureCooldown bounds how long do() refuses to
	// re-POST a known-bad password. Mirrors satisfactory.go's/
	// websocket.go's identically-named field and rationale.
	defaultPalworldAuthFailureCooldown = 15 * time.Second

	// palworldMaxResponseBytes bounds how much of one HTTP response body
	// this client will read, mirroring the read caps the other clients in
	// this package apply (satisfactoryMaxResponseBytes, webSocketReadLimit)
	// so a huge players list can't blow memory.
	palworldMaxResponseBytes = 1 << 20 // 1 MiB
)

// palworldMessagePayload is announce's request body.
type palworldMessagePayload struct {
	Message string `json:"message"`
}

// palworldShutdownPayload is shutdown's request body.
type palworldShutdownPayload struct {
	Waittime int    `json:"waittime"`
	Message  string `json:"message"`
}

// palworldKickBanPayload is kick's and ban's shared request body. Field
// names are lowercase ("userid") to match the request-side wire contract —
// see the package doc comment on the casing asymmetry.
type palworldKickBanPayload struct {
	UserID  string `json:"userid"`
	Message string `json:"message"`
}

// palworldUnbanPayload is unban's request body.
type palworldUnbanPayload struct {
	UserID string `json:"userid"`
}

// Palworld is a client for Palworld Dedicated Server's REST admin API. It's
// safe for use from multiple goroutines; all requests are serialized on a
// single mutex — mirroring the other clients in this package's "one
// operation at a time" contract — even though, unlike them, there's no
// persistent connection or cached token whose consistency depends on it;
// only the auth-failure cooldown bookkeeping needs the protection.
type Palworld struct {
	baseURL string
	passFn  PassFn

	httpClient *http.Client

	// dialTimeout/requestTimeout/authFailureCooldown are struct fields
	// (not package consts) so tests can shrink them — mirrors
	// satisfactory.go's identically-named fields.
	dialTimeout         time.Duration
	requestTimeout      time.Duration
	authFailureCooldown time.Duration

	mu              sync.Mutex
	lastAuthFailure time.Time
}

// NewPalworld builds a Palworld REST API client.
func NewPalworld(host string, port int, pass PassFn) *Palworld {
	if port == 0 {
		port = defaultPalworldPort
	}
	c := &Palworld{
		baseURL:             fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprint(port))),
		passFn:              pass,
		dialTimeout:         defaultPalworldDialTimeout,
		requestTimeout:      defaultPalworldRequestTimeout,
		authFailureCooldown: defaultPalworldAuthFailureCooldown,
	}
	c.httpClient = &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Read c.dialTimeout on every dial (not captured once at
				// construction) so a test that shrinks it after
				// NewPalworld still takes effect.
				d := net.Dialer{Timeout: c.dialTimeout}
				return d.DialContext(ctx, network, addr)
			},
		},
	}
	return c
}

// Close releases any idle connections held by this client. The REST API is
// stateless per-request (no persistent session to tear down, like
// satisfactory.go's Close).
func (c *Palworld) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// Exec parses cmd into a verb and the remainder of the line and dispatches
// to the matching REST endpoint. See the package doc comment for the full
// verb -> endpoint mapping. An unrecognized verb is a hard error — this
// deliberately never silently succeeds on a command it can't map.
func (c *Palworld) Exec(cmd string) (string, error) {
	verb, rest := splitPalworldCommand(cmd)
	if verb == "" {
		return "", fmt.Errorf("palworld rcon: empty command")
	}

	switch strings.ToLower(verb) {
	case "announce":
		// The FULL remainder of the line is the message — Palworld's
		// deprecated RCON truncated at the first space, and this must not
		// repeat that limitation. See the package doc comment.
		if rest == "" {
			return "", fmt.Errorf("palworld rcon: announce requires a message")
		}
		return c.do(http.MethodPost, "/v1/api/announce", palworldMessagePayload{Message: rest})

	case "save":
		return c.do(http.MethodPost, "/v1/api/save", nil)

	case "shutdown":
		waitStr, message := splitPalworldCommand(rest)
		if waitStr == "" {
			return "", fmt.Errorf("palworld rcon: shutdown requires a waittime")
		}
		wait, err := strconv.Atoi(waitStr)
		if err != nil {
			return "", fmt.Errorf("palworld rcon: shutdown: invalid waittime %q: %w", waitStr, err)
		}
		return c.do(http.MethodPost, "/v1/api/shutdown", palworldShutdownPayload{Waittime: wait, Message: message})

	case "stop":
		return c.do(http.MethodPost, "/v1/api/stop", nil)

	case "players":
		// GET endpoints return the raw response body verbatim so a
		// regex-based capability can parse it — see the package doc
		// comment.
		return c.do(http.MethodGet, "/v1/api/players", nil)

	case "info":
		return c.do(http.MethodGet, "/v1/api/info", nil)

	case "metrics":
		return c.do(http.MethodGet, "/v1/api/metrics", nil)

	case "settings":
		return c.do(http.MethodGet, "/v1/api/settings", nil)

	case "kick":
		userID, message := splitPalworldCommand(rest)
		if userID == "" {
			return "", fmt.Errorf("palworld rcon: kick requires a userid")
		}
		return c.do(http.MethodPost, "/v1/api/kick", palworldKickBanPayload{UserID: userID, Message: message})

	case "ban":
		userID, message := splitPalworldCommand(rest)
		if userID == "" {
			return "", fmt.Errorf("palworld rcon: ban requires a userid")
		}
		return c.do(http.MethodPost, "/v1/api/ban", palworldKickBanPayload{UserID: userID, Message: message})

	case "unban":
		if rest == "" {
			return "", fmt.Errorf("palworld rcon: unban requires a userid")
		}
		return c.do(http.MethodPost, "/v1/api/unban", palworldUnbanPayload{UserID: rest})

	default:
		return "", fmt.Errorf("palworld rcon: unknown command %q", verb)
	}
}

// do sends one request to the REST API and returns the response body. It
// attaches HTTP Basic auth on every call (this API has no token/session —
// see the package doc comment) and self-locks c.mu for the duration: unlike
// satisfactory.go/websocket.go/battleye.go there's no persistent
// connection or cached credential whose consistency spans a multi-step
// Exec, only the auth-failure cooldown bookkeeping below, which fits
// entirely inside this one call.
//
// payload, when non-nil, is marshaled as the JSON request body; nil means
// no body (save, stop). The returned string is the raw response body for a
// GET, or "" for a successful POST (see the package doc comment on why
// POST responses aren't surfaced — none of these endpoints document a
// meaningful success body).
func (c *Palworld) do(method, path string, payload any) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cooldown := c.authFailureCooldown; !c.lastAuthFailure.IsZero() && cooldown > 0 && time.Since(c.lastAuthFailure) < cooldown {
		return "", fmt.Errorf("palworld rcon %s %s: %w (cached, retrying after cooldown)", method, path, ErrAuth)
	}

	pw, err := c.passFn()
	if err != nil {
		return "", fmt.Errorf("palworld rcon: resolve password: %w", err)
	}

	var bodyReader io.Reader
	if payload != nil {
		b, merr := json.Marshal(payload)
		if merr != nil {
			return "", fmt.Errorf("palworld rcon %s %s: marshal request: %w", method, path, merr)
		}
		bodyReader = bytes.NewReader(b)
	}

	d := c.requestTimeout
	if d <= 0 {
		d = defaultPalworldRequestTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return "", fmt.Errorf("palworld rcon %s %s: build request: %w", method, path, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Basic auth on EVERY request — this API has no token/session, unlike
	// satisfactory.go's bearer token.
	req.SetBasicAuth("admin", pw)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("palworld rcon %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, palworldMaxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("palworld rcon %s %s: read response body: %w", method, path, err)
	}

	// A 401 is the only status this API uses to signal a rejected
	// password — there's no structured error body to disambiguate
	// further (see the package doc comment). Anything else, including a
	// 400 bad request or a 5xx server error, is NOT proof the credential
	// is wrong and must not arm the cooldown or be reported as ErrAuth:
	// doing so would freeze every poller on what may be a one-off
	// transient failure.
	if resp.StatusCode == http.StatusUnauthorized {
		c.lastAuthFailure = time.Now()
		return "", fmt.Errorf("palworld rcon %s %s: %w (http 401)", method, path, ErrAuth)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("palworld rcon %s %s: unexpected status %d", method, path, resp.StatusCode)
	}

	// A clean response proves the password is good; drop any stale
	// cooldown so a corrected password (or a spurious earlier 401) never
	// outlives a request that actually succeeded.
	c.lastAuthFailure = time.Time{}

	if method == http.MethodGet {
		return string(respBody), nil
	}
	return "", nil
}

// splitPalworldCommand splits cmd at its first SPACE, returning that leading
// token and the (trimmed) remainder of the line. Space specifically, not any
// whitespace: the remainder becomes a free-text message that must keep its
// own spacing byte-for-byte (the deprecated RCON truncated at the first
// space — not repeating that is this client's whole point). Used twice per
// multi-argument
// command: once to split the verb from its arguments, and again — for
// shutdown/kick/ban — to split the first argument (waittime/userid) from
// the trailing free-text message, so the message keeps any internal
// spacing exactly as authored.
func splitPalworldCommand(cmd string) (first, rest string) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return "", ""
	}
	idx := strings.IndexByte(trimmed, ' ')
	if idx < 0 {
		return trimmed, ""
	}
	return trimmed[:idx], strings.TrimSpace(trimmed[idx+1:])
}
