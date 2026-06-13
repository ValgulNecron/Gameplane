package handlers

import "net/http"

type loginProvider struct {
	Kind  string `json:"kind"`  // "local" | "oidc"
	Label string `json:"label"` // button text for the login page
}

type loginProvidersResp struct {
	Providers []loginProvider `json:"providers"`
}

// AuthProvidersHandler serves GET /auth/providers — a public, pre-auth
// endpoint listing the enabled login methods and their display labels so
// the login page only offers providers that actually work (the OIDC
// button previously rendered unconditionally and 404'd on default
// installs).
//
// Per the login-privacy rule this returns ONLY provider kinds and labels:
// no version, cluster name, host, issuer URL, or counts. The OIDC label
// is admin-configured (KESTREL_OIDC_DISPLAY_NAME); it never derives from
// the issuer URL, which would leak a hostname.
func AuthProvidersHandler(oidcEnabled bool, oidcLabel string) http.HandlerFunc {
	resp := loginProvidersResp{
		Providers: []loginProvider{{Kind: "local", Label: "Local account"}},
	}
	if oidcEnabled {
		if oidcLabel == "" {
			oidcLabel = "Single sign-on"
		}
		resp.Providers = append(resp.Providers, loginProvider{Kind: "oidc", Label: oidcLabel})
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, resp)
	}
}
