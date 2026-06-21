// Package caps mirrors the GameTemplate spec.capabilities schema
// (operator/api/v1alpha1) that the operator serializes into the
// KESTREL_CAPABILITIES env var. The agent interprets these declared
// console commands at runtime, so modules add moderation and quiesce
// support without agent code changes.
package caps

import (
	"encoding/json"
	"fmt"
)

// Spec is the root capabilities document.
type Spec struct {
	Players *PlayerActions `json:"players,omitempty"`
	Quiesce *Quiesce       `json:"quiesce,omitempty"`
	Actions []ServerAction `json:"actions,omitempty"`
	Status  *Status        `json:"status,omitempty"`
	Mods    *Mods          `json:"mods,omitempty"`
}

// ServerAction mirrors GameTemplate spec.capabilities.actions[]: a named
// operator action whose Command (a Go text/template rendered with
// .Params) is sent over RCON.
type ServerAction struct {
	ID          string        `json:"id"`
	DisplayName string        `json:"displayName,omitempty"`
	Description string        `json:"description,omitempty"`
	Icon        string        `json:"icon,omitempty"`
	Command     string        `json:"command"`
	Params      []ActionParam `json:"params,omitempty"`
	Confirm     bool          `json:"confirm,omitempty"`
	Danger      bool          `json:"danger,omitempty"`
}

// ActionParam is one declared input for a ServerAction.
type ActionParam struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
	// Type is one of "string", "int", "bool", "enum" (default "string").
	Type     string   `json:"type,omitempty"`
	Default  string   `json:"default,omitempty"`
	Enum     []string `json:"enum,omitempty"`
	Required bool     `json:"required,omitempty"`
}

// Status mirrors spec.capabilities.status: live metrics read over RCON.
type Status struct {
	Metrics []StatusMetric `json:"metrics,omitempty"`
}

// Mods mirrors spec.capabilities.mods: the mod/plugin directory and
// install policy the agent enforces.
type Mods struct {
	Path       string      `json:"path"`
	Extensions []string    `json:"extensions,omitempty"`
	Install    *ModInstall `json:"install,omitempty"`
	// Extract unpacks archive mods (e.g. Thunderstore .zip) into a per-mod
	// folder under Path, so loaders like BepInEx find the contained files.
	Extract bool `json:"extract,omitempty"`
}

// ModInstall configures URL-based mod installs with an SSRF host
// allowlist and a size cap.
type ModInstall struct {
	AllowedHosts []string `json:"allowedHosts"`
	MaxSizeMB    int32    `json:"maxSizeMB,omitempty"`
}

// StatusMetric reads one live readout from an RCON command's output via a
// named-group regex (group "value").
type StatusMetric struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	Command     string `json:"command"`
	Regex       string `json:"regex"`
	Unit        string `json:"unit,omitempty"`
}

// PlayerActions maps moderation actions to console command templates
// (Go text/template, rendered with .Player and .Reason). Empty actions
// are unsupported.
type PlayerActions struct {
	Kick    string   `json:"kick,omitempty"`
	Ban     string   `json:"ban,omitempty"`
	Unban   string   `json:"unban,omitempty"`
	BanList *BanList `json:"banList,omitempty"`
}

// BanList reads and parses the game's ban list.
type BanList struct {
	Command string `json:"command"`
	// EntryRegex matches one banned player per line via the named
	// groups "name" (required), "source" and "reason" (optional).
	EntryRegex string `json:"entryRegex"`
}

// Quiesce declares the command sequences run around a backup snapshot.
type Quiesce struct {
	Quiesce   []string `json:"quiesce"`
	Unquiesce []string `json:"unquiesce"`
	// FailurePattern, when it matches a quiesce command's output
	// (matched case-insensitively), fails the step even though the
	// command returned.
	FailurePattern string `json:"failurePattern,omitempty"`
}

// Parse decodes the KESTREL_CAPABILITIES env value. Empty input means
// no declared capabilities (nil, nil).
func Parse(raw string) (*Spec, error) {
	if raw == "" {
		return nil, nil
	}
	var s Spec
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("parse capabilities: %w", err)
	}
	return &s, nil
}
