package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the API group and version for cocoonset resources.
	GroupVersion = schema.GroupVersion{Group: "cocoonset.cocoonstack.io", Version: "v1"}

	// SchemeBuilder registers cocoonset types with a runtime scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme adds cocoonset types to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion,
		&CocoonSet{}, &CocoonSetList{},
		&CocoonHibernation{}, &CocoonHibernationList{},
	)
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}
