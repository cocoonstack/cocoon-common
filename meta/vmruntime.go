package meta

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

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
	setIfNotEmpty(a, AnnotationVNCPort, formatPort(r.VNCPort))
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
