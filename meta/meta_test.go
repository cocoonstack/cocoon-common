package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVMNamingHelpers(t *testing.T) {
	if got := VMNameForDeployment("prod", "demo", 2); got != "vk-prod-demo-2" {
		t.Fatalf("deployment vm name mismatch: got %q", got)
	}
	if got := VMNameForPod("prod", "toolbox"); got != "vk-prod-toolbox" {
		t.Fatalf("pod vm name mismatch: got %q", got)
	}
	if got := ExtractSlotFromVMName("vk-prod-demo-2"); got != 2 {
		t.Fatalf("slot mismatch: got %d", got)
	}
	if got := ExtractSlotFromVMName("vk-prod-toolbox"); got != -1 {
		t.Fatalf("expected non-slot vm name to return -1, got %d", got)
	}
	if got := MainAgentVMName("vk-prod-demo-2"); got != "vk-prod-demo-0" {
		t.Fatalf("main agent name mismatch: got %q", got)
	}
}

func TestInferRoleFromVMName(t *testing.T) {
	if got := InferRoleFromVMName("vk-prod-demo-0"); got != RoleMain {
		t.Fatalf("expected role %q, got %q", RoleMain, got)
	}
	if got := InferRoleFromVMName("vk-prod-demo-3"); got != RoleSubAgent {
		t.Fatalf("expected role %q, got %q", RoleSubAgent, got)
	}
}

func TestConnectionType(t *testing.T) {
	cases := []struct {
		name       string
		osType     string
		hasVNCPort bool
		want       string
	}{
		{name: "vnc wins", osType: "windows", hasVNCPort: true, want: "vnc"},
		{name: "windows", osType: "windows", want: "rdp"},
		{name: "android", osType: "android", want: "adb"},
		{name: "default", osType: "linux", want: "ssh"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ConnectionType(tc.osType, tc.hasVNCPort); got != tc.want {
				t.Fatalf("connection type mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestOwnerDeploymentName(t *testing.T) {
	ownerRefs := []metav1.OwnerReference{
		{Kind: "ReplicaSet", Name: "demo-7b7c9d9d5f"},
	}
	if got := OwnerDeploymentName(ownerRefs); got != "demo" {
		t.Fatalf("deployment name mismatch: got %q", got)
	}
}

func TestHasCocoonToleration(t *testing.T) {
	tolerations := []corev1.Toleration{{Key: TolerationKey}}
	if !HasCocoonToleration(tolerations) {
		t.Fatalf("expected toleration to be detected")
	}
}

func TestLegacyAnnotationKey(t *testing.T) {
	cases := []struct {
		canonical string
		legacy    string
	}{
		{AnnotationMode, "cocoon.cis/mode"},
		{AnnotationImage, "cocoon.cis/image"},
		{AnnotationVMID, "cocoon.cis/vm-id"},
		{AnnotationVMName, "cocoon.cis/vm-name"},
		{AnnotationHibernate, "cocoon.cis/hibernate"},
		{AnnotationNetwork, "cocoon.cis/network"},
		{"cocoonset.cocoonstack.io/unknown", ""}, // not in the map
	}
	for _, c := range cases {
		t.Run(c.canonical, func(t *testing.T) {
			if got := LegacyAnnotationKey(c.canonical); got != c.legacy {
				t.Fatalf("LegacyAnnotationKey(%q) = %q, want %q", c.canonical, got, c.legacy)
			}
		})
	}
}

func TestReadAnnotationPrefersCanonical(t *testing.T) {
	annotations := map[string]string{
		AnnotationMode:    "clone",
		"cocoon.cis/mode": "run", // legacy mirror, must lose to canonical
	}
	if got := ReadAnnotation(annotations, AnnotationMode); got != "clone" {
		t.Fatalf("ReadAnnotation canonical preference: got %q, want %q", got, "clone")
	}
}

func TestReadAnnotationFallsBackToLegacy(t *testing.T) {
	annotations := map[string]string{"cocoon.cis/image": "ubuntu:24.04"}
	if got := ReadAnnotation(annotations, AnnotationImage); got != "ubuntu:24.04" {
		t.Fatalf("ReadAnnotation legacy fallback: got %q, want %q", got, "ubuntu:24.04")
	}
}

func TestReadAnnotationNilMap(t *testing.T) {
	if got := ReadAnnotation(nil, AnnotationMode); got != "" {
		t.Fatalf("ReadAnnotation(nil) = %q, want empty", got)
	}
}

func TestReadAnnotationMissing(t *testing.T) {
	annotations := map[string]string{"unrelated": "x"}
	if got := ReadAnnotation(annotations, AnnotationMode); got != "" {
		t.Fatalf("ReadAnnotation missing: got %q, want empty", got)
	}
}

func TestWriteAnnotationMirrorsLegacy(t *testing.T) {
	m := map[string]string{}
	WriteAnnotation(m, AnnotationVMID, "abc123")
	if got := m[AnnotationVMID]; got != "abc123" {
		t.Errorf("canonical key not set: %q", got)
	}
	if got := m["cocoon.cis/vm-id"]; got != "abc123" {
		t.Errorf("legacy mirror not set: %q", got)
	}
}

func TestWriteAnnotationNoLegacyMapping(t *testing.T) {
	m := map[string]string{}
	WriteAnnotation(m, "cocoonset.cocoonstack.io/unknown", "v")
	if got := m["cocoonset.cocoonstack.io/unknown"]; got != "v" {
		t.Errorf("canonical key not set: %q", got)
	}
	if _, ok := m["cocoon.cis/unknown"]; ok {
		t.Errorf("unexpected legacy key created for unmapped canonical")
	}
}

func TestWriteAnnotationNilMapNoOp(t *testing.T) {
	WriteAnnotation(nil, AnnotationMode, "clone") // must not panic
}

func TestAddLegacyAnnotationsMirrorsAll(t *testing.T) {
	m := map[string]string{
		AnnotationMode:  "clone",
		AnnotationImage: "ubuntu:24.04",
		AnnotationVMID:  "vm-1",
		"unrelated":     "x",
	}
	AddLegacyAnnotations(m)
	wants := map[string]string{
		"cocoon.cis/mode":  "clone",
		"cocoon.cis/image": "ubuntu:24.04",
		"cocoon.cis/vm-id": "vm-1",
	}
	for k, v := range wants {
		if got := m[k]; got != v {
			t.Errorf("legacy mirror %q = %q, want %q", k, got, v)
		}
	}
	if got := m["unrelated"]; got != "x" {
		t.Errorf("unrelated key was disturbed: %q", got)
	}
	if _, ok := m["cocoon.cis/unrelated"]; ok {
		t.Errorf("unrelated key spawned a legacy mirror")
	}
}

func TestAddLegacyAnnotationsNilMapNoOp(t *testing.T) {
	AddLegacyAnnotations(nil) // must not panic
}
