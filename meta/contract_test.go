package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cocoonv1 "github.com/cocoonstack/cocoon-common/apis/v1"
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
		ForcePull:      true,
		ConnType:       "rdp",
		Backend:        "firecracker",
	}
	spec.Apply(pod)

	// Anchor the annotation key contract — a roundtrip-only assertion
	// would silently pass if Apply and Parse both used the wrong key.
	wantKeys := map[string]string{
		AnnotationVMName:         spec.VMName,
		AnnotationImage:          spec.Image,
		AnnotationMode:           spec.Mode,
		AnnotationOS:             spec.OS,
		AnnotationStorage:        spec.Storage,
		AnnotationNetwork:        spec.Network,
		AnnotationSnapshotPolicy: spec.SnapshotPolicy,
		AnnotationForkFrom:       spec.ForkFrom,
		AnnotationManaged:        annotationTrue,
		AnnotationForcePull:      annotationTrue,
		AnnotationConnType:       spec.ConnType,
		AnnotationBackend:        spec.Backend,
	}
	for key, want := range wantKeys {
		if got := pod.Annotations[key]; got != want {
			t.Errorf("annotation %q = %q, want %q", key, got, want)
		}
	}

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

func TestDefaultSnapshotTagConstant(t *testing.T) {
	if DefaultSnapshotTag != "latest" {
		t.Errorf("DefaultSnapshotTag = %q, want %q", DefaultSnapshotTag, "latest")
	}
	if DefaultSnapshotTag == HibernateSnapshotTag {
		t.Errorf("DefaultSnapshotTag must differ from HibernateSnapshotTag")
	}
}

func TestShouldSnapshotVM(t *testing.T) {
	cases := []struct {
		name   string
		policy cocoonv1.SnapshotPolicy
		vmName string
		want   bool
	}{
		{"always/slot0", cocoonv1.SnapshotPolicyAlways, "vk-prod-demo-0", true},
		{"always/slot3", cocoonv1.SnapshotPolicyAlways, "vk-prod-demo-3", true},
		{"always/toolbox", cocoonv1.SnapshotPolicyAlways, "vk-prod-my-tb", true},
		{"empty-defaults-to-always", "", "vk-prod-demo-0", true},
		{"empty-defaults-to-always/sub", "", "vk-prod-demo-2", true},

		{"never/slot0", cocoonv1.SnapshotPolicyNever, "vk-prod-demo-0", false},
		{"never/slot3", cocoonv1.SnapshotPolicyNever, "vk-prod-demo-3", false},
		{"never/toolbox", cocoonv1.SnapshotPolicyNever, "vk-prod-my-tb", false},

		{"main-only/slot0", cocoonv1.SnapshotPolicyMainOnly, "vk-prod-demo-0", true},
		{"main-only/slot3", cocoonv1.SnapshotPolicyMainOnly, "vk-prod-demo-3", false},
		{"main-only/toolbox", cocoonv1.SnapshotPolicyMainOnly, "vk-prod-my-tb", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := VMSpec{VMName: c.vmName, SnapshotPolicy: string(c.policy)}
			if got := ShouldSnapshotVM(spec); got != c.want {
				t.Errorf("ShouldSnapshotVM(%s, %q) = %v, want %v", c.policy, c.vmName, got, c.want)
			}
		})
	}
}
