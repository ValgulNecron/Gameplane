// Package v1alpha1 contains API Schema definitions for the gameplane.gg
// v1alpha1 API group.
//
// +kubebuilder:object:generate=true
// +groupName=gameplane.gg
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion = schema.GroupVersion{Group: "gameplane.gg", Version: "v1alpha1"}

	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	AddToScheme = SchemeBuilder.AddToScheme
)
