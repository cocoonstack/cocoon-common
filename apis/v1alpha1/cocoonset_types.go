package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CocoonSetSpec is the desired state of a CocoonSet.
type CocoonSetSpec struct {
	// Suspend, when true, hibernates every managed pod.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// SnapshotPolicy controls when vk-cocoon snapshots VMs before destroying them.
	// +optional
	// +kubebuilder:default=always
	SnapshotPolicy SnapshotPolicy `json:"snapshotPolicy,omitempty"`

	// NodePool selects the cocoon node pool that should host this CocoonSet.
	// vk-cocoon nodes are labeled with cocoonstack.io/pool=<name>.
	// +optional
	// +kubebuilder:default=default
	NodePool string `json:"nodePool,omitempty"`

	// Agent describes the main agent VM and any sub-agent replicas.
	// +kubebuilder:validation:Required
	Agent AgentSpec `json:"agent"`

	// Toolboxes are companion VMs scheduled alongside the agents.
	// +optional
	Toolboxes []ToolboxSpec `json:"toolboxes,omitempty"`
}

// AgentSpec describes a CocoonSet's main agent and sub-agent replicas.
type AgentSpec struct {
	// Replicas is the number of sub-agents to fork from the main agent.
	// The main agent is always created in addition to Replicas.
	// +optional
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`

	// Image is the epoch reference or boot image URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Mode controls how the VM is brought up.
	// +optional
	// +kubebuilder:default=clone
	Mode AgentMode `json:"mode,omitempty"`

	// OS is the guest operating system.
	// +optional
	// +kubebuilder:default=linux
	OS OSType `json:"os,omitempty"`

	// Network is the CNI conflist name to use; empty means cocoon default.
	// +optional
	Network string `json:"network,omitempty"`

	// Storage is the COW disk size to allocate for each VM.
	// +optional
	Storage *resource.Quantity `json:"storage,omitempty"`

	// Resources passes through CPU/memory hints to the underlying pod.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// EnvFrom is forwarded to the agent container's envFrom field.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// ServiceAccountName overrides the agent pod's serviceAccountName.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// ToolboxSpec describes a companion toolbox VM scheduled alongside agents.
type ToolboxSpec struct {
	// Name is unique within the CocoonSet and must follow RFC 1123 label rules.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// OS is the guest operating system.
	// +optional
	// +kubebuilder:default=linux
	OS OSType `json:"os,omitempty"`

	// Image is the epoch reference or boot image URL. Required for non-static modes.
	// +optional
	Image string `json:"image,omitempty"`

	// Mode controls how the toolbox VM is brought up.
	// +optional
	// +kubebuilder:default=run
	Mode ToolboxMode `json:"mode,omitempty"`

	// Storage is the COW disk size to allocate.
	// +optional
	Storage *resource.Quantity `json:"storage,omitempty"`

	// StaticIP is the pre-assigned IP for static-mode toolboxes.
	// +optional
	StaticIP string `json:"staticIP,omitempty"`

	// StaticVMID is the pre-assigned VM identifier for static-mode toolboxes.
	// +optional
	StaticVMID string `json:"staticVMID,omitempty"`

	// VNCPort is the VNC port for graphical access (Windows, Android).
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	VNCPort int32 `json:"vncPort,omitempty"`

	// Resources passes through CPU/memory hints to the underlying pod.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// CocoonSetStatus is the observed state of a CocoonSet.
type CocoonSetStatus struct {
	// ObservedGeneration is the .metadata.generation the controller last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Phase is the high-level lifecycle phase.
	// +optional
	Phase CocoonSetPhase `json:"phase,omitempty"`

	// ReadyAgents is the count of agent pods in the Running phase.
	// +optional
	ReadyAgents int32 `json:"readyAgents"`

	// DesiredAgents is the total number of agents requested by spec (1 + Replicas).
	// +optional
	DesiredAgents int32 `json:"desiredAgents"`

	// Agents reports per-agent runtime state.
	// +optional
	Agents []AgentStatus `json:"agents,omitempty"`

	// Toolboxes reports per-toolbox runtime state.
	// +optional
	Toolboxes []ToolboxStatus `json:"toolboxes,omitempty"`

	// Conditions follow the standard Kubernetes condition pattern.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// AgentStatus is the per-agent runtime state.
type AgentStatus struct {
	// Slot is the agent index. Slot 0 is always the main agent.
	Slot int32 `json:"slot"`

	// Role is "main" or "sub-agent".
	Role string `json:"role"`

	// PodName is the backing pod's name.
	PodName string `json:"podName,omitempty"`

	// VMName is the deterministic VM name vk-cocoon uses.
	VMName string `json:"vmName,omitempty"`

	// VMID is the runtime VM identifier reported by vk-cocoon.
	VMID string `json:"vmID,omitempty"`

	// IP is the VM's primary IP address.
	IP string `json:"ip,omitempty"`

	// Phase mirrors the backing pod's phase.
	Phase string `json:"phase,omitempty"`

	// ForkedFrom is the parent main VM name (sub-agents only).
	ForkedFrom string `json:"forkedFrom,omitempty"`
}

// ToolboxStatus is the per-toolbox runtime state.
type ToolboxStatus struct {
	// Name matches the spec entry.
	Name string `json:"name"`

	// PodName is the backing pod's name.
	PodName string `json:"podName,omitempty"`

	// VMName is the deterministic VM name vk-cocoon uses.
	VMName string `json:"vmName,omitempty"`

	// VMID is the runtime VM identifier reported by vk-cocoon.
	VMID string `json:"vmID,omitempty"`

	// IP is the toolbox VM's primary IP address.
	IP string `json:"ip,omitempty"`

	// Phase mirrors the backing pod's phase.
	Phase string `json:"phase,omitempty"`

	// ConnType is the preferred connection protocol (ssh / rdp / adb / vnc).
	ConnType string `json:"connType,omitempty"`

	// VNCPort mirrors the spec when VNC access is configured.
	VNCPort int32 `json:"vncPort,omitempty"`
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

// CocoonSetList is a list of CocoonSet.
type CocoonSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CocoonSet `json:"items"`
}
