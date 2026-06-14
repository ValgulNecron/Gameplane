package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kestrel-gg/kestrel/api/internal/auth"
	"github.com/kestrel-gg/kestrel/api/internal/db"
	"github.com/kestrel-gg/kestrel/api/internal/httperr"
	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

// Owner annotations record which user a GameServer belongs to. Ownership
// is informational (display + transfer + audit), not an access boundary —
// RBAC remains role/namespace based.
const (
	ownerIDAnnotation = "kestrel.gg/owner-id"
	ownerAnnotation   = "kestrel.gg/owner"
)

// stampOwner records the authenticated caller as the owner of obj. Called
// on GameServer creation; overrides any client-supplied owner annotations
// so ownership can't be spoofed.
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
	obj.SetAnnotations(ann)
}

// MountOwnership wires the server ownership-transfer endpoint.
func MountOwnership(r chi.Router, k *kube.Client, store *db.Store) {
	h := &ownershipHandler{k: k, db: store}
	r.Post("/servers/{name}:transfer", h.transfer)
}

type ownershipHandler struct {
	k  *kube.Client
	db *db.Store
}

type transferReq struct {
	UserID int64 `json:"userId"`
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
