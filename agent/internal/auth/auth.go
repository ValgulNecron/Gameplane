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
	"strings"
)

type Config struct {
	ClientCAFile string // mTLS CA bundle
	TokenFile    string // shared-secret fallback
}

type Authenticator struct {
	mode  string
	token []byte
}

func New(cfg Config) (*Authenticator, error) {
	switch {
	case cfg.ClientCAFile != "":
		// mTLS is enforced by the TLS layer; no token needed.
		return &Authenticator{mode: "mtls"}, nil
	case cfg.TokenFile != "":
		b, err := os.ReadFile(cfg.TokenFile)
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

// ServerTLS builds a tls.Config enforcing client-cert verification
// against the supplied CA bundle.
func ServerTLS(certFile, keyFile, clientCAFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("server keypair: %w", err)
	}
	pool := x509.NewCertPool()
	if clientCAFile != "" {
		ca, err := os.ReadFile(clientCAFile)
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
