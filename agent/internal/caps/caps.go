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
