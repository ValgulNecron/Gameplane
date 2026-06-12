package players

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newSrv(t *testing.T, game string, rc Rcon) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	Mount(r, rc, game, nil)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func testGet(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return http.DefaultClient.Do(req)
}

func testPost(t *testing.T, url, contentType string, body io.Reader) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}

func TestBanHappyPath(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) { return "Banned alice", nil }}
	srv := newSrv(t, "minecraft-java", rc)
	body, _ := json.Marshal(modReq{Name: "alice", Reason: "x"})
	resp, err := testPost(t, srv.URL+"/players/ban", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, b)
	}
	if rc.last != "ban alice x" {
		t.Fatalf("rcon=%q", rc.last)
	}
}

func TestUnbanHappyPath_IgnoresReason(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) { return "Pardoned alice", nil }}
	srv := newSrv(t, "minecraft-java", rc)
	body, _ := json.Marshal(modReq{Name: "alice", Reason: "ignored"})
	resp, err := testPost(t, srv.URL+"/players/unban", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if rc.last != "pardon alice" {
		t.Fatalf("rcon=%q", rc.last)
	}
}

func TestRunMod_InvalidJSON(t *testing.T) {
	rc := &fakeRcon{}
	srv := newSrv(t, "minecraft-java", rc)
	resp, err := testPost(t, srv.URL+"/players/kick", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestServe_RconError(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "", errors.New("upstream broken: 127.0.0.1:25575")
	}}
	srv := newSrv(t, "minecraft-java", rc)
	resp, err := testGet(t, srv.URL+"/players")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(b), "127.0.0.1") {
		t.Fatal("upstream detail leaked")
	}
}

func TestServe_CacheReturnsLastResult(t *testing.T) {
	count := 0
	rc := &fakeRcon{respond: func(string) (string, error) {
		count++
		return "There are 1 of a max of 20 players online: alice", nil
	}}
	srv := newSrv(t, "minecraft-java", rc)
	for i := 0; i < 3; i++ {
		resp, err := testGet(t, srv.URL+"/players")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		_ = resp.Body.Close()
	}
	if count != 1 {
		t.Fatalf("rcon called %d times, want 1 (cache should hold)", count)
	}
}

func TestBanned_SupportedGameSuccess(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) {
		return strings.Join([]string{
			"There are 1 bans:",
			"griefer was banned by Server: rule violation",
		}, "\n"), nil
	}}
	srv := newSrv(t, "minecraft-java", rc)
	resp, err := testGet(t, srv.URL+"/players/banned")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var got []BannedPlayer
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "griefer" {
		t.Fatalf("got %+v", got)
	}
	if rc.last != "banlist players" {
		t.Fatalf("rcon=%q", rc.last)
	}
}

func TestBanned_RconError(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) { return "", errors.New("rcon down") }}
	srv := newSrv(t, "minecraft-java", rc)
	resp, err := testGet(t, srv.URL+"/players/banned")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestUnsupportedCommander_ParseBanList(t *testing.T) {
	got := unsupportedCommander{}.ParseBanList("anything")
	if got != nil {
		t.Fatalf("got %+v", got)
	}
}
