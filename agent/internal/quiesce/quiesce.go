// Package quiesce exposes /quiesce and /unquiesce endpoints used by the
// Backup controller to flush in-flight game state to disk before a
// snapshot is taken (and resume it after).
//
// The implementation is per-game: Minecraft toggles auto-save with
// "save-off" + "save-all flush" / "save-on" over RCON. Games that have
// no equivalent (Valheim, Factorio, raw Source servers) get a
// best-effort no-op so the backup pipeline can proceed unconditionally
// — backups should never fail because a game can't be paused.
package quiesce

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Rcon is the slice of *rcon.Client we actually use. Defined here as
// an interface so tests can swap in a recording fake.
type Rcon interface {
	Exec(cmd string) (string, error)
}

// Quiescer encapsulates the game-specific RCON sequence for pausing
// writes (Quiesce) and resuming them (Unquiesce). Implementations
// return Supported()=false when the game cannot quiesce; the HTTP
// handler turns that into a 200 with quiesced=false rather than a
// hard error.
type Quiescer interface {
	Supported() bool
	Quiesce(rc Rcon) error
	Unquiesce(rc Rcon) error
}

// Pick returns a Quiescer for the given game identifier. Unknown
// games fall through to the unsupported quiescer.
func Pick(game string) Quiescer {
	switch strings.ToLower(strings.TrimSpace(game)) {
	case "minecraft", "minecraft-java":
		return minecraftQuiescer{}
	default:
		return unsupportedQuiescer{}
	}
}

type response struct {
	Quiesced bool   `json:"quiesced"`
	Reason   string `json:"reason,omitempty"`
}

// Mount registers POST /quiesce and POST /unquiesce on r.
func Mount(r chi.Router, rc Rcon, game string) {
	q := Pick(game)
	r.Post("/quiesce", func(w http.ResponseWriter, _ *http.Request) {
		if !q.Supported() {
			writeJSON(w, http.StatusOK, response{Quiesced: false, Reason: "game does not support quiesce"})
			return
		}
		if err := q.Quiesce(rc); err != nil {
			slog.Warn("quiesce failed", "game", game, "err", err)
			writeJSON(w, http.StatusBadGateway, response{Quiesced: false, Reason: "rcon error"})
			return
		}
		writeJSON(w, http.StatusOK, response{Quiesced: true})
	})
	r.Post("/unquiesce", func(w http.ResponseWriter, _ *http.Request) {
		if !q.Supported() {
			writeJSON(w, http.StatusOK, response{Quiesced: false, Reason: "game does not support quiesce"})
			return
		}
		if err := q.Unquiesce(rc); err != nil {
			// Unquiesce failure is more dangerous than quiesce failure
			// (the game is now stuck with auto-save off), but we still
			// must not block the controller — surface 502 so the
			// operator can record an event.
			slog.Warn("unquiesce failed", "game", game, "err", err)
			writeJSON(w, http.StatusBadGateway, response{Quiesced: true, Reason: "unquiesce rcon error"})
			return
		}
		writeJSON(w, http.StatusOK, response{Quiesced: false})
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// --- Minecraft (vanilla / paper / spigot / forge / fabric) ---------------

type minecraftQuiescer struct{}

func (minecraftQuiescer) Supported() bool { return true }

func (minecraftQuiescer) Quiesce(rc Rcon) error {
	if _, err := rc.Exec("save-off"); err != nil {
		return err
	}
	out, err := rc.Exec("save-all flush")
	if err != nil {
		// Try to flip auto-save back on so we don't leave the world
		// frozen if the second step explodes.
		_, _ = rc.Exec("save-on")
		return err
	}
	if strings.Contains(strings.ToLower(out), "saving failed") {
		_, _ = rc.Exec("save-on")
		return errors.New("save-all flush reported failure")
	}
	return nil
}

func (minecraftQuiescer) Unquiesce(rc Rcon) error {
	_, err := rc.Exec("save-on")
	return err
}

// --- Default ----------------------------------------------------------

type unsupportedQuiescer struct{}

func (unsupportedQuiescer) Supported() bool        { return false }
func (unsupportedQuiescer) Quiesce(Rcon) error     { return nil }
func (unsupportedQuiescer) Unquiesce(Rcon) error   { return nil }
