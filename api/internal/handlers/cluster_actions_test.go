package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/kestrel-gg/kestrel/api/internal/kube"
)

// testCAPEM returns a self-signed CA cert PEM for caCertHash/kubeconfig.
func testCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func clusterActionsClient(t *testing.T) *kube.Client {
	return &kube.Client{
		Typed: fake.NewSimpleClientset(),
		Config: &rest.Config{
			Host:            "https://api.test:6443",
			TLSClientConfig: rest.TLSClientConfig{CAData: testCAPEM(t)},
		},
	}
}

func TestClusterActions_DisabledReturns501(t *testing.T) {
	r := chi.NewRouter()
	MountClusterActions(r, clusterActionsClient(t), false)

	for _, path := range []string{"/cluster/nodes:join", "/cluster/kubeconfig"} {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, nil))
		if rr.Code != http.StatusNotImplemented {
			t.Fatalf("%s: status = %d, want 501 when clusterOps disabled", path, rr.Code)
		}
	}
}

func TestClusterActions_AddNodeCreatesBootstrapToken(t *testing.T) {
	k := clusterActionsClient(t)
	r := chi.NewRouter()
	MountClusterActions(r, k, true)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/cluster/nodes:join", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var resp joinResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(resp.CACertHash, "sha256:") {
		t.Fatalf("caCertHash = %q, want sha256: prefix", resp.CACertHash)
	}
	if resp.Endpoint != "api.test:6443" {
		t.Fatalf("endpoint = %q", resp.Endpoint)
	}
	// token is <6>.<16>
	parts := strings.SplitN(resp.Token, ".", 2)
	if len(parts) != 2 || len(parts[0]) != 6 || len(parts[1]) != 16 {
		t.Fatalf("token = %q, want 6.16 format", resp.Token)
	}
	if !strings.Contains(resp.Command, "kubeadm join api.test:6443 --token") {
		t.Fatalf("command = %q", resp.Command)
	}

	// The bootstrap-token Secret was created in kube-system.
	secs, err := k.Typed.CoreV1().Secrets("kube-system").List(t.Context(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list secrets: %v", err)
	}
	if len(secs.Items) != 1 || !strings.HasPrefix(secs.Items[0].Name, "bootstrap-token-") {
		t.Fatalf("expected one bootstrap-token secret, got %d", len(secs.Items))
	}
	if secs.Items[0].Type != corev1.SecretType("bootstrap.kubernetes.io/token") {
		t.Fatalf("secret type = %q", secs.Items[0].Type)
	}
}
