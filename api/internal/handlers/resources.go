// Package handlers implements the REST surface of the API. The design
// deliberately sticks close to the Kubernetes API shape — the dashboard
// talks to the API, the API to the dynamic client, and the dynamic
// client to Kestrel CRDs. No intermediate DTOs.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kestrel-gg/kestrel/api/internal/httperr"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
	"github.com/kestrel-gg/kestrel/api/internal/scope"
)

// MountResources wires /servers, /templates, /backups, /schedules.
//
// Templates are cluster-scoped; the others are namespaced. The kube
// package's GVR map determines which is which via resource path.
func MountResources(r chi.Router, k *kube.Client) {
	for path, gvr := range kube.GVRs {
		mountOne(r, k, path, gvr)
	}
}

func mountOne(r chi.Router, k *kube.Client, path string, gvr schema.GroupVersionResource) {
	r.Route("/"+path, func(r chi.Router) {
		r.Get("/", listHandler(k, gvr))
		r.Post("/", createHandler(k, gvr))
		r.Get("/{name}", getHandler(k, gvr))
		r.Put("/{name}", updateHandler(k, gvr))
		r.Delete("/{name}", deleteHandler(k, gvr))
	})
}

// resolveNS validates the namespace query parameter and writes a 403
// response when the caller is not permitted to use it. ok=false means
// the caller should stop processing and return.
func resolveNS(w http.ResponseWriter, req *http.Request) (string, bool) {
	ns, err := scope.Resolve(req)
	if err != nil {
		httperr.Write(w, req, err)
		return "", false
	}
	return ns, true
}

func listHandler(k *kube.Client, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var (
			list *unstructured.UnstructuredList
			err  error
		)
		if cluster(gvr) {
			list, err = k.Dynamic.Resource(gvr).List(req.Context(), metav1.ListOptions{})
		} else {
			ns, ok := resolveNS(w, req)
			if !ok {
				return
			}
			list, err = k.Dynamic.Resource(gvr).Namespace(ns).List(req.Context(), metav1.ListOptions{})
		}
		writeOrErr(w, req, list, err)
	}
}

func getHandler(k *kube.Client, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		var (
			obj *unstructured.Unstructured
			err error
		)
		if cluster(gvr) {
			obj, err = k.Dynamic.Resource(gvr).Get(req.Context(), name, metav1.GetOptions{})
		} else {
			ns, ok := resolveNS(w, req)
			if !ok {
				return
			}
			obj, err = k.Dynamic.Resource(gvr).Namespace(ns).Get(req.Context(), name, metav1.GetOptions{})
		}
		writeOrErr(w, req, obj, err)
	}
}

func createHandler(k *kube.Client, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		obj, err := decode(req.Body)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		var created *unstructured.Unstructured
		if cluster(gvr) {
			created, err = k.Dynamic.Resource(gvr).Create(req.Context(), obj, metav1.CreateOptions{})
		} else {
			ns, ok := resolveNS(w, req)
			if !ok {
				return
			}
			obj.SetNamespace(ns)
			created, err = k.Dynamic.Resource(gvr).Namespace(ns).Create(req.Context(), obj, metav1.CreateOptions{})
		}
		writeOrErr(w, req, created, err)
	}
}

func updateHandler(k *kube.Client, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		obj, err := decode(req.Body)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		name := chi.URLParam(req, "name")
		obj.SetName(name)
		if gvr.Resource == "gametemplates" {
			if blocked, err := managedTemplateBlocked(req, k, name); err != nil {
				httperr.Write(w, req, err)
				return
			} else if blocked != "" {
				httperr.WriteCode(w, req, http.StatusConflict, errors.New(blocked))
				return
			}
		}
		var updated *unstructured.Unstructured
		if cluster(gvr) {
			updated, err = k.Dynamic.Resource(gvr).Update(req.Context(), obj, metav1.UpdateOptions{})
		} else {
			ns, ok := resolveNS(w, req)
			if !ok {
				return
			}
			obj.SetNamespace(ns)
			updated, err = k.Dynamic.Resource(gvr).Namespace(ns).Update(req.Context(), obj, metav1.UpdateOptions{})
		}
		writeOrErr(w, req, updated, err)
	}
}

func deleteHandler(k *kube.Client, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		if gvr.Resource == "gametemplates" {
			if blocked, err := managedTemplateBlocked(req, k, name); err != nil {
				httperr.Write(w, req, err)
				return
			} else if blocked != "" {
				httperr.WriteCode(w, req, http.StatusConflict, errors.New(blocked))
				return
			}
		}
		var err error
		if cluster(gvr) {
			err = k.Dynamic.Resource(gvr).Delete(req.Context(), name, metav1.DeleteOptions{})
		} else {
			ns, ok := resolveNS(w, req)
			if !ok {
				return
			}
			err = k.Dynamic.Resource(gvr).Namespace(ns).Delete(req.Context(), name, metav1.DeleteOptions{})
		}
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// managedTemplateBlocked returns a non-empty reason when the named
// GameTemplate is managed by a Module (kestrel.gg/managed-by=Module).
// In that case, direct mutations via /templates are refused — the user
// must go through /modules to install/upgrade/uninstall.
func managedTemplateBlocked(req *http.Request, k *kube.Client, name string) (string, error) {
	tmpl, err := k.Dynamic.Resource(kube.GVRs["templates"]).Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil // create / new — guard doesn't apply
		}
		return "", err
	}
	if tmpl.GetLabels()["kestrel.gg/managed-by"] == "Module" {
		modName := tmpl.GetLabels()["kestrel.gg/module-name"]
		if modName == "" {
			modName = name
		}
		return "GameTemplate \"" + name + "\" is managed by Module \"" + modName +
			"\"; mutate via /modules instead", nil
	}
	return "", nil
}

func cluster(gvr schema.GroupVersionResource) bool {
	return gvr.Resource == "gametemplates"
}

// maxBodyBytes caps request body size in JSON decoders. CRD objects
// we accept are far smaller than this; exceeding it indicates abuse.
const maxBodyBytes = 1 << 20 // 1 MiB

func decode(r io.ReadCloser) (*unstructured.Unstructured, error) {
	defer r.Close()
	dec := json.NewDecoder(io.LimitReader(r, maxBodyBytes))
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: raw}, nil
}

func writeOrErr(w http.ResponseWriter, req *http.Request, v any, err error) {
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	writeJSON(w, v)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
