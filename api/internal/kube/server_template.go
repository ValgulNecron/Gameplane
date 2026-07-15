package kube

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// LoadServerAndTemplate fetches a GameServer and its GameTemplate. A server
// with no templateRef returns (gs, nil, nil) — what that means (unsupported
// capability, 501, unknown action, etc.) is left to the caller, since
// different surfaces (mod registry browse, id-list mods, quick actions)
// fail closed differently.
func (c *Client) LoadServerAndTemplate(ctx context.Context, ns, name string) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	gs, err := c.Dynamic.Resource(GVRs["servers"]).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	tmplName, _, _ := unstructured.NestedString(gs.Object, "spec", "templateRef", "name")
	if tmplName == "" {
		return gs, nil, nil
	}
	tmpl, err := c.Dynamic.Resource(GVRs["templates"]).Get(ctx, tmplName, metav1.GetOptions{})
	if err != nil {
		return gs, nil, err
	}
	return gs, tmpl, nil
}
