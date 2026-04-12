package meta

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"

	cocoonv1 "github.com/cocoonstack/cocoon-common/apis/v1"
)

const (
	HibernateSnapshotTag = "hibernate"
	DefaultSnapshotTag   = "latest"
	annotationTrue       = "true"
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
	if s.Managed {
		a[AnnotationManaged] = annotationTrue
	}
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

// VMRuntime is the typed annotation contract vk-cocoon writes back after VM creation.
type VMRuntime struct {
	VMID    string
	IP      string
	VNCPort int32
}

// Apply writes VMRuntime into pod annotations. Zero VNCPort is not emitted.
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

// ParseVMRuntime extracts a VMRuntime from pod annotations. Nil pods are tolerated.
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
type HibernateState bool

// Apply writes HibernateState into pod annotations. False removes the annotation entirely.
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

// ReadHibernateState reads the hibernate annotation from a pod.
func ReadHibernateState(pod *corev1.Pod) HibernateState {
	if pod == nil {
		return false
	}
	return HibernateState(pod.Annotations[AnnotationHibernate] == annotationTrue)
}

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
