package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Steam browses the Steam Workshop through the Steam Web API, faceted to
// one app id (see steamApp below — a template picks the app the way
// Thunderstore picks a community).
//
// Workshop content cannot be downloaded over HTTP: it is fetched by
// steamcmd running INSIDE the game container using the app's own depot
// credentials, a path this API server has no way to reach or mint a URL
// for. So Versions() always returns an EMPTY version slice (zero Files is
// not enough — see below) — that is not a stub, it deliberately makes Steam
// a title/thumbnail preview browser with no Install button. Do NOT invent a
// DownloadURL here.
//
// Unlike curseforge.go — which drops each individual file-less File
// row but still returns the surrounding Version when at least one other
// file exists — every Steam item is file-less, always, so Versions()
// returns no versions at all rather than one Version with an empty Files
// slice. The dashboard's empty state is keyed on zero *versions*
// (`list.length === 0`), not zero files inside a version; a lone
// Files-less Version would instead render a populated version dropdown
// next to a permanently-disabled Install button with no explanation.
//
// Two Steam Web API endpoints are used:
//   - IPublishedFileService/QueryFiles (search/browse) requires a Steam
//     Web API key — Search facets it to the template's app id and, for
//     the Modpacks tab (SearchQuery.ProjectType == "modpack"), asks for
//     Collections instead of individual items so a Workshop Collection id
//     can flow into the existing modpacks.refEnv install path.
//   - ISteamRemoteStorage/GetPublishedFileDetails (resolve-by-id) is
//     keyless. A numeric search term is routed there instead of
//     QueryFiles, since a raw Workshop id wouldn't text-match a title
//     anyway and this lets an already-known id resolve even without
//     spending a keyed search call. Versions() also uses it: given a
//     project id, resolving its current title/preview needs no search at
//     all.
type Steam struct {
	client    *http.Client
	userAgent string
	apiKey    string
	baseURL   string // overridable in tests; default https://api.steampowered.com
}

func newSteam(client *http.Client, userAgent, apiKey string) *Steam {
	return &Steam{client: client, userAgent: userAgent, apiKey: apiKey, baseURL: "https://api.steampowered.com"}
}

// steamApp binds the Steam engine to one template's app id so it satisfies
// the Provider interface, mirroring thunderstoreCommunity's per-config
// wrapper.
type steamApp struct {
	steam *Steam
	appID int32
}

func (a *steamApp) Search(ctx context.Context, q SearchQuery) ([]Project, error) {
	return a.steam.search(ctx, q, a.appID)
}

func (a *steamApp) Versions(ctx context.Context, projectID string, _ Filter) ([]Version, error) {
	return a.steam.versions(ctx, projectID, a.appID)
}

// ModpackDeps is a no-op: a Steam Workshop collection installs as a whole
// via modpacks.refEnv (steamcmd resolves the collection's members itself
// inside the container), never by resolving individual dependency mods
// the way Thunderstore/BepInEx packs do.
func (a *steamApp) ModpackDeps(_ context.Context, _ string) ([]File, error) {
	return nil, nil
}

// Steam Workshop query-type (EPublishedFileQueryType) and query-filetype
// (EPublishedFileInfoMatchingFileType) enum values used by QueryFiles, from
// the Steamworks partner API docs.
const (
	steamQueryRankedByTrend      = 3  // browse (no search text): rank by recent popularity
	steamQueryRankedByTextSearch = 12 // search: rank by text-match relevance

	// steamTrendDays is the RankedByTrend lookback window. The Steamworks
	// API effectively requires "days" whenever query_type is
	// RankedByTrend — third-party clients send 7, and omitting it can make
	// browse (the no-search-term case) return an empty result.
	steamTrendDays = 7

	// EPublishedFileInfoMatchingFileType: Items=0, ItemsMtx=1,
	// ItemsReadyToUse=2, Collections=3, Artwork=4, ... — confirmed against
	// the Steamworks partner docs enum, not just this file's own prior
	// (incorrect) value.
	steamFileTypeItems       = 0 // individual Workshop items (mods, maps, ...)
	steamFileTypeCollections = 3 // Workshop Collections (bundles of items)
)

func (s *Steam) search(ctx context.Context, q SearchQuery, appID int32) ([]Project, error) {
	term := strings.TrimSpace(q.Term)
	if isDigits(term) {
		return s.resolveByID(ctx, term, appID)
	}
	return s.queryFiles(ctx, q, term, appID)
}

func (s *Steam) queryFiles(ctx context.Context, q SearchQuery, term string, appID int32) ([]Project, error) {
	limit := clampLimit(q.Limit)
	// QueryFiles paginates by an opaque cursor or a 1-based page number;
	// Provider.Search only carries an integer Offset (the same contract
	// every other engine here uses), so it's translated to a page number
	// rather than threading a cursor through the interface.
	page := 1
	if limit > 0 {
		page = q.Offset/limit + 1
	}

	params := url.Values{}
	params.Set("key", s.apiKey)
	params.Set("appid", strconv.Itoa(int(appID)))
	params.Set("numperpage", strconv.Itoa(limit))
	params.Set("page", strconv.Itoa(page))
	params.Set("return_previews", "true")
	params.Set("return_short_description", "true")
	if q.modpack() {
		// Collections are how the Workshop bundles multiple items — this
		// is what makes a Collection id browsable on the Modpacks tab.
		params.Set("filetype", strconv.Itoa(steamFileTypeCollections))
	} else {
		params.Set("filetype", strconv.Itoa(steamFileTypeItems))
	}
	if term != "" {
		params.Set("query_type", strconv.Itoa(steamQueryRankedByTextSearch))
		params.Set("search_text", term)
	} else {
		params.Set("query_type", strconv.Itoa(steamQueryRankedByTrend))
		params.Set("days", strconv.Itoa(steamTrendDays))
	}

	var resp steamQueryResponse
	u := s.baseURL + "/IPublishedFileService/QueryFiles/v1/?" + params.Encode()
	if err := httpGetJSON(ctx, s.client, s.userAgent, u, &resp, defaultMaxRespBytes); err != nil {
		return nil, fmt.Errorf("steam workshop query: %w", err)
	}
	out := make([]Project, 0, len(resp.Response.PublishedFileDetails))
	for _, it := range resp.Response.PublishedFileDetails {
		out = append(out, it.project())
	}
	return out, nil
}

// resolveByID resolves one Workshop item by id via the keyless
// GetPublishedFileDetails endpoint. Items belonging to a different app, or
// ids the API doesn't recognize, resolve to no match rather than an error
// — a bad/foreign id is a normal empty search result, not a failure.
func (s *Steam) resolveByID(ctx context.Context, id string, appID int32) ([]Project, error) {
	item, found, err := s.getDetails(ctx, id)
	if err != nil {
		return nil, err
	}
	if !found || item.ConsumerAppID != appID {
		return []Project{}, nil
	}
	return []Project{item.project()}, nil
}

// versions resolves projectID's current details. It never has a
// downloadable file to offer (Workshop content is fetched by steamcmd
// inside the game container, not over HTTP — see the Steam doc comment
// above), so it always reports zero versions rather than one Version with
// an empty Files slice: that keeps the dashboard's empty state ("No
// compatible files.", gated on zero *versions*) firing correctly instead
// of showing a populated version dropdown next to a dead Install button.
func (s *Steam) versions(ctx context.Context, projectID string, _ int32) ([]Version, error) {
	_, _, err := s.getDetails(ctx, projectID)
	if err != nil {
		return nil, err
	}
	// A resolved item (matching appID or not) and an unknown id both land
	// here: zero versions, since Workshop items never have a downloadable
	// file (see above). The id lookup exists only to surface a genuine
	// upstream failure as an error rather than a silent empty result.
	return []Version{}, nil
}

// getDetails calls the keyless ISteamRemoteStorage/GetPublishedFileDetails
// endpoint for one id. found is false for an unknown id (API result code 9
// = k_EResultFileNotFound) as well as any other non-OK result code.
func (s *Steam) getDetails(ctx context.Context, id string) (steamWorkshopItem, bool, error) {
	form := url.Values{}
	form.Set("itemcount", "1")
	form.Set("publishedfileids[0]", id)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/ISteamRemoteStorage/GetPublishedFileDetails/v1/", strings.NewReader(form.Encode()))
	if err != nil {
		return steamWorkshopItem{}, false, fmt.Errorf("steam request: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		// This endpoint is keyless (no query string at all today), but go
		// through the same redaction as httpGetJSON anyway: a *url.Error's
		// Error() embeds the full request URL, and this is exactly the kind
		// of call site that would silently start leaking a secret if a key
		// were ever added to it later without this guard in place.
		return steamWorkshopItem{}, false, sanitizeUpstreamErr("steam GET", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return steamWorkshopItem{}, false, fmt.Errorf("steam GET: upstream status %d", resp.StatusCode)
	}

	var out steamQueryResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, defaultMaxRespBytes)).Decode(&out); err != nil {
		return steamWorkshopItem{}, false, fmt.Errorf("steam decode: %w", err)
	}
	if len(out.Response.PublishedFileDetails) == 0 {
		return steamWorkshopItem{}, false, nil
	}
	item := out.Response.PublishedFileDetails[0]
	// result 1 = OK; anything else (e.g. 9 = file not found) is no match.
	const steamResultOK = 1
	if item.Result != steamResultOK {
		return steamWorkshopItem{}, false, nil
	}
	return item, true, nil
}

// steamQueryResponse is IPublishedFileService/QueryFiles's response
// envelope.
type steamQueryResponse struct {
	Response struct {
		PublishedFileDetails []steamWorkshopItem `json:"publishedfiledetails"`
	} `json:"response"`
}

// steamWorkshopItem is one Workshop item as returned by both QueryFiles and
// GetPublishedFileDetails (a superset of fields; each endpoint populates
// what it has). ConsumerAppID is only present on GetPublishedFileDetails
// responses — QueryFiles is already scoped by the appid request param, so
// it needs no re-check.
type steamWorkshopItem struct {
	PublishedFileID string `json:"publishedfileid"`
	Result          int    `json:"result"`
	Title           string `json:"title"`
	// Creator is the uploader's SteamID64, not a display name — resolving
	// a display name needs a second call (ISteamUser/GetPlayerSummaries)
	// this engine doesn't make, so Project.Author carries the raw id.
	Creator       string `json:"creator"`
	PreviewURL    string `json:"preview_url"`
	Subscriptions int64  `json:"subscriptions"`
	ConsumerAppID int32  `json:"consumer_app_id"`
	// ShortDesc is QueryFiles' blurb field (return_short_description=true).
	// GetPublishedFileDetails — the resolve-by-id path used by every
	// Versions() call and by a digit-term Search — never populates it;
	// that endpoint's blurb comes back as Description instead.
	ShortDesc       string `json:"short_description"`
	Description     string `json:"description"`
	FileDescription string `json:"file_description"`
}

func (it steamWorkshopItem) project() Project {
	desc := it.ShortDesc
	if desc == "" {
		desc = it.Description
	}
	if desc == "" {
		desc = truncate(it.FileDescription, tsMaxDescLen)
	}
	return Project{
		ID:          it.PublishedFileID,
		Title:       it.Title,
		Description: desc,
		Author:      it.Creator,
		IconURL:     it.PreviewURL,
		Downloads:   it.Subscriptions,
		PageURL:     "https://steamcommunity.com/sharedfiles/filedetails/?id=" + it.PublishedFileID,
		Provider:    "steam",
	}
}
