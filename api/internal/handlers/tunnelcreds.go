package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// Provider key names as documented in docs/tunnels.md.
var tunnelProviderKeys = map[string][]string{
	"frp":       {"token"},
	"tailscale": {"authKey"},
	"playit":    {"secretKey"},
}

// MountTunnelCredentials wires tunnel credential management endpoints.
// Routes use the :tunnel-credentials verb so parseServerPath correctly
// extracts the server name and verb, and RBAC checks servers:write.
func MountTunnelCredentials(r chi.Router, reg *kube.Registry) {
	h := &tunnelCredsHandler{reg: reg}
	r.Put("/servers/{name}:tunnel-credentials", h.put)
	r.Get("/servers/{name}:tunnel-credentials", h.get)
	r.Delete("/servers/{name}:tunnel-credentials", h.delete)
}

type tunnelCredsHandler struct {
	reg *kube.Registry
}

// putReq is the request body for PUT /servers/{name}:tunnel-credentials.
type putReq struct {
	Provider string            `json:"provider"`
	Values   map[string]string `json:"values"`
}

// getResp is the response body for GET /servers/{name}:tunnel-credentials.
type getResp struct {
	Configured bool     `json:"configured"`
	SecretName string   `json:"secretName"`
	Keys       []string `json:"keys"`
}

// put creates or updates a tunnel credential Secret and patches the GameServer
// to reference it. The Secret is named <server>-tunnel-auth and is owned by
// the GameServer so it is garbage-collected when the server is deleted.
func (h *tunnelCredsHandler) put(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}

	var body putReq
	if err := json.NewDecoder(io.LimitReader(req.Body, 1<<16)).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate provider is known.
	expectedKeys, ok := tunnelProviderKeys[body.Provider]
	if !ok {
		http.Error(w, fmt.Sprintf("unknown provider: %s", body.Provider), http.StatusBadRequest)
		return
	}

	// Validate values match provider's expected keys.
	if err := validateTunnelValues(body.Values, expectedKeys); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Fetch the GameServer to get its UID for the owner reference.
	gs, err := k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Create or update the Secret with the credential values.
	secretName := name + "-tunnel-auth"
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "gameplane.local/v1alpha1",
					Kind:       "GameServer",
					Name:       name,
					UID:        gs.GetUID(),
				},
			},
		},
		StringData: body.Values,
	}

	// Try create first; on AlreadyExists, fall through to a JSON-merge patch.
	// This preserves any extra fields an admin may have set via kubectl and
	// avoids the resourceVersion issue that would fail a blind Update.
	_, err = k.Typed.CoreV1().Secrets(ns).Create(req.Context(), desired, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		httperr.Write(w, req, err)
		return
	}
	if err != nil {
		// Secret already exists; patch it instead to preserve extra fields.
		patch := map[string]any{
			"stringData": body.Values,
		}
		patchBytes, _ := json.Marshal(patch)
		_, err = k.Typed.CoreV1().Secrets(ns).Patch(
			req.Context(), secretName, types.MergePatchType, patchBytes, metav1.PatchOptions{},
		)
		if err != nil {
			httperr.Write(w, req, err)
			return
		}
	}

	// Patch the GameServer to set the credentialsSecretRef.
	patch, _ := json.Marshal(map[string]any{
		"spec": map[string]any{
			"networking": map[string]any{
				"tunnel": map[string]any{
					"credentialsSecretRef": map[string]any{
						"name": secretName,
					},
				},
			},
		},
	})
	_, err = k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Patch(req.Context(), name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// get reports whether tunnel credentials are configured, returning only the
// secret name and expected keys — never the credential values themselves.
func (h *tunnelCredsHandler) get(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}

	// Fetch the GameServer to check if tunnel credentials are set.
	gs, err := k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Check if credentialsSecretRef is set.
	secretRef, ok, err := getNestedString(gs.Object, "spec", "networking", "tunnel", "credentialsSecretRef", "name")
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	if !ok || secretRef == "" {
		// Not configured.
		writeJSON(w, getResp{
			Configured: false,
			SecretName: "",
			Keys:       []string{},
		})
		return
	}

	// Check if the Secret exists and infer the provider from its keys.
	secret, err := k.Typed.CoreV1().Secrets(ns).Get(req.Context(), secretRef, metav1.GetOptions{})
	if err != nil {
		// Secret referenced but doesn't exist (missing or deleted).
		// Report as not configured.
		writeJSON(w, getResp{
			Configured: false,
			SecretName: secretRef,
			Keys:       []string{},
		})
		return
	}

	// Determine the provider and expected keys from the Secret's keys.
	var expectedKeys []string
	for _, keys := range tunnelProviderKeys {
		if hasKeys(secret.Data, keys) {
			expectedKeys = keys
			break
		}
	}

	writeJSON(w, getResp{
		Configured: true,
		SecretName: secretRef,
		Keys:       expectedKeys,
	})
}

// delete removes the tunnel credential Secret and clears the credentialsSecretRef
// from the GameServer. If the tunnel is currently enabled, delete is rejected
// because the CRD's CEL rule requires credentialsSecretRef when enabled.
func (h *tunnelCredsHandler) delete(w http.ResponseWriter, req *http.Request) {
	k, ok := resolveCluster(w, req, h.reg)
	if !ok {
		return
	}
	name := chi.URLParam(req, "name")
	ns, ok := resolveNS(w, req)
	if !ok {
		return
	}

	// Fetch the GameServer to check tunnel state and get the Secret name.
	gs, err := k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Get(req.Context(), name, metav1.GetOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Check if tunnel is enabled; if so, refuse the DELETE.
	tunnelEnabled, ok, err := getNestedBool(gs.Object, "spec", "networking", "tunnel", "enabled")
	if err != nil {
		httperr.Write(w, req, err)
		return
	}
	if ok && tunnelEnabled {
		http.Error(w, "cannot delete tunnel credentials while tunnel is enabled; disable the tunnel first or use PUT to rotate credentials", http.StatusConflict)
		return
	}

	secretRef, ok, err := getNestedString(gs.Object, "spec", "networking", "tunnel", "credentialsSecretRef", "name")
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Patch the GameServer to clear the credentialsSecretRef first, before deleting the Secret.
	// This ensures the spec stays valid according to the CEL rule.
	patch, _ := json.Marshal(map[string]any{
		"spec": map[string]any{
			"networking": map[string]any{
				"tunnel": map[string]any{
					"credentialsSecretRef": nil,
				},
			},
		},
	})
	_, err = k.Dynamic.Resource(kube.GVRs["servers"]).
		Namespace(ns).
		Patch(req.Context(), name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		httperr.Write(w, req, err)
		return
	}

	// Now delete the Secret if it exists. Report the error if deletion fails.
	if ok && secretRef != "" {
		err = k.Typed.CoreV1().Secrets(ns).Delete(req.Context(), secretRef, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// Secret deletion failed and it wasn't just "not found"; report it.
			httperr.Write(w, req, fmt.Errorf("patched spec but failed to delete credential Secret: %w", err))
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// validateTunnelValues checks that the provided values map contains exactly
// the expected keys for the provider, and that all values are non-empty.
func validateTunnelValues(values map[string]string, expectedKeys []string) error {
	if len(values) != len(expectedKeys) {
		return fmt.Errorf("expected %d key(s) for provider, got %d", len(expectedKeys), len(values))
	}

	seen := make(map[string]bool)
	for key, val := range values {
		seen[key] = true
		if val == "" {
			return fmt.Errorf("empty value for key: %s", key)
		}
	}

	for _, expected := range expectedKeys {
		if !seen[expected] {
			return fmt.Errorf("missing required key: %s", expected)
		}
	}

	return nil
}

// hasKeys checks if a map[string][]byte contains all of the specified keys.
func hasKeys(m map[string][]byte, keys []string) bool {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}

// getNestedString retrieves a string value from a nested map using a path of keys.
// Returns (value, found, error). Error is returned if a non-final key points to
// a non-map value.
func getNestedString(obj map[string]any, path ...string) (string, bool, error) {
	var current any = obj
	for i, key := range path {
		if m, ok := current.(map[string]any); ok {
			v, found := m[key]
			if !found {
				return "", false, nil
			}
			if i == len(path)-1 {
				// Final key: convert to string.
				s, ok := v.(string)
				return s, ok, nil
			}
			current = v
		} else {
			return "", false, fmt.Errorf("expected map at %s", strings.Join(path[:i], "."))
		}
	}
	return "", false, nil
}

// getNestedBool retrieves a bool value from a nested map using a path of keys.
// Returns (value, found, error). Error is returned if a non-final key points to
// a non-map value.
func getNestedBool(obj map[string]any, path ...string) (bool, bool, error) {
	var current any = obj
	for i, key := range path {
		if m, ok := current.(map[string]any); ok {
			v, found := m[key]
			if !found {
				return false, false, nil
			}
			if i == len(path)-1 {
				// Final key: convert to bool.
				b, ok := v.(bool)
				return b, ok, nil
			}
			current = v
		} else {
			return false, false, fmt.Errorf("expected map at %s", strings.Join(path[:i], "."))
		}
	}
	return false, false, nil
}
