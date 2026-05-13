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
	// A pod-style name (no slot suffix) must be returned unchanged —
	// the trailing dash inside the name is not a slot separator.
	if got := MainAgentVMName("vk-prod-toolbox"); got != "vk-prod-toolbox" {
		t.Fatalf("MainAgentVMName must not coerce non-slot names, got %q", got)
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
			// The legacy ExtractSlotFromVMName would mis-read this as
			// slot 2 because it splits at the last dash. ExtractAgentSlot
			// rejects it because the suffix after the agent prefix
			// contains a dash.
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

func TestMainAgentVMNameFor(t *testing.T) {
	if got := MainAgentVMNameFor("prod", "demo"); got != "vk-prod-demo-0" {
		t.Errorf("got %q, want %q", got, "vk-prod-demo-0")
	}
}

func TestInferRoleFromAgentSlot(t *testing.T) {
	if got := InferRoleFromAgentSlot(0); got != RoleMain {
		t.Errorf("slot 0 = %q, want %q", got, RoleMain)
	}
	if got := InferRoleFromAgentSlot(7); got != RoleSubAgent {
		t.Errorf("slot 7 = %q, want %q", got, RoleSubAgent)
	}
}

func TestConnectionType(t *testing.T) {
	cases := []struct {
		name       string
		osType     string
		hasVNCPort bool
		override   string
		want       string
	}{
		{name: "vnc wins", osType: "windows", hasVNCPort: true, want: "vnc"},
		{name: "windows", osType: "windows", want: "rdp"},
		{name: "android", osType: "android", want: "adb"},
		{name: "default", osType: "linux", want: "ssh"},
		{name: "override beats os", osType: "linux", override: "rdp", want: "rdp"},
		{name: "override beats vnc port", osType: "linux", hasVNCPort: true, override: "rdp", want: "rdp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ConnectionType(tc.osType, tc.hasVNCPort, tc.override); got != tc.want {
				t.Fatalf("connection type mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestOwnerDeploymentName(t *testing.T) {
	cases := []struct {
		name   string
		owners []metav1.OwnerReference
		want   string
		wantOK bool
	}{
		{
			name:   "replicaset with hash suffix",
			owners: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "demo-7b7c9d9d5f"}},
			want:   "demo",
			wantOK: true,
		},
		{
			name:   "no owners",
			owners: nil,
			wantOK: false,
		},
		{
			name:   "non-replicaset owner",
			owners: []metav1.OwnerReference{{Kind: "Deployment", Name: "demo"}},
			wantOK: false,
		},
		{
			name:   "replicaset with no hash suffix",
			owners: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "demo"}},
			wantOK: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := OwnerDeploymentName(tt.owners)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("name = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasCocoonTolerationKey(t *testing.T) {
	tolerations := []corev1.Toleration{{Key: TolerationKey}}
	if !HasCocoonTolerationKey(tolerations) {
		t.Fatalf("expected toleration to be detected")
	}
	if HasCocoonTolerationKey(nil) {
		t.Errorf("expected nil tolerations to be rejected")
	}
	if HasCocoonTolerationKey([]corev1.Toleration{{Key: "other"}}) {
		t.Errorf("expected unrelated toleration to be rejected")
	}
}
