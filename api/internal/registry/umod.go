package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Umod searches uMod (formerly Oxide) — the plugin ecosystem for
// Rust/Hurtworld/7 Days to Die and other games running the Oxide/uMod mod
// loader (umod.org, keyless).
//
// uMod's *documented* API (umod.org/documentation/api/*) returned 403 to
// unauthenticated fetches during development, and the "modern" static
// manifest it points to (assets.umod.org/plugins.json) turned out to be a
// 3-field pointer object, not a catalog — it is not a usable replacement.
// This engine instead uses the JSON endpoints umod.org's own website calls,
// confirmed live by hand-probing production umod.org on 2026-07-12/13:
//   - search/browse: GET /plugins/search.json?query=&page=&sort=&sortdir=&categories[]=
//     A uMod community-forum post from a staff member describes this exact
//     shape as "not officially an API, so no guarantees on usability" — it
//     is the best available option and was confirmed returning real,
//     current plugin data (verified against umod.org/plugins/vanish.json,
//     whose latest_release_at matched "today" at verification time).
//   - plugin detail: GET /plugins/{slug}.json — confirmed live.
//   - version history: GET /plugins/{slug}/versions.json — NOT referenced
//     in any uMod documentation found (nor in the forum thread above);
//     discovered by probing for a plausible URL and confirmed live. It
//     also returns admin-only action URLs (revert_url/toggle_url/edit_url)
//     alongside the public fields, which confirms it is the site's own
//     internal API rather than a contract meant for third parties — it
//     could change or disappear without notice. If it ever does,
//     Versions() falls back to the single latest release from the
//     plugin-detail call rather than failing the whole Mods tab.
//
// Plugin files are raw ".cs" C# source (Oxide/uMod compiles them inside
// the game process at runtime) — that is normal for this ecosystem, not a
// broken download. Downloads are served directly from umod.org itself with
// no redirect (confirmed live), so no other host needs allowlisting.
//
// uMod has no loader/game-version dimension in Gameplane's sense (a plugin
// targets a *game*, e.g. rust/hurtworld, not a Minecraft-style loader), so
// Filter is ignored; SearchQuery.Category is repurposed as uMod's game
// slug (confirmed live: categories[]=rust narrows the listing to Rust
// plugins), the same "reuse a generic facet for a provider-specific
// dimension" trick Nexus's Community field uses for its own game domain.
// uMod has no modpack concept.
type Umod struct {
	client    *http.Client
	userAgent string
	baseURL   string // overridable in tests; default https://umod.org
}

func newUmod(client *http.Client, userAgent string) *Umod {
	return &Umod{client: client, userAgent: userAgent, baseURL: "https://umod.org"}
}

// umodPageSize is the fixed page size umod.org/plugins/search.json returns
// — confirmed live; there is no page-size query parameter to request more
// or fewer per call. Offset is mapped onto this fixed unit rather than the
// caller's Limit, so a caller requesting a different page size may see a
// misaligned window; that's an acceptable tradeoff for a keyless "load
// more" browse against an undocumented endpoint.
const umodPageSize = 10

func umodSort(sort string) string {
	switch sort {
	case "newest":
		return "created_at"
	case "updated":
		return "latest_release_at"
	default:
		return "downloads"
	}
}

// Search runs uMod's site search, or browses the full catalog (empty term)
// ordered by downloads.
func (u *Umod) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	if q.modpack() {
		return nil, nil
	}
	params := url.Values{}
	params.Set("query", q.Term)
	params.Set("page", strconv.Itoa(q.Offset/umodPageSize+1))
	params.Set("sort", umodSort(q.Sort))
	params.Set("sortdir", "desc")
	if q.Category != "" {
		params.Set("categories[]", q.Category)
	}

	var resp umodSearchResponse
	reqURL := u.baseURL + "/plugins/search.json?" + params.Encode()
	if err := httpGetJSON(ctx, u.client, u.userAgent, reqURL, &resp, defaultMaxRespBytes); err != nil {
		return nil, fmt.Errorf("umod search: %w", err)
	}
	out := make([]Project, 0, len(resp.Data))
	for _, p := range resp.Data {
		out = append(out, p.project())
	}
	return out, nil
}

// Versions lists a plugin's release history, newest first, from the
// undocumented versions.json endpoint. If that call fails or returns no
// data, it falls back to reporting just the current latest release from
// the plugin-detail endpoint (see the package doc comment above) — a
// genuinely unknown slug still surfaces as an error via that fallback
// call's own 404 handling.
func (u *Umod) Versions(ctx context.Context, projectID string, _ Filter) ([]Version, error) {
	var resp umodVersionsResponse
	reqURL := u.baseURL + "/plugins/" + url.PathEscape(projectID) + "/versions.json"
	if err := httpGetJSON(ctx, u.client, u.userAgent, reqURL, &resp, defaultMaxRespBytes); err == nil && len(resp.Data) > 0 {
		out := make([]Version, 0, len(resp.Data))
		for _, v := range resp.Data {
			out = append(out, v.version())
		}
		return out, nil
	}

	p, err := u.getPlugin(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return []Version{p.latestVersion()}, nil
}

// ModpackDeps is a no-op: uMod has no modpack concept.
func (u *Umod) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

func (u *Umod) getPlugin(ctx context.Context, slug string) (umodPlugin, error) {
	var p umodPlugin
	reqURL := u.baseURL + "/plugins/" + url.PathEscape(slug) + ".json"
	if err := httpGetJSON(ctx, u.client, u.userAgent, reqURL, &p, defaultMaxRespBytes); err != nil {
		return umodPlugin{}, fmt.Errorf("umod plugin %s: %w", slug, err)
	}
	return p, nil
}

type umodSearchResponse struct {
	Data []umodPlugin `json:"data"`
}

type umodPlugin struct {
	Slug                          string `json:"slug"`
	Title                         string `json:"title"`
	Description                   string `json:"description"`
	Author                        string `json:"author"`
	IconURL                       string `json:"icon_url"`
	Downloads                     int64  `json:"downloads"`
	URL                           string `json:"url"`
	DownloadURL                   string `json:"download_url"`
	LatestReleaseVersion          string `json:"latest_release_version"`
	LatestReleaseVersionFormatted string `json:"latest_release_version_formatted"`
}

func (p umodPlugin) project() Project {
	return Project{
		ID:          p.Slug,
		Slug:        p.Slug,
		Title:       p.Title,
		Description: p.Description,
		Author:      p.Author,
		IconURL:     p.IconURL,
		Downloads:   p.Downloads,
		PageURL:     p.URL,
		Provider:    "umod",
	}
}

// latestVersion reports the plugin's single current release as a synthetic
// Version — used as Versions()'s fallback when the (undocumented, may
// disappear) version-history endpoint is unavailable.
func (p umodPlugin) latestVersion() Version {
	var files []File
	if p.DownloadURL != "" {
		files = []File{{
			Filename:    umodFilenameFromURL(p.DownloadURL, p.LatestReleaseVersion),
			DownloadURL: p.DownloadURL,
			Primary:     true,
		}}
	}
	name := p.LatestReleaseVersionFormatted
	if name == "" {
		name = p.LatestReleaseVersion
	}
	return Version{
		ID:            p.LatestReleaseVersion,
		Name:          name,
		VersionNumber: p.LatestReleaseVersion,
		Files:         files,
	}
}

type umodVersionsResponse struct {
	Data []umodVersion `json:"data"`
}

type umodVersion struct {
	Version          string `json:"version"`
	VersionFormatted string `json:"version_formatted"`
	DownloadURL      string `json:"download_url"`
}

func (v umodVersion) version() Version {
	var files []File
	if v.DownloadURL != "" {
		files = []File{{
			Filename:    umodFilenameFromURL(v.DownloadURL, v.Version),
			DownloadURL: v.DownloadURL,
			Primary:     true,
		}}
	}
	name := v.VersionFormatted
	if name == "" {
		name = v.Version
	}
	return Version{
		ID:            v.Version,
		Name:          name,
		VersionNumber: v.Version,
		Files:         files,
	}
}

// umodFilenameFromURL derives a version-qualified filename from a
// download_url like ".../plugins/Vanish.cs?version=2.1.3" →
// "Vanish-2.1.3.cs", since uMod's API gives no separate filename field.
func umodFilenameFromURL(rawURL, version string) string {
	base := "plugin.cs"
	if parsed, err := url.Parse(rawURL); err == nil {
		if i := strings.LastIndexByte(parsed.Path, '/'); i >= 0 && i+1 < len(parsed.Path) {
			base = parsed.Path[i+1:]
		}
	}
	if version == "" {
		return base
	}
	return strings.TrimSuffix(base, ".cs") + "-" + version + ".cs"
}
