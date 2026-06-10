package quiesce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
)

type fakeRcon struct {
	mu       sync.Mutex
	got      []string
	respond  func(cmd string) (string, error)
	failNext map[string]error
}

func (f *fakeRcon) Exec(cmd string) (string, error) {
	f.mu.Lock()
	f.got = append(f.got, cmd)
	respond := f.respond
	if err, ok := f.failNext[cmd]; ok {
		delete(f.failNext, cmd)
		f.mu.Unlock()
		return "", err
	}
	f.mu.Unlock()
	if respond == nil {
		return "", nil
	}
	return respond(cmd)
}

func (f *fakeRcon) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.got))
	copy(out, f.got)
	return out
}

func TestPick(t *testing.T) {
	cases := []struct {
		game    string
		support bool
	}{
		{"minecraft", true},
		{"minecraft-java", true},
		{"  Minecraft-Java ", true},
		{"valheim", false},
		{"", false},
		{"unknown", false},
	}
	for _, tc := range cases {
		t.Run(tc.game, func(t *testing.T) {
			if got := Pick(tc.game).Supported(); got != tc.support {
				t.Errorf("Pick(%q).Supported() = %v, want %v", tc.game, got, tc.support)
			}
		})
	}
}

func TestMinecraftQuiesceSequence(t *testing.T) {
	rc := &fakeRcon{}
	q := minecraftQuiescer{}
	if err := q.Quiesce(rc); err != nil {
		t.Fatalf("Quiesce: %v", err)
	}
	want := []string{"save-off", "save-all flush"}
	if !reflect.DeepEqual(rc.calls(), want) {
		t.Errorf("Quiesce calls = %v, want %v", rc.calls(), want)
	}
}

func TestMinecraftQuiesceSaveAllFailureRollsBack(t *testing.T) {
	rc := &fakeRcon{failNext: map[string]error{"save-all flush": errors.New("connection reset")}}
	q := minecraftQuiescer{}
	if err := q.Quiesce(rc); err == nil {
		t.Fatal("expected error from save-all flush failure")
	}
	want := []string{"save-off", "save-all flush", "save-on"}
	if !reflect.DeepEqual(rc.calls(), want) {
		t.Errorf("Quiesce rollback calls = %v, want %v", rc.calls(), want)
	}
}

func TestMinecraftQuiesceFailureFromSavingFailedString(t *testing.T) {
	rc := &fakeRcon{respond: func(cmd string) (string, error) {
		if cmd == "save-all flush" {
			return "Saving failed: nothing to save", nil
		}
		return "", nil
	}}
	q := minecraftQuiescer{}
	if err := q.Quiesce(rc); err == nil {
		t.Fatal("expected quiesce error when server reports saving failed")
	}
	if calls := rc.calls(); calls[len(calls)-1] != "save-on" {
		t.Errorf("expected rollback save-on, got calls=%v", calls)
	}
}

func TestMinecraftUnquiesce(t *testing.T) {
	rc := &fakeRcon{}
	if err := (minecraftQuiescer{}).Unquiesce(rc); err != nil {
		t.Fatalf("Unquiesce: %v", err)
	}
	if got := rc.calls(); len(got) != 1 || got[0] != "save-on" {
		t.Errorf("Unquiesce calls = %v, want [save-on]", got)
	}
}

func TestUnsupportedQuiescerNoOp(t *testing.T) {
	rc := &fakeRcon{}
	u := unsupportedQuiescer{}
	if u.Supported() {
		t.Fatal("unsupported.Supported() = true")
	}
	if err := u.Quiesce(rc); err != nil {
		t.Errorf("Quiesce: %v", err)
	}
	if err := u.Unquiesce(rc); err != nil {
		t.Errorf("Unquiesce: %v", err)
	}
	if calls := rc.calls(); len(calls) != 0 {
		t.Errorf("unsupported quiescer should not invoke RCON, got %v", calls)
	}
}

// --- HTTP-level smoke tests --------------------------------------------------

func newTestRouter(rc Rcon, game string) *chi.Mux {
	r := chi.NewRouter()
	Mount(r, rc, game)
	return r
}

func decodeResponse(t *testing.T, body []byte) response {
	t.Helper()
	var got response
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got
}

func doPOST(t *testing.T, srv *httptest.Server, path string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return resp.StatusCode, out
}

func TestQuiesceHandlerMinecraftHappyPath(t *testing.T) {
	rc := &fakeRcon{}
	srv := httptest.NewServer(newTestRouter(rc, "minecraft-java"))
	defer srv.Close()

	status, body := doPOST(t, srv, "/quiesce")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	got := decodeResponse(t, body)
	if !got.Quiesced {
		t.Errorf("response = %+v, want Quiesced=true", got)
	}
}

func TestQuiesceHandlerUnsupportedGameDegrades(t *testing.T) {
	rc := &fakeRcon{}
	srv := httptest.NewServer(newTestRouter(rc, "valheim"))
	defer srv.Close()

	status, body := doPOST(t, srv, "/quiesce")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	got := decodeResponse(t, body)
	if got.Quiesced {
		t.Errorf("unsupported game should not report Quiesced=true: %+v", got)
	}
	if got.Reason == "" {
		t.Errorf("expected a non-empty Reason for unsupported game")
	}
	if calls := rc.calls(); len(calls) != 0 {
		t.Errorf("unsupported game must not invoke RCON, got %v", calls)
	}
}

func TestQuiesceHandlerRconErrorIs502(t *testing.T) {
	rc := &fakeRcon{failNext: map[string]error{"save-off": errors.New("connection refused")}}
	srv := httptest.NewServer(newTestRouter(rc, "minecraft"))
	defer srv.Close()

	status, _ := doPOST(t, srv, "/quiesce")
	if status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", status)
	}
}

func TestUnquiesceHandlerHappyPath(t *testing.T) {
	rc := &fakeRcon{}
	srv := httptest.NewServer(newTestRouter(rc, "minecraft-java"))
	defer srv.Close()

	status, body := doPOST(t, srv, "/unquiesce")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	got := decodeResponse(t, body)
	if got.Quiesced {
		t.Errorf("response = %+v, want Quiesced=false after unquiesce", got)
	}
	if calls := rc.calls(); len(calls) != 1 || calls[0] != "save-on" {
		t.Errorf("rcon calls = %v, want [save-on]", calls)
	}
}
