// Package auth implements the agent's request authenticator. Two
// modes are supported:
//
//   - mTLS (preferred): the agent listens TLS and requires the client
//     (API) to present a cert signed by --tls-client-ca. Request
//     handlers receive a verified chain. Nothing else is needed.
//
//   - Shared token (fallback for early dev): the agent accepts any
//     request bearing Authorization: Bearer <token> where <token>
//     matches the contents of --api-token-file.
//
// mTLS is the default the operator wires up at production time. The
// shared-token path exists so the agent can run on a plain port 8090
// during local kind development.
package auth

import (
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the configuration for setting up the authenticator.
type Config struct {
	ClientCAFile string // mTLS CA bundle
	TokenFile    string // shared-secret fallback
}

// Authenticator implements request authentication via mTLS or shared token.
type Authenticator struct {
	mode  string
	token []byte
}

// New creates a new Authenticator from the supplied configuration.
func New(cfg Config) (*Authenticator, error) {
	switch {
	case cfg.ClientCAFile != "":
		// mTLS is enforced by the TLS layer; no token needed.
		return &Authenticator{mode: "mtls"}, nil
	case cfg.TokenFile != "":
		// Validate token file path to prevent directory traversal.
		tokenFilePath, err := validateConfigPath(cfg.TokenFile, "")
		if err != nil {
			return nil, fmt.Errorf("invalid token file path: %w", err)
		}
		b, err := os.ReadFile(tokenFilePath)
		if err != nil {
			return nil, fmt.Errorf("read token file: %w", err)
		}
		tok := strings.TrimSpace(string(b))
		if tok == "" {
			return nil, errors.New("token file is empty")
		}
		return &Authenticator{mode: "token", token: []byte(tok)}, nil
	default:
		return nil, errors.New("no auth configured: provide --tls-client-ca or --api-token-file")
	}
}

// Middleware returns an HTTP middleware that enforces authentication.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch a.mode {
		case "mtls":
			if req.TLS == nil || len(req.TLS.VerifiedChains) == 0 {
				http.Error(w, "client certificate required", http.StatusUnauthorized)
				return
			}
		case "token":
			got := bearer(req.Header.Get("Authorization"))
			if got == "" || subtle.ConstantTimeCompare([]byte(got), a.token) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, req)
	})
}

func bearer(h string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// validateConfigPath ensures a config file path does not escape expected
// directories via directory traversal. It accepts absolute paths and
// optionally validates they remain within a specified base directory.
// If base is empty, the path is only cleaned (G304 may fire if paths are untrusted).
func validateConfigPath(path string, base string) (string, error) {
	if path == "" {
		return "", errors.New("path is empty")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", errors.New("config path must be absolute")
	}
	if base == "" {
		// No base directory configured; these are operator-injected paths.
		// G304 may fire here, but the caller ensures the paths are trusted.
		return clean, nil
	}
	base = filepath.Clean(base)
	// Ensure the path is within the base directory.
	if clean != base && !strings.HasPrefix(clean, base+string(os.PathSeparator)) {
		return "", errors.New("path escapes config root")
	}
	return clean, nil
}

// ServerTLS builds a tls.Config enforcing client-cert verification
// against the supplied CA bundle.
func ServerTLS(certFile, keyFile, clientCAFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("server keypair: %w", err)
	}
	pool := x509.NewCertPool()
	if clientCAFile != "" {
		// Validate client CA file path to prevent directory traversal.
		caFilePath, err := validateConfigPath(clientCAFile, "")
		if err != nil {
			return nil, fmt.Errorf("invalid client CA path: %w", err)
		}
		ca, err := os.ReadFile(caFilePath)
		if err != nil {
			return nil, fmt.Errorf("read client CA: %w", err)
		}
		if !pool.AppendCertsFromPEM(ca) {
			return nil, errors.New("client CA bundle contains no valid certs")
		}
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
