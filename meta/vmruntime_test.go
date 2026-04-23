package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseVMRuntimeNilPod(t *testing.T) {
	if got := ParseVMRuntime(nil); got != (VMRuntime{}) {
		t.Errorf("ParseVMRuntime(nil) = %+v, want zero", got)
	}
}

func TestVMRuntimeApplyAndParse(t *testing.T) {
	pod := &corev1.Pod{}
	r := VMRuntime{VMID: "qemu-1234", IP: "10.88.100.7", VNCPort: 5901}
	r.Apply(pod)
	if got := ParseVMRuntime(pod); got != r {
		t.Fatalf("roundtrip mismatch:\n got %+v\nwant %+v", got, r)
	}
}

func TestVMRuntimeApplyZeroVNCPortNotEmitted(t *testing.T) {
	pod := &corev1.Pod{}
	VMRuntime{VMID: "id", IP: "1.2.3.4"}.Apply(pod)
	if _, ok := pod.Annotations[AnnotationVNCPort]; ok {
		t.Errorf("zero VNCPort should not emit annotation")
	}
}

func TestVMRuntimeParseMalformedVNCPort(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnotationVNCPort: "not-a-number",
	}}}
	if got := ParseVMRuntime(pod); got.VNCPort != 0 {
		t.Errorf("malformed VNCPort should parse as 0, got %d", got.VNCPort)
	}
}

func TestVMRuntimeApplyNilPod(t *testing.T) {
	VMRuntime{VMID: "x"}.Apply(nil) // must not panic
}
