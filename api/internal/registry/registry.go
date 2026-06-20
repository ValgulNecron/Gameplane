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
// An empty Term is a valid "browse" request — providers return their
// popular/all listing, paginated by Offset+Limit and ordered by Sort.
type SearchQuery struct {
	Term        string
	Loader      string
	GameVersion string
	// ProjectType is "modpack" for the modpacks browser; empty (or "mod")
	// is the regular mod/plugin browser, which excludes modpacks.
	ProjectType string
	// Category narrows the browse to a provider category (e.g. Modrinth
	// "optimization"). Empty means all categories.
	Category string
	// Sort is the provider ordering key for browse: "downloads" (popular),
	// "updated", "newest", or "relevance" (the default when Term is set).
	Sort   string
	Limit  int
	Offset int
}

// modpack reports whether this query targets modpacks rather than mods.
func (q SearchQuery) modpack() bool { return q.ProjectType == "modpack" }

// Filter narrows a Versions lookup to the active loader + game version.
type Filter struct {
	Loader      string
	GameVersion string
}

// Provider is one registry engine bound to its configuration.
type Provider interface {
	Search(ctx context.Context, q SearchQuery) ([]Project, error)
	Versions(ctx context.Context, projectID string, f Filter) ([]Version, error)
	// ModpackDeps resolves a modpack's dependency mods into installable
	// files — used by games that install a modpack by installing its
	// dependencies (e.g. Thunderstore/BepInEx). Providers whose modpacks
	// install via a game-image env (e.g. Modrinth on itzg) return nil.
	ModpackDeps(ctx context.Context, projectID string) ([]File, error)
}

// Config is the per-game registry selection, mirroring the CRD's
// capabilities.mods.registry block.
type Config struct {
	Provider  string
	Community string
}

// Set holds the constructed engines, sharing one HTTP client. Build once
// at startup and reuse; the Thunderstore engine caches its community
// package lists internally. curseforge is nil when no API key is
// configured (the provider is then reported unavailable).
type Set struct {
	modrinth     *Modrinth
	thunderstore *Thunderstore
	curseforge   *Curseforge
	hangar       *Hangar
}

// NewSet builds the engine set. version tags the outbound User-Agent
// (Modrinth asks callers to identify themselves). curseforgeKey enables the
// CurseForge engine (its API requires an x-api-key); empty disables it.
func NewSet(version, curseforgeKey string) *Set {
	client := &http.Client{Timeout: 15 * time.Second}
	ua := "kestrel/" + version + " (+https://kestrel.gg)"
	s := &Set{
		modrinth:     newModrinth(client, ua),
		thunderstore: newThunderstore(client, ua),
		hangar:       newHangar(client, ua),
	}
	if curseforgeKey != "" {
		s.curseforge = newCurseforge(client, ua, curseforgeKey)
	}
	return s
}

// For returns the provider for a module's registry config. ok is false
// when the provider is unknown/unset, a required parameter is missing
// (Thunderstore needs a community), or the engine isn't configured
// (CurseForge without a key) — the handler maps that to 501 so the
// dashboard hides the provider.
func (s *Set) For(cfg Config) (Provider, bool) {
	switch cfg.Provider {
	case "modrinth":
		return s.modrinth, true
	case "thunderstore":
		if cfg.Community == "" {
			return nil, false
		}
		return &thunderstoreCommunity{ts: s.thunderstore, community: cfg.Community}, true
	case "curseforge":
		if s.curseforge == nil {
			return nil, false
		}
		return s.curseforge, true
	case "hangar":
		return s.hangar, true
	default:
		return nil, false
	}
}

// Available reports whether a provider's engine is usable independent of a
// specific server — the providers listing uses it to mark the dashboard's
// provider switch. CurseForge needs a configured API key.
func (s *Set) Available(provider string) bool {
	switch provider {
	case "modrinth", "thunderstore", "hangar":
		return true
	case "curseforge":
		return s.curseforge != nil
	default:
		return false
	}
}

// Response-body caps. Modrinth search/version responses are small;
// Thunderstore's per-community package list is the whole catalog (~150 MB
// for Valheim), so it gets a far larger ceiling. The Thunderstore engine
// stream-decodes within this bound, so the cap limits bytes read, not
// resident memory.
const (
	defaultMaxRespBytes = 8 << 20   // 8 MiB
	tsMaxRespBytes      = 320 << 20 // 320 MiB
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
