package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// Owner annotations record which user a GameServer belongs to. Ownership
// is informational (display + transfer + audit), and can grant per-server
// access to the owner and collaborators.
const (
	ownerIDAnnotation           = "gameplane.local/owner-id"
	ownerAnnotation             = "gameplane.local/owner"
	collaboratorsAnnotation     = "gameplane.local/collaborators"
	collaboratorNamesAnnotation = "gameplane.local/collaborator-names"
)

// stampOwner records the authenticated caller as the owner of obj. Called
// on GameServer creation; overrides any client-supplied owner annotations
// so ownership can't be spoofed. Also strips client-supplied collaborator
// annotations to prevent spoofing the collaborator list.
func stampOwner(obj *unstructured.Unstructured, req *http.Request) {
	u := auth.UserFromContext(req.Context())
	if u == nil {
		return
	}
	ann := obj.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[ownerIDAnnotation] = strconv.FormatInt(u.ID, 10)
	ann[ownerAnnotation] = u.Username
	// Strip client-supplied collaborator annotations.
	delete(ann, collaboratorsAnnotation)
	delete(ann, collaboratorNamesAnnotation)
	obj.SetAnnotations(ann)
}

// MountOwnership wires the server ownership and collaborator endpoints.
func MountOwnership(r chi.Router, k *kube.Client, store *db.Store) {
	h := &ownershipHandler{k: k, db: store}
	r.Post("/servers/{name}:transfer", h.transfer)
	r.Put("/servers/{name}:collaborators", h.setCollaborators)
	r.Get("/users/me/servers", h.getOwnedServers)
}

type ownershipHandler struct {
	k  *kube.Client
	db *db.Store
}

type transferReq struct {
	UserID int64 `json:"userId"`
}

type setCollaboratorsReq struct {
	UserIDs   []int64  `json:"userIds"`
	Usernames []string `json:"usernames"`
}

// transfer reassigns a server's owner annotations to another user after
// validating the target exists. The audit middleware records the actor.
func (h *ownershipHandler) transfer(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	var body transferReq
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<16)).Decode(&body); err != nil || body.UserID <= 0 {
		http.Error(w, "userId required", http.StatusBadRequest)
		return
	}
	var username string
	err := h.db.DB.QueryRowContext(req.Context(),
		`SELECT username FROM users WHERE id = ?`, body.UserID).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	patch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				ownerIDAnnotation: strconv.FormatInt(body.UserID, 10),
				ownerAnnotation:   username,
			},
		},
	})
	if _, err := h.k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Patch(req.Context(), name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		httperr.Write(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setCollaborators updates the collaborators list for a GameServer.
// Validates all user IDs exist, dedupes, drops the owner ID, and
// merges both ID and name annotations. Empty list clears both.
func (h *ownershipHandler) setCollaborators(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}
	var body setCollaboratorsReq
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<16)).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Get the server to check ownership and access current state.
	obj, err := h.k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Build a set of provided user IDs to resolve all usernames.
	seenIDs := make(map[int64]struct{})
	for _, id := range body.UserIDs {
		seenIDs[id] = struct{}{}
	}
	for _, name := range body.Usernames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var id int64
		err := h.db.DB.QueryRowContext(req.Context(),
			`SELECT id FROM users WHERE username = ?`, name).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found: "+name, http.StatusBadRequest)
			return
		}
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		seenIDs[id] = struct{}{}
	}

	// Validate all IDs exist, resolve usernames, and build a map.
	idToName := make(map[int64]string)
	finalIDs := make([]int64, 0, len(seenIDs))
	for id := range seenIDs {
		var username string
		err := h.db.DB.QueryRowContext(req.Context(),
			`SELECT username FROM users WHERE id = ?`, id).Scan(&username)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "user not found", http.StatusBadRequest)
			return
		}
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		// Skip the owner ID.
		ownerIDStr := obj.GetAnnotations()["gameplane.local/owner-id"]
		if ownerIDStr != "" && ownerIDStr == strconv.FormatInt(id, 10) {
			continue
		}
		finalIDs = append(finalIDs, id)
		idToName[id] = username
	}

	// Sort IDs numerically ascending and build annotations with aligned names.
	sort.Slice(finalIDs, func(i, j int) bool { return finalIDs[i] < finalIDs[j] })

	collabIDsStr := ""
	collabNamesStr := ""
	if len(finalIDs) > 0 {
		idStrs := make([]string, len(finalIDs))
		nameStrs := make([]string, len(finalIDs))
		for i, id := range finalIDs {
			idStrs[i] = strconv.FormatInt(id, 10)
			nameStrs[i] = idToName[id]
		}
		collabIDsStr = strings.Join(idStrs, ",")
		collabNamesStr = strings.Join(nameStrs, ",")
	}

	// Patch the server.
	patch, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				collaboratorsAnnotation:     collabIDsStr,
				collaboratorNamesAnnotation: collabNamesStr,
			},
		},
	})
	if _, err := h.k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Patch(req.Context(), name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		httperr.Write(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getOwnedServers returns GameServers where the caller is owner or collaborator.
func (h *ownershipHandler) getOwnedServers(w http.ResponseWriter, req *http.Request) {
	u := auth.UserFromContext(req.Context())
	if u == nil {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}

	// List all servers across all namespaces.
	list, err := h.k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace("").
		List(req.Context(), metav1.ListOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Filter to owned/collaborated servers.
	filtered := &unstructured.UnstructuredList{
		Object: list.Object,
		Items:  make([]unstructured.Unstructured, 0, len(list.Items)),
	}
	for i := range list.Items {
		ann := list.Items[i].GetAnnotations()
		if ann == nil {
			continue
		}
		// Check if caller is owner.
		if ownerIDStr := ann["gameplane.local/owner-id"]; ownerIDStr != "" {
			if ownerIDStr == strconv.FormatInt(u.ID, 10) {
				gateStaleAgent(&list.Items[i])
				filtered.Items = append(filtered.Items, list.Items[i])
				continue
			}
		}
		// Check if caller is in collaborators list.
		if collabsStr := ann["gameplane.local/collaborators"]; collabsStr != "" {
			userIDStr := strconv.FormatInt(u.ID, 10)
			for _, id := range strings.Split(collabsStr, ",") {
				id = strings.TrimSpace(id)
				if id == userIDStr {
					gateStaleAgent(&list.Items[i])
					filtered.Items = append(filtered.Items, list.Items[i])
					break
				}
			}
		}
	}

	writeJSON(w, filtered)
}
