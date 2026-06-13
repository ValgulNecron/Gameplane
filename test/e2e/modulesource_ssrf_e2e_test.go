//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestModuleSource_RejectsSSRFTarget asserts the operator's outbound SSRF
// guard refuses a ModuleSource fetch aimed at a high-value SSRF target. No
// registry/oras infra is needed — the guard fires while building the fetcher,
// before any network I/O, surfacing on status.conditions[Synced]=False with
// reason IndexFailed and an empty catalog/lastSync.
//
// IMPORTANT: the operator guard (operator/internal/netguard) DELIBERATELY
// allows loopback and RFC1918 — self-hosted registries legitimately live
// there — so only link-local (the cloud metadata range) and the metadata
// hostnames are rejected. Do NOT use 127.0.0.1 / 10.x here; they are allowed
// by design and the assertion would never hold.
func TestModuleSource_RejectsSSRFTarget(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name      string
		source    string
		spec      map[string]any
		msgSubstr string
	}{
		{
			name:   "http link-local metadata IP",
			source: "e2e-ssrf-http",
			spec: map[string]any{
				"type": "http",
				"http": map[string]any{
					// 169.254.169.254 is the cloud instance-metadata endpoint.
					"url":      "http://169.254.169.254/latest/modules.tar.gz",
					"insecure": true,
				},
				"refreshInterval": "10m",
			},
			msgSubstr: "blocked address",
		},
		{
			name:   "git metadata hostname",
			source: "e2e-ssrf-git",
			spec: map[string]any{
				"type": "git",
				"git": map[string]any{
					"url": "https://metadata.google.internal/repo.git",
				},
				"refreshInterval": "10m",
			},
			msgSubstr: "metadata endpoints are not allowed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "kestrel.gg/v1alpha1",
				"kind":       "ModuleSource",
				"metadata":   map[string]any{"name": tc.source},
				"spec":       tc.spec,
			}}
			if _, err := envInstance.Dyn.Resource(moduleSourceGVR).
				Create(ctx, src, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
				t.Fatalf("create ssrf modulesource: %v", err)
			}
			t.Cleanup(func() {
				_ = envInstance.Dyn.Resource(moduleSourceGVR).
					Delete(context.Background(), tc.source, metav1.DeleteOptions{})
			})

			envInstance.Eventually(t, 90*time.Second, func() (bool, string) {
				got, err := envInstance.Dyn.Resource(moduleSourceGVR).
					Get(ctx, tc.source, metav1.GetOptions{})
				if err != nil {
					return false, "get modulesource: " + err.Error()
				}
				// A blocked fetch never populates the catalog or bumps lastSync.
				if modules, _, _ := unstructured.NestedSlice(got.Object, "status", "modules"); len(modules) > 0 {
					return false, "status.modules unexpectedly populated for a blocked source"
				}
				if ls, _, _ := unstructured.NestedString(got.Object, "status", "lastSync"); ls != "" {
					return false, "status.lastSync set despite a blocked fetch: " + ls
				}
				synced := findCondition(got.Object, "Synced")
				if synced == nil {
					return false, "no Synced condition yet"
				}
				if synced["status"] != "False" {
					return false, "Synced status=" + asString(synced["status"]) + " (want False)"
				}
				if synced["reason"] != "IndexFailed" {
					return false, "Synced reason=" + asString(synced["reason"]) + " (want IndexFailed)"
				}
				msg := asString(synced["message"])
				if !strings.Contains(msg, tc.msgSubstr) {
					return false, "Synced message lacks " + tc.msgSubstr + ": " + msg
				}
				return true, ""
			})
		})
	}

	// The guard must reject without crashing the controller.
	pods, err := envInstance.K8s.CoreV1().Pods("kestrel-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=kestrel-operator",
	})
	if err != nil {
		t.Fatalf("list operator pods: %v", err)
	}
	if len(pods.Items) == 0 {
		t.Fatal("no operator pod after SSRF test — controller crash?")
	}
	for _, p := range pods.Items {
		ready := false
		for _, c := range p.Status.Conditions {
			if c.Type == "Ready" && c.Status == "True" {
				ready = true
				break
			}
		}
		if !ready {
			t.Errorf("operator pod %s not Ready after SSRF test", p.Name)
		}
	}
}

// findCondition returns the status.conditions entry of the given type, or nil.
func findCondition(obj map[string]any, condType string) map[string]any {
	conditions, _, _ := unstructured.NestedSlice(obj, "status", "conditions")
	for _, cIface := range conditions {
		c, ok := cIface.(map[string]any)
		if !ok {
			continue
		}
		if c["type"] == condType {
			return c
		}
	}
	return nil
}
