package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Thunderstore searches a community's package index on thunderstore.io
// (keyless). It suits BepInEx games like Valheim. The only programmatic
// endpoint is the v1 package list, which returns the community's ENTIRE
// catalog in one response — for Valheim that's ~150 MB. To stay well under
// the API pod's memory limit we never hold the raw catalog: the body is
// stream-decoded one package at a time into a compact index (a few small
// fields per package, versions capped), and only that index is cached.
// (Thunderstore's lighter paginated frontend search API sits behind
// Cloudflare bot protection and isn't usable from a server.)
//
// Thunderstore has no loader or game-version dimension, so the
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

// tsPackage is the compact, cached form of one package — only what search
// and the version picker need. Heavy raw fields (dependencies, per-version
// descriptions, etc.) are dropped during streaming.
type tsPackage struct {
	Name      string
	FullName  string
	Owner     string
	PageURL   string
	Icon      string
	Desc      string
	Rating    int64
	Downloads int64
	Versions  []tsVersion
}

type tsVersion struct {
	FullName      string
	VersionNumber string
	DownloadURL   string
	FileSize      int64
}

// tsRawPackage mirrors only the v1 fields we read. encoding/json drops
// every untagged field (notably the large per-version "dependencies"
// arrays), so decoding one element already discards most of the payload.
type tsRawPackage struct {
	Name         string `json:"name"`
	FullName     string `json:"full_name"`
	Owner        string `json:"owner"`
	PackageURL   string `json:"package_url"`
	RatingScore  int64  `json:"rating_score"`
	IsDeprecated bool   `json:"is_deprecated"`
	Versions     []struct {
		FullName      string `json:"full_name"`
		VersionNumber string `json:"version_number"`
		Description   string `json:"description"`
		Icon          string `json:"icon"`
		DownloadURL   string `json:"download_url"`
		Downloads     int64  `json:"downloads"`
		FileSize      int64  `json:"file_size"`
	} `json:"versions"`
}

const (
	// tsMaxVersionsPerPackage caps how many versions (newest first) we keep
	// per package — the install picker only needs recent ones, and this
	// bounds the cached index for packages with long histories.
	tsMaxVersionsPerPackage = 20
	// tsMaxDescLen truncates the package description kept for the cards.
	tsMaxDescLen = 200
)

func newThunderstore(client *http.Client, userAgent string) *Thunderstore {
	return &Thunderstore{
		client:    client,
		userAgent: userAgent,
		baseURL:   "https://thunderstore.io",
		ttl:       10 * time.Minute,
		cache:     map[string]tsCacheEntry{},
	}
}

// packages returns the community's compact package index, served from the
// per-community cache when fresh. Concurrent misses may both fetch; that's
// acceptable for a rare, idempotent GET.
func (t *Thunderstore) packages(ctx context.Context, community string) ([]tsPackage, error) {
	t.mu.Lock()
	e, ok := t.cache[community]
	fresh := ok && time.Since(e.fetched) < t.ttl
	t.mu.Unlock()
	if fresh {
		return e.pkgs, nil
	}

	pkgs, err := t.fetchIndex(ctx, community)
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	t.cache[community] = tsCacheEntry{pkgs: pkgs, fetched: time.Now()}
	t.mu.Unlock()
	return pkgs, nil
}

// fetchIndex streams the v1 package list and builds the compact index
// without ever materializing the whole catalog in memory.
func (t *Thunderstore) fetchIndex(ctx context.Context, community string) ([]tsPackage, error) {
	u := t.baseURL + "/c/" + url.PathEscape(community) + "/api/v1/package/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("thunderstore request: %w", err)
	}
	req.Header.Set("User-Agent", t.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("thunderstore GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("thunderstore GET: upstream status %d", resp.StatusCode)
	}

	// Stream-decode the top-level array one element at a time. tsMaxRespBytes
	// caps total bytes read (a DoS bound); because we read incrementally and
	// keep only the compact form, peak memory stays a small multiple of one
	// package, not the ~150 MB catalog.
	dec := json.NewDecoder(io.LimitReader(resp.Body, tsMaxRespBytes))
	if _, err := dec.Token(); err != nil { // opening '['
		return nil, fmt.Errorf("thunderstore decode: %w", err)
	}
	var out []tsPackage
	for dec.More() {
		var raw tsRawPackage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("thunderstore decode: %w", err)
		}
		if raw.IsDeprecated {
			continue
		}
		out = append(out, compactPackage(raw))
	}
	return out, nil
}

func compactPackage(raw tsRawPackage) tsPackage {
	p := tsPackage{
		Name:     raw.Name,
		FullName: raw.FullName,
		Owner:    raw.Owner,
		PageURL:  raw.PackageURL,
		Rating:   raw.RatingScore,
	}
	if len(raw.Versions) > 0 {
		p.Icon = raw.Versions[0].Icon
		p.Desc = truncate(raw.Versions[0].Description, tsMaxDescLen)
	}
	for i := range raw.Versions {
		p.Downloads += raw.Versions[i].Downloads
		if i < tsMaxVersionsPerPackage {
			p.Versions = append(p.Versions, tsVersion{
				FullName:      raw.Versions[i].FullName,
				VersionNumber: raw.Versions[i].VersionNumber,
				DownloadURL:   raw.Versions[i].DownloadURL,
				FileSize:      raw.Versions[i].FileSize,
			})
		}
	}
	return p
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
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
		if term != "" &&
			!strings.Contains(strings.ToLower(p.Name), term) &&
			!strings.Contains(strings.ToLower(p.Owner), term) {
			continue
		}
		matched = append(matched, p)
	}
	// Rank by community rating; ties keep catalog order (stable).
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].Rating > matched[j].Rating })
	if limit := clampLimit(q.Limit); len(matched) > limit {
		matched = matched[:limit]
	}

	out := make([]Project, 0, len(matched))
	for _, p := range matched {
		out = append(out, Project{
			ID:          p.FullName,
			Slug:        p.FullName,
			Title:       p.Name,
			Description: p.Desc,
			Author:      p.Owner,
			IconURL:     p.Icon,
			Downloads:   p.Downloads,
			PageURL:     p.PageURL,
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
				Name:          v.FullName,
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
