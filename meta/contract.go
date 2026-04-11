package meta

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"

	cocoonv1 "github.com/cocoonstack/cocoon-common/apis/v1"
)

const (
	// HibernateSnapshotTag is the OCI tag vk-cocoon uses when pushing a
	// hibernation snapshot to epoch and the operator probes when
	// checking whether a hibernation has completed.
	HibernateSnapshotTag = "hibernate"

	// DefaultSnapshotTag is the OCI tag vk-cocoon publishes routine
	// (non-hibernate) VM snapshots under, and the tag cocoon-operator
	// garbage-collects during CocoonSet deletion. Keeping producer
	// and GC side on one constant makes the cross-component contract
	// impossible to drift.
	DefaultSnapshotTag = "latest"

	// annotationTrue is the canonical truthy annotation value.
	annotationTrue = "true"
)

// VMSpec is the typed contract that the operator (and webhook) writes
// onto a managed pod for vk-cocoon to consume. Wrapping the loose
// annotation map in a struct lets every consumer share one source of
// truth and gives the compiler a chance to catch field renames.
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
}

// Apply writes the VMSpec into a pod's annotation map. If the map
// is nil it allocates one. Empty fields are skipped so callers can
// layer partial updates without clobbering existing values.
//
// Limitation: because empty values are skipped, Apply cannot
// *clear* a previously set field. To remove an annotation use
// delete(pod.Annotations, meta.Annotation*) directly. The Managed
// flag follows the same rule — Managed=false on a pod that already
// has the managed annotation will not remove it.
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
	if s.Managed {
		a[AnnotationManaged] = annotationTrue
	}
}

// ParseVMSpec extracts a VMSpec from a pod's annotations. Missing
// fields come back as the zero value; nil pods are tolerated.
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
	}
}

// ShouldSnapshotVM reports whether the VM described by spec should
// be snapshotted to epoch at pod-delete time, and (by symmetry) whether
// its snapshot manifest should be garbage-collected when the owning
// CocoonSet is deleted. Centralizing the decoder here keeps vk-cocoon's
// producer side and cocoon-operator's GC side from drifting: if
// vk-cocoon skips the push under "main-only" for sub-agents, the
// operator must also skip the delete, otherwise the registry logs
// spurious 404s.
//
// Policies:
//
//   - never:     no VM is snapshotted or GC'd
//   - main-only: only slot-0 (the main agent) is snapshotted and GC'd
//   - always:    every VM is snapshotted and GC'd (empty defaults here)
//
// Any VMName whose slot index cannot be parsed (e.g. a toolbox VM,
// which uses VMNameForPod rather than the trailing-slot form) is
// treated as "not the main agent" and therefore excluded under the
// main-only policy.
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

// VMRuntime is the typed contract that vk-cocoon writes back onto a
// managed pod after VM creation or discovery.
//
// VNCPort is intentionally asymmetric: cocoon-operator pre-writes it
// from ToolboxSpec.VNCPort for static toolboxes (typically Windows
// VMs running on an external QEMU host with a VNC server), and
// vk-cocoon leaves it at zero for every VM it brings up itself
// because cloud-hypervisor does not expose a VNC server. A
// dynamically-created toolbox whose OS needs graphical access falls
// back to RDP/SSH via meta.ConnectionType, which is the
// deliberate behavior — not a gap in vk-cocoon's writeback.
type VMRuntime struct {
	VMID    string
	IP      string
	VNCPort int32
}

// Apply writes the VMRuntime into a pod's annotation map. If the map
// is nil it allocates one. Zero VNCPort is treated as "not set" and is
// not emitted; pass an explicit value to overwrite.
func (r VMRuntime) Apply(pod *corev1.Pod) {
	a := ensurePodAnnotations(pod)
	if a == nil {
		return
	}
	setIfNotEmpty(a, AnnotationVMID, r.VMID)
	setIfNotEmpty(a, AnnotationIP, r.IP)
	if r.VNCPort > 0 {
		a[AnnotationVNCPort] = strconv.FormatInt(int64(r.VNCPort), 10)
	}
}

// ParseVMRuntime extracts a VMRuntime from a pod's annotations.
// Missing or malformed VNCPort comes back as 0; nil pods are tolerated.
func ParseVMRuntime(pod *corev1.Pod) VMRuntime {
	if pod == nil {
		return VMRuntime{}
	}
	a := pod.Annotations
	r := VMRuntime{
		VMID: a[AnnotationVMID],
		IP:   a[AnnotationIP],
	}
	if v := a[AnnotationVNCPort]; v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			r.VNCPort = int32(n)
		}
	}
	return r
}

// HibernateState is the typed contract for the hibernate annotation.
// Truthy means the operator wants vk-cocoon to snapshot and tear down
// the VM while keeping the backing pod alive.
type HibernateState bool

// Apply writes the HibernateState into a pod's annotation map. False
// removes the annotation entirely (rather than writing "false") to
// keep the absence-as-default semantics that vk-cocoon expects.
func (s HibernateState) Apply(pod *corev1.Pod) {
	if pod == nil {
		return
	}
	if !bool(s) {
		delete(pod.Annotations, AnnotationHibernate)
		return
	}
	a := ensurePodAnnotations(pod)
	a[AnnotationHibernate] = annotationTrue
}

// ReadHibernateState extracts the HibernateState from a pod's
// annotations. Anything other than the literal string annotationTrue reads as
// false.
func ReadHibernateState(pod *corev1.Pod) HibernateState {
	if pod == nil {
		return false
	}
	return HibernateState(pod.Annotations[AnnotationHibernate] == annotationTrue)
}

// ensurePodAnnotations returns the pod's annotation map, allocating it
// if needed. Returns nil if pod itself is nil so callers can use the
// nil return as a single combined "no pod" guard.
func ensurePodAnnotations(pod *corev1.Pod) map[string]string {
	if pod == nil {
		return nil
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	return pod.Annotations
}

func setIfNotEmpty(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}
