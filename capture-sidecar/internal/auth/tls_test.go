package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerTLS(t *testing.T) {
	ca := newTestCA(t)
	certPath, keyPath := ca.issue(t, "capture-sidecar")

	t.Run("success without a client CA bundle", func(t *testing.T) {
		cfg, err := ServerTLS(certPath, keyPath, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Errorf("ClientAuth = %v, want RequireAndVerifyClientCert", cfg.ClientAuth)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tls.VersionTLS12)
		}
		if len(cfg.Certificates) != 1 {
			t.Errorf("Certificates = %d, want 1", len(cfg.Certificates))
		}
	})

	t.Run("success with a client CA bundle", func(t *testing.T) {
		cfg, err := ServerTLS(certPath, keyPath, ca.certPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientCAs == nil {
			t.Fatal("ClientCAs is nil; the CA bundle was not loaded")
		}
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Errorf("ClientAuth = %v, want RequireAndVerifyClientCert", cfg.ClientAuth)
		}
	})

	t.Run("missing keypair", func(t *testing.T) {
		_, err := ServerTLS(filepath.Join(t.TempDir(), "nope.crt"), filepath.Join(t.TempDir(), "nope.key"), "")
		if err == nil || !strings.Contains(err.Error(), "server keypair") {
			t.Fatalf("err = %v, want a server keypair error", err)
		}
	})

	t.Run("malformed keypair", func(t *testing.T) {
		dir := t.TempDir()
		badCert := filepath.Join(dir, "bad.crt")
		badKey := filepath.Join(dir, "bad.key")
		writeFile(t, badCert, "not a certificate")
		writeFile(t, badKey, "not a key")
		_, err := ServerTLS(badCert, badKey, "")
		if err == nil || !strings.Contains(err.Error(), "server keypair") {
			t.Fatalf("err = %v, want a server keypair error", err)
		}
	})

	t.Run("missing client CA file", func(t *testing.T) {
		_, err := ServerTLS(certPath, keyPath, filepath.Join(t.TempDir(), "nope.crt"))
		if err == nil || !strings.Contains(err.Error(), "read client CA") {
			t.Fatalf("err = %v, want a read client CA error", err)
		}
	})

	t.Run("client CA file holds no certificates", func(t *testing.T) {
		bogus := filepath.Join(t.TempDir(), "ca.crt")
		writeFile(t, bogus, "-----BEGIN NOT A CERTIFICATE-----\n")
		_, err := ServerTLS(certPath, keyPath, bogus)
		if err == nil || !strings.Contains(err.Error(), "no valid certs") {
			t.Fatalf("err = %v, want a no-valid-certs error", err)
		}
	})
}

func TestMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name  string
		state *tls.ConnectionState
		want  int
	}{
		{"plaintext request", nil, http.StatusUnauthorized},
		{"TLS without a verified chain", &tls.ConnectionState{}, http.StatusUnauthorized},
		{"TLS with a verified chain", &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{}}}, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, "/captures/cap-1/status", nil)
			req.TLS = tc.state
			rr := httptest.NewRecorder()

			Middleware(next).ServeHTTP(rr, req)

			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d", rr.Code, tc.want)
			}
			if called != (tc.want == http.StatusOK) {
				t.Fatalf("next called = %v, want %v", called, tc.want == http.StatusOK)
			}
		})
	}
}

// TestServerTLSRejectsClientWithoutCertificate drives a real TLS handshake
// against a listener configured by ServerTLS. The middleware alone cannot
// prove mTLS is enforced - only the TLS layer refuses a client that presents
// no certificate at all, and that is the sidecar's actual security boundary.
func TestServerTLSRejectsClientWithoutCertificate(t *testing.T) {
	ca := newTestCA(t)
	serverCert, serverKey := ca.issue(t, "capture-sidecar")
	clientCert, clientKey := ca.issue(t, "gameplane-operator")

	cfg, err := ServerTLS(serverCert, serverKey, ca.certPath)
	if err != nil {
		t.Fatalf("ServerTLS: %v", err)
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Handler:           Middleware(handler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = srv.Close() })

	url := "https://" + listener.Addr().String() + "/captures/cap-1/status"

	t.Run("no client certificate is refused at the TLS layer", func(t *testing.T) {
		client := httpClient(t, nil)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			t.Fatalf("request succeeded with status %d; want a TLS failure", resp.StatusCode)
		}
	})

	t.Run("a CA-signed client certificate is accepted", func(t *testing.T) {
		pair, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			t.Fatalf("load client keypair: %v", err)
		}
		client := httpClient(t, &pair)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
	})
}

// helpers

// httpClient builds a client that skips server-name verification (the test
// certificates carry no SANs) but presents cert, if any, to the server. Only
// the server's client-certificate enforcement is under test here.
func httpClient(t *testing.T, cert *tls.Certificate) *http.Client {
	t.Helper()
	cfg := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
	}
	if cert != nil {
		cfg.Certificates = []tls.Certificate{*cert}
	}
	transport := &http.Transport{TLSClientConfig: cfg}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 10 * time.Second}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// testCA is a throwaway certificate authority that issues the server and
// client certificates used by these tests.
type testCA struct {
	dir      string
	certPath string
	cert     *x509.Certificate
	key      *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gameplane-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}

	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.crt")
	writeFile(t, certPath, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))

	return &testCA{dir: dir, certPath: certPath, cert: cert, key: key}
}

// issue writes a CA-signed keypair usable for both server and client auth.
func (c *testCA) issue(t *testing.T, name string) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key for %s: %v", name, err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, &key.PublicKey, c.key)
	if err != nil {
		t.Fatalf("sign certificate for %s: %v", name, err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key for %s: %v", name, err)
	}

	certPath = filepath.Join(c.dir, name+".crt")
	keyPath = filepath.Join(c.dir, name+".key")
	writeFile(t, certPath, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})))
	writeFile(t, keyPath, string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})))
	return certPath, keyPath
}
