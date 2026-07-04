// Package mods serves the mod/plugin management API. Mods are files
// under a directory on the game's data volume (declared by the module's
// spec.capabilities.mods). The dashboard is generic: it lists, installs,
// and removes mods by calling these endpoints; the per-game specifics
// (directory, allowed download hosts) live in the template.
//
// Install downloads a user-supplied URL into the mods directory. Because
// the agent runs in-cluster, that download is a classic SSRF risk, so it
// is guarded three ways: the host must be on the module's allowlist, the
// dialed IP must be public (blocks DNS-rebinding to cluster/metadata
// addresses), and the body is size-capped.
package mods

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/agent/internal/caps"
	"github.com/ValgulNecron/gameplane/netguard"
)

const defaultMaxBytes = 256 << 20 // 256 MiB

type handler struct {
	dir      string   // absolute mods directory, "" when unconfigured
	exts     []string // lowercased extension allowlist ("" = any)
	allowed  []string // download host allowlist; nil disables installs
	extract  bool     // unpack downloaded archives into per-mod folders
	maxBytes int64
	client   *http.Client
	mu       sync.Mutex // serializes manifest read-modify-write cycles
}

// Mod is one installed mod file. Meta is nil for unmanaged files (placed
// out-of-band or installed before the manifest existed).
type Mod struct {
	Name    string   `json:"name"`
	Size    int64    `json:"size"`
	ModTime string   `json:"modTime"`
	Meta    *ModMeta `json:"meta,omitempty"`
}

// Mount registers the mods endpoints. spec is the module's declared mods
// config (nil when the template declares none — every endpoint then
// degrades rather than 404ing, so the dashboard renders uniformly).
func Mount(r chi.Router, dataRoot string, spec *caps.Mods) {
	h := newHandler(dataRoot, spec)
	r.Get("/mods", h.list)
	r.Post("/mods/install", h.install)
	r.Delete("/mods", h.remove)
}

func newHandler(dataRoot string, spec *caps.Mods) *handler {
	h := &handler{}
	if spec == nil || spec.Path == "" {
		return h
	}
	// Resolve the mods dir under the data root, rejecting traversal. The
	// CRD already forbids ".." at apply time; this is defense in depth in
	// case a bundle reaches the agent another way.
	root := filepath.Clean(dataRoot)
	dir := filepath.Join(root, filepath.Clean(spec.Path))
	if dir != root && !strings.HasPrefix(dir, root+string(os.PathSeparator)) {
		slog.Warn("mods path escapes data root; mods disabled", "path", spec.Path)
		return h
	}
	h.dir = dir
	h.extract = spec.Extract
	for _, e := range spec.Extensions {
		h.exts = append(h.exts, strings.ToLower(e))
	}
	if spec.Install != nil && len(spec.Install.AllowedHosts) > 0 {
		h.allowed = spec.Install.AllowedHosts
		h.maxBytes = int64(spec.Install.MaxSizeMB) << 20
		if h.maxBytes <= 0 {
			h.maxBytes = defaultMaxBytes
		}
		h.client = newSafeClient(h.allowed)
	}
	return h
}

func (h *handler) list(w http.ResponseWriter, _ *http.Request) {
	if h.dir == "" {
		writeJSON(w, http.StatusOK, []Mod{})
		return
	}
	entries, err := os.ReadDir(h.dir)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, []Mod{})
		return
	}
	if err != nil {
		slog.Warn("mods list", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not read mods directory")
		return
	}
	meta := loadManifest(h.dir).Mods
	out := []Mod{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // in-flight temp dirs/files and the manifest itself
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if h.extract {
			// Extract loaders store each mod as a folder.
			if !e.IsDir() {
				continue
			}
			out = append(out, Mod{
				Name:    name,
				Size:    dirSize(filepath.Join(h.dir, name)),
				ModTime: info.ModTime().UTC().Format(time.RFC3339),
				Meta:    meta[name],
			})
			continue
		}
		if e.IsDir() || !h.extAllowed(name) {
			continue
		}
		out = append(out, Mod{
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
			Meta:    meta[name],
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type installReq struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
	// Replaces names an existing mod to swap out after the new one lands
	// (in-place upgrade): install new → remove old → swap manifest entry.
	Replaces string `json:"replaces,omitempty"`
	// Meta is the registry identity recorded in the manifest. Absent for a
	// plain URL install, which makes the result unmanaged.
	Meta *ModMeta `json:"meta,omitempty"`
}

func (h *handler) install(w http.ResponseWriter, req *http.Request) {
	if h.dir == "" || h.client == nil {
		writeErr(w, http.StatusNotImplemented, "mod installs are not enabled for this game")
		return
	}
	var body installReq
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 4<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	u, err := url.Parse(strings.TrimSpace(body.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeErr(w, http.StatusBadRequest, "url must be an absolute http(s) URL")
		return
	}
	if !hostAllowed(u.Hostname(), h.allowed) {
		writeErr(w, http.StatusForbidden, "download host is not allowed by this module")
		return
	}

	name := body.Name
	if name == "" {
		name = path.Base(u.Path)
	}
	name, err = safeName(name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.extAllowed(name) {
		writeErr(w, http.StatusBadRequest, "file type is not an accepted mod for this game")
		return
	}
	replaces := ""
	if body.Replaces != "" {
		replaces, err = safeName(body.Replaces)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "replaces: "+err.Error())
			return
		}
	}

	// Extract loaders (e.g. BepInEx): the download is an archive unpacked
	// into its own folder so the loader's recursive scan finds the files.
	installName := name
	var size int64
	if h.extract {
		installName = archiveFolderName(name)
		size, err = h.installArchive(req.Context(), u.String(), installName)
	} else {
		size, err = h.download(req.Context(), u.String(), name)
	}
	switch {
	case errors.Is(err, errTooLarge):
		writeErr(w, http.StatusRequestEntityTooLarge, "mod exceeds the size limit")
		return
	case errors.Is(err, netguard.ErrBlockedAddr) || errors.Is(err, errHostNotAllowed):
		writeErr(w, http.StatusForbidden, "download was redirected to a disallowed address")
		return
	case errors.Is(err, errBadArchive):
		writeErr(w, http.StatusBadGateway, "downloaded file is not a valid archive")
		return
	case err != nil:
		// Never reflect the raw error — it may carry internal addresses.
		slog.Warn("mod install", "host", u.Hostname(), "err", err)
		writeErr(w, http.StatusBadGateway, "could not download the mod")
		return
	}

	// Upgrade path: the new mod is in place, so removing the old one now
	// can only leave a visible duplicate on crash — never a missing mod.
	if replaces != "" && replaces != installName {
		if rmErr := h.removeEntry(replaces); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			slog.Warn("mod replace cleanup", "name", replaces, "err", rmErr)
		}
	}
	meta := body.Meta
	if meta != nil {
		stamped := *meta
		stamped.SourceURL = u.String()
		stamped.InstalledAt = time.Now().UTC().Format(time.RFC3339)
		meta = &stamped
	}
	h.updateManifest(func(mods map[string]*ModMeta) {
		delete(mods, replaces)
		if meta != nil {
			mods[installName] = meta
		} else {
			// A plain URL (re)install carries no identity — drop any stale
			// entry so the listing doesn't claim a provenance it lost.
			delete(mods, installName)
		}
	})
	writeJSON(w, http.StatusOK, Mod{Name: installName, Size: size, Meta: meta})
}

// removeEntry deletes an installed mod by name — the whole folder for
// extract loaders, a single file otherwise.
func (h *handler) removeEntry(name string) error {
	target := filepath.Join(h.dir, name)
	if h.extract {
		return os.RemoveAll(target)
	}
	return os.Remove(target)
}

// downloadTemp streams url into a temp file under the mods dir, enforcing
// the size cap. The caller owns the returned path (rename it into place or
// unpack it), and must remove it.
func (h *handler) downloadTemp(ctx context.Context, url string) (string, int64, error) {
	if err := os.MkdirAll(h.dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("mkdir mods: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	if resp.ContentLength > h.maxBytes {
		return "", 0, errTooLarge
	}

	tmp, err := os.CreateTemp(h.dir, ".dl-*")
	if err != nil {
		return "", 0, err
	}
	tmpName := tmp.Name()
	// LimitReader caps the copy at maxBytes+1 so an oversize body (with no
	// or a lying Content-Length) is caught mid-stream.
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, h.maxBytes+1))
	closeErr := tmp.Close()
	if err != nil || closeErr != nil || n > h.maxBytes {
		os.Remove(tmpName)
		switch {
		case err != nil:
			return "", 0, err
		case closeErr != nil:
			return "", 0, closeErr
		default:
			return "", 0, errTooLarge
		}
	}
	return tmpName, n, nil
}

// download streams url into <dir>/<name>, then atomically renames it.
func (h *handler) download(ctx context.Context, url, name string) (int64, error) {
	tmpName, n, err := h.downloadTemp(ctx, url)
	if err != nil {
		return 0, err
	}
	if err := os.Rename(tmpName, filepath.Join(h.dir, name)); err != nil {
		os.Remove(tmpName)
		return 0, err
	}
	return n, nil
}

// installArchive downloads a zip and unpacks it into <dir>/<folder>/,
// replacing any existing folder of that name. Returns the downloaded size.
func (h *handler) installArchive(ctx context.Context, url, folder string) (int64, error) {
	tmpZip, size, err := h.downloadTemp(ctx, url)
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmpZip)

	staging, err := os.MkdirTemp(h.dir, ".ex-*")
	if err != nil {
		return 0, err
	}
	if err := unzipInto(tmpZip, staging, h.maxBytes); err != nil {
		os.RemoveAll(staging)
		return 0, err
	}
	final := filepath.Join(h.dir, folder)
	if err := os.RemoveAll(final); err != nil {
		os.RemoveAll(staging)
		return 0, err
	}
	if err := os.Rename(staging, final); err != nil {
		os.RemoveAll(staging)
		return 0, err
	}
	return size, nil
}

// unzipInto extracts zipPath into dst, guarding against zip-slip and
// capping the total uncompressed size.
func unzipInto(zipPath, dst string, maxBytes int64) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errBadArchive, err)
	}
	defer zr.Close()

	dstClean := filepath.Clean(dst)
	var total int64
	for _, f := range zr.File {
		target := filepath.Join(dstClean, filepath.Clean(f.Name))
		// Reject entries that resolve outside the staging dir (zip-slip).
		if target != dstClean && !strings.HasPrefix(target, dstClean+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, maxBytes-total+1))
		rc.Close()
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		total += n
		if total > maxBytes {
			return errTooLarge
		}
	}
	return nil
}

// archiveFolderName strips a known archive extension to name the per-mod
// folder (e.g. "Owner-Mod-1.0.0.zip" → "Owner-Mod-1.0.0").
func archiveFolderName(name string) string {
	lower := strings.ToLower(name)
	for _, ext := range []string{".tar.gz", ".tgz", ".zip"} {
		if strings.HasSuffix(lower, ext) {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

// dirSize sums the size of every regular file under dir.
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, e := d.Info(); e == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func (h *handler) remove(w http.ResponseWriter, req *http.Request) {
	if h.dir == "" {
		writeErr(w, http.StatusNotImplemented, "mods are not configured for this game")
		return
	}
	name, err := safeName(req.URL.Query().Get("name"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	target := filepath.Join(h.dir, name)
	if _, statErr := os.Stat(target); errors.Is(statErr, os.ErrNotExist) {
		writeErr(w, http.StatusNotFound, "no such mod")
		return
	}
	if err := h.removeEntry(name); err != nil {
		slog.Warn("mod remove", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not remove mod")
		return
	}
	h.updateManifest(func(mods map[string]*ModMeta) {
		delete(mods, name)
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) extAllowed(name string) bool {
	if len(h.exts) == 0 {
		return true
	}
	lower := strings.ToLower(name)
	for _, e := range h.exts {
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	return false
}

// safeName accepts a bare filename and rejects anything that could escape
// the mods directory (path separators, "..", leading dot/dash, control
// chars). Returns the cleaned base name.
func safeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("name is required")
	}
	if name != filepath.Base(name) || name != path.Base(name) {
		return "", errors.New("name must not contain a path")
	}
	if name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return "", errors.New("invalid mod name")
	}
	if len(name) > 200 {
		return "", errors.New("name is too long")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return "", errors.New("name contains an illegal character")
		}
	}
	return name, nil
}

// hostAllowed matches host against the allowlist: an exact hostname, or a
// leading-dot suffix (".example.com") matching that domain and any
// subdomain. Comparison is case-insensitive.
func hostAllowed(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, ".") {
			if host == a[1:] || strings.HasSuffix(host, a) {
				return true
			}
			continue
		}
		if host == a {
			return true
		}
	}
	return false
}

var (
	errTooLarge       = errors.New("download exceeds size limit")
	errHostNotAllowed = errors.New("redirect host not allowed")
	errBadArchive     = errors.New("not a valid archive")
)

// ssrfPolicy decides whether the agent may dial an address. It defaults to
// the shared public-only policy (netguard.IsPublic) and is a package var only
// so tests can permit loopback to reach an httptest server.
var ssrfPolicy = netguard.IsPublic

// newSafeClient builds an HTTP client that refuses to connect to non-public
// addresses (the shared SSRF guard, applied to the actual dialed IP so DNS
// rebinding can't slip past the host allowlist) and re-checks the host
// allowlist on every redirect (which the shared client leaves to the caller).
func newSafeClient(allowed []string) *http.Client {
	client := netguard.HTTPClient(2*time.Minute, ssrfPolicy)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if !hostAllowed(req.URL.Hostname(), allowed) {
			return errHostNotAllowed
		}
		return nil
	}
	return client
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
