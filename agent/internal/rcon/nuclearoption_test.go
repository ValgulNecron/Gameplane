package rcon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseNuclearOptionCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantArgs []string
	}{
		{
			name:     "empty string",
			input:    "",
			wantName: "",
			wantArgs: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			wantName: "",
			wantArgs: nil,
		},
		{
			name:     "command no args",
			input:    "get-player-list",
			wantName: "get-player-list",
			wantArgs: nil,
		},
		{
			name:     "command with one simple arg",
			input:    "kick-player 76561198123456789",
			wantName: "kick-player",
			wantArgs: []string{"76561198123456789"},
		},
		{
			name:     "send-chat-message preserves spaces in message",
			input:    "send-chat-message hello world from test",
			wantName: "send-chat-message",
			wantArgs: []string{"hello world from test"},
		},
		{
			name:     "send-chat-message with single word",
			input:    "send-chat-message hello",
			wantName: "send-chat-message",
			wantArgs: []string{"hello"},
		},
		{
			name:     "send-chat-message empty message",
			input:    "send-chat-message ",
			wantName: "send-chat-message",
			wantArgs: []string{""},
		},
		{
			name:     "banlist-add with steamid and reason",
			input:    "banlist-add 76561198123456789 cheating",
			wantName: "banlist-add",
			wantArgs: []string{"76561198123456789", "cheating"},
		},
		{
			name:     "set-next-mission with three args",
			input:    "set-next-mission BuiltIn Escalation 3600.0",
			wantName: "set-next-mission",
			wantArgs: []string{"BuiltIn", "Escalation", "3600.0"},
		},
		{
			name:     "command with extra spaces",
			input:    "kick-player   76561198123456789",
			wantName: "kick-player",
			wantArgs: []string{"76561198123456789"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotArgs := parseNuclearOptionCommand(tt.input)
			if gotName != tt.wantName {
				t.Errorf("name: got %q, want %q", gotName, tt.wantName)
			}
			if !slicesEqual(gotArgs, tt.wantArgs) {
				t.Errorf("args: got %v, want %v", gotArgs, tt.wantArgs)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fakeTCPServer simulates a Nuclear Option server for testing.
// It reads requests and sends back configured responses.
type fakeTCPServer struct {
	listener net.Listener
	addr     string
	done     chan struct{}
	wg       sync.WaitGroup
	// responses is a map from command name to (status, body).
	// If a key is not present, a 4004 (unknown command) is returned.
	responses map[string][2]interface{}
}

func newFakeTCPServer(t *testing.T) *fakeTCPServer {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &fakeTCPServer{
		listener:  listener,
		addr:      listener.Addr().String(),
		done:      make(chan struct{}),
		responses: make(map[string][2]interface{}),
	}

	server.wg.Add(1)
	go server.serve(t)

	return server
}

func (s *fakeTCPServer) serve(t *testing.T) {
	defer s.wg.Done()
	for {
		select {
		case <-s.done:
			return
		default:
		}

		// Accept with timeout to avoid blocking forever.
		s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond))
		conn, err := s.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "timeout") {
				continue
			}
			return
		}

		s.handleConn(t, conn)
	}
}

func (s *fakeTCPServer) handleConn(_ *testing.T, conn net.Conn) {
	defer conn.Close()

	for {
		// Read request: 4-byte length + body.
		lengthBuf := make([]byte, 4)
		n, err := conn.Read(lengthBuf)
		if err != nil {
			return // Client closed or error.
		}
		if n < 4 {
			// Send a truncated response to test that case.
			conn.Write([]byte{0x00, 0x00})
			return
		}

		bodyLen := binary.LittleEndian.Uint32(lengthBuf)
		bodyBuf := make([]byte, bodyLen)
		if _, err := conn.Read(bodyBuf); err != nil {
			return
		}

		var req nuclearOptionRequest
		if err := json.Unmarshal(bodyBuf, &req); err != nil {
			// Malformed JSON: return 4003.
			s.sendResponse(conn, 4003, nil)
			continue
		}

		// Look up the command in responses.
		resp, ok := s.responses[req.Name]
		if !ok {
			// Unknown command: return 4004.
			s.sendResponse(conn, 4004, nil)
			continue
		}

		status := resp[0].(uint32)
		var body interface{}
		if resp[1] != nil {
			body = resp[1]
		}
		s.sendResponse(conn, status, body)
	}
}

func (s *fakeTCPServer) sendResponse(conn net.Conn, status uint32, body interface{}) {
	statusBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(statusBuf, status)
	conn.Write(statusBuf)

	var bodyBytes []byte
	if body != nil {
		if str, ok := body.(string); ok {
			bodyBytes = []byte(str)
		} else {
			bodyBytes, _ = json.Marshal(body)
		}
	}

	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(bodyBytes)))
	conn.Write(lenBuf)

	if len(bodyBytes) > 0 {
		conn.Write(bodyBytes)
	}
}

func (s *fakeTCPServer) close(_ *testing.T) {
	close(s.done)
	s.listener.Close()
	s.wg.Wait()
}

func TestNuclearOptionExec(t *testing.T) {
	server := newFakeTCPServer(t)
	defer server.close(t)

	// Parse the server address.
	parts := strings.Split(server.addr, ":")
	if len(parts) != 2 {
		t.Fatalf("unexpected address format: %s", server.addr)
	}
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()

	// Shrink timeouts for tests.
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	tests := []struct {
		name       string
		setupResp  map[string][2]interface{}
		cmd        string
		wantBody   string
		wantErrMsg string
	}{
		{
			name: "success with JSON body",
			setupResp: map[string][2]interface{}{
				"get-server-id": {
					uint32(2000),
					map[string]interface{}{"serverId": "90291415221858321"},
				},
			},
			cmd:      "get-server-id",
			wantBody: `{"serverId":"90291415221858321"}`,
		},
		{
			name: "success with empty body",
			setupResp: map[string][2]interface{}{
				"kick-player": {
					uint32(2000),
					nil, // Empty body for kick success.
				},
			},
			cmd:      "kick-player 76561198123456789",
			wantBody: "",
		},
		{
			name: "4003 malformed JSON with no body",
			setupResp: map[string][2]interface{}{
				"malformed": {
					uint32(4003),
					nil, // No detail body.
				},
			},
			cmd:        "malformed",
			wantErrMsg: "status 4003",
		},
		{
			// setupResp is left nil: no entry for "bogus-command" means
			// the server responds 4004.
			name:       "4004 unknown command",
			cmd:        "bogus-command",
			wantErrMsg: "status 4004",
		},
		{
			name: "4005 bad arguments with error detail",
			setupResp: map[string][2]interface{}{
				"set-next-mission": {
					uint32(4005),
					map[string]interface{}{
						"message": "Expected Arguments [string Group, string Name, float MaxTime]",
					},
				},
			},
			cmd:        "set-next-mission bad-arg",
			wantErrMsg: "Expected Arguments [string Group, string Name, float MaxTime]",
		},
		{
			name: "send-chat-message with spaces preserved",
			setupResp: map[string][2]interface{}{
				"send-chat-message": {
					uint32(2000),
					nil,
				},
			},
			cmd:      "send-chat-message hello world from nuclear option",
			wantBody: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set up responses.
			server.responses = tt.setupResp

			body, err := client.Exec(tt.cmd)

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("expected error, got none")
				} else if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error: got %v, want to contain %q", err, tt.wantErrMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if body != tt.wantBody {
					t.Errorf("body: got %q, want %q", body, tt.wantBody)
				}
			}

			// Close the connection for the next test to avoid reuse issues.
			client.Close()
		})
	}
}

// TestNuclearOptionTruncatedHeader tests handling of truncated response headers.
func TestNuclearOptionTruncatedHeader(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	parts := strings.Split(addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	// Server that sends incomplete response.
	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		// Read request (don't care about it).
		buf := make([]byte, 1024)
		conn.Read(buf)
		// Send truncated status (only 2 bytes instead of 4).
		conn.Write([]byte{0x00, 0x00})
	}()

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	_, err = client.Exec("test-command")
	if err == nil {
		t.Errorf("expected error for truncated header, got none")
	}
}

// TestNuclearOptionBodyLengthMismatch tests handling of body length that
// disagrees with actual bytes sent.
func TestNuclearOptionBodyLengthMismatch(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	parts := strings.Split(addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	// Server that sends mismatched body length.
	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		// Read request.
		buf := make([]byte, 1024)
		conn.Read(buf)
		// Send valid status and claim body length of 100, but only send 4 bytes.
		statusBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(statusBuf, 2000)
		conn.Write(statusBuf)

		lenBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(lenBuf, 100) // Claim 100 bytes.
		conn.Write(lenBuf)

		conn.Write([]byte("only4")) // But only send 5.
	}()

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	_, err = client.Exec("test-command")
	if err == nil {
		t.Errorf("expected error for body length mismatch, got none")
	}
}

// TestNuclearOptionBodyTooLarge tests that oversized body lengths are rejected.
func TestNuclearOptionBodyTooLarge(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	parts := strings.Split(addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	// Server that claims a body larger than the max allowed.
	go func() {
		conn, _ := listener.Accept()
		defer conn.Close()
		// Read request.
		buf := make([]byte, 1024)
		conn.Read(buf)
		// Send status 2000 with a body length exceeding max.
		statusBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(statusBuf, 2000)
		conn.Write(statusBuf)

		lenBuf := make([]byte, 4)
		// Claim body larger than nuclearOptionMaxBodyLength.
		binary.LittleEndian.PutUint32(lenBuf, nuclearOptionMaxBodyLength+1)
		conn.Write(lenBuf)
	}()

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	_, err = client.Exec("test-command")
	if err == nil {
		t.Errorf("expected error for oversized body, got none")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error: got %v, want to contain 'too large'", err)
	}
}

// TestNuclearOptionMessageWithSpaces verifies that a message argument
// containing spaces is preserved intact when sent to the server.
func TestNuclearOptionMessageWithSpaces(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	parts := strings.Split(listener.Addr().String(), ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	// The captured request travels through the channel rather than through a
	// shared variable, so the send happens-before the read with no extra
	// synchronisation and no data race.
	capturedCh := make(chan nuclearOptionRequest, 1)
	go func() {
		conn, aerr := listener.Accept()
		if aerr != nil {
			return
		}
		defer conn.Close()

		lengthBuf := make([]byte, 4)
		if _, rerr := io.ReadFull(conn, lengthBuf); rerr != nil {
			return
		}
		bodyBuf := make([]byte, binary.LittleEndian.Uint32(lengthBuf))
		if _, rerr := io.ReadFull(conn, bodyBuf); rerr != nil {
			return
		}
		var req nuclearOptionRequest
		if uerr := json.Unmarshal(bodyBuf, &req); uerr != nil {
			return
		}
		capturedCh <- req

		resp := make([]byte, 8)
		binary.LittleEndian.PutUint32(resp[0:4], 2000)
		binary.LittleEndian.PutUint32(resp[4:8], 0)
		conn.Write(resp)
	}()

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	msg := "hello world from nuclear option"
	if _, eerr := client.Exec("send-chat-message " + msg); eerr != nil {
		t.Errorf("exec: %v", eerr)
	}

	var capturedReq nuclearOptionRequest
	select {
	case capturedReq = <-capturedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for captured request")
	}

	if len(capturedReq.Arguments) != 1 {
		t.Fatalf("arguments count: got %d, want 1", len(capturedReq.Arguments))
	}
	if capturedReq.Arguments[0] != msg {
		t.Errorf("message: got %q, want %q", capturedReq.Arguments[0], msg)
	}
}

// TestNuclearOptionEmptyCommand tests that an empty command is rejected.
func TestNuclearOptionEmptyCommand(t *testing.T) {
	server := newFakeTCPServer(t)
	defer server.close(t)

	parts := strings.Split(server.addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	_, err := client.Exec("")
	if err == nil {
		t.Errorf("expected error for empty command, got none")
	}
}

// TestNuclearOptionConnectionReuse verifies that the connection is reused
// across multiple Exec calls and is properly closed on error.
func TestNuclearOptionConnectionReuse(t *testing.T) {
	server := newFakeTCPServer(t)
	defer server.close(t)

	parts := strings.Split(server.addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	server.responses = map[string][2]interface{}{
		"test-cmd": {uint32(2000), nil},
	}

	// First call should establish a connection.
	_, err := client.Exec("test-cmd")
	if err != nil {
		t.Errorf("first exec: %v", err)
	}

	// Second call should reuse the same connection (no new dial).
	_, err = client.Exec("test-cmd")
	if err != nil {
		t.Errorf("second exec: %v", err)
	}
}

// failNthWriteConn wraps a net.Conn and fails exactly the Nth Write call
// with a synthetic error, letting earlier writes pass through to the
// underlying connection unmodified. Used to deterministically exercise the
// write-error paths in Exec without depending on real network failure
// timing.
type failNthWriteConn struct {
	net.Conn
	n     int
	calls int
}

func (f *failNthWriteConn) Write(p []byte) (int, error) {
	f.calls++
	if f.calls == f.n {
		return 0, fmt.Errorf("simulated write failure")
	}
	return f.Conn.Write(p)
}

// failSetDeadlineConn wraps a net.Conn and always fails SetDeadline, letting
// Read/Write pass through to the underlying connection unmodified.
type failSetDeadlineConn struct {
	net.Conn
}

func (f *failSetDeadlineConn) SetDeadline(time.Time) error {
	return fmt.Errorf("simulated set deadline failure")
}

// TestNuclearOptionDefaultPort verifies that port 0 is replaced with the
// protocol's documented default port.
func TestNuclearOptionDefaultPort(t *testing.T) {
	client := NewNuclearOption("127.0.0.1", 0, nil)
	if client.port != defaultNuclearOptionPort {
		t.Errorf("port: got %d, want %d", client.port, defaultNuclearOptionPort)
	}
}

// TestNuclearOptionDialFailure verifies that a dial failure (nothing
// listening on the target port) surfaces as an error from Exec.
func TestNuclearOptionDialFailure(t *testing.T) {
	// Reserve a port, then close the listener so nothing is listening there.
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	parts := strings.Split(addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 500 * time.Millisecond
	client.execDeadline = 1 * time.Second

	_, err = client.Exec("test-command")
	if err == nil {
		t.Fatal("expected dial error, got none")
	}
	if !strings.Contains(err.Error(), "dial") {
		t.Errorf("error: got %v, want to contain 'dial'", err)
	}
}

// TestNuclearOptionWriteLengthError verifies that a failure writing the
// request length header is reported and the connection is reset.
func TestNuclearOptionWriteLengthError(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()

	client := NewNuclearOption("127.0.0.1", 1, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second
	client.conn = &failNthWriteConn{Conn: local, n: 1}

	_, err := client.Exec("test-command")
	if err == nil {
		t.Fatal("expected write-length error, got none")
	}
	if !strings.Contains(err.Error(), "write length") {
		t.Errorf("error: got %v, want to contain 'write length'", err)
	}
	if client.conn != nil {
		t.Error("expected conn to be reset to nil after write error")
	}
}

// TestNuclearOptionWriteBodyError verifies that a failure writing the
// request body (after the length header succeeded) is reported and the
// connection is reset.
func TestNuclearOptionWriteBodyError(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()

	// The drain goroutine's completion travels through a buffered channel,
	// not a shared variable, so there's no race between it and the assertion
	// below.
	drainDone := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(io.Discard, remote)
		drainDone <- struct{}{}
	}()

	client := NewNuclearOption("127.0.0.1", 1, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second
	client.conn = &failNthWriteConn{Conn: local, n: 2}

	_, err := client.Exec("test-command")
	if err == nil {
		t.Fatal("expected write-body error, got none")
	}
	if !strings.Contains(err.Error(), "write body") {
		t.Errorf("error: got %v, want to contain 'write body'", err)
	}
	if client.conn != nil {
		t.Error("expected conn to be reset to nil after write error")
	}

	remote.Close()
	select {
	case <-drainDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for drain goroutine to finish")
	}
}

// TestNuclearOptionSetDeadlineError verifies that a failure setting the
// response read deadline (after the request was fully written) is reported
// and the connection is reset.
func TestNuclearOptionSetDeadlineError(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()

	drainDone := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(io.Discard, remote)
		drainDone <- struct{}{}
	}()

	client := NewNuclearOption("127.0.0.1", 1, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second
	client.conn = &failSetDeadlineConn{Conn: local}

	_, err := client.Exec("test-command")
	if err == nil {
		t.Fatal("expected set-deadline error, got none")
	}
	if !strings.Contains(err.Error(), "set deadline") {
		t.Errorf("error: got %v, want to contain 'set deadline'", err)
	}
	if client.conn != nil {
		t.Error("expected conn to be reset to nil after set-deadline error")
	}

	remote.Close()
	select {
	case <-drainDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for drain goroutine to finish")
	}
}

// TestNuclearOptionZeroExecDeadlineFallsBackToDefault verifies that a
// non-positive execDeadline falls back to the package default instead of
// leaving the read deadline unset.
func TestNuclearOptionZeroExecDeadlineFallsBackToDefault(t *testing.T) {
	server := newFakeTCPServer(t)
	defer server.close(t)

	parts := strings.Split(server.addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 0 // Falls back to defaultNuclearOptionExecDeadline.

	server.responses = map[string][2]interface{}{
		"test-cmd": {uint32(2000), nil},
	}

	if _, err := client.Exec("test-cmd"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestNuclearOptionTruncatedBodyLength tests handling of a connection that
// sends a full, valid status header but is then closed before the body
// length header arrives.
func TestNuclearOptionTruncatedBodyLength(t *testing.T) {
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	parts := strings.Split(addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	acceptDone := make(chan struct{}, 1)
	go func() {
		defer func() { acceptDone <- struct{}{} }()
		conn, aerr := listener.Accept()
		if aerr != nil {
			return
		}
		defer conn.Close()
		// Read request (don't care about it).
		buf := make([]byte, 1024)
		conn.Read(buf)
		// Send a full, valid status header, then close before sending the
		// body length header.
		statusBuf := make([]byte, 4)
		binary.LittleEndian.PutUint32(statusBuf, 2000)
		conn.Write(statusBuf)
	}()

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	_, err = client.Exec("test-command")
	if err == nil {
		t.Errorf("expected error for truncated body length, got none")
	}
	if !strings.Contains(err.Error(), "read body length") {
		t.Errorf("error: got %v, want to contain 'read body length'", err)
	}

	select {
	case <-acceptDone:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server goroutine to finish")
	}
}
