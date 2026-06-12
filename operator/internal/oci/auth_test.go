package oci

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"oras.land/oras-go/v2/registry/remote/auth"
)

func newFakeClient(t *testing.T, objs ...runtime.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...)
}

func TestCredentialFromSecret_NilRefIsAnonymous(t *testing.T) {
	cli := newFakeClient(t).Build()
	fn, err := CredentialFromSecret(context.Background(), cli, "ns", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	cred, _ := fn(context.Background(), "ghcr.io")
	if cred != auth.EmptyCredential {
		t.Fatalf("expected empty, got %+v", cred)
	}
}

func TestCredentialFromSecret_EmptyNameIsAnonymous(t *testing.T) {
	cli := newFakeClient(t).Build()
	fn, err := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	cred, _ := fn(context.Background(), "ghcr.io")
	if cred != auth.EmptyCredential {
		t.Fatalf("expected empty")
	}
}

func TestCredentialFromSecret_MissingSecret(t *testing.T) {
	cli := newFakeClient(t).Build()
	_, err := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{Name: "missing"})
	if err == nil || !strings.Contains(err.Error(), "get pull secret") {
		t.Fatalf("got %v", err)
	}
}

func TestCredentialFromSecret_MissingDockerKey(t *testing.T) {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "ns"},
		Data:       map[string][]byte{"unrelated": []byte("x")},
	}
	cli := newFakeClient(t, sec).Build()
	_, err := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{Name: "ps"})
	if err == nil || !strings.Contains(err.Error(), "no") {
		t.Fatalf("got %v", err)
	}
}

func TestCredentialFromSecret_BadJSON(t *testing.T) {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "ns"},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte("not json"),
		},
	}
	cli := newFakeClient(t, sec).Build()
	_, err := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{Name: "ps"})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("got %v", err)
	}
}

func makeDockerSecret(t *testing.T, payload string) *corev1.Secret {
	t.Helper()
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "ns"},
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(payload)},
	}
}

func TestCredentialFromSecret_LookupHits(t *testing.T) {
	payload := `{"auths":{"ghcr.io":{"username":"u","password":"p"}}}`
	cli := newFakeClient(t, makeDockerSecret(t, payload)).Build()
	fn, err := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{Name: "ps"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	cred, _ := fn(context.Background(), "ghcr.io")
	if cred.Username != "u" || cred.Password != "p" {
		t.Fatalf("got %+v", cred)
	}
}

func TestCredentialFromSecret_RegistryMissIsAnonymous(t *testing.T) {
	payload := `{"auths":{"ghcr.io":{"username":"u","password":"p"}}}`
	cli := newFakeClient(t, makeDockerSecret(t, payload)).Build()
	fn, _ := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{Name: "ps"})
	cred, _ := fn(context.Background(), "quay.io")
	if cred != auth.EmptyCredential {
		t.Fatalf("got %+v", cred)
	}
}

func TestCredentialFromSecret_BasicAuthFallback(t *testing.T) {
	authStr := base64.StdEncoding.EncodeToString([]byte("user:pw"))
	payload := `{"auths":{"ghcr.io":{"auth":"` + authStr + `"}}}`
	cli := newFakeClient(t, makeDockerSecret(t, payload)).Build()
	fn, _ := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{Name: "ps"})
	cred, err := fn(context.Background(), "ghcr.io")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if cred.Username != "user" || cred.Password != "pw" {
		t.Fatalf("got %+v", cred)
	}
}

func TestCredentialFromSecret_BasicAuthBadBase64(t *testing.T) {
	payload := `{"auths":{"ghcr.io":{"auth":"!!!not base64!!!"}}}`
	cli := newFakeClient(t, makeDockerSecret(t, payload)).Build()
	fn, _ := CredentialFromSecret(context.Background(), cli, "ns", &corev1.LocalObjectReference{Name: "ps"})
	_, err := fn(context.Background(), "ghcr.io")
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestParseBasicAuth(t *testing.T) {
	enc := base64.StdEncoding.EncodeToString([]byte("alice:bob"))
	u, p, err := parseBasicAuth(enc)
	if err != nil || u != "alice" || p != "bob" {
		t.Fatalf("u=%q p=%q err=%v", u, p, err)
	}
}

func TestParseBasicAuth_NoColon(t *testing.T) {
	enc := base64.StdEncoding.EncodeToString([]byte("nopayload"))
	if _, _, err := parseBasicAuth(enc); err == nil ||
		!strings.Contains(err.Error(), "missing ':'") {
		t.Fatalf("got %v", err)
	}
}

func TestParseBasicAuth_BadBase64(t *testing.T) {
	if _, _, err := parseBasicAuth("!!!"); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestBase64Decode(t *testing.T) {
	got, err := base64Decode(base64.StdEncoding.EncodeToString([]byte("hi")))
	if err != nil || string(got) != "hi" {
		t.Fatalf("err=%v got=%q", err, got)
	}
}
