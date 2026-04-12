package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the API group and version for cocoonset resources.
	GroupVersion = schema.GroupVersion{Group: "cocoonset.cocoonstack.io", Version: "v1"}

	// SchemeBuilder registers cocoonset types with a runtime scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds cocoonset types to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
