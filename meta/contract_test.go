package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVMSpecApplyAndParse(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{}}
	spec := VMSpec{
		VMName:         "vk-prod-demo-0",
		Image:          "ghcr.io/cocoonstack/cocoon/ubuntu:24.04",
		Mode:           "clone",
		OS:             "linux",
		Storage:        "100G",
		Network:        "default",
		SnapshotPolicy: "always",
		ForkFrom:       "vk-prod-demo-main-0",
		Managed:        true,
	}
	spec.Apply(pod)

	got := ParseVMSpec(pod)
	if got != spec {
		t.Fatalf("roundtrip mismatch:\n got %+v\nwant %+v", got, spec)
	}
}

func TestVMSpecApplyNilPod(t *testing.T) {
	VMSpec{VMName: "x"}.Apply(nil) // must not panic
}

func TestVMSpecApplySkipsEmptyFields(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnotationImage: "preset",
	}}}
	VMSpec{VMName: "vk-x-0"}.Apply(pod)
	if pod.Annotations[AnnotationImage] != "preset" {
		t.Errorf("Apply must not clobber existing fields it has no value for")
	}
	if pod.Annotations[AnnotationVMName] != "vk-x-0" {
		t.Errorf("Apply must write non-empty fields")
	}
}

func TestVMSpecApplyManagedFlag(t *testing.T) {
	pod := &corev1.Pod{}
	VMSpec{Managed: true}.Apply(pod)
	if pod.Annotations[AnnotationManaged] != "true" {
		t.Errorf("Managed=true should set %s=true", AnnotationManaged)
	}

	pod2 := &corev1.Pod{}
	VMSpec{Managed: false}.Apply(pod2)
	if _, ok := pod2.Annotations[AnnotationManaged]; ok {
		t.Errorf("Managed=false should leave %s unset", AnnotationManaged)
	}
}

func TestParseVMSpecNilPod(t *testing.T) {
	if got := ParseVMSpec(nil); got != (VMSpec{}) {
		t.Errorf("ParseVMSpec(nil) = %+v, want zero", got)
	}
}

func TestParseVMRuntimeNilPod(t *testing.T) {
	if got := ParseVMRuntime(nil); got != (VMRuntime{}) {
		t.Errorf("ParseVMRuntime(nil) = %+v, want zero", got)
	}
}

func TestVMSpecApplyManagedFalseDoesNotClobber(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnotationManaged: annotationTrue,
	}}}
	VMSpec{VMName: "vk-x-0", Managed: false}.Apply(pod)
	if got := pod.Annotations[AnnotationManaged]; got != annotationTrue {
		t.Errorf("Managed=false must not clear an existing managed annotation, got %q", got)
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

func TestHibernateStateApplyTrue(t *testing.T) {
	pod := &corev1.Pod{}
	HibernateState(true).Apply(pod)
	if pod.Annotations[AnnotationHibernate] != annotationTrue {
		t.Errorf("HibernateState(true) should set %s=%s", AnnotationHibernate, annotationTrue)
	}
}

func TestHibernateStateApplyFalseOnNilAnnotations(t *testing.T) {
	pod := &corev1.Pod{} // pod.Annotations is nil
	// delete on a nil map must not panic.
	HibernateState(false).Apply(pod)
	if got, ok := pod.Annotations[AnnotationHibernate]; ok {
		t.Errorf("HibernateState(false) on nil annotations should remain absent, got %q", got)
	}
}

func TestHibernateStateApplyFalseNilPod(t *testing.T) {
	HibernateState(false).Apply(nil) // must not panic
}

func TestHibernateStateApplyFalseRemoves(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		AnnotationHibernate: "true",
	}}}
	HibernateState(false).Apply(pod)
	if _, ok := pod.Annotations[AnnotationHibernate]; ok {
		t.Errorf("HibernateState(false) should delete the annotation, not write false")
	}
}

func TestReadHibernateState(t *testing.T) {
	cases := []struct {
		name string
		ann  map[string]string
		want HibernateState
	}{
		{"missing", nil, false},
		{"true", map[string]string{AnnotationHibernate: "true"}, true},
		{"false-string", map[string]string{AnnotationHibernate: "false"}, false},
		{"empty", map[string]string{AnnotationHibernate: ""}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Annotations: c.ann}}
			if got := ReadHibernateState(pod); got != c.want {
				t.Errorf("ReadHibernateState = %v, want %v", got, c.want)
			}
		})
	}
}

func TestReadHibernateStateNilPod(t *testing.T) {
	if got := ReadHibernateState(nil); got {
		t.Errorf("ReadHibernateState(nil) = true, want false")
	}
}

func TestHibernateSnapshotTagConstant(t *testing.T) {
	if HibernateSnapshotTag != "hibernate" {
		t.Errorf("HibernateSnapshotTag = %q, want %q", HibernateSnapshotTag, "hibernate")
	}
}
