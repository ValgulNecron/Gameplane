// websocket.go implements the Rust WebRcon protocol used by Rust game servers.
// The Rust game server exposes a WebSocket-based RCON endpoint when launched
// with +rcon.web 1.
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
// Auth failure is signaled by the server closing the connection after accepting
// the handshake.
package rcon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	defaultWebSocketExecDeadline = 30 * time.Second
	webSocketMaxCommandLength    = 1000
	defaultWebSocketPort         = 28016
)

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

	// execDeadline bounds a single command exchange (see Exec).
	// Exposed as a field so tests can inject shorter timeouts.
	execDeadline time.Duration

	mu   sync.Mutex
	conn *websocket.Conn
}

// NewWebSocket creates a new Rust WebRcon client. The connection is dialed
// lazily on the first Exec.
func NewWebSocket(host string, port int, pw PassFn) *WebSocket {
	if port == 0 {
		port = defaultWebSocketPort
	}
	return &WebSocket{
		host:         host,
		port:         port,
		passFn:       pw,
		baseURL:      net.JoinHostPort(host, fmt.Sprint(port)),
		execDeadline: defaultWebSocketExecDeadline,
		idOffset:     1000, // Start IDs at 1000 to ensure positive and distinct
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
		c.dropLocked()
		return "", fmt.Errorf("websocket rcon exec %q: %w", cmd, err)
	}

	// Loop reading frames until we match the Identifier or deadline expires.
	// The server sends unsolicited frames with different Identifiers;
	// we must discard those and keep reading.
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			c.dropLocked()
			return "", fmt.Errorf("websocket rcon exec %q: %w", cmd, err)
		}

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

	pw, err := c.passFn()
	if err != nil {
		return fmt.Errorf("websocket rcon: resolve password: %w", err)
	}

	// URL-escape the password and build the connection URL
	escapedPw := url.PathEscape(pw)
	wsURL := fmt.Sprintf("ws://%s/%s", c.baseURL, escapedPw)

	// Dial with a timeout
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("websocket rcon: dial %s: %w", c.baseURL, err)
	}

	// Try reading a frame to detect auth failure.
	// The server closes immediately on auth failure, so a read after
	// accepting the handshake is how we detect bad auth.
	// Use a short timeout for this check.
	authCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, _, err = conn.Read(authCtx)
	if err != nil {
		// If we got an error immediately after dialing, it's likely auth failure
		_ = conn.Close(websocket.StatusNormalClosure, "")
		if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "EOF") ||
			errors.Is(err, context.DeadlineExceeded) {
			return errors.New("websocket rcon: authentication failed (connection closed)")
		}
		return fmt.Errorf("websocket rcon: connection failed: %w", err)
	}

	// We got a frame during the auth check. That's unexpected but harmless —
	// it's an unsolicited frame. We keep the connection and treat the auth as
	// successful. On the next Exec, we'll see this frame and discard it.

	c.conn = conn
	return nil
}

func (c *WebSocket) dropLocked() {
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
		c.conn = nil
	}
}

// allocID returns a unique positive Identifier for this request.
// IDs start at idOffset (1000) and increment monotonically.
func (c *WebSocket) allocID() int64 {
	return c.idOffset + c.nextID.Add(1)
}

// isTimeout reports whether err is a network timeout (context deadline) as
// opposed to a hard connection error.
func isWebSocketTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
