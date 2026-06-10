package ws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"k8s.io/client-go/tools/remotecommand"
)

// TestTermSizeQueue covers Next/push/close in isolation — pure
// concurrency primitives that can be unit-tested without the rest of
// the attach pipeline.
func TestTermSizeQueue_PushAndNext(t *testing.T) {
	q := newTermSizeQueue()
	q.push(remotecommand.TerminalSize{Width: 80, Height: 24})
	got := q.Next()
	if got == nil || got.Width != 80 || got.Height != 24 {
		t.Fatalf("got %+v", got)
	}
}

func TestTermSizeQueue_NextReturnsNilOnClose(t *testing.T) {
	q := newTermSizeQueue()
	done := make(chan *remotecommand.TerminalSize, 1)
	go func() { done <- q.Next() }()
	q.close()
	select {
	case got := <-done:
		if got != nil {
			t.Fatalf("expected nil after close, got %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("Next did not return after close")
	}
}

func TestTermSizeQueue_PushDropsWhenFull(t *testing.T) {
	q := newTermSizeQueue()
	for i := 0; i < 10; i++ {
		q.push(remotecommand.TerminalSize{Width: uint16(i + 1), Height: 24})
	}
	// First 4 enqueued, the rest dropped — drain.
	count := 0
	for count < 4 {
		got := q.Next()
		if got == nil {
			break
		}
		count++
	}
	q.close()
	if count != 4 {
		t.Fatalf("got %d messages from a buffer of 4", count)
	}
}

func TestTermSizeQueue_DoubleClose(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("double close panicked: %v", r)
		}
	}()
	q := newTermSizeQueue()
	q.close()
	q.close()
}

// TestPumpStdout sends a single read of bytes through the pump and
// asserts the WS receives a base64-framed envelope.
func TestPumpStdout(t *testing.T) {
	upgraded := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		upgraded <- c
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, dialResp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if dialResp != nil && dialResp.Body != nil {
		defer dialResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close(websocket.StatusNormalClosure, "")
	srvConn := <-upgraded
	defer srvConn.Close(websocket.StatusNormalClosure, "")

	pr, pw := io.Pipe()
	go pumpStdout(ctx, srvConn, pr)

	go func() { _, _ = pw.Write([]byte("hello world")); _ = pw.Close() }()

	mt, data, err := cli.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.MessageText {
		t.Fatalf("kind=%v", mt)
	}
	var env ptyEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Kind != "stdout" {
		t.Fatalf("kind=%q", env.Kind)
	}
	raw, _ := base64.StdEncoding.DecodeString(env.Body)
	if string(raw) != "hello world" {
		t.Fatalf("body=%q", raw)
	}
}

// TestPumpBrowser drives stdin + resize envelopes from the client side
// and asserts the writer + termSizeQueue receive them.
func TestPumpBrowser_StdinAndResize(t *testing.T) {
	upgraded := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		upgraded <- c
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cli, dialResp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if dialResp != nil && dialResp.Body != nil {
		defer dialResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer cli.Close(websocket.StatusNormalClosure, "")
	srvConn := <-upgraded

	pr, pw := io.Pipe()
	sizes := newTermSizeQueue()
	pumpCtx, pumpCancel := context.WithCancel(ctx)
	defer pumpCancel()
	go pumpBrowser(pumpCtx, srvConn, pw, sizes, pumpCancel)

	// Send "stdin" + "resize" + an unknown kind + a malformed JSON frame
	// (which should be ignored, not crash the pump).
	stdin := ptyEnvelope{Kind: "stdin", Body: base64.StdEncoding.EncodeToString([]byte("ls\n"))}
	resize := ptyEnvelope{Kind: "resize", Cols: 100, Rows: 30}
	zero := ptyEnvelope{Kind: "resize", Cols: 0, Rows: 30}
	for _, e := range []ptyEnvelope{stdin, resize, zero, {Kind: "unknown"}} {
		b, _ := json.Marshal(e)
		_ = cli.Write(ctx, websocket.MessageText, b)
	}
	_ = cli.Write(ctx, websocket.MessageText, []byte("not json"))

	// Read what the pump wrote to stdin.
	got := make([]byte, 16)
	n, err := pr.Read(got)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	if string(got[:n]) != "ls\n" {
		t.Fatalf("stdin=%q", got[:n])
	}

	// Resize event landed in queue.
	sz := sizes.Next()
	if sz == nil || sz.Width != 100 || sz.Height != 30 {
		t.Fatalf("resize=%+v", sz)
	}

	// Closing the WS triggers the pump to exit and call cancel.
	_ = cli.Close(websocket.StatusNormalClosure, "")
	select {
	case <-pumpCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not cancel context on read error")
	}
}

func TestPumpBrowser_BadBase64Stdin(t *testing.T) {
	upgraded := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		upgraded <- c
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cli, dialResp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if dialResp != nil && dialResp.Body != nil {
		defer dialResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	srvConn := <-upgraded

	pr, pw := io.Pipe()
	defer pr.Close()
	pumpCtx, pumpCancel := context.WithCancel(ctx)
	defer pumpCancel()
	go pumpBrowser(pumpCtx, srvConn, pw, newTermSizeQueue(), pumpCancel)

	// Bad base64 → continue, no write to stdin.
	bad, _ := json.Marshal(ptyEnvelope{Kind: "stdin", Body: "!!!not base64!!!"})
	_ = cli.Write(ctx, websocket.MessageText, bad)

	// Then a valid frame.
	good, _ := json.Marshal(ptyEnvelope{Kind: "stdin", Body: base64.StdEncoding.EncodeToString([]byte("x"))})
	_ = cli.Write(ctx, websocket.MessageText, good)

	got := make([]byte, 4)
	if _, err := pr.Read(got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got[0] != 'x' {
		t.Fatalf("got %q", got[:1])
	}
	_ = cli.Close(websocket.StatusNormalClosure, "")
}

// TestWriteEnvErr exercises writeEnvErr against a closed connection
// (must not panic, must not block).
func TestWriteEnvErr_OnClosedConn(t *testing.T) {
	upgraded := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		upgraded <- c
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cli, dialResp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if dialResp != nil && dialResp.Body != nil {
		defer dialResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	srvConn := <-upgraded
	_ = cli.Close(websocket.StatusNormalClosure, "")
	// Give the close a moment to propagate.
	time.Sleep(20 * time.Millisecond)
	writeEnvErr(ctx, srvConn, "anything") // must not panic
}
