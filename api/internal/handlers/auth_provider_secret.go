package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
	"github.com/ValgulNecron/gameplane/api/internal/httperr"
	"github.com/ValgulNecron/gameplane/api/internal/kube"
)

// providerSecretPrefix mirrors the registry's default configRef naming
// (auth.Registry resolves an empty configRef to this prefix + name).
const providerSecretPrefix = "gameplane-auth-"

// MountAuthProviderSecrets exposes the managed clientSecret Secret behind
// an identity provider: the Add-provider form PUTs the value here, then
// saves the auth config row referencing it. Same labelled-Secret contract
// as notification sinks, under the auth-provider feature label.
func MountAuthProviderSecrets(r chi.Router, k *kube.Client, controlNS string) {
	h := authProviderSecretHandler{k: k, ns: controlNS}
	r.Put("/admin/auth/providers/{name}/secret", h.put)
	r.Delete("/admin/auth/providers/{name}/secret", h.del)
}

type authProviderSecretHandler struct {
	k  *kube.Client
	ns string
}

func (h authProviderSecretHandler) secretName(w http.ResponseWriter, name string) (string, bool) {
	if !nameRE.MatchString(name) || name == auth.HelmProviderName {
		http.Error(w, "provider name must be a lowercase DNS label (and not the reserved \"helm\")", http.StatusUnprocessableEntity)
		return "", false
	}
	secretName := providerSecretPrefix + name
	if len(secretName) > 63 {
		http.Error(w, fmt.Sprintf("provider name too long: %q must fit a DNS label with the %q prefix", name, providerSecretPrefix), http.StatusUnprocessableEntity)
		return "", false
	}
	return secretName, true
}

func (h authProviderSecretHandler) put(w http.ResponseWriter, req *http.Request) {
	secretName, ok := h.secretName(w, chi.URLParam(req, "name"))
	if !ok {
		return
	}
	var body struct {
		ClientSecret string `json:"clientSecret"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.ClientSecret == "" {
		http.Error(w, "clientSecret is required", http.StatusUnprocessableEntity)
		return
	}
	data := map[string]string{"clientSecret": body.ClientSecret}
	if err := upsertLabelledSecret(req.Context(), h.k, h.ns, secretName, auth.ProviderSecretLabel, data); err != nil {
		if errors.Is(err, errNotManagedSecret) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		httperr.Write(w, req, err)
		return
	}
	// Never echo the value — just the ref the config row should carry.
	writeJSON(w, map[string]any{"name": secretName, "keys": []string{"clientSecret"}})
}

func (h authProviderSecretHandler) del(w http.ResponseWriter, req *http.Request) {
	secretName, ok := h.secretName(w, chi.URLParam(req, "name"))
	if !ok {
		return
	}
	if err := deleteManagedSecret(req.Context(), h.k, h.ns, secretName, auth.ProviderSecretLabel); err != nil {
		httperr.Write(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
