package handlers

import (
	"net/http"

	"github.com/ValgulNecron/gameplane/api/internal/auth"
)

type loginProvider struct {
	Name  string `json:"name"`  // route slug for /auth/oidc/{name}/start
	Kind  string `json:"kind"`  // "local" | "oidc" | "google" | "github"
	Label string `json:"label"` // button text for the login page
}

type loginProvidersResp struct {
	Providers []loginProvider `json:"providers"`
}

// AuthProvidersHandler serves GET /auth/providers — a public, pre-auth
// endpoint listing the enabled login methods so the login page only
// offers providers that actually work. Resolved through the registry per
// request, so a config save shows up without a restart.
//
// Per the login-privacy rule this returns ONLY names, kinds, and labels:
// no version, cluster name, host, issuer URL, client ID, or counts.
// Labels are admin-configured display names (or neutral fallbacks) and
// never derive from the issuer URL, which would leak a hostname.
func AuthProvidersHandler(reg *auth.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		enabled := reg.Enabled(req.Context())
		resp := loginProvidersResp{Providers: make([]loginProvider, 0, len(enabled))}
		for _, p := range enabled {
			resp.Providers = append(resp.Providers, loginProvider{
				Name:  p.Name,
				Kind:  p.Kind,
				Label: p.Label(),
			})
		}
		writeJSON(w, resp)
	}
}
