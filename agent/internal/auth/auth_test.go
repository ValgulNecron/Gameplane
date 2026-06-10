package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("mtls mode does not need a token", func(t *testing.T) {
		a, err := New(Config{ClientCAFile: "any-path"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.mode != "mtls" || a.token != nil {
			t.Fatalf("got mode=%q token=%v", a.mode, a.token)
		}
	})

	t.Run("token mode reads and trims token file", func(t *testing.T) {
		path := writeFile(t, "  secret-tok\n")
		a, err := New(Config{TokenFile: path})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.mode != "token" || string(a.token) != "secret-tok" {
			t.Fatalf("got mode=%q token=%q", a.mode, string(a.token))
		}
	})

	t.Run("missing token file errors", func(t *testing.T) {
		_, err := New(Config{TokenFile: filepath.Join(t.TempDir(), "missing")})
		if err == nil || !strings.Contains(err.Error(), "read token file") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("empty token file errors", func(t *testing.T) {
		path := writeFile(t, "   \n\t\n")
		_, err := New(Config{TokenFile: path})
		if err == nil || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("no auth configured errors", func(t *testing.T) {
		_, err := New(Config{})
		if err == nil || !strings.Contains(err.Error(), "no auth configured") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestMiddleware_MTLS(t *testing.T) {
	a := &Authenticator{mode: "mtls"}
	next := okHandler()

	t.Run("plain HTTP rejected", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		a.Middleware(next).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d", rr.Code)
		}
	})

	t.Run("TLS without verified chains rejected", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.TLS = &tls.ConnectionState{}
		a.Middleware(next).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d", rr.Code)
		}
	})

	t.Run("TLS with verified chain passes", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.TLS = &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{}}}}
		a.Middleware(next).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d (body=%q)", rr.Code, rr.Body.String())
		}
	})
}

func TestMiddleware_Token(t *testing.T) {
	a := &Authenticator{mode: "token", token: []byte("s3cret")}
	next := okHandler()

	cases := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"non-bearer scheme", "Basic abc", http.StatusUnauthorized},
		{"bearer prefix only", "Bearer ", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"correct token", "Bearer s3cret", http.StatusOK},
		{"correct token with surrounding space", "Bearer   s3cret  ", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			a.Middleware(next).ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("want %d, got %d", tc.want, rr.Code)
			}
		})
	}
}

func TestBearer(t *testing.T) {
	cases := map[string]string{
		"":               "",
		"Bearer":         "",
		"Basic xyz":      "",
		"Bearer abc":     "abc",
		"Bearer   abc  ": "abc",
		"Bearer\tabc":    "",
	}
	for in, want := range cases {
		if got := bearer(in); got != want {
			t.Errorf("bearer(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestServerTLS(t *testing.T) {
	certPath, keyPath := writeKeypair(t)
	caPath := writeFile(t, mustReadFile(t, certPath))

	t.Run("success without CA", func(t *testing.T) {
		cfg, err := ServerTLS(certPath, keyPath, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Fatalf("ClientAuth=%v", cfg.ClientAuth)
		}
		if cfg.MinVersion != tls.VersionTLS12 || len(cfg.Certificates) != 1 {
			t.Fatalf("bad cfg: %+v", cfg)
		}
	})

	t.Run("success with CA bundle", func(t *testing.T) {
		cfg, err := ServerTLS(certPath, keyPath, caPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientCAs == nil {
			t.Fatalf("expected ClientCAs to be populated")
		}
	})

	t.Run("bad keypair path", func(t *testing.T) {
		_, err := ServerTLS("/no/such/cert", "/no/such/key", "")
		if err == nil || !strings.Contains(err.Error(), "server keypair") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("bad CA path", func(t *testing.T) {
		_, err := ServerTLS(certPath, keyPath, "/no/such/ca")
		if err == nil || !strings.Contains(err.Error(), "read client CA") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("CA file with no valid certs", func(t *testing.T) {
		bogus := writeFile(t, "not a pem certificate")
		_, err := ServerTLS(certPath, keyPath, bogus)
		if err == nil || !strings.Contains(err.Error(), "no valid certs") {
			t.Fatalf("got %v", err)
		}
	})
}

// helpers

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(b)
}

func writeKeypair(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("createcert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshalkey: %v", err)
	}
	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}
