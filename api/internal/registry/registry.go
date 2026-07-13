// Package registry implements read-only browse/search of external game-mod
// registries (Modrinth, Thunderstore, CurseForge, Hangar, the Factorio mod
// portal) for the dashboard's mod manager.
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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
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
// dashboard hands to the agent's install endpoint. RequiresAuth marks files
// the portal only serves with the user's own credentials appended (e.g. the
// Factorio mod portal's username+token query params) — the dashboard then
// offers a from-URL handoff instead of one-click install, and the server
// never embeds credentials in URLs it returns to browsers.
type File struct {
	Filename     string `json:"filename"`
	DownloadURL  string `json:"downloadUrl"`
	Size         int64  `json:"size,omitempty"`
	Primary      bool   `json:"primary,omitempty"`
	RequiresAuth bool   `json:"requiresAuth,omitempty"`
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

// KeyFunc returns the API key for a provider, or "" if unconfigured.
type KeyFunc func(ctx context.Context, provider string) string

// StaticKeys wraps a map of static API keys for use with NewSet. Useful for
// command-line flag initialization where the keys don't change at runtime.
func StaticKeys(keys map[string]string) KeyFunc {
	return func(ctx context.Context, provider string) string {
		return keys[provider]
	}
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

// Cache windows: a successful build is reused until the key hash changes or
// the entry ages out; a failed build is remembered briefly so a missing key
// can't hammer the engine constructor on every request.
const (
	rebuildTTL      = 10 * time.Minute
	errorBackoffTTL = 30 * time.Second
)

type builtEntry struct {
	provider *Curseforge
	err      error
	builtAt  time.Time
	keyHash  [sha256.Size]byte
}

// Set holds the constructed engines, sharing one HTTP client. Keyless engines
// (modrinth, thunderstore, hangar, factorio) are built once and reused; the
// Thunderstore engine caches its community package lists internally. The keyed
// engine (curseforge) is built lazily behind a TTL + key-hash cache so the
// key can be changed at runtime without restarting. When a key is empty, the
// provider reports unavailable (existing behavior).
type Set struct {
	// immutable keyless engines
	modrinth     *Modrinth
	thunderstore *Thunderstore
	hangar       *Hangar
	factorio     *Factorio

	// keyed engine cache
	mu      sync.Mutex
	built   builtEntry
	keyFunc KeyFunc
	client  *http.Client
	ua      string

	// test seam
	now func() time.Time
}

// NewSet builds the engine set. version tags the outbound User-Agent
// (Modrinth asks callers to identify themselves). keyFunc provides API keys
// for providers that need them (currently only CurseForge, which requires an
// x-api-key). Keys are resolved lazily per request and cached with a TTL.
func NewSet(version string, keyFunc KeyFunc) *Set {
	client := &http.Client{Timeout: 15 * time.Second}
	ua := "gameplane/" + version + " (+https://github.com/ValgulNecron/gameplane)"
	return &Set{
		modrinth:     newModrinth(client, ua),
		thunderstore: newThunderstore(client, ua),
		hangar:       newHangar(client, ua),
		factorio:     newFactorio(client, ua),
		keyFunc:      keyFunc,
		client:       client,
		ua:           ua,
		now:          time.Now,
	}
}

// For returns the provider for a module's registry config. ok is false
// when the provider is unknown/unset, a required parameter is missing
// (Thunderstore needs a community), or the engine isn't configured
// (CurseForge without a key) — the handler maps that to 501 so the
// dashboard hides the provider. For CurseForge, the key is resolved lazily
// via context and cached with a TTL.
func (s *Set) For(ctx context.Context, cfg Config) (Provider, bool) {
	switch cfg.Provider {
	case "modrinth":
		return s.modrinth, true
	case "thunderstore":
		if cfg.Community == "" {
			return nil, false
		}
		return &thunderstoreCommunity{ts: s.thunderstore, community: cfg.Community}, true
	case "curseforge":
		cf, err := s.curseforgeLazy(ctx)
		if err != nil || cf == nil {
			return nil, false
		}
		return cf, true
	case "hangar":
		return s.hangar, true
	case "factorio":
		return s.factorio, true
	default:
		return nil, false
	}
}

// Available reports whether a provider's engine is usable independent of a
// specific server — the providers listing uses it to mark the dashboard's
// provider switch. CurseForge availability depends on whether an API key is
// currently configured.
func (s *Set) Available(ctx context.Context, provider string) bool {
	switch provider {
	case "modrinth", "thunderstore", "hangar", "factorio":
		return true
	case "curseforge":
		cf, err := s.curseforgeLazy(ctx)
		return err == nil && cf != nil
	default:
		return false
	}
}

// curseforgeLazy resolves the CurseForge key and builds the engine lazily,
// caching the result against the key hash with a rebuild TTL. When the key
// is empty, returns (nil, nil). On build failure, caches the error briefly
// so repeated requests don't hammer the constructor.
func (s *Set) curseforgeLazy(ctx context.Context) (*Curseforge, error) {
	key := s.keyFunc(ctx, "curseforge")
	if key == "" {
		return nil, nil
	}

	keyHash := sha256.Sum256([]byte(key))

	s.mu.Lock()
	entry := s.built
	s.mu.Unlock()

	if entry.provider != nil || entry.err != nil {
		if entry.keyHash == keyHash {
			age := s.now().Sub(entry.builtAt)
			if entry.err == nil && age < rebuildTTL {
				return entry.provider, nil
			}
			if entry.err != nil && age < errorBackoffTTL {
				return nil, entry.err
			}
		}
	}

	// Key is new or cache expired; build the engine
	cf := newCurseforge(s.client, s.ua, key)

	s.mu.Lock()
	s.built = builtEntry{provider: cf, err: nil, builtAt: s.now(), keyHash: keyHash}
	s.mu.Unlock()

	return cf, nil
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
