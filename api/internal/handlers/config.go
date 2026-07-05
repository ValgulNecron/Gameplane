package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"

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
// section name. The set of valid sections is closed (see newValidators)
// so the API never round-trips arbitrary keys, which keeps the surface
// small enough to audit and bounds the value column's worst case.
//
// helmOIDCPresent reports whether the Helm-flag OIDC provider is
// configured — it counts as an always-enabled provider for validateAuth's
// lockout guard.
func MountConfig(r chi.Router, store *db.Store, helmOIDCPresent bool) {
	h := &configHandler{db: store, validators: newValidators(helmOIDCPresent)}
	r.Route("/admin/config", func(r chi.Router) {
		r.Get("/", h.getAll)
		r.Put("/{section}", h.put)
	})
}

type configHandler struct {
	db         *db.Store
	validators map[string]func([]byte) (json.RawMessage, error)
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
		if _, ok := h.validators[key]; !ok {
			continue
		}
		out[key] = json.RawMessage(value)
	}
	writeJSON(w, out)
}

func (h *configHandler) put(w http.ResponseWriter, req *http.Request) {
	section := chi.URLParam(req, "section")
	validate, ok := h.validators[section]
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

// newValidators owns both the closed allowlist of section names and the
// per-section schema. Each validator returns the canonicalized JSON that
// gets persisted — it isn't a passthrough of the request body, so
// unknown fields silently drop instead of accumulating in the database.
func newValidators(helmOIDCPresent bool) map[string]func([]byte) (json.RawMessage, error) {
	return map[string]func([]byte) (json.RawMessage, error){
		"general":       validateGeneral,
		"auth":          validateAuth(helmOIDCPresent),
		"notifications": validateNotifications,
		"telemetry":     validateTelemetry,
	}
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

// authRoleMappings mirrors auth.RoleMappings: per dashboard role, the IdP
// group values that grant it.
type authRoleMappings struct {
	Admin    []string `json:"admin,omitempty"`
	Operator []string `json:"operator,omitempty"`
	Viewer   []string `json:"viewer,omitempty"`
}

type authProvider struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"` // "local" | "oidc" | "google" | "github"
	DisplayName string `json:"displayName,omitempty"`
	Enabled     bool   `json:"enabled"`
	// Non-local kinds: the issuer + client id are public identifiers and
	// live here for UI visibility; only the clientSecret hides in the
	// ConfigRef Secret (default gameplane-auth-<name>).
	Issuer    string `json:"issuer,omitempty"`
	ClientID  string `json:"clientID,omitempty"`
	ConfigRef string `json:"configRef,omitempty"` // K8s Secret name
	// Group→role mapping (non-local kinds only; mirrors auth.Provider).
	Scopes       []string          `json:"scopes,omitempty"`
	GroupsClaim  string            `json:"groupsClaim,omitempty"`
	RoleMappings *authRoleMappings `json:"roleMappings,omitempty"`
	DefaultRole  string            `json:"defaultRole,omitempty"`
}

type authCfg struct {
	Providers []authProvider `json:"providers"`
}

var validAuthKinds = map[string]bool{"local": true, "oidc": true, "google": true, "github": true}

var validDefaultRoles = map[string]bool{"": true, "viewer": true, "operator": true, "admin": true, "deny": true}

// validateProviderMapping checks the group→role mapping fields of one
// provider entry, trimming scope tokens and the groups claim in place so
// the canonical blob stores clean values. Local providers carry none of
// these fields.
func validateProviderMapping(i int, p *authProvider) error {
	if p.Kind == "local" {
		if len(p.Scopes) > 0 || p.GroupsClaim != "" || p.RoleMappings != nil || p.DefaultRole != "" {
			return fmt.Errorf("providers[%d]: scopes, groupsClaim, roleMappings, and defaultRole are not valid for the local provider", i)
		}
		return nil
	}
	for j, s := range p.Scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("providers[%d].scopes[%d] must not be empty", i, j)
		}
		if strings.IndexFunc(s, unicode.IsSpace) >= 0 {
			return fmt.Errorf("providers[%d].scopes[%d] must be a single scope token without whitespace", i, j)
		}
		p.Scopes[j] = s
	}
	if p.GroupsClaim != "" {
		claim := strings.TrimSpace(p.GroupsClaim)
		if claim == "" {
			return fmt.Errorf("providers[%d].groupsClaim must not be blank", i)
		}
		p.GroupsClaim = claim
	}
	if !validDefaultRoles[p.DefaultRole] {
		return fmt.Errorf("providers[%d].defaultRole must be one of viewer|operator|admin|deny", i)
	}
	if p.DefaultRole != "" && p.RoleMappings == nil {
		return fmt.Errorf("providers[%d].defaultRole requires roleMappings", i)
	}
	if p.RoleMappings != nil {
		for role, groups := range map[string][]string{
			"admin":    p.RoleMappings.Admin,
			"operator": p.RoleMappings.Operator,
			"viewer":   p.RoleMappings.Viewer,
		} {
			for j, g := range groups {
				if strings.TrimSpace(g) == "" {
					return fmt.Errorf("providers[%d].roleMappings.%s[%d] must not be empty", i, role, j)
				}
			}
		}
	}
	return nil
}

// validateAuth returns the auth-section validator. helmOIDCPresent makes
// the Helm-flag provider count as always-enabled for the lockout guard —
// it can't be disabled from the dashboard.
func validateAuth(helmOIDCPresent bool) func([]byte) (json.RawMessage, error) {
	return func(body []byte) (json.RawMessage, error) {
		var c authCfg
		if err := json.Unmarshal(body, &c); err != nil {
			return nil, fmt.Errorf("invalid json: %w", err)
		}
		seen := map[string]bool{}
		anyEnabled := helmOIDCPresent
		locals := 0
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
			if p.Kind == "local" {
				locals++
			} else {
				// Non-local names become URL path segments
				// (/auth/oidc/{name}/…) and default Secret names — bound
				// them to DNS labels. "helm" is the synthetic Helm-flag
				// provider's reserved slug.
				if !dnsLabelRE.MatchString(p.Name) {
					return nil, fmt.Errorf("providers[%d].name must be a lowercase DNS label", i)
				}
				if p.Name == "helm" {
					return nil, fmt.Errorf(`providers[%d].name "helm" is reserved for the Helm-configured provider`, i)
				}
				u, err := url.Parse(p.Issuer)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
					return nil, fmt.Errorf("providers[%d].issuer must be an http(s) URL", i)
				}
				if p.ClientID == "" {
					return nil, fmt.Errorf("providers[%d].clientID is required", i)
				}
			}
			// Index into the slice (not the loop copy) so the trims
			// applied by the helper survive into the canonical blob.
			if err := validateProviderMapping(i, &c.Providers[i]); err != nil {
				return nil, err
			}
			if p.Enabled {
				anyEnabled = true
			}
		}
		if locals > 1 {
			return nil, fmt.Errorf("at most one local provider is allowed")
		}
		// Saving a config where nothing can authenticate would lock every
		// admin out at their next logout — refuse it here rather than
		// trust each client to guard the toggle.
		if !anyEnabled {
			return nil, fmt.Errorf("at least one identity provider must stay enabled")
		}
		return json.Marshal(c)
	}
}

// Backup destinations used to live in this admin-config blob; they're
// now first-class labelled Secrets served by handlers/destinations.go.
// The single source of truth is the cluster — no parallel registry.

type notifSink struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // "discord" | "slack" | "smtp" | "webhook" | "ntfy"
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

// The "updates" section used to persist a mutable release channel here,
// but nothing ever consumed it — Gameplane upgrades happen via Helm. The
// channel is now the chart's informational updates.channel value, served
// read-only on /cluster/info. getAll skips the legacy DB row because the
// key is no longer in sectionValidators.
