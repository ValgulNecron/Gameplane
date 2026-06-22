// Module bundle upload surface (admin-only via RBAC):
//
//   - POST   /modules/sources/{name}/upload            — store a bundle
//   - DELETE /modules/sources/{name}/upload/{module}   — remove a bundle
//
// The body is a tar.gz or zip holding one module directory (module.yaml
// + template.yaml [+ README.md, icon.png]). After validation the files
// are written into a ConfigMap labeled gameplane.gg/module-upload=true in
// the operator namespace — the exact shape an upload-type ModuleSource
// indexes, and the same thing `kubectl apply` of such a ConfigMap
// produces, so the operator stays authoritative. ?dryRun=true validates
// and returns the parsed metadata without storing anything.

package handlers

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// Bundle files are a couple of YAML docs and an icon; the ConfigMap
// backing them caps out at 1 MiB including metadata, so stay under it.
const maxUploadBundleBytes = 900 << 10

// labelModuleUpload mirrors the operator's v1alpha1.LabelModuleUpload
// (the api module doesn't depend on the operator module).
const (
	labelModuleUpload     = "gameplane.gg/module-upload"
	labelUploadModuleName = "gameplane.gg/module-name"
)

// bundleFileNames are the canonical files copied into the ConfigMap;
// anything else in the archive (junk like .DS_Store, nested docs) is
// ignored.
var bundleFileNames = []string{"module.yaml", "template.yaml", "README.md", "icon.png"}

// uploadedMetadata is the subset of module.yaml the API validates and
// echoes back for the dashboard's preview.
type uploadedMetadata struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Version     string `json:"version"`
	Game        string `json:"game"`
	Summary     string `json:"summary,omitempty"`
}

type uploadResponse struct {
	Module    uploadedMetadata `json:"module"`
	ConfigMap string           `json:"configMap,omitempty"`
	DryRun    bool             `json:"dryRun,omitempty"`
}

func (h modulesHandler) uploadBundle(w http.ResponseWriter, req *http.Request) {
	srcName := chi.URLParam(req, "name")
	if err := h.requireUploadSource(req, srcName); err != nil {
		writeUploadSourceErr(w, req, err)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, maxUploadBundleBytes+1))
	if err != nil {
		httperr.WriteCode(w, req, http.StatusBadRequest, err)
		return
	}
	if len(body) > maxUploadBundleBytes {
		httperr.WriteCode(w, req, http.StatusRequestEntityTooLarge,
			fmt.Errorf("bundle exceeds the %d KiB limit", maxUploadBundleBytes>>10))
		return
	}

	files, meta, err := parseUploadedBundle(body)
	if err != nil {
		httperr.WriteCode(w, req, http.StatusBadRequest, err)
		return
	}

	if req.URL.Query().Get("dryRun") == "true" {
		writeJSON(w, uploadResponse{Module: *meta, DryRun: true})
		return
	}

	cmName := "module-upload-" + meta.Name
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: h.namespace,
			Labels: map[string]string{
				labelModuleUpload:     "true",
				labelUploadModuleName: meta.Name,
			},
		},
		BinaryData: files,
	}
	cms := h.k.Typed.CoreV1().ConfigMaps(h.namespace)
	if _, err := cms.Create(req.Context(), cm, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			httperr.Write(w, req, err)
			return
		}
		existing, err := cms.Get(req.Context(), cmName, metav1.GetOptions{})
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		// Refuse to clobber a ConfigMap that isn't an upload bundle.
		if existing.Labels[labelModuleUpload] != "true" {
			httperr.WriteCode(w, req, http.StatusConflict,
				fmt.Errorf("configmap %q exists and is not a module upload", cmName))
			return
		}
		existing.Labels[labelUploadModuleName] = meta.Name
		existing.BinaryData = files
		existing.Data = nil
		if _, err := cms.Update(req.Context(), existing, metav1.UpdateOptions{}); err != nil {
			httperr.Write(w, req, err)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, uploadResponse{Module: *meta, ConfigMap: cmName})
}

func (h modulesHandler) deleteUpload(w http.ResponseWriter, req *http.Request) {
	srcName := chi.URLParam(req, "name")
	moduleName := chi.URLParam(req, "module")
	if err := h.requireUploadSource(req, srcName); err != nil {
		writeUploadSourceErr(w, req, err)
		return
	}

	// Installed Modules pull their bundle through the source on every
	// upgrade/re-apply; removing the bundle would strand them.
	mods, err := h.k.Dynamic.Resource(kube.GVRModule).List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	for i := range mods.Items {
		name, _, _ := unstructured.NestedString(mods.Items[i].Object, "spec", "name")
		src, _, _ := unstructured.NestedString(mods.Items[i].Object, "spec", "source", "name")
		if name == moduleName && src == srcName {
			httperr.WriteCode(w, req, http.StatusConflict,
				fmt.Errorf("module %q is installed as %q; uninstall it before removing the upload",
					moduleName, mods.Items[i].GetName()))
			return
		}
	}

	cms := h.k.Typed.CoreV1().ConfigMaps(h.namespace)
	list, err := cms.List(req.Context(), metav1.ListOptions{
		LabelSelector: labelModuleUpload + "=true," + labelUploadModuleName + "=" + moduleName,
	})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	if len(list.Items) == 0 {
		httperr.WriteCode(w, req, http.StatusNotFound,
			fmt.Errorf("no uploaded bundle for module %q", moduleName))
		return
	}
	for i := range list.Items {
		if err := cms.Delete(req.Context(), list.Items[i].Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			httperr.Write(w, req, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// errNotUploadSource marks the named-source-has-wrong-type condition;
// the handlers surface it as a 409 with the safe message intact.
var errNotUploadSource = errors.New("uploads need an upload-type source")

// requireUploadSource confirms the named ModuleSource exists and is
// upload-typed.
func (h modulesHandler) requireUploadSource(req *http.Request, name string) error {
	src, err := h.k.Dynamic.Resource(kube.GVRModuleSource).Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	typ, _, _ := unstructured.NestedString(src.Object, "spec", "type")
	if typ != "upload" {
		return fmt.Errorf("source %q is type %q: %w", name, typ, errNotUploadSource)
	}
	return nil
}

func writeUploadSourceErr(w http.ResponseWriter, req *http.Request, err error) {
	if errors.Is(err, errNotUploadSource) {
		httperr.WriteCode(w, req, http.StatusConflict, err)
		return
	}
	httperr.Write(w, req, err)
}

// parseUploadedBundle extracts and validates one module bundle.
func parseUploadedBundle(body []byte) (map[string][]byte, *uploadedMetadata, error) {
	extracted, err := extractUploadArchive(body)
	if err != nil {
		return nil, nil, err
	}

	// Locate the (single) directory holding module.yaml — flat archives
	// and one-folder-wrapped archives both work.
	var dirs []string
	for p := range extracted {
		if path.Base(p) == "module.yaml" {
			dirs = append(dirs, path.Dir(p))
		}
	}
	switch len(dirs) {
	case 0:
		return nil, nil, errors.New("bundle has no module.yaml")
	case 1:
	default:
		return nil, nil, fmt.Errorf("bundle holds %d modules; upload exactly one", len(dirs))
	}
	dir := dirs[0]

	files := map[string][]byte{}
	total := 0
	for _, name := range bundleFileNames {
		key := name
		if dir != "." {
			key = dir + "/" + name
		}
		if data, ok := extracted[key]; ok {
			files[name] = data
			total += len(data)
		}
	}
	if total > maxUploadBundleBytes {
		return nil, nil, fmt.Errorf("bundle files exceed the %d KiB limit", maxUploadBundleBytes>>10)
	}

	var meta uploadedMetadata
	if err := yaml.Unmarshal(files["module.yaml"], &meta); err != nil {
		return nil, nil, fmt.Errorf("parse module.yaml: %w", err)
	}
	switch {
	case meta.Name == "":
		return nil, nil, errors.New("module.yaml: name is required")
	case !moduleNameRE.MatchString(meta.Name):
		return nil, nil, errors.New("module.yaml: name must be a DNS label (lowercase, digits, hyphens)")
	case meta.DisplayName == "":
		return nil, nil, errors.New("module.yaml: displayName is required")
	case meta.Version == "":
		return nil, nil, errors.New("module.yaml: version is required")
	case meta.Game == "":
		return nil, nil, errors.New("module.yaml: game is required")
	}

	tmplRaw, ok := files["template.yaml"]
	if !ok || len(tmplRaw) == 0 {
		return nil, nil, errors.New("bundle has no template.yaml")
	}
	var tmpl struct {
		Spec map[string]any `json:"spec"`
	}
	if err := yaml.Unmarshal(tmplRaw, &tmpl); err != nil {
		return nil, nil, fmt.Errorf("parse template.yaml: %w", err)
	}
	if img, _ := tmpl.Spec["image"].(string); img == "" {
		return nil, nil, errors.New("template.yaml: spec.image is required")
	}

	return files, &meta, nil
}

// extractUploadArchive unpacks a tar.gz or zip body (detected by magic
// bytes) with path-traversal rejection. Size is pre-capped by the
// caller, so only the file count needs guarding here.
func extractUploadArchive(body []byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	add := func(name string, r io.Reader) error {
		p := path.Clean(strings.ReplaceAll(name, `\`, "/"))
		if p == "." || strings.HasSuffix(name, "/") {
			return nil
		}
		if path.IsAbs(p) || p == ".." || strings.HasPrefix(p, "../") {
			return fmt.Errorf("archive member %q escapes the extraction root", name)
		}
		if len(out) >= 256 {
			return errors.New("archive has too many files")
		}
		data, err := io.ReadAll(io.LimitReader(r, maxUploadBundleBytes+1))
		if err != nil {
			return err
		}
		if len(data) > maxUploadBundleBytes {
			return fmt.Errorf("archive member %q exceeds the %d KiB limit", name, maxUploadBundleBytes>>10)
		}
		out[p] = data
		return nil
	}

	switch {
	case len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b:
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			if err != nil {
				return nil, err
			}
			if hdr.Typeflag != tar.TypeReg {
				continue
			}
			if err := add(hdr.Name, tr); err != nil {
				return nil, err
			}
		}
	case bytes.HasPrefix(body, []byte("PK\x03\x04")):
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			err = add(f.Name, rc)
			_ = rc.Close()
			if err != nil {
				return nil, err
			}
		}
		return out, nil
	default:
		return nil, errors.New("unsupported bundle format (upload a .tar.gz or .zip)")
	}
}
