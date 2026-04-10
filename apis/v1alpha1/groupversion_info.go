package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// GroupVersion is the API Group Version used to register the objects.
var GroupVersion = schema.GroupVersion{Group: "cocoonset.cocoonstack.io", Version: "v1alpha1"}

// SchemeBuilder collects type registrations for this group; consumers
// call AddToScheme to install them on a runtime.Scheme.
var SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

// AddToScheme adds the types in this group-version to the given scheme.
var AddToScheme = SchemeBuilder.AddToScheme
