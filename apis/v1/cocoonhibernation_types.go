package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HibernationDesire defines the desired hibernation state.
// +kubebuilder:validation:Enum=Hibernate;Wake
type HibernationDesire string

const (
	HibernationDesireHibernate HibernationDesire = "Hibernate"
	HibernationDesireWake      HibernationDesire = "Wake"
)

// CocoonHibernationPhase represents the lifecycle phase of a CocoonHibernation.
// +kubebuilder:validation:Enum=Pending;Hibernating;Hibernated;Waking;Active;Failed
type CocoonHibernationPhase string

const (
	CocoonHibernationPhasePending     CocoonHibernationPhase = "Pending"
	CocoonHibernationPhaseHibernating CocoonHibernationPhase = "Hibernating"
	CocoonHibernationPhaseHibernated  CocoonHibernationPhase = "Hibernated"
	CocoonHibernationPhaseWaking      CocoonHibernationPhase = "Waking"
	CocoonHibernationPhaseActive      CocoonHibernationPhase = "Active"
	CocoonHibernationPhaseFailed      CocoonHibernationPhase = "Failed"
)

// CocoonHibernationSpec defines the desired state of a CocoonHibernation.
type CocoonHibernationSpec struct {
	// +kubebuilder:validation:Required
	PodRef corev1.LocalObjectReference `json:"podRef"`

	// +kubebuilder:validation:Required
	Desire HibernationDesire `json:"desire"`
}

// CocoonHibernationStatus represents the observed state of a CocoonHibernation.
type CocoonHibernationStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Phase CocoonHibernationPhase `json:"phase,omitempty"`

	// +optional
	VMName string `json:"vmName,omitempty"`

	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ch;cocoonhibernations,categories={cocoon}
// +kubebuilder:printcolumn:name="Pod",type=string,JSONPath=`.spec.podRef.name`
// +kubebuilder:printcolumn:name="Desire",type=string,JSONPath=`.spec.desire`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CocoonHibernation is the Schema for the cocoonhibernations API.
type CocoonHibernation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CocoonHibernationSpec   `json:"spec,omitempty"`
	Status CocoonHibernationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CocoonHibernationList contains a list of CocoonHibernation resources.
type CocoonHibernationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CocoonHibernation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CocoonHibernation{}, &CocoonHibernationList{})
}
