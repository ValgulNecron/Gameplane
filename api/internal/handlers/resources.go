// Package handlers implements the REST surface of the API. The design
// deliberately sticks close to the Kubernetes API shape — the dashboard
// talks to the API, the API to the dynamic client, and the dynamic
// client to Gameplane CRDs. No intermediate DTOs.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// heartbeatStaleTTL is how long the agent's reported status.agent values
// (playersOnline, gameVersion) are trusted. It matches the operator's
// heartbeatFreshness window. Once a heartbeat is older than this, the
// counts are no longer current — a crashed agent would otherwise show
// its last player count forever — so the API blanks playersOnline and
// flags status.agent.stale on read.
const heartbeatStaleTTL = 60 * time.Second

// MountResources wires /servers, /templates, /backups, /schedules.
//
// Templates are cluster-scoped; the others are namespaced. The kube
// package's GVR map determines which is which via resource path. Every
// handler resolves its target cluster per request from reg via the
// `?cluster=` selector (resolveCluster) — a request with no selector
// resolves to scope.DefaultCluster, preserving single-cluster behavior.
func MountResources(r chi.Router, reg *kube.Registry) {
	for path, gvr := range kube.GVRs {
		mountOne(r, reg, path, gvr)
	}
}

func mountOne(r chi.Router, reg *kube.Registry, path string, gvr schema.GroupVersionResource) {
	r.Route("/"+path, func(r chi.Router) {
		r.Get("/", listHandler(reg, gvr))
		r.Post("/", createHandler(reg, gvr))
		r.Get("/{name}", getHandler(reg, gvr))
		r.Put("/{name}", updateHandler(reg, gvr))
		r.Delete("/{name}", deleteHandler(reg, gvr))
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

// resolveCluster validates the ?cluster= query param and returns the target
// cluster's client. ok=false means a response was already written; stop.
func resolveCluster(w http.ResponseWriter, req *http.Request, reg *kube.Registry) (*kube.Client, bool) {
	id, err := scope.ResolveCluster(req, reg)
	if err != nil {
		httperr.Write(w, req, err)
		return nil, false
	}
	c, ok := reg.Get(id)
	if !ok {
		httperr.Write(w, req, scope.ErrForbiddenCluster)
		return nil, false
	}
	return c, true
}

func listHandler(reg *kube.Registry, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}
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
		if err == nil && list != nil && gvr.Resource == "gameservers" {
			for i := range list.Items {
				gateStaleAgent(&list.Items[i])
			}
		}
		writeOrErr(w, req, list, err)
	}
}

func getHandler(reg *kube.Registry, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}
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
		if err == nil && obj != nil && gvr.Resource == "gameservers" {
			gateStaleAgent(obj)
		}
		writeOrErr(w, req, obj, err)
	}
}

func createHandler(reg *kube.Registry, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}
		obj, err := decode(req.Body)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		// Record the creating user as the server's owner (informational).
		if gvr.Resource == "gameservers" {
			stampOwner(obj, req)
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
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	}
}

func updateHandler(reg *kube.Registry, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}
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
		// For GameServers, preserve ownership annotations from the live object
		// so clients can't mutate them via PUT.
		if gvr.Resource == "gameservers" {
			ns, ok := resolveNS(w, req)
			if !ok {
				return
			}
			live, err := k.Dynamic.Resource(gvr).Namespace(ns).
				Get(req.Context(), name, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				httperr.Write(w, req, err)
				return
			}
			if live != nil {
				// Copy ownership annotations from the live object.
				liveAnn := live.GetAnnotations()
				objAnn := obj.GetAnnotations()
				if objAnn == nil {
					objAnn = map[string]string{}
				}
				for _, key := range []string{
					"gameplane.local/owner-id",
					"gameplane.local/owner",
					"gameplane.local/collaborators",
					"gameplane.local/collaborator-names",
				} {
					if v, ok := liveAnn[key]; ok {
						objAnn[key] = v
					} else {
						delete(objAnn, key)
					}
				}
				obj.SetAnnotations(objAnn)
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

func deleteHandler(reg *kube.Registry, gvr schema.GroupVersionResource) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}
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
// GameTemplate is managed by a Module (gameplane.local/managed-by=Module).
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
	if tmpl.GetLabels()["gameplane.local/managed-by"] == "Module" {
		modName := tmpl.GetLabels()["gameplane.local/module-name"]
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

// gateStaleAgent blanks the agent-reported values on a GameServer whose
// heartbeat has gone stale (or never arrived), so the dashboard renders
// "unknown" instead of a frozen-forever value from a dead agent. It
// mutates obj's status.agent in place: drops playersOnline and the
// resource-usage readings, and sets stale=true. Fresh heartbeats are
// left untouched.
func gateStaleAgent(obj *unstructured.Unstructured) {
	agent, found, err := unstructured.NestedMap(obj.Object, "status", "agent")
	if !found || err != nil || agent == nil {
		return
	}
	fresh := false
	if lh, ok := agent["lastHeartbeat"].(string); ok && lh != "" {
		if t, perr := time.Parse(time.RFC3339, lh); perr == nil {
			fresh = time.Since(t) < heartbeatStaleTTL
		}
	}
	if fresh {
		return
	}
	agent["stale"] = true
	for _, k := range []string{
		"playersOnline",
		"cpuMillicores", "cpuLimitMillicores",
		"memoryBytes", "memoryLimitBytes",
		"diskUsedBytes", "diskTotalBytes",
	} {
		delete(agent, k)
	}
	_ = unstructured.SetNestedMap(obj.Object, agent, "status", "agent")
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
