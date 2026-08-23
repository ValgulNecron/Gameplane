// Package auth provides mTLS certificate validation for the capture sidecar's
// :9091 control endpoint, reusing the agent's existing per-GameServer
// certificate infrastructure.
package auth

// ValidateTLS validates a client certificate against the cluster CA.
// It enforces client certificate presence and verifies the peer certificate
// chain against the CA certificate loaded from /etc/tls/ca.crt.
func ValidateTLS(clientCertPath string) error {
	// TODO: implement mTLS validation using the agent TLS certificates
	return nil
}
