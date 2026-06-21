package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Curseforge searches the CurseForge v1 API (api.curseforge.com), which
// requires an x-api-key. Suits Minecraft mods + modpacks. Loader filtering
// maps Kestrel loader ids to CurseForge's numeric modLoaderType; the clean
// game-version token is the gameVersion filter.
type Curseforge struct {
	client    *http.Client
	userAgent string
	apiKey    string
	baseURL   string // overridable in tests; default https://api.curseforge.com/v1
}

func newCurseforge(client *http.Client, userAgent, apiKey string) *Curseforge {
	return &Curseforge{client: client, userAgent: userAgent, apiKey: apiKey, baseURL: "https://api.curseforge.com/v1"}
}

// CurseForge numeric constants (stable, from the official API schema).
const (
	cfGameMinecraft = 432
	cfClassMods     = 6
	cfClassModpacks = 4471
)

// cfLoaderType maps a Kestrel loader id to CurseForge's modLoaderType enum.
// 0 (Any) means "no loader filter" — used for plugin loaders CurseForge
// doesn't model as mod loaders.
func cfLoaderType(loader string) int {
	switch loader {
	case "forge":
		return 1
	case "fabric":
		return 4
	case "quilt":
		return 5
	case "neoforge":
		return 6
	default:
		return 0
	}
}

// cfSortField maps a normalized Sort to CurseForge's sortField enum.
func cfSortField(sort string) int {
	switch sort {
	case "downloads":
		return 6 // TotalDownloads
	case "updated":
		return 3 // LastUpdated
	case "newest":
		return 11 // ReleasedDate
	default:
		return 2 // Popularity
	}
}

func (c *Curseforge) classID(q SearchQuery) int {
	if q.modpack() {
		return cfClassModpacks
	}
	return cfClassMods
}

// Search runs a CurseForge mod/modpack search scoped to Minecraft.
func (c *Curseforge) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	params := url.Values{}
	params.Set("gameId", strconv.Itoa(cfGameMinecraft))
	params.Set("classId", strconv.Itoa(c.classID(q)))
	if q.Term != "" {
		params.Set("searchFilter", q.Term)
	}
	if q.GameVersion != "" {
		params.Set("gameVersion", q.GameVersion)
	}
	if lt := cfLoaderType(q.Loader); lt != 0 {
		params.Set("modLoaderType", strconv.Itoa(lt))
	}
	params.Set("sortField", strconv.Itoa(cfSortField(q.Sort)))
	params.Set("sortOrder", "desc")
	params.Set("pageSize", strconv.Itoa(clampLimit(q.Limit)))
	if q.Offset > 0 {
		params.Set("index", strconv.Itoa(q.Offset))
	}

	var resp struct {
		Data []cfMod `json:"data"`
	}
	if err := c.get(ctx, c.baseURL+"/mods/search?"+params.Encode(), &resp); err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(resp.Data))
	for _, m := range resp.Data {
		out = append(out, m.project())
	}
	return out, nil
}

type cfMod struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Summary       string `json:"summary"`
	DownloadCount int64  `json:"downloadCount"`
	Authors       []struct {
		Name string `json:"name"`
	} `json:"authors"`
	Logo struct {
		URL string `json:"url"`
	} `json:"logo"`
	Links struct {
		WebsiteURL string `json:"websiteUrl"`
	} `json:"links"`
}

func (m cfMod) project() Project {
	author := ""
	if len(m.Authors) > 0 {
		author = m.Authors[0].Name
	}
	return Project{
		ID:          strconv.Itoa(m.ID),
		Slug:        m.Slug,
		Title:       m.Name,
		Description: m.Summary,
		Author:      author,
		IconURL:     m.Logo.URL,
		Downloads:   m.DownloadCount,
		PageURL:     m.Links.WebsiteURL,
		Provider:    "curseforge",
	}
}

// Versions lists a mod's files filtered to the active loader + game version,
// newest first (CurseForge returns files newest-first already). Files whose
// author disabled third-party distribution carry no downloadUrl and are
// skipped — the dashboard can't install them.
func (c *Curseforge) Versions(ctx context.Context, projectID string, f Filter) ([]Version, error) {
	params := url.Values{}
	if f.GameVersion != "" {
		params.Set("gameVersion", f.GameVersion)
	}
	if lt := cfLoaderType(f.Loader); lt != 0 {
		params.Set("modLoaderType", strconv.Itoa(lt))
	}
	params.Set("pageSize", "50")
	u := c.baseURL + "/mods/" + url.PathEscape(projectID) + "/files"
	if enc := params.Encode(); enc != "" {
		u += "?" + enc
	}

	var resp struct {
		Data []cfFile `json:"data"`
	}
	if err := c.get(ctx, u, &resp); err != nil {
		return nil, err
	}
	out := make([]Version, 0, len(resp.Data))
	for _, fl := range resp.Data {
		if fl.DownloadURL == "" {
			continue
		}
		out = append(out, Version{
			ID:            strconv.Itoa(fl.ID),
			Name:          fl.DisplayName,
			VersionNumber: fl.DisplayName,
			GameVersions:  fl.GameVersions,
			Files: []File{{
				Filename:    fl.FileName,
				DownloadURL: fl.DownloadURL,
				Size:        fl.FileLength,
				Primary:     true,
			}},
		})
	}
	return out, nil
}

type cfFile struct {
	ID           int      `json:"id"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	DownloadURL  string   `json:"downloadUrl"`
	FileLength   int64    `json:"fileLength"`
	GameVersions []string `json:"gameVersions"`
}

// ModpackDeps is a no-op: the Minecraft modules install CurseForge content
// as single mods, not via dependency resolution.
func (c *Curseforge) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

// get performs an authenticated CurseForge GET into v.
func (c *Curseforge) get(ctx context.Context, rawURL string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("curseforge request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("curseforge GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("curseforge GET: upstream status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, defaultMaxRespBytes)).Decode(v); err != nil {
		return fmt.Errorf("curseforge decode: %w", err)
	}
	return nil
}
