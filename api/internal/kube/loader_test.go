package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigFromKubeconfig_Valid(t *testing.T) {
	validKubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com:6443
  name: test
contexts:
- context:
    cluster: test
    user: admin
  name: test
current-context: test
users:
- name: admin
  user:
    token: fake-token`)

	cfg, err := ConfigFromKubeconfig(validKubeconfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Host != "https://example.com:6443" {
		t.Fatalf("unexpected host: %q", cfg.Host)
	}
}

func TestConfigFromKubeconfig_Invalid(t *testing.T) {
	invalidKubeconfig := []byte("not valid yaml")

	_, err := ConfigFromKubeconfig(invalidKubeconfig)
	if err == nil {
		t.Fatal("expected error for invalid kubeconfig")
	}
}

func TestClientFromSecret_Success(t *testing.T) {
	ctx := context.Background()

	// Build a fake typed client with a labeled Secret.
	typed := fake.NewClientset()
	kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com:6443
  name: test
contexts:
- context:
    cluster: test
    user: admin
  name: test
current-context: test
users:
- name: admin
  user:
    token: fake-token`)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeconfig",
			Namespace: "gameplane-system",
			Labels: map[string]string{
				ClusterKubeconfigLabel: "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}
	_, err := typed.CoreV1().Secrets("gameplane-system").Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}

	// Create a minimal home client with the fake typed client.
	home := &Client{Typed: typed}

	c, err := ClientFromSecret(ctx, home, "gameplane-system", "test-kubeconfig", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestClientFromSecret_MissingLabel(t *testing.T) {
	ctx := context.Background()

	typed := fake.NewClientset()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unlabeled",
			Namespace: "gameplane-system",
			// No label
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": []byte("fake"),
		},
	}
	_, _ = typed.CoreV1().Secrets("gameplane-system").Create(ctx, secret, metav1.CreateOptions{})

	home := &Client{Typed: typed}
	_, err := ClientFromSecret(ctx, home, "gameplane-system", "unlabeled", "")
	if err == nil {
		t.Fatal("expected error for missing label")
	}
}

func TestClientFromSecret_CustomKey(t *testing.T) {
	ctx := context.Background()

	typed := fake.NewClientset()
	kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://example.com:6443
  name: test
contexts:
- context:
    cluster: test
    user: admin
  name: test
current-context: test
users:
- name: admin
  user:
    token: fake-token`)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "gameplane-system",
			Labels: map[string]string{
				ClusterKubeconfigLabel: "true",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"custom-key": kubeconfig,
		},
	}
	_, _ = typed.CoreV1().Secrets("gameplane-system").Create(ctx, secret, metav1.CreateOptions{})

	home := &Client{Typed: typed}
	c, err := ClientFromSecret(ctx, home, "gameplane-system", "test-secret", "custom-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
