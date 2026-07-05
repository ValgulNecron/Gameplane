package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/notify"
)

// dnsLabelRE matches an RFC1123 label: lowercase alphanumeric and dashes,
// no leading/trailing dash, 1-63 chars. Used for K8s namespace and Secret
// name fields where the value will eventually be passed to the API server.
var dnsLabelRE = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// MountConfig exposes the admin config store at /admin/config.
//
// Each AdminSettings section is persisted as a single JSON blob keyed by
// section name. The set of valid sections is closed (see sectionValidators)
// so the API never round-trips arbitrary keys, which keeps the surface
// small enough to audit and bounds the value column's worst case.
func MountConfig(r chi.Router, store *db.Store) {
	h := &configHandler{db: store}
	r.Route("/admin/config", func(r chi.Router) {
		r.Get("/", h.getAll)
		r.Put("/{section}", h.put)
	})
}

type configHandler struct {
	db *db.Store
}

func (h *configHandler) getAll(w http.ResponseWriter, req *http.Request) {
	rows, err := h.db.DB.QueryContext(req.Context(),
		`SELECT key, value FROM config`)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	defer rows.Close()

	out := map[string]json.RawMessage{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			httperr.Write(w, req, err)
			return
		}
		// Skip rows for sections we no longer recognize; the table
		// allows arbitrary keys at the schema level but the API
		// surface only ever exposes the validated set.
		if _, ok := sectionValidators[key]; !ok {
			continue
		}
		out[key] = json.RawMessage(value)
	}
	writeJSON(w, out)
}

func (h *configHandler) put(w http.ResponseWriter, req *http.Request) {
	section := chi.URLParam(req, "section")
	validate, ok := sectionValidators[section]
	if !ok {
		http.Error(w, "unknown section", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	canon, err := validate(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if _, err := h.db.DB.ExecContext(req.Context(),
		`INSERT INTO config(key, value, updated_at)
		 VALUES (?, ?, datetime('now'))
		 ON CONFLICT(key) DO UPDATE SET
		     value      = excluded.value,
		     updated_at = excluded.updated_at`,
		section, string(canon),
	); err != nil {
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, map[string]any{"section": section, "value": canon})
}

// sectionValidators owns both the closed allowlist of section names and
// the per-section schema. The validator returns the canonicalized JSON
// that gets persisted — it isn't a passthrough of the request body, so
// unknown fields silently drop instead of accumulating in the database.
var sectionValidators = map[string]func([]byte) (json.RawMessage, error){
	"general":       validateGeneral,
	"auth":          validateAuth,
	"notifications": validateNotifications,
	"telemetry":     validateTelemetry,
	"updates":       validateUpdates,
}

type generalCfg struct {
	InstanceName     string `json:"instanceName"`
	ExternalURL      string `json:"externalURL"`
	DefaultNamespace string `json:"defaultNamespace"`
}

func validateGeneral(body []byte) (json.RawMessage, error) {
	var c generalCfg
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if c.InstanceName == "" {
		return nil, fmt.Errorf("instanceName is required")
	}
	if c.DefaultNamespace == "" {
		return nil, fmt.Errorf("defaultNamespace is required")
	}
	if !dnsLabelRE.MatchString(c.DefaultNamespace) {
		return nil, fmt.Errorf("defaultNamespace must match RFC1123 label")
	}
	if c.ExternalURL != "" {
		u, err := url.Parse(c.ExternalURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("externalURL must be an http(s) URL")
		}
	}
	return json.Marshal(c)
}

type authProvider struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // "local" | "oidc" | "google" | "github"
	Enabled   bool   `json:"enabled"`
	ConfigRef string `json:"configRef,omitempty"` // K8s Secret name
}

type authCfg struct {
	Providers []authProvider `json:"providers"`
}

var validAuthKinds = map[string]bool{"local": true, "oidc": true, "google": true, "github": true}

func validateAuth(body []byte) (json.RawMessage, error) {
	var c authCfg
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	seen := map[string]bool{}
	anyEnabled := false
	for i, p := range c.Providers {
		if p.Name == "" {
			return nil, fmt.Errorf("providers[%d].name is required", i)
		}
		if seen[p.Name] {
			return nil, fmt.Errorf("providers[%d].name duplicate: %s", i, p.Name)
		}
		seen[p.Name] = true
		if !validAuthKinds[p.Kind] {
			return nil, fmt.Errorf("providers[%d].kind must be one of local|oidc|google|github", i)
		}
		if p.ConfigRef != "" && !dnsLabelRE.MatchString(p.ConfigRef) {
			return nil, fmt.Errorf("providers[%d].configRef must match RFC1123 label", i)
		}
		if p.Enabled {
			anyEnabled = true
		}
	}
	// Saving a config where nothing can authenticate would lock every
	// admin out at their next logout — refuse it here rather than trust
	// each client to guard the toggle.
	if !anyEnabled {
		return nil, fmt.Errorf("at least one identity provider must stay enabled")
	}
	return json.Marshal(c)
}

// Backup destinations used to live in this admin-config blob; they're
// now first-class labelled Secrets served by handlers/destinations.go.
// The single source of truth is the cluster — no parallel registry.

type notifSink struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`                // "discord" | "slack" | "smtp" | "webhook" | "ntfy"
	Enabled   bool     `json:"enabled"`
	ConfigRef string   `json:"configRef,omitempty"` // K8s Secret name holding the sink's credentials
	Events    []string `json:"events,omitempty"`    // subset of notify.AllEvents; empty = notify.DefaultOn
}

type notifCfg struct {
	Sinks []notifSink `json:"sinks"`
}

var validSinkKinds = map[string]bool{"discord": true, "slack": true, "smtp": true, "webhook": true, "ntfy": true}

func validateNotifications(body []byte) (json.RawMessage, error) {
	var c notifCfg
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	seen := map[string]bool{}
	for i, s := range c.Sinks {
		if s.Name == "" {
			return nil, fmt.Errorf("sinks[%d].name is required", i)
		}
		if seen[s.Name] {
			return nil, fmt.Errorf("sinks[%d].name duplicate: %s", i, s.Name)
		}
		seen[s.Name] = true
		if !validSinkKinds[s.Kind] {
			return nil, fmt.Errorf("sinks[%d].kind must be one of discord|slack|smtp|webhook|ntfy", i)
		}
		// configRef stays optional so sink rows persisted before the
		// delivery pipeline existed keep loading; the dispatcher skips
		// enabled sinks without one and the UI flags them.
		if s.ConfigRef != "" && !dnsLabelRE.MatchString(s.ConfigRef) {
			return nil, fmt.Errorf("sinks[%d].configRef must match RFC1123 label", i)
		}
		seenEv := map[string]bool{}
		for _, ev := range s.Events {
			if !notify.ValidEvent(ev) {
				return nil, fmt.Errorf("sinks[%d].events: unknown event %q", i, ev)
			}
			if seenEv[ev] {
				return nil, fmt.Errorf("sinks[%d].events: duplicate %q", i, ev)
			}
			seenEv[ev] = true
		}
	}
	return json.Marshal(c)
}

type telemetryCfg struct {
	SendMetrics bool `json:"sendMetrics"`
}

func validateTelemetry(body []byte) (json.RawMessage, error) {
	var c telemetryCfg
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return json.Marshal(c)
}

type updatesCfg struct {
	Channel string `json:"channel"`
}

var validChannels = map[string]bool{"stable": true, "beta": true, "nightly": true}

func validateUpdates(body []byte) (json.RawMessage, error) {
	var c updatesCfg
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if !validChannels[c.Channel] {
		return nil, fmt.Errorf("channel must be one of stable|beta|nightly")
	}
	return json.Marshal(c)
}
