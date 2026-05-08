package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kestrel-gg/kestrel/api/internal/httperr"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

// MountLifecycle wires start/stop/restart/clone verbs on GameServers.
//
// start/stop/restart are expressed as patches to spec.suspend (the
// operator reconciles the rest). Restart is a pair of patches with a
// short pause between them; we implement it as a stop+start sequence
// the client can also do manually, but the UI gets a single endpoint.
func MountLifecycle(r chi.Router, k *kube.Client) {
	r.Post("/servers/{name}:start", patchSuspend(k, false))
	r.Post("/servers/{name}:stop", patchSuspend(k, true))
	r.Post("/servers/{name}:restart", restartHandler(k))
	r.Post("/servers/{name}:clone", cloneHandler(k))
}

func patchSuspend(k *kube.Client, suspend bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		ns, ok := resolveNS(w, req)
		if !ok {
			return
		}
		body, _ := json.Marshal(map[string]any{"spec": map[string]any{"suspend": suspend}})
		_, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Patch(req.Context(), name, types.MergePatchType, body, metav1.PatchOptions{})
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func restartHandler(k *kube.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		ns, ok := resolveNS(w, req)
		if !ok {
			return
		}
		stop, _ := json.Marshal(map[string]any{"spec": map[string]any{"suspend": true}})
		start, _ := json.Marshal(map[string]any{"spec": map[string]any{"suspend": false}})

		if _, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Patch(req.Context(), name, types.MergePatchType, stop, metav1.PatchOptions{}); err != nil {
			httperr.Write(w, req, err)
			return
		}
		// Operator waits for pods to fully stop before scaling back up,
		// so no manual sleep is required.
		if _, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Patch(req.Context(), name, types.MergePatchType, start, metav1.PatchOptions{}); err != nil {
			httperr.Write(w, req, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

type cloneReq struct {
	NewName string `json:"newName"`
}

func cloneHandler(k *kube.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		ns, ok := resolveNS(w, req)
		if !ok {
			return
		}
		var body cloneReq
		if err := json.NewDecoder(io.LimitReader(req.Body, 1<<16)).Decode(&body); err != nil || body.NewName == "" {
			http.Error(w, "newName required", http.StatusBadRequest)
			return
		}
		src, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Get(req.Context(), name, metav1.GetOptions{})
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		clone := src.DeepCopy()
		clone.SetName(body.NewName)
		clone.SetResourceVersion("")
		clone.SetUID("")
		unstructured.RemoveNestedField(clone.Object, "status")

		created, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Create(req.Context(), clone, metav1.CreateOptions{})
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		writeOrErr(w, req, created, nil)
	}
}
