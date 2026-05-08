package console

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

type fakeRcon struct {
	gotCmd string
	out    string
	err    error
}

func (f *fakeRcon) Exec(cmd string) (string, error) {
	f.gotCmd = cmd
	return f.out, f.err
}

func newServer(t *testing.T, rc Rcon) (*httptest.Server, string) {
	t.Helper()
	r := chi.NewRouter()
	Mount(r, rc)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/console"
	return srv, wsURL
}

func dial(t *testing.T, wsURL string) (*websocket.Conn, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		cancel()
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		cancel()
	})
	return conn, ctx, cancel
}

func TestConsole_RoundTrip(t *testing.T) {
	rc := &fakeRcon{out: "pong"}
	_, wsURL := newServer(t, rc)
	conn, ctx, _ := dial(t, wsURL)

	if err := wsjson.Write(ctx, conn, Envelope{Kind: "cmd", Body: "ping"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	var got Envelope
	if err := wsjson.Read(ctx, conn, &got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Kind != "out" || got.Body != "pong" {
		t.Fatalf("got %+v", got)
	}
	if rc.gotCmd != "ping" {
		t.Fatalf("rcon got %q", rc.gotCmd)
	}
}

func TestConsole_RconError(t *testing.T) {
	_, wsURL := newServer(t, &fakeRcon{err: errors.New("rcon offline")})
	conn, ctx, _ := dial(t, wsURL)

	_ = wsjson.Write(ctx, conn, Envelope{Kind: "cmd", Body: "list"})
	var got Envelope
	if err := wsjson.Read(ctx, conn, &got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Kind != "err" || got.Body != "rcon offline" {
		t.Fatalf("got %+v", got)
	}
}

func TestConsole_IgnoresEmptyAndNonCmd(t *testing.T) {
	rc := &fakeRcon{out: "ok"}
	_, wsURL := newServer(t, rc)
	conn, ctx, _ := dial(t, wsURL)

	// Non-"cmd" kind: server should not call Rcon and should not respond.
	_ = wsjson.Write(ctx, conn, Envelope{Kind: "noop", Body: "x"})
	// Empty body cmd: also ignored.
	_ = wsjson.Write(ctx, conn, Envelope{Kind: "cmd", Body: ""})
	// Real one: must respond.
	_ = wsjson.Write(ctx, conn, Envelope{Kind: "cmd", Body: "go"})

	var got Envelope
	if err := wsjson.Read(ctx, conn, &got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Body != "ok" || rc.gotCmd != "go" {
		t.Fatalf("got=%+v rcon=%q", got, rc.gotCmd)
	}
}

func TestConsole_ClientCloseEndsLoop(t *testing.T) {
	_, wsURL := newServer(t, &fakeRcon{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := conn.Close(websocket.StatusNormalClosure, "bye"); err != nil {
		t.Fatalf("close: %v", err)
	}
	// Give the server loop a moment to observe the close. Nothing to
	// assert beyond "no panic, no goroutine leak"; coverage of the
	// read-error return path is the goal.
	time.Sleep(50 * time.Millisecond)
}
