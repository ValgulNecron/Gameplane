// Package registry implements read-only browse/search of external game-mod
// registries (Modrinth, Thunderstore) for the dashboard's mod manager.
//
// The engines here are generic: a GameTemplate's capabilities.mods.registry
// block picks a provider and supplies its parameters, and the handler in
// internal/handlers resolves the active version's loader + game-version
// token before calling Search/Versions. Installing a chosen mod is NOT done
// here — the dashboard hands the returned File.DownloadURL to the existing
// agent install path, which re-checks the host allowlist and SSRF guard.
// Search itself only ever GETs fixed, admin-trusted hostnames
// (api.modrinth.com, thunderstore.io), so it carries no per-request SSRF
// guard of its own.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Project is one search hit, normalized across providers.
type Project struct {
	ID          string `json:"id"`
	Slug        string `json:"slug,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	IconURL     string `json:"iconUrl,omitempty"`
	Downloads   int64  `json:"downloads,omitempty"`
	PageURL     string `json:"pageUrl,omitempty"`
	Provider    string `json:"provider"`
}

// File is a downloadable artifact of a Version. DownloadURL is what the
// dashboard hands to the agent's install endpoint.
type File struct {
	Filename    string `json:"filename"`
	DownloadURL string `json:"downloadUrl"`
	Size        int64  `json:"size,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

// Version is one release of a Project, newest first.
type Version struct {
	ID            string   `json:"id"`
	Name          string   `json:"name,omitempty"`
	VersionNumber string   `json:"versionNumber,omitempty"`
	GameVersions  []string `json:"gameVersions,omitempty"`
	Loaders       []string `json:"loaders,omitempty"`
	Files         []File   `json:"files"`
}

// SearchQuery is a normalized search request. Loader and GameVersion are
// the active version's loader id (used verbatim as the provider facet) and
// clean game-version token; either may be empty (no facet → all results).
type SearchQuery struct {
	Term        string
	Loader      string
	GameVersion string
	Limit       int
}

// Filter narrows a Versions lookup to the active loader + game version.
type Filter struct {
	Loader      string
	GameVersion string
}

// Provider is one registry engine bound to its configuration.
type Provider interface {
	Search(ctx context.Context, q SearchQuery) ([]Project, error)
	Versions(ctx context.Context, projectID string, f Filter) ([]Version, error)
}

// Config is the per-game registry selection, mirroring the CRD's
// capabilities.mods.registry block.
type Config struct {
	Provider  string
	Community string
}

// Set holds the constructed engines, sharing one HTTP client. Build once
// at startup and reuse; the Thunderstore engine caches its community
// package lists internally.
type Set struct {
	modrinth     *Modrinth
	thunderstore *Thunderstore
}

// NewSet builds the engine set. version tags the outbound User-Agent
// (Modrinth asks callers to identify themselves).
func NewSet(version string) *Set {
	client := &http.Client{Timeout: 15 * time.Second}
	ua := "kestrel/" + version + " (+https://kestrel.gg)"
	return &Set{
		modrinth:     newModrinth(client, ua),
		thunderstore: newThunderstore(client, ua),
	}
}

// For returns the provider for a module's registry config. ok is false
// when the provider is unknown/unset, or a required parameter is missing
// (Thunderstore needs a community) — the handler maps that to 501 so the
// dashboard hides the Browse tab.
func (s *Set) For(cfg Config) (Provider, bool) {
	switch cfg.Provider {
	case "modrinth":
		return s.modrinth, true
	case "thunderstore":
		if cfg.Community == "" {
			return nil, false
		}
		return &thunderstoreCommunity{ts: s.thunderstore, community: cfg.Community}, true
	default:
		return nil, false
	}
}

// Response-body caps. Modrinth search/version responses are small;
// Thunderstore's per-community package list is the whole catalog (several
// MiB), so it gets a far larger ceiling.
const (
	defaultMaxRespBytes = 8 << 20  // 8 MiB
	tsMaxRespBytes      = 64 << 20 // 64 MiB
)

// httpGetJSON GETs rawURL and decodes a JSON body into v, capping the body
// at maxBytes. Hosts are fixed/trusted, so there is no SSRF check here.
func httpGetJSON(ctx context.Context, client *http.Client, userAgent, rawURL string, v any, maxBytes int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("registry request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("registry GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry GET: upstream status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBytes)).Decode(v); err != nil {
		return fmt.Errorf("registry decode: %w", err)
	}
	return nil
}
