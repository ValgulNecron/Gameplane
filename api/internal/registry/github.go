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

// GitHub browses one repository's Releases (api.github.com, keyless but
// rate-limited to 60 requests/hour per source IP for unauthenticated
// callers — Gameplane has no admin-configurable GitHub token today, so
// every module using this provider shares that budget from wherever the
// API pod's egress IP is).
//
// GitHub has no cross-repo mod search, so unlike every other engine in this
// package a template must bind this provider to exactly ONE repository via
// ModProvider.GitHub{Owner,Repo} — see githubRepo below, which mirrors
// thunderstoreCommunity's per-config wrapper.
//
// Confirmed against the live API (2026-07-12): GET
// /repos/{owner}/{repo}/releases (list, tag_name/name/published_at/assets
// fields, per_page+page pagination) and each asset's browser_download_url,
// e.g. https://github.com/{owner}/{repo}/releases/download/{tag}/{file},
// which 302s to a release-assets.githubusercontent.com URL — confirmed
// live too. That is why BOTH github.com and .githubusercontent.com must be
// in a module's capabilities.mods.install.allowedHosts: allowlisting only
// github.com lets the request start but the agent's redirect re-check then
// blocks the githubusercontent.com hop and the download silently fails
// partway through.
type GitHub struct {
	client    *http.Client
	userAgent string
	baseURL   string // overridable in tests; default https://api.github.com
}

func newGitHub(client *http.Client, userAgent string) *GitHub {
	return &GitHub{client: client, userAgent: userAgent, baseURL: "https://api.github.com"}
}

// githubRepo binds the GitHub engine to one owner/repo so it satisfies the
// Provider interface, mirroring thunderstoreCommunity's per-config wrapper.
type githubRepo struct {
	gh    *GitHub
	owner string
	repo  string
}

// Search lists the repo's releases and filters them by a case-insensitive
// substring match against the release's title/tag. This is NOT a real
// ranked search — the GitHub Releases API has no query parameter, and
// there is no cross-repo index to search in the first place — it is a
// client-side filter over this one repo's release list, which is enough
// for the common case of a handful to a few dozen releases.
func (r *githubRepo) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	if q.modpack() {
		return nil, nil
	}
	releases, err := r.gh.listReleases(ctx, r.owner, r.repo, q.Limit, q.Offset)
	if err != nil {
		return nil, err
	}
	term := strings.ToLower(strings.TrimSpace(q.Term))
	out := make([]Project, 0, len(releases))
	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		if term != "" &&
			!strings.Contains(strings.ToLower(rel.displayName()), term) &&
			!strings.Contains(strings.ToLower(rel.TagName), term) {
			continue
		}
		out = append(out, rel.project(r.owner, r.repo))
	}
	return out, nil
}

// Versions resolves the single release identified by projectID (a release
// id, as returned by Search) and reports it as one Version whose Files are
// that release's assets — "versions" here means "this release's
// downloadable files", since GitHub has no separate project/version
// hierarchy above a release the way Modrinth or CurseForge do.
func (r *githubRepo) Versions(ctx context.Context, projectID string, _ Filter) ([]Version, error) {
	rel, err := r.gh.getRelease(ctx, r.owner, r.repo, projectID)
	if err != nil {
		return nil, err
	}
	return []Version{rel.version()}, nil
}

// ModpackDeps is a no-op: GitHub Releases has no dependency-resolution
// concept.
func (r *githubRepo) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

// errGitHubRateLimited signals GitHub's REST API rejected the request due
// to rate limiting (60 req/hr/IP for unauthenticated callers, or a
// secondary/abuse limit). It is returned as a clean wrapped error, never
// retried — Gameplane has no key to attach for a higher quota today.
var errGitHubRateLimited = errors.New("github: API rate limit exceeded (60 requests/hour for unauthenticated callers)")

// errGitHubNotFound distinguishes a 404 (unknown repo/release — a normal
// empty result for Search, a real error for Versions since a specific id
// was requested) from other upstream failures.
var errGitHubNotFound = errors.New("github: not found")

func (g *GitHub) listReleases(ctx context.Context, owner, repo string, limit, offset int) ([]githubRelease, error) {
	l := clampLimit(limit)
	page := offset/l + 1
	reqURL := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d",
		g.baseURL, url.PathEscape(owner), url.PathEscape(repo), l, page)
	var releases []githubRelease
	if err := g.get(ctx, reqURL, &releases); err != nil {
		if errors.Is(err, errGitHubNotFound) {
			return nil, fmt.Errorf("github: repo %s/%s not found", owner, repo)
		}
		return nil, fmt.Errorf("github list releases %s/%s: %w", owner, repo, err)
	}
	return releases, nil
}

func (g *GitHub) getRelease(ctx context.Context, owner, repo, releaseID string) (githubRelease, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s/releases/%s",
		g.baseURL, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(releaseID))
	var rel githubRelease
	if err := g.get(ctx, reqURL, &rel); err != nil {
		return githubRelease{}, fmt.Errorf("github release %s/%s#%s: %w", owner, repo, releaseID, err)
	}
	return rel, nil
}

// get performs a GitHub REST GET into v, distinguishing rate-limit and
// not-found responses from other upstream failures.
func (g *GitHub) get(ctx context.Context, rawURL string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("github request: %w", err)
	}
	req.Header.Set("User-Agent", g.userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("github GET: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return errGitHubRateLimited
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return errGitHubRateLimited
	case resp.StatusCode == http.StatusNotFound:
		return errGitHubNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("github GET: upstream status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, defaultMaxRespBytes)).Decode(v); err != nil {
		return fmt.Errorf("github decode: %w", err)
	}
	return nil
}

type githubRelease struct {
	ID      int64  `json:"id"`
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Draft   bool   `json:"draft"`
	Author  struct {
		Login string `json:"login"`
	} `json:"author"`
	Assets []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// displayName prefers the release's title, falling back to its tag when
// the author left the title blank (a common shortcut for simple repos).
func (rel githubRelease) displayName() string {
	if rel.Name != "" {
		return rel.Name
	}
	return rel.TagName
}

func (rel githubRelease) project(owner, repo string) Project {
	return Project{
		ID:    strconv.FormatInt(rel.ID, 10),
		Title: rel.displayName(),
		// Description intentionally omitted: a release's "body" is a full
		// markdown changelog, not a short blurb, and dumping it into the
		// search-results card would be unreadable.
		Author:   rel.Author.Login,
		PageURL:  fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, url.PathEscape(rel.TagName)),
		Provider: "github",
	}
}

// version maps the release's assets to Files. Assets with no
// browser_download_url (shouldn't happen for a published release, but
// guarded for consistency with curseforge.go's own file-skip pattern) are
// skipped rather than surfaced as a broken download.
func (rel githubRelease) version() Version {
	files := make([]File, 0, len(rel.Assets))
	for _, a := range rel.Assets {
		if a.BrowserDownloadURL == "" {
			continue
		}
		files = append(files, File{
			Filename:    a.Name,
			DownloadURL: a.BrowserDownloadURL,
			Size:        a.Size,
			Primary:     len(files) == 0,
		})
	}
	return Version{
		ID:            strconv.FormatInt(rel.ID, 10),
		Name:          rel.displayName(),
		VersionNumber: rel.TagName,
		Files:         files,
	}
}
