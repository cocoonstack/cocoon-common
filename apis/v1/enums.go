package v1

// +kubebuilder:validation:Enum=clone;run

type AgentMode string

const (
	AgentModeClone AgentMode = "clone"
	AgentModeRun   AgentMode = "run"
)

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

// +kubebuilder:validation:Enum=run;clone;static
type ToolboxMode string

const (
	ToolboxModeRun    ToolboxMode = "run"
	ToolboxModeClone  ToolboxMode = "clone"
	ToolboxModeStatic ToolboxMode = "static"
)

func (m ToolboxMode) IsValid() bool {
	return m == ToolboxModeRun || m == ToolboxModeClone || m == ToolboxModeStatic
}

func (m ToolboxMode) Default() ToolboxMode {
	if m == "" {
		return ToolboxModeRun
	}
	return m
}

// +kubebuilder:validation:Enum=linux;windows;android
type OSType string

const (
	OSLinux   OSType = "linux"
	OSWindows OSType = "windows"
	OSAndroid OSType = "android"
)

func (o OSType) IsValid() bool {
	return o == OSLinux || o == OSWindows || o == OSAndroid
}

func (o OSType) Default() OSType {
	if o == "" {
		return OSLinux
	}
	return o
}

// +kubebuilder:validation:Enum=always;main-only;never
type SnapshotPolicy string

const (
	SnapshotPolicyAlways   SnapshotPolicy = "always"
	SnapshotPolicyMainOnly SnapshotPolicy = "main-only"
	SnapshotPolicyNever    SnapshotPolicy = "never"
)

func (p SnapshotPolicy) IsValid() bool {
	return p == SnapshotPolicyAlways || p == SnapshotPolicyMainOnly || p == SnapshotPolicyNever
}

func (p SnapshotPolicy) Default() SnapshotPolicy {
	if p == "" {
		return SnapshotPolicyAlways
	}
	return p
}

// +kubebuilder:validation:Enum=Pending;Running;Scaling;Suspended;Failed
type CocoonSetPhase string

const (
	CocoonSetPhasePending   CocoonSetPhase = "Pending"
	CocoonSetPhaseRunning   CocoonSetPhase = "Running"
	CocoonSetPhaseScaling   CocoonSetPhase = "Scaling"
	CocoonSetPhaseSuspended CocoonSetPhase = "Suspended"
	CocoonSetPhaseFailed    CocoonSetPhase = "Failed"
)
