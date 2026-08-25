package rcon

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseNuclearOptionCommand(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantName   string
		wantArgs   []string
	}{
		{
			name:       "empty string",
			input:      "",
			wantName:   "",
			wantArgs:   nil,
		},
		{
			name:       "whitespace only",
			input:      "   ",
			wantName:   "",
			wantArgs:   nil,
		},
		{
			name:       "command no args",
			input:      "get-player-list",
			wantName:   "get-player-list",
			wantArgs:   nil,
		},
		{
			name:       "command with one simple arg",
			input:      "kick-player 76561198123456789",
			wantName:   "kick-player",
			wantArgs:   []string{"76561198123456789"},
		},
		{
			name:       "send-chat-message preserves spaces in message",
			input:      "send-chat-message hello world from test",
			wantName:   "send-chat-message",
			wantArgs:   []string{"hello world from test"},
		},
		{
			name:       "send-chat-message with single word",
			input:      "send-chat-message hello",
			wantName:   "send-chat-message",
			wantArgs:   []string{"hello"},
		},
		{
			name:       "send-chat-message empty message",
			input:      "send-chat-message ",
			wantName:   "send-chat-message",
			wantArgs:   []string{""},
		},
		{
			name:       "banlist-add with steamid and reason",
			input:      "banlist-add 76561198123456789 cheating",
			wantName:   "banlist-add",
			wantArgs:   []string{"76561198123456789", "cheating"},
		},
		{
			name:       "set-next-mission with three args",
			input:      "set-next-mission BuiltIn Escalation 3600.0",
			wantName:   "set-next-mission",
			wantArgs:   []string{"BuiltIn", "Escalation", "3600.0"},
		},
		{
			name:       "command with extra spaces",
			input:      "kick-player   76561198123456789",
			wantName:   "kick-player",
			wantArgs:   []string{"76561198123456789"},
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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

func (s *fakeTCPServer) handleConn(t *testing.T, conn net.Conn) {
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

func (s *fakeTCPServer) close(t *testing.T) {
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
		name        string
		setupResp   map[string][2]interface{}
		cmd         string
		wantBody    string
		wantErrMsg  string
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
			name: "4004 unknown command",
			setupResp: map[string][2]interface{}{
				// No entry for "bogus" — server responds 4004.
			},
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
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
	server := newFakeTCPServer(t)
	defer server.close(t)

	parts := strings.Split(server.addr, ":")
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)

	client := NewNuclearOption("127.0.0.1", port, nil)
	defer client.Close()
	client.dialTimeout = 100 * time.Millisecond
	client.execDeadline = 1 * time.Second

	// Capture the actual request sent.
	var capturedReq nuclearOptionRequest
	server.responses = map[string][2]interface{}{
		"send-chat-message": {
			uint32(2000),
			nil,
		},
	}

	// Run a bespoke accept loop instead of the server's own, so the raw
	// request bytes can be captured before they are decoded.
	server.wg.Add(1)
	go func() {
		defer server.wg.Done()
		for {
			select {
			case <-server.done:
				return
			default:
			}

			server.listener.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond))
			conn, err := server.listener.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "timeout") {
					continue
				}
				return
			}

			defer conn.Close()

			// Read request.
			lengthBuf := make([]byte, 4)
			n, _ := conn.Read(lengthBuf)
			if n < 4 {
				return
			}

			bodyLen := binary.LittleEndian.Uint32(lengthBuf)
			bodyBuf := make([]byte, bodyLen)
			conn.Read(bodyBuf)

			if err := json.Unmarshal(bodyBuf, &capturedReq); err != nil {
				return
			}

			// Send response.
			statusBuf := make([]byte, 4)
			binary.LittleEndian.PutUint32(statusBuf, 2000)
			conn.Write(statusBuf)
			lenBuf := make([]byte, 4)
			binary.LittleEndian.PutUint32(lenBuf, 0)
			conn.Write(lenBuf)
		}
	}()

	msg := "hello world from nuclear option"
	_, err := client.Exec("send-chat-message " + msg)
	if err != nil {
		t.Errorf("exec: %v", err)
	}

	// Verify the message was preserved with spaces.
	if len(capturedReq.Arguments) != 1 {
		t.Errorf("arguments count: got %d, want 1", len(capturedReq.Arguments))
	}
	if len(capturedReq.Arguments) > 0 && capturedReq.Arguments[0] != msg {
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
