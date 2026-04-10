package v1alpha1

// AgentMode controls how an agent VM is brought up.
//
// +kubebuilder:validation:Enum=clone;run
type AgentMode string

const (
	// AgentModeClone forks the VM from a snapshot in epoch.
	AgentModeClone AgentMode = "clone"
	// AgentModeRun boots the VM cold from a cloud image.
	AgentModeRun AgentMode = "run"
)

// ToolboxMode controls how a toolbox VM is brought up.
//
// +kubebuilder:validation:Enum=run;clone;static
type ToolboxMode string

const (
	// ToolboxModeRun boots the toolbox VM cold from a cloud image.
	ToolboxModeRun ToolboxMode = "run"
	// ToolboxModeClone forks the toolbox VM from a snapshot in epoch.
	ToolboxModeClone ToolboxMode = "clone"
	// ToolboxModeStatic attaches to an externally managed VM and never
	// creates or destroys it; requires StaticIP and StaticVMID.
	ToolboxModeStatic ToolboxMode = "static"
)

// OSType is the guest operating system family.
//
// +kubebuilder:validation:Enum=linux;windows;android
type OSType string

const (
	OSLinux   OSType = "linux"
	OSWindows OSType = "windows"
	OSAndroid OSType = "android"
)

// SnapshotPolicy controls when vk-cocoon takes snapshots.
//
// +kubebuilder:validation:Enum=always;main-only;never
type SnapshotPolicy string

const (
	// SnapshotPolicyAlways snapshots every agent before destroy.
	SnapshotPolicyAlways SnapshotPolicy = "always"
	// SnapshotPolicyMainOnly snapshots only the main agent (slot 0).
	SnapshotPolicyMainOnly SnapshotPolicy = "main-only"
	// SnapshotPolicyNever skips snapshots entirely.
	SnapshotPolicyNever SnapshotPolicy = "never"
)

// CocoonSetPhase is the high-level lifecycle phase reported in status.
//
// +kubebuilder:validation:Enum=Pending;Running;Scaling;Suspended;Failed
type CocoonSetPhase string

const (
	CocoonSetPhasePending   CocoonSetPhase = "Pending"
	CocoonSetPhaseRunning   CocoonSetPhase = "Running"
	CocoonSetPhaseScaling   CocoonSetPhase = "Scaling"
	CocoonSetPhaseSuspended CocoonSetPhase = "Suspended"
	CocoonSetPhaseFailed    CocoonSetPhase = "Failed"
)
