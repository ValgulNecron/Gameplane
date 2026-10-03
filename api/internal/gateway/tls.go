package gateway

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// TLSFiles names mounted credentials; no private material belongs in flags.
type TLSFiles struct {
	Certificate string
	Key         string
	CA          string
}

func readPool(path string) (*x509.CertPool, error) {
	// The path comes from mounted-credential configuration, never a request.
	// Normalize it without restricting the administrator's Secret mount location.
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read trust bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(raw) {
		return nil, errors.New("trust bundle has no certificates")
	}
	return pool, nil
}

func checkPeer(state tls.ConnectionState, caFile, peerURI string) error {
	if len(state.PeerCertificates) == 0 {
		return errors.New("client certificate required")
	}
	pool, err := readPool(caFile)
	if err != nil {
		return err
	}
	intermediate := x509.NewCertPool()
	for _, cert := range state.PeerCertificates[1:] {
		intermediate.AddCert(cert)
	}
	leaf := state.PeerCertificates[0]
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: pool, Intermediates: intermediate, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, CurrentTime: time.Now()}); err != nil {
		return fmt.Errorf("verify central peer: %w", err)
	}
	for _, identity := range leaf.URIs {
		if identity.String() == peerURI {
			return nil
		}
	}
	return errors.New("central peer identity not allowed")
}

// ServerTLS reloads mounted trust and key material on each handshake. Requests
// also revalidate the client against current trust, including reused connections.
func ServerTLS(files TLSFiles, peerURI string) (*tls.Config, error) {
	identity, err := url.Parse(peerURI)
	if err != nil || identity.Scheme == "" || identity.Host == "" || identity.User != nil || identity.RawQuery != "" || identity.Fragment != "" {
		return nil, errors.New("peer URI must be an absolute identity URI")
	}
	load := func() (*tls.Config, error) {
		cert, err := tls.LoadX509KeyPair(files.Certificate, files.Key)
		if err != nil {
			return nil, fmt.Errorf("load gateway keypair: %w", err)
		}
		pool, err := readPool(files.CA)
		if err != nil {
			return nil, err
		}
		return &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert}, ClientCAs: pool,
			ClientAuth:             tls.RequireAndVerifyClientCert,
			SessionTicketsDisabled: true,
			VerifyConnection:       func(state tls.ConnectionState) error { return checkPeer(state, files.CA, peerURI) },
		}, nil
	}
	initial, err := load()
	if err != nil {
		return nil, err
	}
	initial.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) { return load() }
	return initial, nil
}

func agentTransport(files TLSFiles) (*httpTransport, error) {
	cert, err := tls.LoadX509KeyPair(files.Certificate, files.Key)
	if err != nil {
		return nil, fmt.Errorf("load agent client keypair: %w", err)
	}
	pool, err := readPool(files.CA)
	if err != nil {
		return nil, err
	}
	return newHTTPTransport(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, RootCAs: pool}), nil
}
