package registry

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Hangar searches PaperMC's Hangar (hangar.papermc.io) — the registry for
// Paper/Velocity/Waterfall plugins (keyless). Project ids are "owner~slug"
// (a path-safe separator so the id survives as a single URL path segment in
// the versions route); Hangar hosts plugins, not modpacks.
const hangarSep = "~"
type Hangar struct {
	client    *http.Client
	userAgent string
	baseURL   string // overridable in tests; default https://hangar.papermc.io/api/v1
}

func newHangar(client *http.Client, userAgent string) *Hangar {
	return &Hangar{client: client, userAgent: userAgent, baseURL: "https://hangar.papermc.io/api/v1"}
}

// hangarPlatforms is the preference order when a version ships for several
// platforms — Paper first since these are Minecraft servers.
var hangarPlatforms = []string{"PAPER", "WATERFALL", "VELOCITY"}

func hangarSort(sort string) string {
	switch sort {
	case "newest":
		return "-newest"
	case "updated":
		return "-updated"
	default:
		return "-downloads"
	}
}

// Search runs a Hangar project search. Modpacks aren't a Hangar concept, so
// the modpacks browser gets nothing here.
func (h *Hangar) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	if q.modpack() {
		return nil, nil
	}
	params := url.Values{}
	if q.Term != "" {
		params.Set("query", q.Term)
	}
	params.Set("limit", strconv.Itoa(clampLimit(q.Limit)))
	if q.Offset > 0 {
		params.Set("offset", strconv.Itoa(q.Offset))
	}
	params.Set("sort", hangarSort(q.Sort))
	if q.Category != "" {
		params.Set("category", q.Category)
	}

	var resp struct {
		Result []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			AvatarURL   string `json:"avatarUrl"`
			Namespace   struct {
				Owner string `json:"owner"`
				Slug  string `json:"slug"`
			} `json:"namespace"`
			Stats struct {
				Downloads int64 `json:"downloads"`
			} `json:"stats"`
		} `json:"result"`
	}
	if err := httpGetJSON(ctx, h.client, h.userAgent, h.baseURL+"/projects?"+params.Encode(), &resp, defaultMaxRespBytes); err != nil {
		return nil, err
	}

	out := make([]Project, 0, len(resp.Result))
	for _, p := range resp.Result {
		if p.Namespace.Owner == "" || p.Namespace.Slug == "" {
			continue
		}
		out = append(out, Project{
			ID:          p.Namespace.Owner + hangarSep + p.Namespace.Slug,
			Slug:        p.Namespace.Slug,
			Title:       p.Name,
			Description: p.Description,
			Author:      p.Namespace.Owner,
			IconURL:     p.AvatarURL,
			Downloads:   p.Stats.Downloads,
			PageURL:     "https://hangar.papermc.io/" + p.Namespace.Owner + "/" + p.Namespace.Slug,
			Provider:    "hangar",
		})
	}
	return out, nil
}

// Versions lists a plugin's versions, newest first (Hangar returns them so).
func (h *Hangar) Versions(ctx context.Context, projectID string, _ Filter) ([]Version, error) {
	owner, slug, ok := splitHangarID(projectID)
	if !ok {
		return nil, nil
	}
	u := h.baseURL + "/projects/" + url.PathEscape(owner) + "/" + url.PathEscape(slug) + "/versions?limit=25"

	var resp struct {
		Result []struct {
			Name      string `json:"name"`
			Downloads map[string]struct {
				FileInfo struct {
					Name      string `json:"name"`
					SizeBytes int64  `json:"sizeBytes"`
				} `json:"fileInfo"`
				ExternalURL string `json:"externalUrl"`
				DownloadURL string `json:"downloadUrl"`
			} `json:"downloads"`
			PlatformDependencies map[string][]string `json:"platformDependencies"`
		} `json:"result"`
	}
	if err := httpGetJSON(ctx, h.client, h.userAgent, u, &resp, defaultMaxRespBytes); err != nil {
		return nil, err
	}

	out := make([]Version, 0, len(resp.Result))
	for _, v := range resp.Result {
		plat, dl, ok := pickHangarDownload(v.Downloads)
		if !ok {
			continue
		}
		// Prefer the platform's direct/external URL; fall back to Hangar's
		// versioned download endpoint, which 302s to the CDN (both hosts are
		// allowlisted for install).
		downloadURL := dl.DownloadURL
		if downloadURL == "" {
			downloadURL = dl.ExternalURL
		}
		if downloadURL == "" {
			downloadURL = h.baseURL + "/projects/" + url.PathEscape(owner) + "/" + url.PathEscape(slug) +
				"/versions/" + url.PathEscape(v.Name) + "/" + plat + "/download"
		}
		filename := dl.FileInfo.Name
		if filename == "" {
			filename = slug + "-" + v.Name + ".jar"
		}
		out = append(out, Version{
			ID:            v.Name,
			Name:          v.Name,
			VersionNumber: v.Name,
			GameVersions:  v.PlatformDependencies[plat],
			Files: []File{{
				Filename:    filename,
				DownloadURL: downloadURL,
				Size:        dl.FileInfo.SizeBytes,
				Primary:     true,
			}},
		})
	}
	return out, nil
}

// pickHangarDownload chooses the preferred platform's download entry.
func pickHangarDownload[T any](downloads map[string]T) (string, T, bool) {
	for _, plat := range hangarPlatforms {
		if d, ok := downloads[plat]; ok {
			return plat, d, true
		}
	}
	for plat, d := range downloads {
		return plat, d, true
	}
	var zero T
	return "", zero, false
}

// ModpackDeps is a no-op: Hangar has no modpacks.
func (h *Hangar) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

// splitHangarID parses an "owner~slug" project id.
func splitHangarID(id string) (owner, slug string, ok bool) {
	o, s, found := strings.Cut(id, hangarSep)
	if !found || o == "" || s == "" {
		return "", "", false
	}
	return o, s, true
}
