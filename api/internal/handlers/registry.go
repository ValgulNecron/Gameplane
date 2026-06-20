package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kestrel-gg/kestrel/api/internal/httperr"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
	"github.com/kestrel-gg/kestrel/api/internal/registry"
)

// errNoRegistry signals that a server's template declares no browsable mod
// registry (no capabilities.mods.registry, or an unselectable provider).
// The handler maps it to 501 so the dashboard hides the Browse tab and
// falls back to install-by-URL.
var errNoRegistry = errors.New("no mod registry for this server")

// registrySet is the subset of *registry.Set the handler needs, so tests
// can inject a fake provider.
type registrySet interface {
	For(registry.Config) (registry.Provider, bool)
}

// MountRegistry wires read-only mod-registry browse onto r. Routes are
// server-scoped so the API resolves the active version's loader and
// game-version token from the cluster (the operator is authoritative) —
// the client never supplies registry facets. Both are GETs under the
// "servers" segment, so the existing servers:read RBAC rule covers them
// (search is viewer+); installing a result reuses the existing
// servers:write /mods/install route.
func MountRegistry(r chi.Router, k *kube.Client, reg registrySet) {
	h := &registryHandler{k: k, reg: reg}
	r.Get("/servers/{name}/mods/registry/search", h.search)
	r.Get("/servers/{name}/mods/registry/projects/{project}/versions", h.versions)
}

type registryHandler struct {
	k   *kube.Client
	reg registrySet
}

func (h *registryHandler) search(w http.ResponseWriter, req *http.Request) {
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	p, loader, gameVersion, err := h.resolve(req.Context(), ns, chi.URLParam(req, "name"))
	if err != nil {
		h.writeResolveErr(w, req, err)
		return
	}
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	res, err := p.Search(req.Context(), registry.SearchQuery{
		Term:        req.URL.Query().Get("q"),
		Loader:      loader,
		GameVersion: gameVersion,
		Limit:       limit,
	})
	if err != nil {
		httperr.WriteCode(w, req, http.StatusBadGateway, fmt.Errorf("mod registry search failed: %w", err))
		return
	}
	writeJSON(w, res)
}

func (h *registryHandler) versions(w http.ResponseWriter, req *http.Request) {
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	p, loader, gameVersion, err := h.resolve(req.Context(), ns, chi.URLParam(req, "name"))
	if err != nil {
		h.writeResolveErr(w, req, err)
		return
	}
	res, err := p.Versions(req.Context(), chi.URLParam(req, "project"), registry.Filter{
		Loader:      loader,
		GameVersion: gameVersion,
	})
	if err != nil {
		httperr.WriteCode(w, req, http.StatusBadGateway, fmt.Errorf("mod registry versions failed: %w", err))
		return
	}
	writeJSON(w, res)
}

// resolve loads the server + its template, selects the registry provider
// declared by the template, and returns it alongside the active version's
// loader and game-version token (used as registry facets).
func (h *registryHandler) resolve(ctx context.Context, ns, name string) (registry.Provider, string, string, error) {
	gs, err := h.k.Dynamic.Resource(kube.GVRs["servers"]).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, "", "", err
	}
	selected, _, _ := unstructured.NestedString(gs.Object, "spec", "version")
	tmplName, _, _ := unstructured.NestedString(gs.Object, "spec", "templateRef", "name")
	if tmplName == "" {
		return nil, "", "", errNoRegistry
	}
	tmpl, err := h.k.Dynamic.Resource(kube.GVRs["templates"]).Get(ctx, tmplName, metav1.GetOptions{})
	if err != nil {
		return nil, "", "", err
	}

	provider, community, ok := registryConfig(tmpl)
	if !ok {
		return nil, "", "", errNoRegistry
	}
	p, ok := h.reg.For(registry.Config{Provider: provider, Community: community})
	if !ok {
		return nil, "", "", errNoRegistry
	}
	loader, gameVersion := activeVersion(tmpl, selected)
	return p, loader, gameVersion, nil
}

func (h *registryHandler) writeResolveErr(w http.ResponseWriter, req *http.Request, err error) {
	if errors.Is(err, errNoRegistry) {
		httperr.WriteCode(w, req, http.StatusNotImplemented, err)
		return
	}
	httperr.Write(w, req, err)
}

// registryConfig reads the template's capabilities.mods.registry block.
func registryConfig(tmpl *unstructured.Unstructured) (provider, community string, ok bool) {
	reg, found, err := unstructured.NestedMap(tmpl.Object, "spec", "capabilities", "mods", "registry")
	if !found || err != nil || reg == nil {
		return "", "", false
	}
	provider, _ = reg["provider"].(string)
	community, _ = reg["community"].(string)
	if provider == "" {
		return "", "", false
	}
	return provider, community, true
}

// activeVersion mirrors the operator's version selection (and the web's
// activeVersion): the entry whose id matches the server's selection, else
// the default entry, else the first. Returns the entry's loader id and
// clean game-version token. Both may be empty.
func activeVersion(tmpl *unstructured.Unstructured, selected string) (loader, gameVersion string) {
	versions, found, err := unstructured.NestedSlice(tmpl.Object, "spec", "versions")
	if !found || err != nil {
		return "", ""
	}
	var first, def map[string]any
	for _, raw := range versions {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if first == nil {
			first = m
		}
		if id, _ := m["id"].(string); selected != "" && id == selected {
			return loaderGameVersion(m)
		}
		if d, _ := m["default"].(bool); d && def == nil {
			def = m
		}
	}
	switch {
	case def != nil:
		return loaderGameVersion(def)
	case first != nil:
		return loaderGameVersion(first)
	}
	return "", ""
}

func loaderGameVersion(m map[string]any) (loader, gameVersion string) {
	loader, _ = m["loader"].(string)
	gameVersion, _ = m["gameVersion"].(string)
	return loader, gameVersion
}
