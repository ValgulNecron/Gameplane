// Package v1alpha1 contains API Schema definitions for the gameplane.local
// v1alpha1 API group.
//
// +kubebuilder:object:generate=true
// +groupName=gameplane.local
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// schemeBuilder mirrors the now-deprecated sigs.k8s.io/controller-runtime/pkg/scheme.Builder,
// reimplemented locally on top of apimachinery alone. The upstream type is deprecated precisely
// because api packages should depend only on the standard library, k8s.io/apimachinery, and other
// api packages — not on controller-runtime. Keep this local rather than reverting to the
// controller-runtime helper.
type schemeBuilder struct {
	GroupVersion schema.GroupVersion
	runtime.SchemeBuilder
}

// Register adds one or more objects to the SchemeBuilder so they can be added to a Scheme.
// Register mutates b.
func (b *schemeBuilder) Register(objects ...runtime.Object) *schemeBuilder {
	b.SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(b.GroupVersion, objects...)
		metav1.AddToGroupVersion(s, b.GroupVersion)
		return nil
	})
	return b
}

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "gameplane.local", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &schemeBuilder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
