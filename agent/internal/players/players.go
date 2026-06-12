// Package players exposes online-player and moderation endpoints
// backed by RCON.
//
// Many game protocols are different, but "list online players via RCON
// list" works for all Minecraft variants (vanilla/paper/spigot/forge)
// and Source-engine servers. Moderation commands (kick/ban/unban) vary
// per game and are dispatched through a small commander strategy keyed
// off the agent's --game flag.
package players

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kestrel-gg/kestrel/agent/internal/caps"
	"github.com/kestrel-gg/kestrel/agent/internal/rcon"
)

type Rcon interface {
	Exec(cmd string) (string, error)
}

type handler struct {
	rcon Rcon
	cmdr commander
	game string

	mu         sync.Mutex
	lastFetch  time.Time
	lastResult Snapshot
	lastErr    error
}

type Capabilities struct {
	Kick  bool `json:"kick"`
	Ban   bool `json:"ban"`
	Unban bool `json:"unban"`
}

type Snapshot struct {
	Online       int          `json:"online"`
	Max          int          `json:"max"`
	Players      []string     `json:"players"`
	AsOf         string       `json:"asOf"`
	Capabilities Capabilities `json:"capabilities"`
}

type BannedPlayer struct {
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
	Source string `json:"source,omitempty"`
}

// Mount wires the player endpoints. actions carries the module's
// declared moderation commands (nil when the template declares none —
// known games then fall back to built-in commands).
func Mount(r chi.Router, rc Rcon, game string, actions *caps.PlayerActions) {
	h := &handler{rcon: rc, cmdr: pickCommander(game, actions), game: game}
	r.Get("/players", h.serve)
	r.Get("/players/banned", h.banned)
	r.Post("/players/kick", h.kick)
	r.Post("/players/ban", h.ban)
	r.Post("/players/unban", h.unban)
}

const cacheTTL = 5 * time.Second

func (h *handler) serve(w http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	fresh := time.Since(h.lastFetch) < cacheTTL
	result, err := h.lastResult, h.lastErr
	h.mu.Unlock()

	if !fresh {
		result, err = h.fetch()
	}
	if errors.Is(err, rcon.ErrDisabled) {
		// The game has no RCON: player counts are simply unknown. The
		// dashboard contract is a valid snapshot with online=-1, not an
		// error — there is no upstream to be unavailable.
		writeJSON(w, http.StatusOK, Snapshot{
			Online:  -1,
			Max:     -1,
			Players: []string{},
			AsOf:    time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	if err != nil {
		// err can contain RCON protocol detail (addresses, passwords from
		// poorly-written server mods, etc.). Never reflect it.
		slog.Warn("players rcon", "err", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	result.Capabilities = h.cmdr.Capabilities()
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) fetch() (Snapshot, error) {
	raw, err := h.rcon.Exec("list")
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastFetch = time.Now()
	if err != nil {
		h.lastErr = err
		return Snapshot{}, err
	}
	snap := parseList(raw)
	snap.AsOf = h.lastFetch.UTC().Format(time.RFC3339)
	h.lastResult = snap
	h.lastErr = nil
	return snap, nil
}

func (h *handler) bustCache() {
	h.mu.Lock()
	h.lastFetch = time.Time{}
	h.mu.Unlock()
}

type modReq struct {
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
}

type modResp struct {
	OK  bool   `json:"ok"`
	Raw string `json:"raw,omitempty"`
}

func (h *handler) kick(w http.ResponseWriter, req *http.Request) {
	h.runMod(w, req, h.cmdr.Kick, true)
}

func (h *handler) ban(w http.ResponseWriter, req *http.Request) {
	h.runMod(w, req, h.cmdr.Ban, true)
}

func (h *handler) unban(w http.ResponseWriter, req *http.Request) {
	// Unban takes a name only; reason is ignored even if supplied.
	h.runMod(w, req, func(name, _ string) (string, bool) {
		return h.cmdr.Unban(name)
	}, false)
}

func (h *handler) runMod(
	w http.ResponseWriter, req *http.Request,
	build func(name, reason string) (string, bool),
	wantReason bool,
) {
	var body modReq
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 4<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := validateName(body.Name); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if wantReason {
		clean, err := sanitizeReason(body.Reason)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		body.Reason = clean
	}
	cmd, ok := build(body.Name, body.Reason)
	if !ok {
		writeErr(w, http.StatusNotImplemented, fmt.Sprintf("not supported by %s", h.game))
		return
	}
	raw, err := h.rcon.Exec(cmd)
	if errors.Is(err, rcon.ErrDisabled) {
		writeErr(w, http.StatusNotImplemented, fmt.Sprintf("not supported by %s (no RCON)", h.game))
		return
	}
	if err != nil {
		slog.Warn("players moderation rcon", "cmd", cmd, "err", err)
		writeErr(w, http.StatusBadGateway, "upstream unavailable")
		return
	}
	h.bustCache()
	writeJSON(w, http.StatusOK, modResp{OK: true, Raw: strings.TrimSpace(raw)})
}

func (h *handler) banned(w http.ResponseWriter, _ *http.Request) {
	cmd, ok := h.cmdr.BanList()
	if !ok {
		// Empty list rather than 501: callers can render a Banned tab
		// uniformly for every game; capabilities advertise reality.
		writeJSON(w, http.StatusOK, []BannedPlayer{})
		return
	}
	raw, err := h.rcon.Exec(cmd)
	if errors.Is(err, rcon.ErrDisabled) {
		writeJSON(w, http.StatusOK, []BannedPlayer{})
		return
	}
	if err != nil {
		slog.Warn("banlist rcon", "err", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	out := h.cmdr.ParseBanList(raw)
	if out == nil {
		out = []BannedPlayer{}
	}
	writeJSON(w, http.StatusOK, out)
}

// validateName rejects anything that isn't a printable username so that
// the RCON command we build can never be smuggled with extra arguments
// or newlines that would chain a second command.
var nameRE = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)

func validateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if !nameRE.MatchString(name) {
		return errors.New("name must be 1-32 chars [A-Za-z0-9_]")
	}
	return nil
}

func sanitizeReason(reason string) (string, error) {
	if strings.ContainsAny(reason, "\r\n") {
		return "", errors.New("reason must not contain newlines")
	}
	if len(reason) > 256 {
		reason = reason[:256]
	}
	return reason, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Minecraft "list" responses look like one of:
//
//	There are 0 of a max of 20 players online:
//	There are 2 of a max of 20 players online: alice, bob
//
// Older server versions also say "There are 2/20 players online:".
var (
	reMaxOf = regexp.MustCompile(`There are (\d+) of a max of (\d+) players online:?\s*(.*)`)
	reSlash = regexp.MustCompile(`There are (\d+)/(\d+) players online:?\s*(.*)`)
)

func parseList(raw string) Snapshot {
	line := strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))
	for _, re := range []*regexp.Regexp{reMaxOf, reSlash} {
		if m := re.FindStringSubmatch(line); m != nil {
			online, _ := strconv.Atoi(m[1])
			maxN, _ := strconv.Atoi(m[2])
			names := []string{}
			if len(m) > 3 && strings.TrimSpace(m[3]) != "" {
				for _, n := range strings.Split(m[3], ",") {
					if s := strings.TrimSpace(n); s != "" {
						names = append(names, s)
					}
				}
			}
			return Snapshot{Online: online, Max: maxN, Players: names}
		}
	}
	return Snapshot{Players: []string{}}
}
