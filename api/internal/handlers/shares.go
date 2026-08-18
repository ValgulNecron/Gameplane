package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/db"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// MountShareLinks wires authenticated share link operations (create, list, revoke).
// Must be mounted inside the protected group with auth + rbac middleware.
// Ownership is verified per-request; only the server owner can manage shares.
func MountShareLinks(r chi.Router, reg *kube.Registry, store *db.Store) {
	r.Route("/servers/{name}:shares", func(r chi.Router) {
		r.Post("/", createShareHandler(reg, store))
		r.Get("/", listSharesHandler(reg, store))
	})
	r.Delete("/servers/{name}/shares/{id}", revokeShareHandler(reg, store))
}

// MountPublicShares wires unauthenticated share link resolution and start.
// Must be mounted at the root level with only the share rate limiter.
// Token is a path segment to keep it out of Referer headers and browser history;
// audit redaction masks it in recorded paths.
func MountPublicShares(r chi.Router, reg *kube.Registry, store *db.Store, shareLimiter *auth.TokenBucket) {
	r.Route("/shares/{token}", func(r chi.Router) {
		r.With(shareLimiter.Middleware).Get("/", resolveShareHandler(reg, store))
		r.With(shareLimiter.Middleware).Post("/start", startShareHandler(reg, store))
	})
}

type createShareReq struct {
	ExpiresIn *string `json:"expiresIn"` // e.g. "24h", "7d"; nil = default
	CanStart  bool    `json:"canStart"`
}

type shareResp struct {
	ID        string `json:"id"`
	CreatedAt string `json:"createdAt"`
	ExpiresAt string `json:"expiresAt"`
	CanStart  bool   `json:"canStart"`
	Token     string `json:"token,omitempty"` // only in create response
}

// createShareHandler stamps a new share link for the server. Only the owner can
// do this. The token is returned *once* in the response; list operations never
// include it.
func createShareHandler(reg *kube.Registry, store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		u := auth.UserFromContext(req.Context())
		if u == nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		name := chi.URLParam(req, "name")
		ns, ok := resolveNS(w, req)
		if !ok {
			return
		}
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}

		// Verify ownership before allowing share creation.
		obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Get(req.Context(), name, metav1.GetOptions{})
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		if !isServerOwner(obj, u.ID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		var body createShareReq
		if err := json.NewDecoder(io.LimitReader(req.Body, 1<<16)).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Parse expiry: default 7 days, max 90 days.
		expiresAt := time.Now().UTC().AddDate(0, 0, 7)
		if body.ExpiresIn != nil && *body.ExpiresIn != "" {
			d, err := time.ParseDuration(*body.ExpiresIn)
			if err != nil {
				http.Error(w, "invalid expiresIn", http.StatusBadRequest)
				return
			}
			expiresAt = time.Now().UTC().Add(d)
		}
		// Cap at 90 days.
		maxExpiry := time.Now().UTC().AddDate(0, 0, 90)
		if expiresAt.After(maxExpiry) {
			expiresAt = maxExpiry
		}

		rawToken, link, err := store.CreateShareLink(req.Context(), ns, name, u.ID, body.CanStart, expiresAt)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}

		resp := shareResp{
			ID:        link.ID,
			CreatedAt: link.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: link.ExpiresAt.UTC().Format(time.RFC3339),
			CanStart:  link.CanStart,
			Token:     rawToken, // only in create response
		}
		writeOrErr(w, req, resp, nil)
	}
}

// listSharesHandler lists all share links for a server (owner-only).
// The returned list never includes the raw token, only ID + metadata.
func listSharesHandler(reg *kube.Registry, store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		u := auth.UserFromContext(req.Context())
		if u == nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		name := chi.URLParam(req, "name")
		ns, ok := resolveNS(w, req)
		if !ok {
			return
		}
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}

		// Verify ownership.
		obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Get(req.Context(), name, metav1.GetOptions{})
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		if !isServerOwner(obj, u.ID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		links, err := store.ListShareLinks(req.Context(), ns, name)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}

		// Convert to response format, excluding tokens.
		resps := make([]shareResp, len(links))
		for i, link := range links {
			resps[i] = shareResp{
				ID:        link.ID,
				CreatedAt: link.CreatedAt.UTC().Format(time.RFC3339),
				ExpiresAt: link.ExpiresAt.UTC().Format(time.RFC3339),
				CanStart:  link.CanStart,
				// Token deliberately omitted in list response
			}
		}
		writeOrErr(w, req, resps, nil)
	}
}

// revokeShareHandler revokes a share link by ID (owner-only).
func revokeShareHandler(reg *kube.Registry, store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		u := auth.UserFromContext(req.Context())
		if u == nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		name := chi.URLParam(req, "name")
		id := chi.URLParam(req, "id")
		ns, ok := resolveNS(w, req)
		if !ok {
			return
		}
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}

		// Verify ownership.
		obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(ns).
			Get(req.Context(), name, metav1.GetOptions{})
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
		if !isServerOwner(obj, u.ID) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		if err := store.RevokeShareLink(req.Context(), id); err != nil {
			httperr.Write(w, req, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type sharePublicResp struct {
	ServerName    string     `json:"serverName"`
	Status        string     `json:"status"`
	Address       *shareAddr `json:"address,omitempty"`
	PlayersOnline *int32     `json:"playersOnline,omitempty"`
}

type shareAddr struct {
	Host string `json:"host"`
	Port int32  `json:"port"`
}

// resolveShareHandler looks up a share link by token and returns a minimal
// public view of the server (no namespace, cluster, owner, version, etc.).
// Token is a path segment; audit redaction masks it in the recorded path.
// All failures return 404 with identical body so probing reveals nothing.
func resolveShareHandler(reg *kube.Registry, store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		token := chi.URLParam(req, "token")
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		link, err := store.LookupShareLink(req.Context(), token)
		if err != nil {
			if errors.Is(err, db.ErrShareLinkInvalid) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
				return
			}
			httperr.Write(w, req, err)
			return
		}

		// Touch LastUsed and fetch the server.
		_ = store.TouchShareLink(req.Context(), link.ID)

		// Resolve to the cluster. The share is scoped to a namespace; in
		// multi-cluster setups, finding the cluster owning that namespace
		// is a future enhancement. For now, share links are cluster-scoped
		// and always resolve to the local cluster.
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}

		obj, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(link.Namespace).
			Get(req.Context(), link.ServerName, metav1.GetOptions{})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		// Extract public view: status, address, player count.
		status := getPhase(obj)
		address := getPublicAddress(obj)
		players := getPlayersOnline(obj)

		resp := sharePublicResp{
			ServerName:    link.ServerName,
			Status:        status,
			Address:       address,
			PlayersOnline: players,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// startShareHandler wakes a sleeping server if the share link has canStart=true.
// Token is a path segment; audit redaction masks it in the recorded path.
// If canStart is false, returns 404 (not 403).
func startShareHandler(reg *kube.Registry, store *db.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		token := chi.URLParam(req, "token")
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		link, err := store.LookupShareLink(req.Context(), token)
		if err != nil {
			if errors.Is(err, db.ErrShareLinkInvalid) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
				return
			}
			httperr.Write(w, req, err)
			return
		}

		// Verify canStart; fail with 404 (not 403) to avoid confirming the link exists.
		if !link.CanStart {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		// Touch LastUsed.
		_ = store.TouchShareLink(req.Context(), link.ID)

		// Wake the server (same pattern as wakeHandler).
		k, ok := resolveCluster(w, req, reg)
		if !ok {
			return
		}

		_, err = k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(link.Namespace).
			Get(req.Context(), link.ServerName, metav1.GetOptions{})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}

		// Stamp the wake annotation (same as wakeHandler).
		wakeToken := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
		patch, _ := json.Marshal(map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{idleWakeRequestedAnnotation: wakeToken},
			},
		})
		if _, err := k.Dynamic.Resource(kube.GVRs["servers"]).
			Namespace(link.Namespace).
			Patch(req.Context(), link.ServerName, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
			httperr.Write(w, req, err)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

// getPhase extracts the server's phase (status.phase).
func getPhase(obj *unstructured.Unstructured) string {
	phase, ok, err := unstructured.NestedString(obj.Object, "status", "phase")
	if err != nil || !ok {
		return "Unknown"
	}
	return phase
}

// getPlayersOnline extracts player count from status.agent.playersOnline if present.
func getPlayersOnline(obj *unstructured.Unstructured) *int32 {
	val, ok, err := unstructured.NestedFieldNoCopy(obj.Object, "status", "agent", "playersOnline")
	if err != nil || !ok {
		return nil
	}
	// NestedFieldNoCopy returns interface{}, could be float64 (JSON number) or int64.
	switch v := val.(type) {
	case float64:
		if v < 0 || v > 2147483647 {
			return nil
		}
		n := int32(v)
		return &n
	case int64:
		if v < 0 || v > 2147483647 {
			return nil
		}
		n := int32(v)
		return &n
	case int:
		if v < 0 || v > 2147483647 {
			return nil
		}
		n := int32(v)
		return &n
	}
	return nil
}

// getPublicAddress returns the first publicly reachable (non-private) endpoint
// that is served by a tunnel provider. If no tunnel endpoint, falls back to the
// first non-private regular endpoint. Returns nil if no suitable endpoint exists.
func getPublicAddress(obj *unstructured.Unstructured) *shareAddr {
	endpoints, ok, err := unstructured.NestedSlice(obj.Object, "status", "endpoints")
	if err != nil || !ok || len(endpoints) == 0 {
		return nil
	}

	// First pass: prefer tunnel endpoints that are not private.
	for _, ep := range endpoints {
		epMap, ok := ep.(map[string]interface{})
		if !ok {
			continue
		}
		provider, _, _ := unstructured.NestedString(epMap, "tunnelProvider")
		if provider == "" {
			continue
		}
		private, _, _ := unstructured.NestedBool(epMap, "private")
		if private {
			continue
		}
		host, _, _ := unstructured.NestedString(epMap, "host")
		port := nestedPort(epMap)
		if host != "" && port > 0 {
			return &shareAddr{Host: host, Port: port}
		}
	}

	// Second pass: fall back to any non-private endpoint (tunnel or service).
	for _, ep := range endpoints {
		epMap, ok := ep.(map[string]interface{})
		if !ok {
			continue
		}
		private, _, _ := unstructured.NestedBool(epMap, "private")
		if private {
			continue
		}
		host, _, _ := unstructured.NestedString(epMap, "host")
		port := nestedPort(epMap)
		if host != "" && port > 0 {
			return &shareAddr{Host: host, Port: port}
		}
	}

	return nil
}

// nestedPort extracts the "port" field from an endpoint map as an int32.
// unstructured.NestedInt64 only accepts a value already typed as int64, but
// endpoint maps are not guaranteed to come from that exact path: objects
// decoded from raw JSON (encoding/json into map[string]interface{}) yield
// float64 for every number, and objects built directly in Go (e.g. test
// fixtures) may use a plain int. Handle all three so callers get a port
// regardless of how the unstructured object was constructed. Valid ports are
// 0-65535; invalid values return 0.
func nestedPort(epMap map[string]interface{}) int32 {
	switch v := epMap["port"].(type) {
	case int64:
		if v < 0 || v > 65535 {
			return 0
		}
		return int32(v)
	case float64:
		if v < 0 || v > 65535 {
			return 0
		}
		return int32(v)
	case int32:
		if v < 0 || v > 65535 {
			return 0
		}
		return v
	case int:
		if v < 0 || v > 65535 {
			return 0
		}
		return int32(v)
	}
	return 0
}

// isServerOwner checks if a user is the owner of a GameServer by examining the
// gameplane.local/owner-id annotation. This duplicates logic from rbac.ownershipRole
// to avoid circular imports; it only checks ownership, not collaborator status.
func isServerOwner(obj *unstructured.Unstructured, userID int64) bool {
	ann := obj.GetAnnotations()
	if ann == nil {
		return false
	}
	ownerIDStr := ann["gameplane.local/owner-id"]
	if ownerIDStr == "" {
		return false
	}
	ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
	if err != nil {
		return false
	}
	return ownerID == userID
}
