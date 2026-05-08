//go:build e2e

// Package e2e holds end-to-end tests that run against a real
// (kind-based) Kubernetes cluster with the Kestrel chart installed.
package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Env wraps the k8s clients + a kubectl shell helper. One Env is built
// per test process via newEnv, called from the test suite TestMain
// after the cluster is known to be up.
type Env struct {
	Cfg *rest.Config
	K8s kubernetes.Interface
	Dyn dynamic.Interface

	// ClusterName / Tag are set by the e2e bootstrap and used by
	// Kubectl for context selection.
	ClusterName string
	Tag         string
}

func newEnv() (*Env, error) {
	cluster := getenvOr("KESTREL_E2E_CLUSTER", "kestrel-e2e")
	tag := getenvOr("KESTREL_E2E_TAG", "e2e")

	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		loader.ExplicitPath = kc
	}
	override := &clientcmd.ConfigOverrides{
		CurrentContext: "kind-" + cluster,
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, override).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}
	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Env{
		Cfg:         cfg,
		K8s:         k8s,
		Dyn:         dyn,
		ClusterName: cluster,
		Tag:         tag,
	}, nil
}

// getenvOr returns os.Getenv(key) when set, otherwise dflt.
func getenvOr(key, dflt string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dflt
}

// Eventually polls cond every ~1s until it returns true or timeout
// elapses. The condition's message is included on timeout.
func (e *Env) Eventually(t *testing.T, timeout time.Duration, cond func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastMsg string
	for time.Now().Before(deadline) {
		ok, msg := cond()
		if ok {
			return
		}
		lastMsg = msg
		time.Sleep(1 * time.Second)
	}
	if ok, _ := cond(); ok {
		return
	}
	t.Fatalf("Eventually: timed out after %s: %s", timeout, lastMsg)
}

// Kubectl shells out for operations the typed client makes awkward —
// `kubectl exec`, `kubectl apply -f`, etc. Output (stdout+stderr
// combined) is returned along with the error, so callers can include
// it in failure messages.
func (e *Env) Kubectl(args ...string) (string, error) {
	all := append([]string{"--context", "kind-" + e.ClusterName}, args...)
	cmd := exec.Command("kubectl", all...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ApplyYAML reads a YAML manifest from the fixtures directory and
// applies it via kubectl. Path is relative to the fixtures/ directory.
func (e *Env) ApplyYAML(t *testing.T, fixturePath string) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("fixtures", fixturePath))
	if err != nil {
		t.Fatalf("resolve fixture path %s: %v", fixturePath, err)
	}
	if out, err := e.Kubectl("apply", "-f", abs); err != nil {
		t.Fatalf("kubectl apply -f %s: %v\n%s", fixturePath, err, out)
	}
}

// PodIsReady reports whether the named pod has Ready=True.
func (e *Env) PodIsReady(ctx context.Context, ns, name string) (bool, error) {
	pod, err := e.K8s.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True", nil
		}
	}
	return false, nil
}

// CRDExists reports whether the named CustomResourceDefinition has
// been installed in the cluster.
func (e *Env) CRDExists(ctx context.Context, name string) (bool, error) {
	_, err := e.Dyn.Resource(crdGVR).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// crdGVR is the GVR for the CustomResourceDefinition resource itself.
var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

// notFoundOrError unwraps an apierrors.NotFound into a typed (false, nil),
// for use in conditions where "the object isn't there yet" is expected.
func notFoundOrError(err error) (bool, error) {
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return false, err
	}
}

// ensureCluster verifies the e2e cluster is reachable. Used at TestMain
// time before launching tests so failures are fast and clear.
func (e *Env) ensureCluster() error {
	_, err := e.K8s.Discovery().ServerVersion()
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("cluster %q not reachable: %w", e.ClusterName, err)
	}
	return err
}
