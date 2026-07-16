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
}

func TestExtractAgentSlot(t *testing.T) {
	cases := []struct {
		name      string
		ns        string
		cocoonSet string
		vmName    string
		want      int
	}{
		{
			name:      "main agent",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "vk-prod-demo-0",
			want:      0,
		},
		{
			name:      "sub-agent",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "vk-prod-demo-3",
			want:      3,
		},
		{
			// A naive last-dash split would misread this as slot 2;
			// ExtractAgentSlot rejects it because the suffix after the
			// agent prefix contains a dash.
			name:      "toolbox with trailing digit is not an agent slot",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "vk-prod-demo-db-2",
			want:      -1,
		},
		{
			name:      "toolbox without trailing digit",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "vk-prod-demo-toolbox",
			want:      -1,
		},
		{
			name:      "different cocoonset",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "vk-prod-other-0",
			want:      -1,
		},
		{
			name:      "different namespace",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "vk-staging-demo-0",
			want:      -1,
		},
		{
			name:      "non-vk prefix",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "prod-demo-0",
			want:      -1,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractAgentSlot(tt.ns, tt.cocoonSet, tt.vmName); got != tt.want {
				t.Errorf("ExtractAgentSlot(%q,%q,%q) = %d, want %d", tt.ns, tt.cocoonSet, tt.vmName, got, tt.want)
			}
		})
	}
}

func TestInferRoleFromAgentSlot(t *testing.T) {
	if got := InferRoleFromAgentSlot(0); got != RoleMain {
		t.Errorf("slot 0 = %q, want %q", got, RoleMain)
	}
	if got := InferRoleFromAgentSlot(7); got != RoleSubAgent {
		t.Errorf("slot 7 = %q, want %q", got, RoleSubAgent)
	}
	if got := InferRoleFromAgentSlot(-1); got != RoleToolbox {
		t.Errorf("slot -1 = %q, want %q", got, RoleToolbox)
	}
}

func TestRoleForPod(t *testing.T) {
	cocoonSetOwner := []metav1.OwnerReference{{Kind: KindCocoonSet, Name: "cs"}}
	cases := []struct {
		name   string
		pod    *corev1.Pod
		vmName string
		want   string
	}{
		{
			name:   "agent slot 0 is main",
			pod:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", OwnerReferences: cocoonSetOwner}},
			vmName: "vk-ns-cs-0",
			want:   RoleMain,
		},
		{
			name:   "agent slot 2 is sub-agent",
			pod:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", OwnerReferences: cocoonSetOwner}},
			vmName: "vk-ns-cs-2",
			want:   RoleSubAgent,
		},
		{
			// Regression: toolbox "app-0" builds VM name "vk-ns-cs-app-0",
			// which a naive last-dash split would misread as agent slot 0.
			name:   "toolbox named app-0 is not main",
			pod:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", OwnerReferences: cocoonSetOwner}},
			vmName: "vk-ns-cs-app-0",
			want:   RoleToolbox,
		},
		{
			name:   "no CocoonSet owner is toolbox",
			pod:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns"}},
			vmName: "vk-ns-cs-0",
			want:   RoleToolbox,
		},
		{
			name:   "nil pod is toolbox",
			pod:    nil,
			vmName: "vk-ns-cs-0",
			want:   RoleToolbox,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoleForPod(tt.pod, tt.vmName); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
