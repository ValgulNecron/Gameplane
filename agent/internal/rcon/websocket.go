// websocket.go implements the Rust WebRcon client protocol used by Rust
// game servers. The Rust game server exposes a WebSocket-based RCON
// endpoint when launched with +rcon.web 1.
//
// Protocol reference: https://github.com/facepunch/webrcon
//
// Wire format:
//
//	Connect: ws://<host>:<port>/<password> (password URL-escaped, default port 28016)
//	Request frame (JSON text): {"Identifier": 42, "Message": "playerlist", "Name": "WebRcon"}
//	Response frame: {"Identifier": 42, "Message": "…output…", "Type": 3, "Stacktrace": ""}
//
// The server pushes unsolicited frames (console spam, chat, player join/leave)
// with different Identifiers. We assign a unique positive Identifier per request
// and discard non-matching frames until we match or deadline expires.
//
// WebRcon has no positive auth acknowledgement: the server accepts the
// WebSocket handshake unconditionally regardless of password, and a bad
// password only shows up afterward as the server closing the connection —
// a healthy, idle, correctly-authenticated server sends nothing at all.
// That means auth can only be confirmed retroactively, once a frame
// actually arrives; see ensureLocked and Exec below for how this client
// detects a bad password without a synchronous post-dial probe.
package rcon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	defaultWebSocketExecDeadline = 30 * time.Second
	webSocketMaxCommandLength    = 1000
	defaultWebSocketPort         = 28016

	// defaultWebSocketDialTimeout bounds dialing and the WebSocket
	// handshake. A struct field (not just this const) so tests can
	// shrink it — mirrors telnet.go's connectTimeout.
	defaultWebSocketDialTimeout = 5 * time.Second

	// defaultWebSocketAuthFailureCooldown bounds how long ensureLocked
	// refuses to re-dial after Exec has classified a failure as a
	// rejected password. See ensureLocked for why this exists.
	defaultWebSocketAuthFailureCooldown = 15 * time.Second

	// webSocketReadLimit raises coder/websocket's default 32 KiB
	// per-message read limit, which a large playerlist/status reply on a
	// busy server can exceed. Matches telnet.go's maxTelnetReply.
	webSocketReadLimit = 1 << 20 // 1 MiB
)

// ErrAuth reports a rejected RCON password. Rust accepts the WebSocket
// handshake and only then closes, so auth failure surfaces as an early
// close rather than an HTTP 401.
var ErrAuth = errors.New("websocket rcon: authentication failed")

// WebSocketMessage is the wire format for Rust WebRcon protocol.
type WebSocketMessage struct {
	Identifier int64  `json:"Identifier"`
	Message    string `json:"Message"`
	Name       string `json:"Name,omitempty"`
	Type       int    `json:"Type,omitempty"`
	Stacktrace string `json:"Stacktrace,omitempty"`
}

// WebSocket is a lazy, auto-reconnecting client for Rust's WebRcon protocol.
// It's safe for use from multiple goroutines; all ops are serialized on a single conn.
type WebSocket struct {
	host     string
	port     int
	passFn   PassFn
	baseURL  string
	nextID   atomic.Int64
	idOffset int64 // Offset to ensure IDs are always positive (start at 1)

	// dialTimeout/execDeadline are struct fields (not package consts) so
	// tests can shrink them — mirrors telnet.go's connectTimeout/
	// responseDeadline for the same reason.
	dialTimeout  time.Duration
	execDeadline time.Duration

	// authFailureCooldown bounds how long ensureLocked backs off after a
	// classified auth failure before it will dial again. A field so
	// tests can shrink it.
	authFailureCooldown time.Duration

	mu   sync.Mutex
	conn *websocket.Conn

	// authConfirmed is true once ANY frame — solicited or not — has been
	// received on conn. WebRcon has no explicit auth ack, so receiving a
	// frame at all is the only proof the password was accepted. Reset to
	// false on every (re)dial. See Exec and isAuthCloseSignal.
	authConfirmed bool

	// lastAuthFailure records when Exec last classified a Write/Read
	// failure as a rejected password, so ensureLocked can back off
	// instead of re-dialing with a password it already knows is wrong.
	lastAuthFailure time.Time
}

// NewWebSocket creates a new Rust WebRcon client. The connection is dialed
// lazily on the first Exec.
func NewWebSocket(host string, port int, pw PassFn) *WebSocket {
	if port == 0 {
		port = defaultWebSocketPort
	}
	return &WebSocket{
		host:                host,
		port:                port,
		passFn:              pw,
		baseURL:             net.JoinHostPort(host, fmt.Sprint(port)),
		dialTimeout:         defaultWebSocketDialTimeout,
		execDeadline:        defaultWebSocketExecDeadline,
		authFailureCooldown: defaultWebSocketAuthFailureCooldown,
		idOffset:            1000, // Start IDs at 1000 to ensure positive and distinct
	}
}

// Exec runs one RCON command and returns the response.
func (c *WebSocket) Exec(cmd string) (string, error) {
	// Validate command length
	if len(cmd) > webSocketMaxCommandLength {
		return "", fmt.Errorf("websocket rcon: command too long (%d chars, max %d)", len(cmd), webSocketMaxCommandLength)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureLocked(); err != nil {
		return "", err
	}

	conn := c.conn
	d := c.execDeadline
	if d <= 0 {
		d = defaultWebSocketExecDeadline
	}

	// Create a context with the exec deadline
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	// Assign a unique, monotonically increasing positive Identifier
	reqID := c.allocID()

	// Marshal and send the request
	req := WebSocketMessage{
		Identifier: reqID,
		Message:    cmd,
		Name:       "WebRcon",
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("websocket rcon: marshal request: %w", err)
	}

	if err := conn.Write(ctx, websocket.MessageText, reqData); err != nil {
		return "", c.classifyExecErrLocked(cmd, err)
	}

	// Loop reading frames until we match the Identifier or deadline expires.
	// The server sends unsolicited frames with different Identifiers;
	// we must discard those and keep reading.
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return "", c.classifyExecErrLocked(cmd, err)
		}

		// Any frame at all — solicited or not — proves the server
		// accepted the password and is speaking WebRcon back to us.
		// Rust closes the connection right after the handshake on a bad
		// password (see the package doc comment), so once we've seen a
		// frame here, a later close/EOF on this same conn no longer
		// means anything about auth.
		c.authConfirmed = true

		var resp WebSocketMessage
		if err := json.Unmarshal(data, &resp); err != nil {
			c.dropLocked()
			return "", fmt.Errorf("websocket rcon: unmarshal response: %w", err)
		}

		// Check if this is the response we're waiting for
		if resp.Identifier == reqID {
			return resp.Message, nil
		}

		// Discard non-matching frames (unsolicited console spam, chat, etc.)
		// and loop to read the next one
	}
}

// classifyExecErrLocked wraps a Write or Read failure from Exec, promoting
// it to ErrAuth when this connection has never produced a single frame and
// the failure looks like the close WebRcon uses to signal a bad password
// (see isAuthCloseSignal). A deadline expiring is deliberately never
// classified as auth — isAuthCloseSignal only matches a close frame or
// io.EOF, neither of which a context-deadline failure produces (see
// ensureLocked's doc comment on the old auth probe for why that
// distinction matters). Must be called with c.mu held; drops the
// connection as a side effect since neither a Write nor a Read failure
// leaves it usable.
func (c *WebSocket) classifyExecErrLocked(cmd string, err error) error {
	c.dropLocked()
	if !c.authConfirmed && isAuthCloseSignal(err) {
		c.lastAuthFailure = time.Now()
		return fmt.Errorf("websocket rcon exec %q: %w: %w", cmd, ErrAuth, err)
	}
	return fmt.Errorf("websocket rcon exec %q: %w", cmd, err)
}

// Close shuts down the underlying connection.
func (c *WebSocket) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close(websocket.StatusNormalClosure, "")
		c.conn = nil
		return err
	}
	return nil
}

func (c *WebSocket) ensureLocked() error {
	if c.conn != nil {
		return nil
	}

	// Back off after a recent auth failure instead of re-dialing on
	// every call. WebRcon gives us no way to check a password without a
	// full dial + request round-trip (see Exec), and callers like the
	// heartbeat and players pollers call Exec on a fixed timer —
	// without this guard, every tick would re-dial with the same
	// known-bad password and pile onto Rust's login rate limiter. The
	// cooldown expires on its own so a corrected password is picked
	// back up without requiring a process restart.
	if cooldown := c.authFailureCooldown; !c.lastAuthFailure.IsZero() && cooldown > 0 && time.Since(c.lastAuthFailure) < cooldown {
		return fmt.Errorf("websocket rcon: %w (cached, retrying dial after cooldown)", ErrAuth)
	}

	pw, err := c.passFn()
	if err != nil {
		return fmt.Errorf("websocket rcon: resolve password: %w", err)
	}

	// URL-escape the password and build the connection URL
	escapedPw := url.PathEscape(pw)
	wsURL := fmt.Sprintf("ws://%s/%s", c.baseURL, escapedPw)

	dialTimeout := c.dialTimeout
	if dialTimeout <= 0 {
		dialTimeout = defaultWebSocketDialTimeout
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket rcon: dial %s: %w", c.baseURL, err)
	}

	// coder/websocket defaults to a 32 KiB per-message read limit; a big
	// playerlist/status reply on a busy server can exceed that.
	conn.SetReadLimit(webSocketReadLimit)

	// No post-handshake auth probe here — see the package doc comment.
	// WebRcon gives no positive signal that a password was accepted,
	// only a negative one (the server closing) that a healthy, idle
	// server will never produce, so a short read-and-see always
	// misclassified a healthy connection as a bad password. Worse, a
	// read whose context then expired made coder/websocket's
	// timeoutLoop tear down the very connection being "checked" (see
	// (*Conn).timeoutLoop upstream: it closes the conn the instant a
	// read context is Done, timeout or not). Auth is now confirmed, or
	// refuted, lazily by Exec's read loop instead.
	c.conn = conn
	c.authConfirmed = false
	return nil
}

func (c *WebSocket) dropLocked() {
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
		c.conn = nil
	}
	c.authConfirmed = false
}

// allocID returns a unique positive Identifier for this request.
// IDs start at idOffset (1000) and increment monotonically.
func (c *WebSocket) allocID() int64 {
	return c.idOffset + c.nextID.Add(1)
}

// isAuthCloseSignal reports whether err is the kind of close WebRcon uses
// to signal a rejected password: a WebSocket close frame, or the
// connection ending in EOF. This is a type/sentinel check, deliberately
// not a substring match on err.Error(): a real close frame renders as
// "received close frame: status = ...", which contains neither "closed"
// nor "EOF", while an unrelated transport failure like "use of closed
// network connection" contains "closed" and would be misclassified as
// auth by a naive strings.Contains check — which is exactly the bug this
// replaces. A context-deadline failure matches neither errors.As(...,
// *CloseError) nor errors.Is(..., io.EOF), so it always falls through to
// "not an auth signal" here.
func isAuthCloseSignal(err error) bool {
	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		return true
	}
	return errors.Is(err, io.EOF)
}
