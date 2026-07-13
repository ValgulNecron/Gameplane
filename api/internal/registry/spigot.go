package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Spigot searches SpigotMC's plugin catalog through the third-party Spiget
// API (api.spiget.org/v2, keyless) — SpigotMC itself has no public REST
// API. Spiget mirrors SpigotMC's resource listing/search and proxies
// hosted-file downloads through its own CDN (cdn.spiget.org).
//
// Not every SpigotMC resource has a Spiget-hosted file: some are flagged
// "external" (the download lives on the author's own site/GitHub/etc, not
// on SpigotMC) and some are "premium" (paid, gated behind a SpigotMC
// purchase Spiget has no way to authorize on our behalf). Spiget's own docs
// say the resource's `external` field "should be checked before
// downloading, to not receive any unexpected data" — for both cases
// Versions() returns the version with zero Files, exactly like
// curseforge.go's files-with-no-DownloadURL rows and steam.go/nexus.go's
// Files-always-nil versions; the dashboard already renders that as "No
// compatible files." Do NOT invent a DownloadURL for these — Spiget's own
// "download" redirect for an external resource just 302s to the resource's
// GitHub/webpage, not a file.
//
// Confirmed against the live API (2026-07-12): GET /resources?fields=...
// and GET /search/resources/{query}?field=name for browse/search, GET
// /resources/{id} for the external/premium flags, GET
// /resources/{id}/versions for the version list (id/name/releaseDate only
// — no per-version file info), and GET
// /resources/{id}/versions/{version}/download, which 302s to
// cdn.spiget.org for a hosted file or to the external URL otherwise.
//
// Spiget has no loader/game-version facet (a resource's supported versions
// are free-text "testedVersions", not a queryable dimension) and no
// modpack concept — SpigotMC plugins are single jars.
type Spigot struct {
	client    *http.Client
	userAgent string
	baseURL   string // overridable in tests; default https://api.spiget.org/v2
}

func newSpigot(client *http.Client, userAgent string) *Spigot {
	return &Spigot{client: client, userAgent: userAgent, baseURL: "https://api.spiget.org/v2"}
}

// Search runs a Spiget resource search, or — for an empty term — browses
// all resources ordered by downloads.
func (s *Spigot) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	if q.modpack() {
		return nil, nil
	}
	limit := clampLimit(q.Limit)
	params := url.Values{}
	params.Set("size", strconv.Itoa(limit))
	if q.Offset > 0 {
		params.Set("page", strconv.Itoa(q.Offset/limit+1))
	}
	params.Set("fields", "id,name,tag,downloads,icon.url")

	var reqURL string
	term := strings.TrimSpace(q.Term)
	if term == "" {
		params.Set("sort", "-downloads")
		reqURL = s.baseURL + "/resources?" + params.Encode()
	} else {
		params.Set("field", "name")
		reqURL = s.baseURL + "/search/resources/" + url.PathEscape(term) + "?" + params.Encode()
	}

	var resp []spigetResource
	if err := httpGetJSON(ctx, s.client, s.userAgent, reqURL, &resp, defaultMaxRespBytes); err != nil {
		return nil, fmt.Errorf("spigot search: %w", err)
	}
	out := make([]Project, 0, len(resp))
	for _, r := range resp {
		out = append(out, r.project())
	}
	return out, nil
}

// Versions lists a resource's versions, newest first. Files is empty for
// every version of an external/premium resource — see the package doc
// comment above.
func (s *Spigot) Versions(ctx context.Context, projectID string, _ Filter) ([]Version, error) {
	res, err := s.getResource(ctx, projectID)
	if err != nil {
		return nil, err
	}
	noFile := res.External || res.Premium

	reqURL := s.baseURL + "/resources/" + url.PathEscape(projectID) + "/versions?size=25&sort=-releaseDate&fields=id,name,releaseDate"
	var raw []spigetVersion
	if err := httpGetJSON(ctx, s.client, s.userAgent, reqURL, &raw, defaultMaxRespBytes); err != nil {
		return nil, fmt.Errorf("spigot versions %s: %w", projectID, err)
	}

	out := make([]Version, 0, len(raw))
	for _, v := range raw {
		id := strconv.FormatInt(v.ID, 10)
		version := Version{
			ID:            id,
			Name:          v.Name,
			VersionNumber: v.Name,
		}
		if !noFile {
			version.Files = []File{{
				Filename: spigotFilename(res.Name, v.Name),
				DownloadURL: s.baseURL + "/resources/" + url.PathEscape(projectID) +
					"/versions/" + id + "/download",
				Primary: true,
			}}
		}
		out = append(out, version)
	}
	return out, nil
}

// ModpackDeps is a no-op: SpigotMC plugins have no modpack concept.
func (s *Spigot) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

// getResource fetches one resource's details, including the external/
// premium flags Versions() needs.
func (s *Spigot) getResource(ctx context.Context, id string) (spigetResource, error) {
	var r spigetResource
	reqURL := s.baseURL + "/resources/" + url.PathEscape(id) + "?fields=id,name,tag,downloads,external,premium,icon.url"
	if err := httpGetJSON(ctx, s.client, s.userAgent, reqURL, &r, defaultMaxRespBytes); err != nil {
		return spigetResource{}, fmt.Errorf("spigot resource %s: %w", id, err)
	}
	return r, nil
}

type spigetResource struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Tag       string `json:"tag"`
	Downloads int64  `json:"downloads"`
	External  bool   `json:"external"`
	Premium   bool   `json:"premium"`
	Icon      struct {
		URL string `json:"url"`
	} `json:"icon"`
}

// project converts a Spiget resource into a normalized Project. Icon and
// page URLs are relative on Spiget/SpigotMC and resolve against
// www.spigotmc.org (confirmed live); a bare numeric resource id also
// resolves the resource's page without needing its slug.
func (r spigetResource) project() Project {
	icon := ""
	if r.Icon.URL != "" {
		icon = "https://www.spigotmc.org/" + r.Icon.URL
	}
	id := strconv.FormatInt(r.ID, 10)
	return Project{
		ID:          id,
		Title:       r.Name,
		Description: r.Tag,
		IconURL:     icon,
		Downloads:   r.Downloads,
		PageURL:     "https://www.spigotmc.org/resources/" + id + "/",
		Provider:    "spigot",
	}
}

type spigetVersion struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// spigotFilename synthesizes a filename: Spiget's version list carries no
// filename (only an id + free-text version name), unlike Modrinth/
// CurseForge which return one directly.
func spigotFilename(resourceName, version string) string {
	base := sanitizeSpigotToken(resourceName)
	if base == "" {
		base = "plugin"
	}
	if v := sanitizeSpigotToken(version); v != "" {
		base += "-" + v
	}
	return base + ".jar"
}

// sanitizeSpigotToken strips characters outside [A-Za-z0-9._-] (spaces
// become '-') so free-text resource/version names are safe filename
// components.
func sanitizeSpigotToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}
