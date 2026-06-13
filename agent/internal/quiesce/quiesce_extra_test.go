package quiesce

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnquiesceHandler_UnsupportedGame(t *testing.T) {
	rc := &fakeRcon{}
	srv := httptest.NewServer(newTestRouter(rc, nil))
	defer srv.Close()

	status, body := doPOST(t, srv, "/unquiesce")
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, body)
	}
	got := decodeResponse(t, body)
	if got.Reason == "" {
		t.Fatal("want non-empty reason for unsupported game")
	}
}

func TestUnquiesceHandler_RconError(t *testing.T) {
	rc := &fakeRcon{failNext: map[string]error{"save-on": errors.New("connection reset")}}
	srv := httptest.NewServer(newTestRouter(rc, minecraftQuiesceSpec()))
	defer srv.Close()

	status, _ := doPOST(t, srv, "/unquiesce")
	if status != http.StatusBadGateway {
		t.Fatalf("status=%d", status)
	}
}
