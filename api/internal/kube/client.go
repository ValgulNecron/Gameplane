// Package kube wraps the dynamic and typed Kubernetes clients that the
// API uses to read/write Kestrel CRDs.
package kube

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	Dynamic dynamic.Interface
	Typed   kubernetes.Interface
	Config  *rest.Config
}

// GVRs exposed to the REST layer. Keyed by resource-path segment so
// handlers can turn /servers into the right GVR without reflection.
var GVRs = map[string]schema.GroupVersionResource{
	"servers":   {Group: "kestrel.gg", Version: "v1alpha1", Resource: "gameservers"},
	"templates": {Group: "kestrel.gg", Version: "v1alpha1", Resource: "gametemplates"},
	"backups":   {Group: "kestrel.gg", Version: "v1alpha1", Resource: "backups"},
	"schedules": {Group: "kestrel.gg", Version: "v1alpha1", Resource: "backupschedules"},
	"restores":  {Group: "kestrel.gg", Version: "v1alpha1", Resource: "restores"},
}

// ModuleGVRs lists the cluster-scoped CRDs the modules handler manages.
// Kept separate from GVRs because /modules is not a generic CRUD route —
// it serves a merged catalog endpoint and an install/uninstall surface.
var (
	GVRModule       = schema.GroupVersionResource{Group: "kestrel.gg", Version: "v1alpha1", Resource: "modules"}
	GVRModuleSource = schema.GroupVersionResource{Group: "kestrel.gg", Version: "v1alpha1", Resource: "modulesources"}
)

func New(cfg *rest.Config) (*Client, error) {
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	typed, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{Dynamic: dyn, Typed: typed, Config: cfg}, nil
}
