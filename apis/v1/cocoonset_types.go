package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CocoonSetSpec defines the desired state of a CocoonSet.
type CocoonSetSpec struct {
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// +optional
	// +kubebuilder:default=always
	SnapshotPolicy SnapshotPolicy `json:"snapshotPolicy,omitempty"`

	// +optional
	// +kubebuilder:default=default
	NodePool string `json:"nodePool,omitempty"`

	// +kubebuilder:validation:Required
	Agent AgentSpec `json:"agent"`

	// +optional
	Toolboxes []ToolboxSpec `json:"toolboxes,omitempty"`
}

// VMOptions are VM-level knobs shared by AgentSpec and ToolboxSpec.
// Field semantics live on the type godocs (see OSType, ConnType, Backend).
type VMOptions struct {
	// +optional
	// +kubebuilder:default=linux
	OS OSType `json:"os,omitempty"`

	// +optional
	// +kubebuilder:default=cloud-hypervisor
	Backend Backend `json:"backend,omitempty"`

	// +optional
	ConnType ConnType `json:"connType,omitempty"`

	// Network selects the cluster network to attach the VM to.
	// +optional
	Network string `json:"network,omitempty"`

	// ForcePull bypasses the image cache and re-pulls from upstream.
	// +optional
	ForcePull bool `json:"forcePull,omitempty"`

	// NoDirectIO disables O_DIRECT on writable disks, using host page
	// cache instead. Useful for dev/test with few VMs and abundant host
	// memory. Cloud Hypervisor only; ignored by firecracker.
	// +optional
	NoDirectIO bool `json:"noDirectIO,omitempty"`

	// Storage sizes the VM root volume.
	// +optional
	Storage *resource.Quantity `json:"storage,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// AgentSpec defines the configuration for agent VMs in a CocoonSet.
type AgentSpec struct {
	// Replicas is the number of sub-agents; the main agent is always created in addition.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +optional
	// +kubebuilder:default=clone
	Mode AgentMode `json:"mode,omitempty"`

	VMOptions `json:",inline"`

	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// ToolboxSpec defines the configuration for a toolbox VM in a CocoonSet.
type ToolboxSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// +optional
	Image string `json:"image,omitempty"`

	// +optional
	// +kubebuilder:default=run
	Mode ToolboxMode `json:"mode,omitempty"`

	// +optional
	StaticIP string `json:"staticIP,omitempty"`

	// +optional
	StaticVMID string `json:"staticVMID,omitempty"`

	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	VNCPort int32 `json:"vncPort,omitempty"`

	VMOptions `json:",inline"`
}

// CocoonSetStatus represents the observed state of a CocoonSet.
type CocoonSetStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Phase CocoonSetPhase `json:"phase,omitempty"`

	// +optional
	ReadyAgents int32 `json:"readyAgents"`

	// +optional
	DesiredAgents int32 `json:"desiredAgents"`

	// +optional
	Agents []AgentStatus `json:"agents,omitempty"`

	// +optional
	Toolboxes []ToolboxStatus `json:"toolboxes,omitempty"`

	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// AgentStatus represents the observed state of a single agent VM.
type AgentStatus struct {
	Slot       int32  `json:"slot"`
	Role       string `json:"role"`
	PodName    string `json:"podName,omitempty"`
	VMName     string `json:"vmName,omitempty"`
	VMID       string `json:"vmID,omitempty"`
	IP         string `json:"ip,omitempty"`
	Phase      string `json:"phase,omitempty"`
	ForkedFrom string `json:"forkedFrom,omitempty"`
}

// ToolboxStatus represents the observed state of a single toolbox VM.
type ToolboxStatus struct {
	Name     string   `json:"name"`
	PodName  string   `json:"podName,omitempty"`
	VMName   string   `json:"vmName,omitempty"`
	VMID     string   `json:"vmID,omitempty"`
	IP       string   `json:"ip,omitempty"`
	Phase    string   `json:"phase,omitempty"`
	ConnType ConnType `json:"connType,omitempty"`
	VNCPort  int32    `json:"vncPort,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cs;cocoonsets,categories={cocoon}
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.readyAgents`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredAgents`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CocoonSet is the Schema for the cocoonsets API.
type CocoonSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CocoonSetSpec   `json:"spec,omitempty"`
	Status CocoonSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CocoonSetList contains a list of CocoonSet resources.
type CocoonSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CocoonSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CocoonSet{}, &CocoonSetList{})
}
