// Package handlers implements the REST surface of the API. The design
// deliberately sticks close to the Kubernetes API shape — the dashboard
// talks to the API, the API to the dynamic client, and the dynamic
// client to Gameplane CRDs. No intermediate DTOs.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
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

// rejectRemoteCluster 404s a request carrying a non-local `?cluster=`
// selector, for handlers that hold a bare *kube.Client (the LOCAL/home
// cluster only) instead of a cluster-dispatch *kube.Registry — MountModIDs
// and MountModUpdates. rbac.Middleware (api/internal/rbac/rbac.go)
// authorizes namespaced permissions against whatever `?cluster=` the caller
// supplies; without this guard, a user bound only to a registered REMOTE
// cluster could pass `?cluster=<remote>` to satisfy that check while still
// reaching the LOCAL cluster's same-named GameServer here. A non-local
// selector 404s (not 400/403): these handlers have no notion of "that
// cluster" to even be forbidden from. See ws.rejectRemoteCluster for the
// WS-side twin of this guard. Returns true if the request was rejected
// (caller must stop processing).
func rejectRemoteCluster(w http.ResponseWriter, req *http.Request) bool {
	if c := strings.TrimSpace(req.URL.Query().Get("cluster")); c != "" && c != scope.DefaultCluster {
		http.NotFound(w, req)
		return true
	}
	return false
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
			if gvr.Resource == "gameservers" {
				cl := req.URL.Query().Get("cluster")
				if cl == "" {
					cl = scope.DefaultCluster
				}
				if err := validateAndProtectGameServer(req.Context(), k, cl, ns, obj.GetName(), obj, nil, req); err != nil {
					httperr.WriteCode(w, req, http.StatusForbidden, err)
					return
				}
			}
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
		// so clients can't mutate them via PUT, and validate sensitive spec fields.
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
			cl := req.URL.Query().Get("cluster")
			if cl == "" {
				cl = scope.DefaultCluster
			}
			if err := validateAndProtectGameServer(req.Context(), k, cl, ns, name, obj, live, req); err != nil {
				httperr.WriteCode(w, req, http.StatusForbidden, err)
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
	defer func() { _ = r.Close() }()
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

// validateAndProtectGameServer enforces RBAC and security boundaries on
// GameServer spec fields during create and update mutations:
//   - spec.capture: Requires captures:manage permission. Non-admins cannot enable,
//     modify, or configure capture settings.
//   - spec.serviceAccountName: Requires admin privileges. Non-admins cannot set or override
//     the ServiceAccount.
//   - spec.image: Requires admin privileges. Non-admins choose versions from GameTemplate
//     catalog versions instead of supplying arbitrary container images.
//   - spec.env: Non-admins cannot reference arbitrary Secrets (such as backup destination
//     credentials or auth secrets) via valueFrom.secretKeyRef. Secret references are permitted
//     only to server-owned Secrets (e.g. matching OwnerReference, gameplane.local/server-name
//     label, or canonical server credential names).
func validateAndProtectGameServer(
	ctx context.Context,
	k *kube.Client,
	cl, ns, name string,
	desired *unstructured.Unstructured,
	live *unstructured.Unstructured,
	req *http.Request,
) error {
	u := auth.UserFromContext(ctx)
	isAdmin := false
	canManageCaptures := false
	if u != nil {
		isAdmin = u.Role == "admin" || u.Can("*", false, cl, ns)
		canManageCaptures = isAdmin || u.Can("captures:manage", true, cl, ns)
	}

	// 1. Validate spec.capture
	captureRaw, hasCapture, _ := unstructured.NestedFieldNoCopy(desired.Object, "spec", "capture")
	if !canManageCaptures {
		if live == nil {
			if hasCapture && captureRaw != nil {
				return errors.New("configuring capture requires captures:manage permission")
			}
		} else {
			liveCaptureRaw, liveHasCapture, _ := unstructured.NestedFieldNoCopy(live.Object, "spec", "capture")
			if hasCapture && captureRaw != nil {
				if !liveHasCapture || !reflect.DeepEqual(captureRaw, liveCaptureRaw) {
					return errors.New("modifying capture settings requires captures:manage permission")
				}
			} else if liveHasCapture && liveCaptureRaw != nil {
				_ = unstructured.SetNestedField(desired.Object, liveCaptureRaw, "spec", "capture")
			}
		}
	}

	// 2. Validate spec.serviceAccountName
	sa, hasSA, _ := unstructured.NestedString(desired.Object, "spec", "serviceAccountName")
	if !isAdmin {
		if live == nil {
			if hasSA && sa != "" {
				return errors.New("spec.serviceAccountName override requires admin privileges")
			}
		} else {
			liveSA, liveHasSA, _ := unstructured.NestedString(live.Object, "spec", "serviceAccountName")
			if hasSA && sa != "" {
				if !liveHasSA || sa != liveSA {
					return errors.New("spec.serviceAccountName override requires admin privileges")
				}
			} else if liveHasSA && liveSA != "" {
				_ = unstructured.SetNestedField(desired.Object, liveSA, "spec", "serviceAccountName")
			}
		}
	}

	// 3. Validate spec.image
	img, hasImg, _ := unstructured.NestedString(desired.Object, "spec", "image")
	if !isAdmin {
		if live == nil {
			if hasImg && img != "" {
				return errors.New("spec.image override requires admin privileges")
			}
		} else {
			liveImg, liveHasImg, _ := unstructured.NestedString(live.Object, "spec", "image")
			if hasImg && img != "" {
				if !liveHasImg || img != liveImg {
					return errors.New("spec.image override requires admin privileges")
				}
			} else if liveHasImg && liveImg != "" {
				_ = unstructured.SetNestedField(desired.Object, liveImg, "spec", "image")
			}
		}
	}

	// 4. Validate spec.env for unowned secret/configmap references
	if !isAdmin {
		envSlice, hasEnv, _ := unstructured.NestedSlice(desired.Object, "spec", "env")
		if hasEnv {
			for _, item := range envSlice {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				valFromRaw, ok := itemMap["valueFrom"]
				if !ok || valFromRaw == nil {
					continue
				}
				valFrom, ok := valFromRaw.(map[string]any)
				if !ok {
					continue
				}
				if secRefRaw, ok := valFrom["secretKeyRef"]; ok && secRefRaw != nil {
					if secRef, ok := secRefRaw.(map[string]any); ok {
						secName, _ := secRef["name"].(string)
						if secName == "" {
							return errors.New("secretKeyRef name must not be empty")
						}
						var gsUID types.UID
						if live != nil {
							gsUID = live.GetUID()
						}
						if !isServerOwnedSecret(ctx, k, ns, name, gsUID, secName) {
							return fmt.Errorf("secret reference %q in env is not permitted: not a server-owned secret", secName)
						}
					}
				}
				if cmRefRaw, ok := valFrom["configMapKeyRef"]; ok && cmRefRaw != nil {
					if cmRef, ok := cmRefRaw.(map[string]any); ok {
						cmName, _ := cmRef["name"].(string)
						if cmName == "" {
							return errors.New("configMapKeyRef name must not be empty")
						}
						var gsUID types.UID
						if live != nil {
							gsUID = live.GetUID()
						}
						if !isServerOwnedConfigMap(ctx, k, ns, name, gsUID, cmName) {
							return fmt.Errorf("configMap reference %q in env is not permitted: not a server-owned configMap", cmName)
						}
					}
				}
			}
		}
	}

	// 5. Validate spec.networking.tunnel.credentialsSecretRef for unowned secret references
	if !isAdmin {
		tunnelSecName, hasTunnelSec, _ := unstructured.NestedString(desired.Object, "spec", "networking", "tunnel", "credentialsSecretRef", "name")
		if hasTunnelSec && tunnelSecName != "" {
			var gsUID types.UID
			if live != nil {
				gsUID = live.GetUID()
			}
			if !isServerOwnedSecret(ctx, k, ns, name, gsUID, tunnelSecName) {
				return fmt.Errorf("tunnel credentials secret reference %q is not permitted: not a server-owned secret", tunnelSecName)
			}
		}
	}

	return nil
}

func isServerOwnedSecret(ctx context.Context, k *kube.Client, ns, serverName string, gsUID types.UID, secretName string) bool {
	if secretName == "" || serverName == "" {
		return false
	}
	if k != nil && k.Dynamic != nil {
		gvr := schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
		sec, err := k.Dynamic.Resource(gvr).Namespace(ns).Get(ctx, secretName, metav1.GetOptions{})
		if err == nil && sec != nil {
			for _, ref := range sec.GetOwnerReferences() {
				if ref.Kind == "GameServer" && ref.Name == serverName {
					// A create or clone has no live UID yet. An owner reference that
					// names a specific GameServer UID must not be accepted on a name match alone.
					if ref.UID != "" && ref.UID != gsUID {
						continue
					}
					return true
				}
			}
			return false
		}
	}
	return false
}

func isServerOwnedConfigMap(ctx context.Context, k *kube.Client, ns, serverName string, gsUID types.UID, cmName string) bool {
	if cmName == "" || serverName == "" {
		return false
	}
	if k != nil && k.Dynamic != nil {
		gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
		cm, err := k.Dynamic.Resource(gvr).Namespace(ns).Get(ctx, cmName, metav1.GetOptions{})
		if err == nil && cm != nil {
			for _, ref := range cm.GetOwnerReferences() {
				if ref.Kind == "GameServer" && ref.Name == serverName {
					// A create or clone has no live UID yet. An owner reference that
					// names a specific GameServer UID must not be accepted on a name match alone.
					if ref.UID != "" && ref.UID != gsUID {
						continue
					}
					return true
				}
			}
			return false
		}
	}
	return false
}
