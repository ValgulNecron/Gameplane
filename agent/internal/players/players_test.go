package players

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
)

func TestParseList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Snapshot
	}{
		{
			"empty max-of form",
			"There are 0 of a max of 20 players online:",
			Snapshot{Online: 0, Max: 20, Players: []string{}},
		},
		{
			"two players max-of form",
			"There are 2 of a max of 20 players online: alice, bob",
			Snapshot{Online: 2, Max: 20, Players: []string{"alice", "bob"}},
		},
		{
			"slash form",
			"There are 3/30 players online: a, b, c",
			Snapshot{Online: 3, Max: 30, Players: []string{"a", "b", "c"}},
		},
		{
			"unrecognized line",
			"nonsense",
			Snapshot{Players: []string{}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseList(tc.in)
			if !reflect.DeepEqual(got.Players, tc.want.Players) ||
				got.Online != tc.want.Online || got.Max != tc.want.Max {
				t.Errorf("parseList = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseCounts(t *testing.T) {
	cases := []struct {
		name            string
		in              string
		wantOn, wantMax int
		wantOK          bool
	}{
		{"max-of form", "There are 0 of a max of 20 players online:", 0, 20, true},
		{"populated max-of", "There are 2 of a max of 20 players online: alice, bob", 2, 20, true},
		{"slash form", "There are 3/30 players online: a, b, c", 3, 30, true},
		{"unrecognized line", "nonsense", 0, 0, false},
		{"partial line", "There are 4 of a max", 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			on, mx, ok := ParseCounts(tc.in)
			if on != tc.wantOn || mx != tc.wantMax || ok != tc.wantOK {
				t.Errorf("ParseCounts(%q) = (%d, %d, %v), want (%d, %d, %v)",
					tc.in, on, mx, ok, tc.wantOn, tc.wantMax, tc.wantOK)
			}
		})
	}
}

func TestDeclaredMinecraftCommander(t *testing.T) {
	c := newTemplateCommander(minecraftActions())
	caps := c.Capabilities()
	if !caps.Kick || !caps.Ban || !caps.Unban {
		t.Fatalf("minecraft caps want all true, got %+v", caps)
	}

	cases := []struct {
		name          string
		fn            func() (string, bool)
		wantCmd       string
		wantSupported bool
	}{
		{"kick no reason", func() (string, bool) { return c.Kick("alice", "") }, "kick alice", true},
		{"kick with reason", func() (string, bool) { return c.Kick("alice", "griefing") }, "kick alice griefing", true},
		{"ban no reason", func() (string, bool) { return c.Ban("alice", "") }, "ban alice", true},
		{"ban with reason", func() (string, bool) { return c.Ban("alice", "x-ray") }, "ban alice x-ray", true},
		{"unban", func() (string, bool) { return c.Unban("alice") }, "pardon alice", true},
		{"banlist", func() (string, bool) { return c.BanList() }, "banlist players", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, ok := tc.fn()
			if ok != tc.wantSupported {
				t.Errorf("supported = %v, want %v", ok, tc.wantSupported)
			}
			if cmd != tc.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tc.wantCmd)
			}
		})
	}
}

func TestUnsupportedCommander(t *testing.T) {
	c := unsupportedCommander{}
	if caps := c.Capabilities(); caps.Kick || caps.Ban || caps.Unban {
		t.Errorf("unsupported caps want all false, got %+v", caps)
	}
	for _, fn := range []func() (string, bool){
		func() (string, bool) { return c.Kick("a", "") },
		func() (string, bool) { return c.Ban("a", "") },
		func() (string, bool) { return c.Unban("a") },
		func() (string, bool) { return c.BanList() },
	} {
		if _, ok := fn(); ok {
			t.Errorf("unsupported commander returned ok=true")
		}
	}
}

func TestPickCommander(t *testing.T) {
	// Moderation support comes only from declared actions; nothing
	// declared is always unsupported.
	if got := pickCommander(minecraftActions()).Capabilities().Kick; !got {
		t.Error("declared actions should be supported")
	}
	if got := pickCommander(nil).Capabilities().Kick; got {
		t.Error("nil actions should be unsupported")
	}
}

func TestParseBanList(t *testing.T) {
	raw := strings.Join([]string{
		"There are 3 bans:",
		"griefer1 was banned by Server: Cheating",
		"griefer2 was banned by alice: <no reason given>",
		"griefer3 was banned by Server",
		"",
	}, "\n")
	got := newTemplateCommander(minecraftActions()).ParseBanList(raw)
	want := []BannedPlayer{
		{Name: "griefer1", Source: "Server", Reason: "Cheating"},
		{Name: "griefer2", Source: "alice"},
		{Name: "griefer3", Source: "Server"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseBanList = %+v, want %+v", got, want)
	}
}

func TestValidateName(t *testing.T) {
	good := []string{"alice", "Bob_42", "x", strings.Repeat("a", 32)}
	for _, n := range good {
		if err := validateName(n); err != nil {
			t.Errorf("validateName(%q) unexpected err: %v", n, err)
		}
	}
	bad := []string{"", "alice bob", "alice;ban", "alice\nban", "alice/etc", strings.Repeat("a", 33)}
	for _, n := range bad {
		if err := validateName(n); err == nil {
			t.Errorf("validateName(%q) expected error", n)
		}
	}
}

func TestSanitizeReason(t *testing.T) {
	if _, err := sanitizeReason("ok"); err != nil {
		t.Errorf("plain reason: %v", err)
	}
	if _, err := sanitizeReason("multi\nline"); err == nil {
		t.Errorf("expected newline rejection")
	}
	long := strings.Repeat("x", 300)
	got, err := sanitizeReason(long)
	if err != nil {
		t.Fatalf("oversize reason: %v", err)
	}
	if len(got) != 256 {
		t.Errorf("oversize truncated to %d, want 256", len(got))
	}
}

// --- handler-level tests over httptest server ---

type fakeRcon struct {
	mu      sync.Mutex
	last    string
	respond func(cmd string) (string, error)
}

func (f *fakeRcon) Exec(cmd string) (string, error) {
	f.mu.Lock()
	f.last = cmd
	respond := f.respond
	f.mu.Unlock()
	if respond == nil {
		return "", nil
	}
	return respond(cmd)
}

func newTestRouter(t *testing.T, game string, rc Rcon) *chi.Mux {
	t.Helper()
	r := chi.NewRouter()
	Mount(r, rc, game, minecraftActions())
	return r
}

// doJSON sends a request and returns just the status + bytes; it owns
// the response body lifecycle so the test sites don't need to.
func doJSON(t *testing.T, srv *httptest.Server, method, path string, body any) (int, []byte) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, srv.URL+path, buf)
	if err != nil {
		t.Fatalf("new req: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return resp.StatusCode, out
}

func TestKickHappyPath(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) { return "Kicked alice", nil }}
	srv := httptest.NewServer(newTestRouter(t, "minecraft-java", rc))
	defer srv.Close()

	status, body := doJSON(t, srv, "POST", "/players/kick", modReq{Name: "alice", Reason: "afk"})
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got modResp
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK || got.Raw != "Kicked alice" {
		t.Errorf("response = %+v", got)
	}
	if rc.last != "kick alice afk" {
		t.Errorf("rcon called with %q", rc.last)
	}
}

func TestKickValidation(t *testing.T) {
	rc := &fakeRcon{}
	srv := httptest.NewServer(newTestRouter(t, "minecraft-java", rc))
	defer srv.Close()

	cases := []struct {
		name string
		body modReq
		want int
	}{
		{"empty name", modReq{Name: ""}, http.StatusBadRequest},
		{"bad name", modReq{Name: "alice;rm"}, http.StatusBadRequest},
		{"newline reason", modReq{Name: "alice", Reason: "x\ny"}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := doJSON(t, srv, "POST", "/players/kick", tc.body)
			if status != tc.want {
				t.Errorf("status = %d, want %d", status, tc.want)
			}
			if rc.last != "" {
				t.Errorf("rcon should not be invoked, got %q", rc.last)
			}
			rc.last = ""
		})
	}
}

func TestKickUnsupportedGame(t *testing.T) {
	rc := &fakeRcon{}
	r := chi.NewRouter()
	Mount(r, rc, "valheim", nil) // no declared actions → moderation unsupported
	srv := httptest.NewServer(r)
	defer srv.Close()

	status, _ := doJSON(t, srv, "POST", "/players/kick", modReq{Name: "alice"})
	if status != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", status)
	}
	if rc.last != "" {
		t.Errorf("rcon should not be invoked for unsupported game, got %q", rc.last)
	}
}

func TestKickRconError(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "", errors.New("dial 127.0.0.1:25575: connection refused")
	}}
	srv := httptest.NewServer(newTestRouter(t, "minecraft-java", rc))
	defer srv.Close()

	status, body := doJSON(t, srv, "POST", "/players/kick", modReq{Name: "alice"})
	if status != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	// Don't leak rcon error detail (host/port).
	if strings.Contains(string(body), "127.0.0.1") {
		t.Errorf("response leaked upstream detail: %s", body)
	}
}

func TestBannedListEmptyForUnsupported(t *testing.T) {
	rc := &fakeRcon{}
	r := chi.NewRouter()
	Mount(r, rc, "valheim", nil) // no declared actions → ban list unsupported
	srv := httptest.NewServer(r)
	defer srv.Close()

	status, body := doJSON(t, srv, "GET", "/players/banned", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got []BannedPlayer
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty banned list, got %+v", got)
	}
	if rc.last != "" {
		t.Errorf("rcon should not be invoked, got %q", rc.last)
	}
}

func TestServeIncludesCapabilities(t *testing.T) {
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "There are 1 of a max of 20 players online: alice", nil
	}}
	srv := httptest.NewServer(newTestRouter(t, "minecraft-java", rc))
	defer srv.Close()

	status, body := doJSON(t, srv, "GET", "/players", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got Snapshot
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Capabilities.Kick || !got.Capabilities.Ban || !got.Capabilities.Unban {
		t.Errorf("capabilities = %+v, want all true", got.Capabilities)
	}
}

// --- Player list tests ---

func TestPlayerListDefaultCommand(t *testing.T) {
	// When no List is configured, the handler should use "list" command
	// and the built-in Minecraft parser.
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "There are 2 of a max of 20 players online: alice, bob", nil
	}}
	r := chi.NewRouter()
	Mount(r, rc, "minecraft-java", minecraftActions()) // no List configured
	srv := httptest.NewServer(r)
	defer srv.Close()

	status, body := doJSON(t, srv, "GET", "/players", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got Snapshot
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rc.last != "list" {
		t.Errorf("rcon called with %q, want 'list'", rc.last)
	}
	if got.Online != 2 || got.Max != 20 {
		t.Errorf("counts = (%d, %d), want (2, 20)", got.Online, got.Max)
	}
	want := []string{"alice", "bob"}
	if !reflect.DeepEqual(got.Players, want) {
		t.Errorf("players = %v, want %v", got.Players, want)
	}
}

func TestPlayerListCustomCommand(t *testing.T) {
	// When a custom command is configured, it should be used instead of "list".
	actions := &caps.PlayerActions{
		Kick: "kick {{.Player}}",
		List: &caps.PlayerList{
			Command: "online",
		},
	}
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "There are 1 of a max of 10 players online: charlie", nil
	}}
	r := chi.NewRouter()
	Mount(r, rc, "some-game", actions)
	srv := httptest.NewServer(r)
	defer srv.Close()

	status, body := doJSON(t, srv, "GET", "/players", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got Snapshot
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rc.last != "online" {
		t.Errorf("rcon called with %q, want 'online'", rc.last)
	}
	if got.Online != 1 || got.Max != 10 {
		t.Errorf("counts = (%d, %d), want (1, 10)", got.Online, got.Max)
	}
}

func TestPlayerListWithRegex(t *testing.T) {
	// When EntryRegex is configured, custom parsing is used.
	actions := &caps.PlayerActions{
		Kick: "kick {{.Player}}",
		List: &caps.PlayerList{
			Command:    "players",
			EntryRegex: `(?P<name>\w+)`,
		},
	}
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "Online: alice, bob, charlie", nil
	}}
	r := chi.NewRouter()
	Mount(r, rc, "some-game", actions)
	srv := httptest.NewServer(r)
	defer srv.Close()

	status, body := doJSON(t, srv, "GET", "/players", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got Snapshot
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"Online", "alice", "bob", "charlie"}
	if !reflect.DeepEqual(got.Players, want) {
		t.Errorf("players = %v, want %v", got.Players, want)
	}
}

func TestPlayerListRegexWithCaptureGroup(t *testing.T) {
	// When EntryRegex has a capture group, only that group is extracted.
	actions := &caps.PlayerActions{
		Kick: "kick {{.Player}}",
		List: &caps.PlayerList{
			Command:    "players",
			EntryRegex: `\s*(\w+)\s*,?`,
		},
	}
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "alice, bob, charlie", nil
	}}
	r := chi.NewRouter()
	Mount(r, rc, "some-game", actions)
	srv := httptest.NewServer(r)
	defer srv.Close()

	status, body := doJSON(t, srv, "GET", "/players", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got Snapshot
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"alice", "bob", "charlie"}
	if !reflect.DeepEqual(got.Players, want) {
		t.Errorf("players = %v, want %v", got.Players, want)
	}
}

func TestPlayerListInvalidRegexFallback(t *testing.T) {
	// When EntryRegex is invalid, the handler should log and fall back
	// to the built-in parser.
	actions := &caps.PlayerActions{
		Kick: "kick {{.Player}}",
		List: &caps.PlayerList{
			Command:    "list",
			EntryRegex: `(?P<invalid_group)[invalid regex`,
		},
	}
	rc := &fakeRcon{respond: func(string) (string, error) {
		return "There are 1 of a max of 20 players online: alice", nil
	}}
	r := chi.NewRouter()
	Mount(r, rc, "minecraft-java", actions)
	srv := httptest.NewServer(r)
	defer srv.Close()

	status, body := doJSON(t, srv, "GET", "/players", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %s", status, body)
	}
	var got Snapshot
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Should fall back to built-in parser and parse the Minecraft format.
	want := []string{"alice"}
	if !reflect.DeepEqual(got.Players, want) {
		t.Errorf("players = %v, want %v (should have fallen back to built-in)", got.Players, want)
	}
}
