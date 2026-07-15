// Package actions runs module-declared operator actions over RCON.
//
// A GameTemplate declares actions in spec.capabilities.actions[]; the
// operator serializes them into GAMEPLANE_CAPABILITIES and the agent
// interprets them here. Each action carries a Go text/template Command
// rendered with the user-supplied parameters and sent to the game over
// RCON, so modules add new buttons to the dashboard without any agent
// code change.
package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
	"github.com/ValgulNecron/gameplane/agent/internal/rcon"
	"github.com/ValgulNecron/gameplane/gameaction"
)

// Rcon is the slice of *rcon.Client we use. An interface so tests can
// swap in a recording fake.
type Rcon interface {
	Exec(cmd string) (string, error)
}

type handler struct {
	rc      Rcon
	game    string
	actions map[string]*compiled
}

// compiled is one action whose command template parsed successfully at
// mount time. A malformed template drops just that action (logged) so
// one bad declaration can't take down the whole action surface.
type compiled struct {
	spec caps.ServerAction
	cmd  *gameaction.Command
}

// Mount registers POST /actions/run. specs is the module's declared
// actions (nil/empty when the template declares none).
func Mount(r chi.Router, rc Rcon, game string, specs []caps.ServerAction) {
	h := &handler{rc: rc, game: game, actions: compile(specs)}
	r.Post("/actions/run", h.run)
}

func compile(specs []caps.ServerAction) map[string]*compiled {
	out := make(map[string]*compiled, len(specs))
	for _, s := range specs {
		if s.ID == "" || s.Command == "" {
			continue
		}
		cmd, err := gameaction.Compile(s.ID, s.Command)
		if err != nil {
			slog.Warn("invalid action command template; action disabled",
				"action", s.ID, "err", err)
			continue
		}
		out[s.ID] = &compiled{spec: s, cmd: cmd}
	}
	return out
}

// toGameactionParams adapts the agent's caps.ActionParam declarations to
// gameaction.Param, the shared shape Resolve validates against.
func toGameactionParams(ps []caps.ActionParam) []gameaction.Param {
	out := make([]gameaction.Param, len(ps))
	for i, p := range ps {
		out[i] = gameaction.Param{
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Description: p.Description,
			Type:        p.Type,
			Default:     p.Default,
			Enum:        p.Enum,
			Required:    p.Required,
		}
	}
	return out
}

type runReq struct {
	ID     string            `json:"id"`
	Params map[string]string `json:"params,omitempty"`
}

type runResp struct {
	OK  bool   `json:"ok"`
	Raw string `json:"raw,omitempty"`
}

func (h *handler) run(w http.ResponseWriter, req *http.Request) {
	var body runReq
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 16<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	act, ok := h.actions[body.ID]
	if !ok {
		writeErr(w, http.StatusNotFound, "unknown action")
		return
	}

	params, err := gameaction.Resolve(toGameactionParams(act.spec.Params), body.Params)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	cmd, err := act.cmd.Render(params)
	if err != nil {
		slog.Warn("render action command", "action", act.spec.ID, "err", err)
		writeErr(w, http.StatusBadRequest, "could not render action command")
		return
	}
	if cmd == "" {
		writeErr(w, http.StatusUnprocessableEntity, "action produced an empty command")
		return
	}

	raw, err := h.rc.Exec(cmd)
	if errors.Is(err, rcon.ErrDisabled) {
		writeErr(w, http.StatusNotImplemented, fmt.Sprintf("not supported by %s (no RCON)", h.game))
		return
	}
	if err != nil {
		// RCON errors can echo addresses/passwords from buggy server
		// mods — never reflect them to the client.
		slog.Warn("action rcon", "action", act.spec.ID, "err", err)
		writeErr(w, http.StatusBadGateway, "upstream unavailable")
		return
	}
	writeJSON(w, http.StatusOK, runResp{OK: true, Raw: strings.TrimSpace(raw)})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
