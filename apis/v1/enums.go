package v1

import (
	"cmp"
	"slices"
)

const (
	AgentModeClone AgentMode = "clone"
	AgentModeRun   AgentMode = "run"

	ToolboxModeRun    ToolboxMode = "run"
	ToolboxModeClone  ToolboxMode = "clone"
	ToolboxModeStatic ToolboxMode = "static"

	OSLinux   OSType = "linux"
	OSWindows OSType = "windows"
	OSAndroid OSType = "android"
	OSMacos   OSType = "macos"

	SnapshotPolicyAlways   SnapshotPolicy = "always"
	SnapshotPolicyMainOnly SnapshotPolicy = "main-only"
	SnapshotPolicyNever    SnapshotPolicy = "never"

	HibernatePolicyRetain  HibernatePolicy = "retain"
	HibernatePolicyRelease HibernatePolicy = "release"

	CocoonSetPhasePending    CocoonSetPhase = "Pending"
	CocoonSetPhaseRunning    CocoonSetPhase = "Running"
	CocoonSetPhaseScaling    CocoonSetPhase = "Scaling"
	CocoonSetPhaseSuspending CocoonSetPhase = "Suspending"
	CocoonSetPhaseSuspended  CocoonSetPhase = "Suspended"
	CocoonSetPhaseWaking     CocoonSetPhase = "Waking"
	CocoonSetPhaseMigrating  CocoonSetPhase = "Migrating"
	CocoonSetPhaseFailed     CocoonSetPhase = "Failed"

	ConnTypeSSH ConnType = "ssh"
	ConnTypeRDP ConnType = "rdp"
	ConnTypeVNC ConnType = "vnc"
	ConnTypeADB ConnType = "adb"

	BackendCloudHypervisor Backend = "cloud-hypervisor"
	BackendFirecracker     Backend = "firecracker"
)

var (
	agentModeValid       = []AgentMode{AgentModeClone, AgentModeRun}
	toolboxModeValid     = []ToolboxMode{ToolboxModeRun, ToolboxModeClone, ToolboxModeStatic}
	osTypeValid          = []OSType{OSLinux, OSWindows, OSAndroid, OSMacos}
	snapshotPolicyValid  = []SnapshotPolicy{SnapshotPolicyAlways, SnapshotPolicyMainOnly, SnapshotPolicyNever}
	hibernatePolicyValid = []HibernatePolicy{HibernatePolicyRetain, HibernatePolicyRelease}
	connTypeValid        = []ConnType{ConnTypeSSH, ConnTypeRDP, ConnTypeVNC, ConnTypeADB}
	backendValid         = []Backend{BackendCloudHypervisor, BackendFirecracker}
)

// AgentMode defines the mode of an agent VM.
// +kubebuilder:validation:Enum=clone;run
type AgentMode string

// IsValid reports whether m is a recognized AgentMode value.
func (m AgentMode) IsValid() bool { return slices.Contains(agentModeValid, m) }

// Default returns m when set, otherwise AgentModeClone.
func (m AgentMode) Default() AgentMode { return cmp.Or(m, AgentModeClone) }

// ToolboxMode defines the mode of a toolbox VM.
// +kubebuilder:validation:Enum=run;clone;static
type ToolboxMode string

// IsValid reports whether m is a recognized ToolboxMode value.
func (m ToolboxMode) IsValid() bool { return slices.Contains(toolboxModeValid, m) }

// Default returns m when set, otherwise ToolboxModeRun.
func (m ToolboxMode) Default() ToolboxMode { return cmp.Or(m, ToolboxModeRun) }

// OSType defines the guest operating system type.
// +kubebuilder:validation:Enum=linux;windows;android;macos
type OSType string

// IsValid reports whether o is a recognized OSType value.
func (o OSType) IsValid() bool { return slices.Contains(osTypeValid, o) }

// Default returns o when set, otherwise OSLinux.
func (o OSType) Default() OSType { return cmp.Or(o, OSLinux) }

// SnapshotPolicy defines when VM snapshots are taken.
// +kubebuilder:validation:Enum=always;main-only;never
type SnapshotPolicy string

// IsValid reports whether p is a recognized SnapshotPolicy value.
func (p SnapshotPolicy) IsValid() bool { return slices.Contains(snapshotPolicyValid, p) }

// Default returns p when set, otherwise SnapshotPolicyAlways.
func (p SnapshotPolicy) Default() SnapshotPolicy { return cmp.Or(p, SnapshotPolicyAlways) }

// HibernatePolicy selects the scheduling-seat semantics of a suspended
// CocoonSet: retain keeps the placeholder pod bound to its node so wake is
// same-node and guaranteed; release deletes the pods once the hibernate
// snapshot is verified in the registry, freeing capacity — wake then
// reschedules anywhere in the node pool and may wait on a free seat.
// +kubebuilder:validation:Enum=retain;release
type HibernatePolicy string

// IsValid reports whether p is a recognized HibernatePolicy value.
func (p HibernatePolicy) IsValid() bool { return slices.Contains(hibernatePolicyValid, p) }

// Default returns p when set, otherwise HibernatePolicyRetain.
func (p HibernatePolicy) Default() HibernatePolicy { return cmp.Or(p, HibernatePolicyRetain) }

// CocoonSetPhase represents the lifecycle phase of a CocoonSet.
// +kubebuilder:validation:Enum=Pending;Running;Scaling;Suspending;Suspended;Waking;Migrating;Failed
type CocoonSetPhase string

// ConnType is the connection protocol advertised for a VM. Empty
// falls back to OS-based inference (Linux→ssh, Windows→rdp, Android→adb);
// set explicitly to override (e.g. Linux+xrdp→rdp).
// +kubebuilder:validation:Enum=ssh;rdp;vnc;adb
type ConnType string

// IsValid reports whether c is a recognized ConnType value.
func (c ConnType) IsValid() bool { return slices.Contains(connTypeValid, c) }

// Backend selects the hypervisor backend used to run a VM.
// Firecracker uses direct kernel boot and only supports OCI VM images
// (cloudimg URLs and Windows are rejected); the webhook and vk-cocoon
// enforce these constraints at admission and run time.
// +kubebuilder:validation:Enum=cloud-hypervisor;firecracker
type Backend string

// IsValid reports whether b is a recognized Backend value.
func (b Backend) IsValid() bool { return slices.Contains(backendValid, b) }

// Default returns b when set, otherwise BackendCloudHypervisor.
func (b Backend) Default() Backend { return cmp.Or(b, BackendCloudHypervisor) }
