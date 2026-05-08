package ws

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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeKeypair(t *testing.T) (cert, key string) {
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
	keyDER, _ := x509.MarshalECPrivateKey(priv)
	dir := t.TempDir()
	cert = filepath.Join(dir, "cert.pem")
	key = filepath.Join(dir, "key.pem")
	_ = os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	_ = os.WriteFile(key, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600)
	return cert, key
}

func TestAgentTLSConfig_MissingMaterial(t *testing.T) {
	if _, err := agentTLSConfig("", "", ""); err == nil ||
		!strings.Contains(err.Error(), "missing") {
		t.Fatalf("got %v", err)
	}
}

func TestAgentTLSConfig_BadKeypair(t *testing.T) {
	if _, err := agentTLSConfig("/a", "/b", "/c"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAgentTLSConfig_Success(t *testing.T) {
	cert, key := writeKeypair(t)
	caData, _ := os.ReadFile(cert)
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	_ = os.WriteFile(caPath, caData, 0o600)

	cfg, err := agentTLSConfig(caPath, cert, key)
	if err != nil {
		t.Fatalf("agentTLSConfig: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS12 || len(cfg.Certificates) != 1 || cfg.RootCAs == nil {
		t.Fatalf("bad cfg: %+v", cfg)
	}
}

func TestAgentTLSConfig_BadCABundle(t *testing.T) {
	cert, key := writeKeypair(t)
	bogus := filepath.Join(t.TempDir(), "ca.pem")
	_ = os.WriteFile(bogus, []byte("not a cert"), 0o600)
	if _, err := agentTLSConfig(bogus, cert, key); err == nil ||
		!strings.Contains(err.Error(), "empty CA") {
		t.Fatalf("got %v", err)
	}
}

func TestAgentTLSConfig_MissingCAFile(t *testing.T) {
	cert, key := writeKeypair(t)
	if _, err := agentTLSConfig("/no/such/ca", cert, key); err == nil {
		t.Fatal("expected error")
	}
}

func TestCopyProxyHeaders_AllowlistOnly(t *testing.T) {
	src := http.Header{}
	src.Set("Accept", "*/*")
	src.Set("Authorization", "Bearer x")
	src.Set("Cookie", "k=v")
	src.Set("Content-Type", "application/json")
	dst := http.Header{}
	copyProxyHeaders(dst, src)
	if dst.Get("Accept") == "" || dst.Get("Content-Type") == "" {
		t.Fatalf("allowlisted headers missing: %+v", dst)
	}
	if dst.Get("Authorization") != "" || dst.Get("Cookie") != "" {
		t.Fatalf("denied headers leaked: %+v", dst)
	}
}

func TestCopyResponseHeaders_DenylistDropped(t *testing.T) {
	src := http.Header{}
	src.Set("Set-Cookie", "k=v")
	src.Set("Connection", "keep-alive")
	src.Set("Content-Type", "text/plain")
	src.Set("X-Custom", "ok")
	dst := http.Header{}
	copyResponseHeaders(dst, src)
	if dst.Get("Set-Cookie") != "" || dst.Get("Connection") != "" {
		t.Fatalf("hop-by-hop leaked: %+v", dst)
	}
	if dst.Get("Content-Type") == "" || dst.Get("X-Custom") == "" {
		t.Fatalf("regular headers missing: %+v", dst)
	}
}

func TestProxy_AgentHost(t *testing.T) {
	p := &proxy{}
	got := p.agentHost("alpha", "kestrel-games")
	if !strings.Contains(got, "alpha-0.alpha.kestrel-games.svc.cluster.local:8090") {
		t.Fatalf("got %q", got)
	}
}
