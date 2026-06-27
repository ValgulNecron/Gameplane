// Package v1alpha1 contains API Schema definitions for the gameplane.local
// v1alpha1 API group.
//
// +kubebuilder:object:generate=true
// +groupName=gameplane.local
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	GroupVersion = schema.GroupVersion{Group: "gameplane.local", Version: "v1alpha1"}

	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	AddToScheme = SchemeBuilder.AddToScheme
)
