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
	"strconv"
	"strings"
	"text/template"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
	"github.com/ValgulNecron/gameplane/agent/internal/rcon"
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
	tmpl *template.Template
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
		t, err := template.New(s.ID).Option("missingkey=error").Parse(s.Command)
		if err != nil {
			slog.Warn("invalid action command template; action disabled",
				"action", s.ID, "err", err)
			continue
		}
		out[s.ID] = &compiled{spec: s, tmpl: t}
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

	params, err := resolveParams(act.spec.Params, body.Params)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	var sb strings.Builder
	if err := act.tmpl.Execute(&sb, struct{ Params map[string]string }{params}); err != nil {
		slog.Warn("render action command", "action", act.spec.ID, "err", err)
		writeErr(w, http.StatusBadRequest, "could not render action command")
		return
	}
	cmd := strings.TrimSpace(sb.String())
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

// resolveParams validates each declared parameter against the request,
// applying defaults and rejecting console-injection. The returned map
// holds every declared parameter (so missingkey=error only fires on a
// command template that references an undeclared name). Undeclared keys
// in the request are ignored.
func resolveParams(decls []caps.ActionParam, got map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(decls))
	for _, p := range decls {
		val, ok := got[p.Name]
		if !ok || val == "" {
			val = p.Default
		}
		if p.Required && strings.TrimSpace(val) == "" {
			return nil, fmt.Errorf("parameter %q is required", p.Name)
		}
		if val == "" {
			out[p.Name] = ""
			continue
		}
		clean, err := validateParam(p, val)
		if err != nil {
			return nil, err
		}
		out[p.Name] = clean
	}
	return out, nil
}

func validateParam(p caps.ActionParam, val string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(p.Type)) {
	case "int":
		if _, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err != nil {
			return "", fmt.Errorf("parameter %q must be an integer", p.Name)
		}
		return strings.TrimSpace(val), nil
	case "bool":
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "true", "false":
			return strings.ToLower(strings.TrimSpace(val)), nil
		}
		return "", fmt.Errorf("parameter %q must be true or false", p.Name)
	case "enum":
		for _, e := range p.Enum {
			if val == e {
				return val, nil
			}
		}
		return "", fmt.Errorf("parameter %q must be one of the declared options", p.Name)
	default: // string
		if hasControl(val) {
			return "", fmt.Errorf("parameter %q must not contain control characters", p.Name)
		}
		if len(val) > 512 {
			return "", fmt.Errorf("parameter %q is too long (max 512)", p.Name)
		}
		return val, nil
	}
}

// hasControl reports whether s contains an ASCII control character.
// Rejecting these (notably CR/LF) stops a parameter value from chaining a
// second RCON command into the rendered line.
func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
