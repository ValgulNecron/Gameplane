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

// Thunderstore searches a community's package index on thunderstore.io
// (keyless). It suits BepInEx games like Valheim. The v1 endpoint returns
// the community's entire catalog in one (multi-MiB) response, so results
// are filtered client-side and the catalog is cached per community with a
// short TTL. Thunderstore has no loader or game-version dimension, so the
// SearchQuery/Filter facets are ignored.
type Thunderstore struct {
	client    *http.Client
	userAgent string
	baseURL   string // overridable in tests; default https://thunderstore.io
	ttl       time.Duration

	mu    sync.Mutex
	cache map[string]tsCacheEntry
}

type tsCacheEntry struct {
	pkgs    []tsPackage
	fetched time.Time
}

type tsPackage struct {
	Name         string      `json:"name"`
	FullName     string      `json:"full_name"`
	Owner        string      `json:"owner"`
	PackageURL   string      `json:"package_url"`
	RatingScore  int64       `json:"rating_score"`
	IsDeprecated bool        `json:"is_deprecated"`
	Versions     []tsVersion `json:"versions"`
}

type tsVersion struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	VersionNumber string `json:"version_number"`
	Description   string `json:"description"`
	Icon          string `json:"icon"`
	DownloadURL   string `json:"download_url"`
	Downloads     int64  `json:"downloads"`
	FileSize      int64  `json:"file_size"`
}

func newThunderstore(client *http.Client, userAgent string) *Thunderstore {
	return &Thunderstore{
		client:    client,
		userAgent: userAgent,
		baseURL:   "https://thunderstore.io",
		ttl:       10 * time.Minute,
		cache:     map[string]tsCacheEntry{},
	}
}

// packages returns the community's package list, served from the per-
// community cache when fresh. Concurrent misses may both fetch; that's
// acceptable for a rare, idempotent GET.
func (t *Thunderstore) packages(ctx context.Context, community string) ([]tsPackage, error) {
	t.mu.Lock()
	e, ok := t.cache[community]
	fresh := ok && time.Since(e.fetched) < t.ttl
	t.mu.Unlock()
	if fresh {
		return e.pkgs, nil
	}

	u := t.baseURL + "/c/" + url.PathEscape(community) + "/api/v1/package/"
	var raw []tsPackage
	if err := httpGetJSON(ctx, t.client, t.userAgent, u, &raw, tsMaxRespBytes); err != nil {
		return nil, err
	}

	t.mu.Lock()
	t.cache[community] = tsCacheEntry{pkgs: raw, fetched: time.Now()}
	t.mu.Unlock()
	return raw, nil
}

// thunderstoreCommunity binds the Thunderstore engine to one community so
// it satisfies the Provider interface.
type thunderstoreCommunity struct {
	ts        *Thunderstore
	community string
}

func (c *thunderstoreCommunity) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	pkgs, err := c.ts.packages(ctx, c.community)
	if err != nil {
		return nil, err
	}
	term := strings.ToLower(strings.TrimSpace(q.Term))

	matched := make([]tsPackage, 0, clampLimit(q.Limit))
	for _, p := range pkgs {
		if p.IsDeprecated {
			continue
		}
		if term != "" &&
			!strings.Contains(strings.ToLower(p.Name), term) &&
			!strings.Contains(strings.ToLower(p.Owner), term) {
			continue
		}
		matched = append(matched, p)
	}
	// Rank by community rating; ties keep catalog order (stable).
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].RatingScore > matched[j].RatingScore })
	if limit := clampLimit(q.Limit); len(matched) > limit {
		matched = matched[:limit]
	}

	out := make([]Project, 0, len(matched))
	for _, p := range matched {
		var downloads int64
		var icon, desc string
		if len(p.Versions) > 0 {
			icon = p.Versions[0].Icon
			desc = p.Versions[0].Description
			for _, v := range p.Versions {
				downloads += v.Downloads
			}
		}
		out = append(out, Project{
			ID:          p.FullName,
			Slug:        p.FullName,
			Title:       p.Name,
			Description: desc,
			Author:      p.Owner,
			IconURL:     icon,
			Downloads:   downloads,
			PageURL:     p.PackageURL,
			Provider:    "thunderstore",
		})
	}
	return out, nil
}

func (c *thunderstoreCommunity) Versions(ctx context.Context, projectID string, _ Filter) ([]Version, error) {
	pkgs, err := c.ts.packages(ctx, c.community)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		if p.FullName != projectID {
			continue
		}
		// Thunderstore lists versions newest-first already.
		out := make([]Version, 0, len(p.Versions))
		for _, v := range p.Versions {
			out = append(out, Version{
				ID:            v.FullName,
				Name:          v.Name,
				VersionNumber: v.VersionNumber,
				Files: []File{{
					Filename:    v.FullName + ".zip",
					DownloadURL: v.DownloadURL,
					Size:        v.FileSize,
					Primary:     true,
				}},
			})
		}
		return out, nil
	}
	return nil, fmt.Errorf("thunderstore: package %q not found in community %q", projectID, c.community)
}
