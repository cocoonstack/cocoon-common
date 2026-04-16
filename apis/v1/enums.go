package v1

// AgentMode defines the mode of an agent VM.
// +kubebuilder:validation:Enum=clone;run
type AgentMode string

// ToolboxMode defines the mode of a toolbox VM.
// +kubebuilder:validation:Enum=run;clone;static
type ToolboxMode string

// OSType defines the guest operating system type.
// +kubebuilder:validation:Enum=linux;windows;android
type OSType string

// SnapshotPolicy defines when VM snapshots are taken.
// +kubebuilder:validation:Enum=always;main-only;never
type SnapshotPolicy string

// CocoonSetPhase represents the lifecycle phase of a CocoonSet.
// +kubebuilder:validation:Enum=Pending;Running;Scaling;Suspended;Failed
type CocoonSetPhase string

// ConnType is the connection protocol advertised for a VM. Empty
// falls back to OS-based inference (Linux→ssh, Windows→rdp, Android→adb);
// set explicitly to override (e.g. Linux+xrdp→rdp).
// +kubebuilder:validation:Enum=ssh;rdp;vnc;adb
type ConnType string

// Backend selects the hypervisor backend used to run a VM.
// Firecracker uses direct kernel boot and only supports OCI VM images
// (cloudimg URLs and Windows are rejected); the webhook and vk-cocoon
// enforce these constraints at admission and run time.
// +kubebuilder:validation:Enum=cloud-hypervisor;firecracker
type Backend string

const (
	AgentModeClone AgentMode = "clone"
	AgentModeRun   AgentMode = "run"

	ToolboxModeRun    ToolboxMode = "run"
	ToolboxModeClone  ToolboxMode = "clone"
	ToolboxModeStatic ToolboxMode = "static"

	OSLinux   OSType = "linux"
	OSWindows OSType = "windows"
	OSAndroid OSType = "android"

	SnapshotPolicyAlways   SnapshotPolicy = "always"
	SnapshotPolicyMainOnly SnapshotPolicy = "main-only"
	SnapshotPolicyNever    SnapshotPolicy = "never"

	CocoonSetPhasePending   CocoonSetPhase = "Pending"
	CocoonSetPhaseRunning   CocoonSetPhase = "Running"
	CocoonSetPhaseScaling   CocoonSetPhase = "Scaling"
	CocoonSetPhaseSuspended CocoonSetPhase = "Suspended"
	CocoonSetPhaseFailed    CocoonSetPhase = "Failed"

	ConnTypeSSH ConnType = "ssh"
	ConnTypeRDP ConnType = "rdp"
	ConnTypeVNC ConnType = "vnc"
	ConnTypeADB ConnType = "adb"

	BackendCloudHypervisor Backend = "cloud-hypervisor"
	BackendFirecracker     Backend = "firecracker"
)

// IsValid reports whether m is a recognized AgentMode value.
func (m AgentMode) IsValid() bool {
	return m == AgentModeClone || m == AgentModeRun
}

// Default returns m when set, otherwise AgentModeClone.
func (m AgentMode) Default() AgentMode {
	if m == "" {
		return AgentModeClone
	}
	return m
}

// IsValid reports whether m is a recognized ToolboxMode value.
func (m ToolboxMode) IsValid() bool {
	return m == ToolboxModeRun || m == ToolboxModeClone || m == ToolboxModeStatic
}

// Default returns m when set, otherwise ToolboxModeRun.
func (m ToolboxMode) Default() ToolboxMode {
	if m == "" {
		return ToolboxModeRun
	}
	return m
}

// IsValid reports whether o is a recognized OSType value.
func (o OSType) IsValid() bool {
	return o == OSLinux || o == OSWindows || o == OSAndroid
}

// Default returns o when set, otherwise OSLinux.
func (o OSType) Default() OSType {
	if o == "" {
		return OSLinux
	}
	return o
}

// IsValid reports whether p is a recognized SnapshotPolicy value.
func (p SnapshotPolicy) IsValid() bool {
	return p == SnapshotPolicyAlways || p == SnapshotPolicyMainOnly || p == SnapshotPolicyNever
}

// Default returns p when set, otherwise SnapshotPolicyAlways.
func (p SnapshotPolicy) Default() SnapshotPolicy {
	if p == "" {
		return SnapshotPolicyAlways
	}
	return p
}

// IsValid reports whether c is a recognized ConnType value.
func (c ConnType) IsValid() bool {
	return c == ConnTypeSSH || c == ConnTypeRDP || c == ConnTypeVNC || c == ConnTypeADB
}

// IsValid reports whether b is a recognized Backend value.
func (b Backend) IsValid() bool {
	return b == BackendCloudHypervisor || b == BackendFirecracker
}

// Default returns b when set, otherwise BackendCloudHypervisor.
func (b Backend) Default() Backend {
	if b == "" {
		return BackendCloudHypervisor
	}
	return b
}
