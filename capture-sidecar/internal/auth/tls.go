// Package auth provides mTLS certificate validation for the capture sidecar's
// :9091 control endpoint, reusing the agent's existing per-GameServer
// certificate infrastructure.
package auth

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// ServerTLS builds a tls.Config enforcing client-cert verification
// against the supplied CA bundle. Mirrors agent/internal/auth.ServerTLS.
func ServerTLS(certFile, keyFile, clientCAFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("server keypair: %w", err)
	}
	pool := x509.NewCertPool()
	if clientCAFile != "" {
		clientCAFile = filepath.Clean(clientCAFile)
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

// Middleware returns an HTTP middleware that enforces mTLS validation.
// Rejects requests without a verified client certificate.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.TLS == nil || len(req.TLS.VerifiedChains) == 0 {
			http.Error(w, "client certificate required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}
