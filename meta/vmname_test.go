package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVMNamingHelpers(t *testing.T) {
	if got := VMNameForDeployment("prod", "demo", 2); got != "vk-prod-demo-2-496021" {
		t.Fatalf("deployment vm name mismatch: got %q", got)
	}
	if got := VMNameForPod("prod", "toolbox"); got != "vk-prod-toolbox-869218" {
		t.Fatalf("pod vm name mismatch: got %q", got)
	}
	if a, b := VMNameForDeployment("team-a", "dev", 0), VMNameForDeployment("team", "a-dev", 0); a == b {
		t.Fatalf("team-a/dev and team/a-dev share VM name %q", a)
	}
	if got := VMNameForDeployment("prod", "demo.v1", 0); got != "vk-prod-demo-v1-0-ac2538" {
		t.Fatalf("dotted cocoonset vm name mismatch: got %q", got)
	}
	if got := VMNameForDeployment("prod", "demo-v1", 0); got != "vk-prod-demo-v1-0-17548c" {
		t.Fatalf("dashed twin of a dotted cocoonset must keep its own hash: got %q", got)
	}
}

func TestToolboxAndHibernateImportNames(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"toolbox pod", ToolboxPodName("demo", "shell"), "demo-shell"},
		{"toolbox vm", VMNameForPod("prod", ToolboxPodName("demo", "shell")), "vk-prod-demo-shell-c79ab8"},
		{"hibernate import", VMNameForDeployment("prod", "demo", 0) + HibernateImportSuffix, "vk-prod-demo-0-71ea62-hibernate-import"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
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
			vmName:    VMNameForDeployment("prod", "demo", 0),
			want:      0,
		},
		{
			name:      "sub-agent",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    VMNameForDeployment("prod", "demo", 3),
			want:      3,
		},
		{
			name:      "toolbox with trailing digit is not an agent slot",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    VMNameForPod("prod", ToolboxPodName("demo", "db-2")),
			want:      -1,
		},
		{
			name:      "toolbox without trailing digit",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    VMNameForPod("prod", ToolboxPodName("demo", "toolbox")),
			want:      -1,
		},
		{
			name:      "different cocoonset",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    VMNameForDeployment("prod", "other", 0),
			want:      -1,
		},
		{
			name:      "different namespace",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    VMNameForDeployment("staging", "demo", 0),
			want:      -1,
		},
		{
			name:      "toolbox of a dashed cocoonset",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    VMNameForPod("prod", ToolboxPodName("demo-1", "t")),
			want:      -1,
		},
		{
			name:      "dotted cocoonset",
			ns:        "prod",
			cocoonSet: "demo.v1",
			vmName:    VMNameForDeployment("prod", "demo.v1", 1),
			want:      1,
		},
		{
			name:      "dashed twin of a dotted cocoonset",
			ns:        "prod",
			cocoonSet: "demo.v1",
			vmName:    VMNameForDeployment("prod", "demo-v1", 1),
			want:      -1,
		},
		{
			name:      "slot with a foreign hash",
			ns:        "prod",
			cocoonSet: "demo",
			vmName:    "vk-prod-demo-0-000000",
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
			vmName: VMNameForDeployment("ns", "cs", 0),
			want:   RoleMain,
		},
		{
			name:   "agent slot 2 is sub-agent",
			pod:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", OwnerReferences: cocoonSetOwner}},
			vmName: VMNameForDeployment("ns", "cs", 2),
			want:   RoleSubAgent,
		},
		{
			name:   "toolbox named app-0 is not main",
			pod:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", OwnerReferences: cocoonSetOwner}},
			vmName: VMNameForPod("ns", ToolboxPodName("cs", "app-0")),
			want:   RoleToolbox,
		},
		{
			name:   "no CocoonSet owner is toolbox",
			pod:    &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns"}},
			vmName: VMNameForDeployment("ns", "cs", 0),
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
