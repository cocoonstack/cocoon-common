package meta

import (
	corev1 "k8s.io/api/core/v1"

	cocoonv1 "github.com/cocoonstack/cocoon-common/apis/v1"
)

// VMSpec is the typed annotation contract the operator writes for vk-cocoon to consume.
type VMSpec struct {
	VMName         string
	Image          string
	Mode           string
	OS             string
	Storage        string
	Network        string
	SnapshotPolicy string
	ForkFrom       string
	Managed        bool
	ForcePull      bool
	NoDirectIO     bool
	ConnType       string
	Backend        string
	ProbePort      string
}

// Apply writes VMSpec into pod annotations. Empty fields are skipped (cannot clear existing values).
func (s VMSpec) Apply(pod *corev1.Pod) {
	a := ensurePodAnnotations(pod)
	if a == nil {
		return
	}
	setIfNotEmpty(a, AnnotationVMName, s.VMName)
	setIfNotEmpty(a, AnnotationImage, s.Image)
	setIfNotEmpty(a, AnnotationMode, s.Mode)
	setIfNotEmpty(a, AnnotationOS, s.OS)
	setIfNotEmpty(a, AnnotationStorage, s.Storage)
	setIfNotEmpty(a, AnnotationNetwork, s.Network)
	setIfNotEmpty(a, AnnotationSnapshotPolicy, s.SnapshotPolicy)
	setIfNotEmpty(a, AnnotationForkFrom, s.ForkFrom)
	setIfNotEmpty(a, AnnotationConnType, s.ConnType)
	setIfNotEmpty(a, AnnotationBackend, s.Backend)
	if s.Managed {
		a[AnnotationManaged] = annotationTrue
	}
	if s.ForcePull {
		a[AnnotationForcePull] = annotationTrue
	}
	if s.NoDirectIO {
		a[AnnotationNoDirectIO] = annotationTrue
	}
	setIfNotEmpty(a, AnnotationProbePort, s.ProbePort)
}

// ParseVMSpec extracts a VMSpec from pod annotations. Nil pods are tolerated.
func ParseVMSpec(pod *corev1.Pod) VMSpec {
	if pod == nil {
		return VMSpec{}
	}
	a := pod.Annotations
	return VMSpec{
		VMName:         a[AnnotationVMName],
		Image:          a[AnnotationImage],
		Mode:           a[AnnotationMode],
		OS:             a[AnnotationOS],
		Storage:        a[AnnotationStorage],
		Network:        a[AnnotationNetwork],
		SnapshotPolicy: a[AnnotationSnapshotPolicy],
		ForkFrom:       a[AnnotationForkFrom],
		Managed:        a[AnnotationManaged] == annotationTrue,
		ForcePull:      a[AnnotationForcePull] == annotationTrue,
		NoDirectIO:     a[AnnotationNoDirectIO] == annotationTrue,
		ConnType:       a[AnnotationConnType],
		Backend:        a[AnnotationBackend],
		ProbePort:      a[AnnotationProbePort],
	}
}

// FromAgentSpec builds a VMSpec from an AgentSpec. Agent VMs are always managed.
func FromAgentSpec(spec cocoonv1.AgentSpec, vmName string, snapshotPolicy cocoonv1.SnapshotPolicy, forkFrom string) VMSpec {
	return VMSpec{
		VMName:         vmName,
		Image:          spec.Image,
		Mode:           string(spec.Mode.Default()),
		OS:             string(spec.OS.Default()),
		Storage:        QuantityString(spec.Storage),
		Network:        spec.Network,
		SnapshotPolicy: string(snapshotPolicy.Default()),
		ForkFrom:       forkFrom,
		Managed:        true,
		ForcePull:      spec.ForcePull,
		NoDirectIO:     spec.NoDirectIO,
		ConnType:       string(spec.ConnType),
		Backend:        string(spec.Backend.Default()),
		ProbePort:      formatPort(spec.ProbePort),
	}
}

// FromToolboxSpec builds a VMSpec from a ToolboxSpec. Static-mode toolboxes are unmanaged.
func FromToolboxSpec(spec cocoonv1.ToolboxSpec, vmName string, snapshotPolicy cocoonv1.SnapshotPolicy) VMSpec {
	return VMSpec{
		VMName:         vmName,
		Image:          spec.Image,
		Mode:           string(spec.Mode.Default()),
		OS:             string(spec.OS.Default()),
		Storage:        QuantityString(spec.Storage),
		Network:        spec.Network,
		SnapshotPolicy: string(snapshotPolicy.Default()),
		Managed:        spec.Mode != cocoonv1.ToolboxModeStatic,
		ForcePull:      spec.ForcePull,
		NoDirectIO:     spec.NoDirectIO,
		ConnType:       string(spec.ConnType),
		Backend:        string(spec.Backend.Default()),
		ProbePort:      formatPort(spec.ProbePort),
	}
}

// ShouldSnapshotVM reports whether the VM should be snapshotted based on its SnapshotPolicy.
func ShouldSnapshotVM(spec VMSpec) bool {
	switch cocoonv1.SnapshotPolicy(spec.SnapshotPolicy).Default() {
	case cocoonv1.SnapshotPolicyNever:
		return false
	case cocoonv1.SnapshotPolicyMainOnly:
		return ExtractSlotFromVMName(spec.VMName) == 0
	default:
		return true
	}
}
