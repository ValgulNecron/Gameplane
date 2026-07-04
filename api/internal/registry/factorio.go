package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Factorio browses the official Factorio mod portal (mods.factorio.com,
// keyless for browse). Like Thunderstore, the only stable listing endpoint
// returns the whole catalog (`/api/mods?page_size=max`, small: one
// latest_release per mod), so it is fetched once and cached with a TTL;
// per-mod releases come from `/api/mods/{name}/full`.
//
// Downloads are the one asymmetry: the portal requires the player's own
// factorio.com credentials as query parameters, which the server must never
// embed in URLs it returns to browsers. Files are therefore flagged
// RequiresAuth — the dashboard hands the user to the from-URL install form
// instead of one-click installing, and the user appends their own
// username/token.
//
// The portal has no loader dimension and no modpack concept; the
// game-version facet is the mod's info_json.factorio_version major token
// ("1.1", "2.0"), matched against the template's gameVersion.
type Factorio struct {
	client    *http.Client
	userAgent string
	baseURL   string // overridable in tests; default https://mods.factorio.com
	ttl       time.Duration

	mu      sync.Mutex
	catalog []factMod
	fetched time.Time
}

type factMod struct {
	Name           string
	Title          string
	Owner          string
	Summary        string
	Downloads      int64
	Thumbnail      string
	FactorioMajors []string // majors of the latest release ("1.1", "2.0")
}

// factorioMaxRespBytes caps the catalog response. The full listing (one
// latest_release per mod) is a few tens of MB at most — far below
// Thunderstore's per-community catalogs.
const factorioMaxRespBytes = 64 << 20 // 64 MiB

func newFactorio(client *http.Client, userAgent string) *Factorio {
	return &Factorio{
		client:    client,
		userAgent: userAgent,
		baseURL:   "https://mods.factorio.com",
		ttl:       10 * time.Minute,
	}
}

// mods returns the cached catalog, refreshing it when stale. Concurrent
// misses may both fetch; acceptable for a rare, idempotent GET.
func (f *Factorio) mods(ctx context.Context) ([]factMod, error) {
	f.mu.Lock()
	fresh := f.catalog != nil && time.Since(f.fetched) < f.ttl
	catalog := f.catalog
	f.mu.Unlock()
	if fresh {
		return catalog, nil
	}

	var resp struct {
		Results []struct {
			Name           string `json:"name"`
			Title          string `json:"title"`
			Owner          string `json:"owner"`
			Summary        string `json:"summary"`
			DownloadsCount int64  `json:"downloads_count"`
			Thumbnail      string `json:"thumbnail"`
			LatestRelease  *struct {
				InfoJSON struct {
					FactorioVersion string `json:"factorio_version"`
				} `json:"info_json"`
			} `json:"latest_release"`
		} `json:"results"`
	}
	if err := httpGetJSON(ctx, f.client, f.userAgent,
		f.baseURL+"/api/mods?page_size=max", &resp, factorioMaxRespBytes); err != nil {
		return nil, fmt.Errorf("factorio catalog: %w", err)
	}

	out := make([]factMod, 0, len(resp.Results))
	for _, r := range resp.Results {
		m := factMod{
			Name:      r.Name,
			Title:     r.Title,
			Owner:     r.Owner,
			Summary:   truncate(r.Summary, tsMaxDescLen),
			Downloads: r.DownloadsCount,
			Thumbnail: r.Thumbnail,
		}
		if r.LatestRelease != nil && r.LatestRelease.InfoJSON.FactorioVersion != "" {
			m.FactorioMajors = []string{r.LatestRelease.InfoJSON.FactorioVersion}
		}
		out = append(out, m)
	}

	f.mu.Lock()
	f.catalog = out
	f.fetched = time.Now()
	f.mu.Unlock()
	return out, nil
}

func (f *Factorio) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	// The portal has no modpack concept — the modpacks browser is empty.
	if q.modpack() {
		return []Project{}, nil
	}
	catalog, err := f.mods(ctx)
	if err != nil {
		return nil, err
	}

	term := strings.ToLower(strings.TrimSpace(q.Term))
	matched := make([]factMod, 0)
	for _, m := range catalog {
		if q.GameVersion != "" && !hasMajor(m.FactorioMajors, q.GameVersion) {
			continue
		}
		if term != "" &&
			!strings.Contains(strings.ToLower(m.Title), term) &&
			!strings.Contains(strings.ToLower(m.Name), term) &&
			!strings.Contains(strings.ToLower(m.Owner), term) {
			continue
		}
		matched = append(matched, m)
	}
	// Downloads is the only meaningful popularity signal the listing carries.
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].Downloads > matched[j].Downloads })

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
		out = append(out, Project{
			ID:          m.Name,
			Slug:        m.Name,
			Title:       m.Title,
			Description: m.Summary,
			Author:      m.Owner,
			IconURL:     factorioThumbURL(m.Thumbnail),
			Downloads:   m.Downloads,
			PageURL:     "https://mods.factorio.com/mod/" + url.PathEscape(m.Name),
			Provider:    "factorio",
		})
	}
	return out, nil
}

func (f *Factorio) Versions(ctx context.Context, projectID string, filter Filter) ([]Version, error) {
	var resp struct {
		Releases []struct {
			Version     string `json:"version"`
			DownloadURL string `json:"download_url"`
			FileName    string `json:"file_name"`
			ReleasedAt  string `json:"released_at"`
			InfoJSON    struct {
				FactorioVersion string `json:"factorio_version"`
			} `json:"info_json"`
		} `json:"releases"`
	}
	u := f.baseURL + "/api/mods/" + url.PathEscape(projectID) + "/full"
	if err := httpGetJSON(ctx, f.client, f.userAgent, u, &resp, defaultMaxRespBytes); err != nil {
		return nil, fmt.Errorf("factorio mod %q: %w", projectID, err)
	}

	releases := resp.Releases
	// The portal lists releases oldest-first; the picker wants newest-first.
	sort.SliceStable(releases, func(i, j int) bool { return releases[i].ReleasedAt > releases[j].ReleasedAt })

	out := make([]Version, 0, len(releases))
	for _, r := range releases {
		if filter.GameVersion != "" && r.InfoJSON.FactorioVersion != filter.GameVersion {
			continue
		}
		out = append(out, Version{
			ID:            r.Version,
			Name:          r.FileName,
			VersionNumber: r.Version,
			GameVersions:  []string{r.InfoJSON.FactorioVersion},
			Files: []File{{
				Filename:    r.FileName,
				DownloadURL: f.baseURL + r.DownloadURL,
				Primary:     true,
				// Portal downloads need the player's own factorio.com
				// username+token query params — never server-held creds.
				RequiresAuth: true,
			}},
		})
	}
	return out, nil
}

// ModpackDeps is a no-op: the Factorio portal has no modpack concept.
func (f *Factorio) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

// factorioThumbURL resolves the portal's relative thumbnail path. The
// portal's placeholder thumb is dropped so the dashboard renders its own.
func factorioThumbURL(thumb string) string {
	if thumb == "" || strings.HasSuffix(thumb, "/.thumb.png") {
		return ""
	}
	return "https://assets-mod.factorio.com" + thumb
}

// hasMajor reports whether majors contains the wanted game-version token.
func hasMajor(majors []string, want string) bool {
	for _, m := range majors {
		if m == want {
			return true
		}
	}
	return false
}
