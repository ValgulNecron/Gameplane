// Modules wires the OCI-backed module distribution surface:
//
//   - GET    /modules/sources           — list ModuleSources (registry config)
//   - GET    /modules/catalog           — merged catalog: per-module aggregate
//                                         across sources, with installation state
//   - GET    /modules                   — list installed Module CRs
//   - POST   /modules                   — install: create a Module CR
//   - PATCH  /modules/{name}            — upgrade: patch spec.version
//   - DELETE /modules/{name}            — uninstall: delete the Module CR
//
// Catalog data is read directly from ModuleSource.status.modules — the
// operator pulls the registry on a schedule, the API just serves the
// cache. Installs go through the K8s API (Module CR creation) so the
// flow is identical whether driven from this handler or kubectl.

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// MountModules wires the /modules surface onto r. namespace is the
// operator namespace, where uploaded bundle ConfigMaps live.
func MountModules(r chi.Router, k *kube.Client, namespace string) {
	h := modulesHandler{k: k, namespace: namespace}
	r.Route("/modules", func(r chi.Router) {
		r.Get("/", h.listInstalled)
		r.Post("/", h.install)
		r.Get("/sources", h.listSources)
		r.Post("/sources", h.createSource)
		r.Put("/sources/{name}", h.updateSource)
		r.Delete("/sources/{name}", h.deleteSource)
		r.Post("/sources/{name}/upload", h.uploadBundle)
		r.Delete("/sources/{name}/upload/{module}", h.deleteUpload)
		r.Get("/catalog", h.catalog)
		r.Get("/{name}", h.getInstalled)
		r.Patch("/{name}", h.upgrade)
		r.Delete("/{name}", h.uninstall)
	})
}

type modulesHandler struct {
	k         *kube.Client
	namespace string
}

// SourceRef names one ModuleSource offering a catalog entry, with its
// type so the UI can badge where the module comes from.
type SourceRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// CatalogEntry is one row in the merged catalog response.
type CatalogEntry struct {
	Name             string      `json:"name"`
	DisplayName      string      `json:"displayName,omitempty"`
	Summary          string      `json:"summary,omitempty"`
	Game             string      `json:"game,omitempty"`
	Icon             string      `json:"icon,omitempty"`
	Sources          []SourceRef `json:"sources"`
	Versions         []string    `json:"versions,omitempty"`
	LatestVersion    string      `json:"latestVersion,omitempty"`
	Digest           string      `json:"digest,omitempty"`
	Installed        bool        `json:"installed"`
	InstalledVersion string      `json:"installedVersion,omitempty"`
	InstalledFrom    string      `json:"installedFrom,omitempty"` // ModuleSource name
	ModuleName       string      `json:"moduleName,omitempty"`    // Module CR name (= template name)
	Phase            string      `json:"phase,omitempty"`
	LastError        string      `json:"lastError,omitempty"`
	// AppliedDigest is the bundle digest backing the installed version,
	// for auditability. Mirrors Module.status.appliedDigest.
	AppliedDigest string `json:"appliedDigest,omitempty"`
	// PreviousVersion and PreviousDigest record the last-known-good the
	// operator rolls back to (Module.status). Surfaced read-only so the
	// UI can show a rollback target; the operator owns the rollback itself.
	PreviousVersion string `json:"previousVersion,omitempty"`
	PreviousDigest  string `json:"previousDigest,omitempty"`
}

type catalogResponse struct {
	Items []CatalogEntry `json:"items"`
}

type sourceListResponse struct {
	Items []*unstructured.Unstructured `json:"items"`
}

type installRequest struct {
	// Source is the ModuleSource name.
	Source string `json:"source"`
	// Module is the module name within the source.
	Module string `json:"module"`
	// Name is the resulting Module CR name (and GameTemplate name).
	// Defaults to Module when omitted.
	Name string `json:"name,omitempty"`
	// Version is an optional pin; empty tracks latest.
	Version string `json:"version,omitempty"`
}

type upgradeRequest struct {
	Version string `json:"version"`
}

var moduleNameRE = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

func (h modulesHandler) listSources(w http.ResponseWriter, req *http.Request) {
	list, err := h.k.Dynamic.Resource(kube.GVRModuleSource).List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	out := sourceListResponse{Items: make([]*unstructured.Unstructured, 0, len(list.Items))}
	for i := range list.Items {
		out.Items = append(out.Items, &list.Items[i])
	}
	writeJSON(w, out)
}

func (h modulesHandler) listInstalled(w http.ResponseWriter, req *http.Request) {
	list, err := h.k.Dynamic.Resource(kube.GVRModule).List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, list)
}

func (h modulesHandler) getInstalled(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	mod, err := h.k.Dynamic.Resource(kube.GVRModule).Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, mod)
}

func (h modulesHandler) install(w http.ResponseWriter, req *http.Request) {
	var in installRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		httperr.Write(w, req, err)
		return
	}
	if in.Source == "" || in.Module == "" {
		httperr.Write(w, req, errors.New("source and module are required"))
		return
	}
	name := in.Name
	if name == "" {
		name = in.Module
	}
	if !moduleNameRE.MatchString(name) {
		httperr.Write(w, req, errors.New("name must be a DNS label (lowercase, digits, hyphens)"))
		return
	}

	desired := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kestrel.gg/v1alpha1",
		"kind":       "Module",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"source":  map[string]any{"name": in.Source},
			"name":    in.Module,
			"version": in.Version,
		},
	}}
	created, err := h.k.Dynamic.Resource(kube.GVRModule).Create(req.Context(), desired, metav1.CreateOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

func (h modulesHandler) upgrade(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	var in upgradeRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
		httperr.Write(w, req, err)
		return
	}
	patch := map[string]any{"spec": map[string]any{"version": in.Version}}
	body, _ := json.Marshal(patch)
	updated, err := h.k.Dynamic.Resource(kube.GVRModule).Patch(
		req.Context(), name, types.MergePatchType, body, metav1.PatchOptions{},
	)
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, updated)
}

func (h modulesHandler) uninstall(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	// Surface the in-use blocker (set by the operator's finalizer) as a
	// 409 so the UI can render a useful error. Otherwise the Delete call
	// would succeed (DeletionTimestamp set) and the user would never be
	// told the resource is stuck on a finalizer.
	if blocker, err := h.uninstallBlocker(req, name); err == nil && blocker != "" {
		httperr.WriteCode(w, req, http.StatusConflict, errors.New(blocker))
		return
	}
	if err := h.k.Dynamic.Resource(kube.GVRModule).Delete(req.Context(), name, metav1.DeleteOptions{}); err != nil {
		httperr.Write(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// uninstallBlocker returns a non-empty string when the Module is still
// blocking its template's removal — typically a GameServer references
// the GameTemplate. The operator surfaces this on Module.status.lastError
// + Conditions[type=Ready].
func (h modulesHandler) uninstallBlocker(req *http.Request, name string) (string, error) {
	mod, err := h.k.Dynamic.Resource(kube.GVRModule).Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	// We only refuse early when the Module has already been observed to
	// be in-use AND a delete is being initiated; checking Phase=Failed
	// reason=InUse would be authoritative once a delete is in flight,
	// but pre-delete we use Status.AppliedTemplate + GameServer scan.
	tmpl, _, _ := unstructured.NestedString(mod.Object, "status", "appliedTemplate")
	if tmpl == "" {
		tmpl = name
	}
	servers, err := h.k.Dynamic.Resource(kube.GVRs["servers"]).List(req.Context(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	var users []string
	for i := range servers.Items {
		ref, _, _ := unstructured.NestedString(servers.Items[i].Object, "spec", "templateRef", "name")
		if ref == tmpl {
			ns := servers.Items[i].GetNamespace()
			users = append(users, ns+"/"+servers.Items[i].GetName())
		}
	}
	if len(users) == 0 {
		return "", nil
	}
	return "GameTemplate \"" + tmpl + "\" is still in use by: " + joinNames(users), nil
}

func joinNames(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

// catalog merges:
//   - every ModuleSource's status.modules (catalog)
//   - every Module CR (installed state)
//
// into a single per-name aggregate.
func (h modulesHandler) catalog(w http.ResponseWriter, req *http.Request) {
	sources, err := h.k.Dynamic.Resource(kube.GVRModuleSource).List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	mods, err := h.k.Dynamic.Resource(kube.GVRModule).List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	merged := map[string]*CatalogEntry{}
	for i := range sources.Items {
		src := &sources.Items[i]
		srcType, _, _ := unstructured.NestedString(src.Object, "spec", "type")
		if srcType == "" {
			srcType = "oci"
		}
		entries, _, _ := unstructured.NestedSlice(src.Object, "status", "modules")
		for _, raw := range entries {
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			if name == "" {
				continue
			}
			e, ok := merged[name]
			if !ok {
				e = &CatalogEntry{Name: name}
				merged[name] = e
			}
			if dn, _ := m["displayName"].(string); dn != "" && e.DisplayName == "" {
				e.DisplayName = dn
			}
			if s, _ := m["summary"].(string); s != "" && e.Summary == "" {
				e.Summary = s
			}
			if g, _ := m["game"].(string); g != "" && e.Game == "" {
				e.Game = g
			}
			if ic, _ := m["icon"].(string); ic != "" && e.Icon == "" {
				e.Icon = ic
			}
			e.Sources = appendUniqueSource(e.Sources, SourceRef{Name: src.GetName(), Type: srcType})
			vers, _, _ := unstructured.NestedStringSlice(m, "versions")
			e.Versions = mergeVersions(e.Versions, vers)
			if lv, _ := m["latestVersion"].(string); lv != "" {
				if e.LatestVersion == "" || semverDescending(lv, e.LatestVersion) {
					e.LatestVersion = lv
					if d, _ := m["digest"].(string); d != "" {
						e.Digest = d
					}
				}
			}
		}
	}

	for i := range mods.Items {
		mod := &mods.Items[i]
		modName, _, _ := unstructured.NestedString(mod.Object, "spec", "name")
		srcName, _, _ := unstructured.NestedString(mod.Object, "spec", "source", "name")
		appliedVersion, _, _ := unstructured.NestedString(mod.Object, "status", "appliedVersion")
		phase, _, _ := unstructured.NestedString(mod.Object, "status", "phase")
		lastError, _, _ := unstructured.NestedString(mod.Object, "status", "lastError")
		appliedDigest, _, _ := unstructured.NestedString(mod.Object, "status", "appliedDigest")
		previousVersion, _, _ := unstructured.NestedString(mod.Object, "status", "previousVersion")
		previousDigest, _, _ := unstructured.NestedString(mod.Object, "status", "previousDigest")

		e, ok := merged[modName]
		if !ok {
			// Installed but not (yet) in any catalog. Surface anyway so
			// the UI doesn't lose track of pending installs while the
			// source is mid-index.
			e = &CatalogEntry{Name: modName}
			merged[modName] = e
		}
		e.Installed = true
		e.InstalledVersion = appliedVersion
		e.InstalledFrom = srcName
		e.ModuleName = mod.GetName()
		e.Phase = phase
		e.LastError = lastError
		e.AppliedDigest = appliedDigest
		e.PreviousVersion = previousVersion
		e.PreviousDigest = previousDigest
	}

	out := catalogResponse{Items: make([]CatalogEntry, 0, len(merged))}
	for _, e := range merged {
		out.Items = append(out.Items, *e)
	}
	sort.Slice(out.Items, func(i, j int) bool {
		return out.Items[i].Name < out.Items[j].Name
	})
	writeJSON(w, out)
}

func appendUniqueSource(s []SourceRef, v SourceRef) []SourceRef {
	for _, x := range s {
		if x.Name == v.Name {
			return s
		}
	}
	return append(s, v)
}

// mergeVersions returns the union of a and b, semver-descending.
func mergeVersions(a, b []string) []string {
	seen := map[string]struct{}{}
	for _, v := range a {
		seen[v] = struct{}{}
	}
	for _, v := range b {
		seen[v] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return semverDescending(out[i], out[j]) })
	return out
}
