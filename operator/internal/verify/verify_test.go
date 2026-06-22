package verify

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func ociSource(verify *gameplanev1alpha1.VerifySpec) *gameplanev1alpha1.ModuleSource {
	return &gameplanev1alpha1.ModuleSource{
		Spec: gameplanev1alpha1.ModuleSourceSpec{
			Type:   gameplanev1alpha1.ModuleSourceTypeOCI,
			OCI:    &gameplanev1alpha1.OCISourceSpec{URL: "ghcr.io/test/modules"},
			Verify: verify,
		},
	}
}

func TestBuild_NilVerifyReturnsNop(t *testing.T) {
	v, err := Build(context.Background(), fake.NewClientBuilder().Build(), "ns", ociSource(nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := v.(Nop); !ok {
		t.Fatalf("want Nop, got %T", v)
	}
}

func TestBuild_KeyMissingSecretErrors(t *testing.T) {
	src := ociSource(&gameplanev1alpha1.VerifySpec{Key: &corev1.LocalObjectReference{Name: "missing"}})
	if _, err := Build(context.Background(), fake.NewClientBuilder().Build(), "ns", src); err == nil {
		t.Fatal("expected error for missing key secret")
	}
}

func TestBuild_KeyBadPEMErrors(t *testing.T) {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "k", Namespace: "ns"},
		Data:       map[string][]byte{cosignPubKey: []byte("not a key")},
	}
	src := ociSource(&gameplanev1alpha1.VerifySpec{Key: &corev1.LocalObjectReference{Name: "k"}})
	c := fake.NewClientBuilder().WithObjects(sec).Build()
	if _, err := Build(context.Background(), c, "ns", src); err == nil {
		t.Fatal("expected error for malformed public key")
	}
}

func TestBuild_KeyValidPEMReturnsVerifier(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "k", Namespace: "ns"},
		Data:       map[string][]byte{cosignPubKey: pubPEM},
	}
	src := ociSource(&gameplanev1alpha1.VerifySpec{Key: &corev1.LocalObjectReference{Name: "k"}})
	c := fake.NewClientBuilder().WithObjects(sec).Build()
	v, err := Build(context.Background(), c, "ns", src)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, ok := v.(Nop); ok {
		t.Fatal("expected a real verifier, got Nop")
	}
}

func TestAuthFor(t *testing.T) {
	t.Run("anonymous when no ref", func(t *testing.T) {
		a, err := authFor(context.Background(), fake.NewClientBuilder().Build(), "ns", nil)
		if err != nil {
			t.Fatalf("authFor: %v", err)
		}
		if a != authn.Anonymous {
			t.Fatalf("want anonymous, got %#v", a)
		}
	})

	t.Run("from dockerconfigjson", func(t *testing.T) {
		cfg := `{"auths":{"ghcr.io":{"username":"u","password":"p"}}}`
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "ns"},
			Type:       corev1.SecretTypeDockerConfigJson,
			Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(cfg)},
		}
		c := fake.NewClientBuilder().WithObjects(sec).Build()
		a, err := authFor(context.Background(), c, "ns", &corev1.LocalObjectReference{Name: "ps"})
		if err != nil {
			t.Fatalf("authFor: %v", err)
		}
		got, err := a.Authorization()
		if err != nil {
			t.Fatalf("authorization: %v", err)
		}
		if got.Username != "u" || got.Password != "p" {
			t.Fatalf("creds = %q/%q, want u/p", got.Username, got.Password)
		}
	})
}
