package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Nexus browses Nexus Mods (api.nexusmods.com) for one game's catalog,
// selected by the template's Community field, reused here as the API's
// per-game domain slug (e.g. "skyrimspecialedition", "stardewvalley") —
// the same bounded-slug shape Thunderstore uses for its per-community
// index (see nexusGame below).
//
// Nexus is browse-only, deliberately: Versions() never returns a
// downloadable File — in fact it never returns any version at all (an
// empty slice, not one Version with Files: nil), because the dashboard's
// empty state ("No compatible files.", no Install button) is keyed on zero
// *versions*, not zero files inside a version; a lone Files-less Version
// would instead surface a populated version dropdown next to a
// permanently-disabled button with no explanation. Nexus's real download
// links (.../files/{id}/download_link.json) are premium-account-gated and
// are minted for the caller's IP — that caller would be this API pod, but
// the agent pod in the game namespace is what actually performs the
// download, a different egress path the link is plausibly not valid for.
// The agent's download path also does no content-type validation, so
// handing it anything other than a genuine file URL (which is all a
// non-premium caller could ever get here — a mod PAGE url, not a file)
// would silently install an HTML document as if it were the mod file.
//
// This is why Nexus is NOT routed through File.RequiresAuth, the
// "hand the user a from-URL form" escape hatch factorio.go uses for its
// own auth-gated downloads: that hatch assumes a real, stable, public URL
// pattern the user can complete themselves by appending their own
// credentials (factorio.com's username+token query params). Nexus has no
// such self-service completion — there is no URL shape a non-premium user
// could paste in that would actually work — so offering the same hatch here
// would misleadingly imply a working install path that doesn't exist.
//
// The public API also has no full-text search endpoint (only per-game
// listings: trending/latest_added/updated, plus lookup-by-id), so Search
// browses the trending listing and filters it client-side by substring —
// the same "no real search, so filter the fetched listing" shape
// factorio.go and thunderstore.go use for their own catalogs.
type Nexus struct {
	client    *http.Client
	userAgent string
	apiKey    string
	baseURL   string // overridable in tests; default https://api.nexusmods.com
}

func newNexus(client *http.Client, userAgent, apiKey string) *Nexus {
	return &Nexus{client: client, userAgent: userAgent, apiKey: apiKey, baseURL: "https://api.nexusmods.com"}
}

// nexusGame binds the Nexus engine to one game's domain slug so it
// satisfies the Provider interface, mirroring thunderstoreCommunity's
// per-config wrapper.
type nexusGame struct {
	nexus  *Nexus
	domain string
}

func (g *nexusGame) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	return g.nexus.search(ctx, q, g.domain)
}

func (g *nexusGame) Versions(ctx context.Context, projectID string, _ Filter) ([]Version, error) {
	return g.nexus.versions(ctx, projectID, g.domain)
}

// ModpackDeps is a no-op: Nexus's own "Collections" feature is a separate,
// unintegrated system from the dependency-resolution modpacks concept
// here, so this engine does not surface a Modpacks tab.
func (g *nexusGame) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

// errNexusNotFound distinguishes a 404 (unknown mod id — a normal empty
// result) from other upstream failures.
var errNexusNotFound = errors.New("nexus: not found")

func (n *Nexus) search(ctx context.Context, q SearchQuery, domain string) ([]Project, error) {
	if q.modpack() {
		// No Collections integration — see the package doc comment.
		return []Project{}, nil
	}

	term := strings.TrimSpace(q.Term)
	if isDigits(term) {
		m, found, err := n.getMod(ctx, domain, term)
		if err != nil {
			return nil, err
		}
		if !found {
			return []Project{}, nil
		}
		return []Project{m.project(domain)}, nil
	}

	mods, err := n.trending(ctx, domain)
	if err != nil {
		return nil, err
	}

	lower := strings.ToLower(term)
	matched := make([]nexusMod, 0, len(mods))
	for _, m := range mods {
		if lower != "" && !strings.Contains(strings.ToLower(m.Name), lower) {
			continue
		}
		matched = append(matched, m)
	}

	if q.Offset >= len(matched) {
		matched = nil
	} else {
		matched = matched[q.Offset:]
	}
	if limit := clampLimit(q.Limit); len(matched) > limit {
		matched = matched[:limit]
	}

	out := make([]Project, 0, len(matched))
	for _, m := range matched {
		out = append(out, m.project(domain))
	}
	return out, nil
}

// versions resolves projectID to confirm it still exists, but always
// reports zero versions — see the Nexus doc comment above for why one-click
// install is never offered and why that means no versions, not one
// Files-less version.
func (n *Nexus) versions(ctx context.Context, projectID, domain string) ([]Version, error) {
	if _, _, err := n.getMod(ctx, domain, projectID); err != nil {
		return nil, err
	}
	return []Version{}, nil
}

// trending fetches the game's trending-mods listing — the closest thing
// the public API has to a "browse all" endpoint (there is no true catalog
// listing the way Factorio's mod portal has one).
func (n *Nexus) trending(ctx context.Context, domain string) ([]nexusMod, error) {
	var mods []nexusMod
	u := n.baseURL + "/v1/games/" + url.PathEscape(domain) + "/mods/trending.json"
	if err := n.get(ctx, u, &mods); err != nil {
		if errors.Is(err, errNexusNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("nexus trending %q: %w", domain, err)
	}
	return mods, nil
}

// getMod fetches one mod's details by id. found is false for an unknown
// (domain, id) pair (404) rather than an error — a bad id is a normal
// empty result.
func (n *Nexus) getMod(ctx context.Context, domain, modID string) (nexusMod, bool, error) {
	var m nexusMod
	u := n.baseURL + "/v1/games/" + url.PathEscape(domain) + "/mods/" + url.PathEscape(modID) + ".json"
	if err := n.get(ctx, u, &m); err != nil {
		if errors.Is(err, errNexusNotFound) {
			return nexusMod{}, false, nil
		}
		return nexusMod{}, false, fmt.Errorf("nexus mod %s/%s: %w", domain, modID, err)
	}
	return m, true, nil
}

// get performs an authenticated Nexus GET into v.
func (n *Nexus) get(ctx context.Context, rawURL string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("nexus request: %w", err)
	}
	req.Header.Set("User-Agent", n.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apikey", n.apiKey)
	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("nexus GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNexusNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nexus GET: upstream status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, defaultMaxRespBytes)).Decode(v); err != nil {
		return fmt.Errorf("nexus decode: %w", err)
	}
	return nil
}

// nexusMod is one mod as returned by both the trending listing and the
// mods/{id}.json details lookup (same object shape).
type nexusMod struct {
	ModID   int    `json:"mod_id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Picture string `json:"picture_url"`
	Author  string `json:"author"`
	Version string `json:"version"`
}

func (m nexusMod) project(domain string) Project {
	return Project{
		ID:          strconv.Itoa(m.ModID),
		Title:       m.Name,
		Description: m.Summary,
		Author:      m.Author,
		IconURL:     m.Picture,
		PageURL:     fmt.Sprintf("https://www.nexusmods.com/%s/mods/%d", domain, m.ModID),
		Provider:    "nexus",
	}
}
