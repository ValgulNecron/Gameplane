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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kestrel-gg/kestrel/agent/internal/caps"
)

const defaultMaxBytes = 256 << 20 // 256 MiB

type handler struct {
	dir      string   // absolute mods directory, "" when unconfigured
	exts     []string // lowercased extension allowlist ("" = any)
	allowed  []string // download host allowlist; nil disables installs
	maxBytes int64
	client   *http.Client
}

// Mod is one installed mod file.
type Mod struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
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
	out := []Mod{}
	for _, e := range entries {
		if e.IsDir() || !h.extAllowed(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Mod{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type installReq struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
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

	size, err := h.download(req.Context(), u.String(), name)
	switch {
	case errors.Is(err, errTooLarge):
		writeErr(w, http.StatusRequestEntityTooLarge, "mod exceeds the size limit")
		return
	case errors.Is(err, errBlockedAddr) || errors.Is(err, errHostNotAllowed):
		writeErr(w, http.StatusForbidden, "download was redirected to a disallowed address")
		return
	case err != nil:
		// Never reflect the raw error — it may carry internal addresses.
		slog.Warn("mod install", "host", u.Hostname(), "err", err)
		writeErr(w, http.StatusBadGateway, "could not download the mod")
		return
	}
	writeJSON(w, http.StatusOK, Mod{Name: name, Size: size})
}

// download streams url into <dir>/<name> via a temp file, enforcing the
// size cap, then atomically renames it into place.
func (h *handler) download(ctx context.Context, url, name string) (int64, error) {
	if err := os.MkdirAll(h.dir, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir mods: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	if resp.ContentLength > h.maxBytes {
		return 0, errTooLarge
	}

	tmp, err := os.CreateTemp(h.dir, ".dl-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed

	// LimitReader caps the copy at maxBytes+1 so an oversize body (with no
	// or a lying Content-Length) is caught mid-stream.
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, h.maxBytes+1))
	closeErr := tmp.Close()
	if err != nil {
		return 0, err
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if n > h.maxBytes {
		return 0, errTooLarge
	}
	if err := os.Rename(tmpName, filepath.Join(h.dir, name)); err != nil {
		return 0, err
	}
	return n, nil
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
	err = os.Remove(filepath.Join(h.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		writeErr(w, http.StatusNotFound, "no such mod")
		return
	}
	if err != nil {
		slog.Warn("mod remove", "err", err)
		writeErr(w, http.StatusInternalServerError, "could not remove mod")
		return
	}
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
	errBlockedAddr    = errors.New("address is not publicly routable")
	errHostNotAllowed = errors.New("redirect host not allowed")
)

// newSafeClient builds an HTTP client that refuses to connect to
// non-public addresses (the core SSRF guard, applied to the actual
// dialed IP so DNS rebinding can't slip past the host allowlist) and
// re-checks the host allowlist on every redirect.
func newSafeClient(allowed []string) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: 2 * time.Minute,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := &net.Dialer{
					Timeout: dialer.Timeout,
					Control: func(_, addr string, _ syscall.RawConn) error {
						host, _, err := net.SplitHostPort(addr)
						if err != nil {
							return err
						}
						ip := net.ParseIP(host)
						if ip == nil || !ipAllowed(ip) {
							return errBlockedAddr
						}
						return nil
					},
				}
				return d.DialContext(ctx, network, address)
			},
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConns:          2,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !hostAllowed(req.URL.Hostname(), allowed) {
				return errHostNotAllowed
			}
			return nil
		},
	}
}

// ipAllowed decides whether the agent may dial an address. It defaults
// to isPublic (the SSRF guard) and is a package var only so tests can
// permit loopback to reach an httptest server.
var ipAllowed = isPublic

// isPublic reports whether ip is a globally routable unicast address —
// i.e. not loopback, private (RFC1918 / ULA), link-local, multicast, or
// unspecified. This is what blocks the agent from being tricked into
// fetching cluster-internal services or the cloud metadata endpoint.
func isPublic(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
