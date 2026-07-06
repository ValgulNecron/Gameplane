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

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
	"github.com/ValgulNecron/gameplane/agent/internal/rcon"
)

type Rcon interface {
	Exec(cmd string) (string, error)
}

type handler struct {
	rcon Rcon
	cmdr commander
	game string

	listCmd string         // RCON command to fetch online players
	listRE  *regexp.Regexp // optional regex to parse entry output

	mu         sync.Mutex
	lastFetch  time.Time
	lastResult Snapshot
	lastErr    error
}

type Capabilities struct {
	Kick      bool `json:"kick"`
	Ban       bool `json:"ban"`
	Unban     bool `json:"unban"`
	Whitelist bool `json:"whitelist"`
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
// declared moderation commands (nil when the template declares none, in
// which case moderation is reported unsupported). game is retained only
// for log/error context.
func Mount(r chi.Router, rc Rcon, game string, actions *caps.PlayerActions) {
	h := &handler{rcon: rc, cmdr: pickCommander(actions), game: game}

	// Configure the list command and optional regex.
	h.listCmd = "list"
	if actions != nil && actions.List != nil {
		if actions.List.Command != "" {
			h.listCmd = actions.List.Command
		}
		if actions.List.EntryRegex != "" {
			re, err := regexp.Compile(actions.List.EntryRegex)
			switch {
			case err != nil:
				slog.Warn("invalid player list entryRegex; using built-in parser", "err", err)
			default:
				h.listRE = re
			}
		}
	}

	r.Get("/players", h.serve)
	r.Get("/players/banned", h.banned)
	r.Post("/players/kick", h.kick)
	r.Post("/players/ban", h.ban)
	r.Post("/players/unban", h.unban)
	r.Get("/players/whitelist", h.whitelistList)
	r.Post("/players/whitelist/add", h.whitelistAdd)
	r.Post("/players/whitelist/remove", h.whitelistRemove)
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
	raw, err := h.rcon.Exec(h.listCmd)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastFetch = time.Now()
	if err != nil {
		h.lastErr = err
		return Snapshot{}, err
	}
	var snap Snapshot
	if h.listRE != nil {
		snap = h.parseListWithRegex(raw)
	} else {
		snap = parseList(raw)
	}
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

func (h *handler) whitelistAdd(w http.ResponseWriter, req *http.Request) {
	// Whitelist add/remove take a name only; reason is ignored.
	h.runMod(w, req, func(name, _ string) (string, bool) {
		return h.cmdr.WhitelistAdd(name)
	}, false)
}

func (h *handler) whitelistRemove(w http.ResponseWriter, req *http.Request) {
	h.runMod(w, req, func(name, _ string) (string, bool) {
		return h.cmdr.WhitelistRemove(name)
	}, false)
}

func (h *handler) whitelistList(w http.ResponseWriter, _ *http.Request) {
	cmd, ok := h.cmdr.WhitelistList()
	if !ok {
		// Empty list rather than 501 so a Whitelist tab renders uniformly;
		// capabilities advertise whether management is actually available.
		writeJSON(w, http.StatusOK, []string{})
		return
	}
	raw, err := h.rcon.Exec(cmd)
	if errors.Is(err, rcon.ErrDisabled) {
		writeJSON(w, http.StatusOK, []string{})
		return
	}
	if err != nil {
		slog.Warn("whitelist rcon", "err", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	out := h.cmdr.ParseWhitelist(raw)
	if out == nil {
		out = []string{}
	}
	writeJSON(w, http.StatusOK, out)
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

// matchList runs the known "list" response formats against raw and returns
// the matching regexp submatches, or nil when none match.
func matchList(raw string) []string {
	line := strings.TrimSpace(strings.ReplaceAll(raw, "\r", ""))
	for _, re := range []*regexp.Regexp{reMaxOf, reSlash} {
		if m := re.FindStringSubmatch(line); m != nil {
			return m
		}
	}
	return nil
}

// ParseCounts extracts the online and max player counts from a raw RCON
// "list" response. ok is false when the response matches no known format, in
// which case the caller should treat both counts as unknown. It exists so the
// heartbeat can reuse this parser instead of duplicating the formats.
func ParseCounts(raw string) (online, max int, ok bool) {
	m := matchList(raw)
	if m == nil {
		return 0, 0, false
	}
	online, _ = strconv.Atoi(m[1])
	max, _ = strconv.Atoi(m[2])
	return online, max, true
}

func parseList(raw string) Snapshot {
	m := matchList(raw)
	if m == nil {
		return Snapshot{Players: []string{}}
	}
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

// parseListWithRegex uses a custom regex to extract player names from the
// output. It extracts the first capture group if present, otherwise the
// whole match. Each match is one player.
func (h *handler) parseListWithRegex(raw string) Snapshot {
	if h.listRE == nil {
		return Snapshot{Players: []string{}}
	}
	matches := h.listRE.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return Snapshot{Players: []string{}}
	}
	names := []string{}
	for _, m := range matches {
		var name string
		if len(m) > 1 {
			// Use first capture group if present.
			// A participating-but-empty capture group (empty string) is dropped.
			name = strings.TrimSpace(m[1])
		} else if len(m) > 0 {
			// Use whole match if no capture group.
			name = strings.TrimSpace(m[0])
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return Snapshot{Online: len(names), Max: -1, Players: names}
}

// CountWithRegex counts the number of matches in raw using the provided
// regex. It extracts the first capture group if present (when non-empty),
// otherwise counts the whole match. Used by heartbeat to derive player counts
// from custom list commands.
func CountWithRegex(raw string, re *regexp.Regexp) int {
	matches := re.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return 0
	}
	count := 0
	for _, m := range matches {
		var name string
		if len(m) > 1 {
			// Use first capture group if present.
			// A participating-but-empty capture group is dropped.
			name = strings.TrimSpace(m[1])
		} else if len(m) > 0 {
			// Use whole match if no capture group.
			name = strings.TrimSpace(m[0])
		}
		if name != "" {
			count++
		}
	}
	return count
}
