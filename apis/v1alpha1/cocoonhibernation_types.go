package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HibernationDesire is the requested target state for a pod's VM.
//
// +kubebuilder:validation:Enum=Hibernate;Wake
type HibernationDesire string

const (
	// HibernationDesireHibernate asks vk-cocoon to snapshot the VM,
	// destroy it, and keep the backing pod alive.
	HibernationDesireHibernate HibernationDesire = "Hibernate"
	// HibernationDesireWake asks vk-cocoon to restore the VM from a
	// previously hibernated snapshot.
	HibernationDesireWake HibernationDesire = "Wake"
)

// CocoonHibernationPhase is the high-level state reported in status.
//
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

// CocoonHibernationSpec is the desired state of a CocoonHibernation.
type CocoonHibernationSpec struct {
	// PodRef points at the pod whose VM should be hibernated or woken.
	// +kubebuilder:validation:Required
	PodRef corev1.LocalObjectReference `json:"podRef"`

	// Desire is the target state.
	// +kubebuilder:validation:Required
	Desire HibernationDesire `json:"desire"`
}

// CocoonHibernationStatus is the observed state of a CocoonHibernation.
type CocoonHibernationStatus struct {
	// ObservedGeneration is the .metadata.generation the controller last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase reflects the lifecycle state.
	// +optional
	Phase CocoonHibernationPhase `json:"phase,omitempty"`

	// VMName is the deterministic VM name resolved from the pod annotations.
	// +optional
	VMName string `json:"vmName,omitempty"`

	// Conditions follow the standard Kubernetes condition pattern.
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

// CocoonHibernationList is a list of CocoonHibernation.
type CocoonHibernationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CocoonHibernation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CocoonHibernation{}, &CocoonHibernationList{})
}
