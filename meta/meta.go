// Package meta defines shared metadata keys and naming rules used across
// Cocoon controllers, webhooks, dashboards, and providers.
package meta

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	APIVersion    = "cocoon.cis/v1alpha1"
	KindCocoonSet = "CocoonSet"

	TolerationKey = "virtual-kubelet.io/provider"

	LabelCocoonSet = "cocoon.cis/cocoonset"
	LabelRole      = "cocoon.cis/role"
	LabelSlot      = "cocoon.cis/slot"

	AnnotationMode           = "cocoon.cis/mode"
	AnnotationImage          = "cocoon.cis/image"
	AnnotationStorage        = "cocoon.cis/storage"
	AnnotationManaged        = "cocoon.cis/managed"
	AnnotationOS             = "cocoon.cis/os"
	AnnotationForkFrom       = "cocoon.cis/fork-from"
	AnnotationSnapshotPolicy = "cocoon.cis/snapshot-policy"
	AnnotationIP             = "cocoon.cis/ip"
	AnnotationVMID           = "cocoon.cis/vm-id"
	AnnotationVMName         = "cocoon.cis/vm-name"
	AnnotationVNCPort        = "cocoon.cis/vnc-port"
	AnnotationHibernate      = "cocoon.cis/hibernate"

	RoleMain     = "main"
	RoleSubAgent = "sub-agent"
	RoleToolbox  = "toolbox"
)

// HasCocoonToleration reports whether any toleration matches the virtual-kubelet provider key.
func HasCocoonToleration(tolerations []corev1.Toleration) bool {
	for _, tol := range tolerations {
		if tol.Key == TolerationKey {
			return true
		}
	}
	return false
}

// OwnerDeploymentName extracts the deployment name from a ReplicaSet owner reference.
func OwnerDeploymentName(ownerRefs []metav1.OwnerReference) string {
	for _, ref := range ownerRefs {
		if ref.Kind != "ReplicaSet" {
			continue
		}
		parts := strings.Split(ref.Name, "-")
		if len(parts) >= 2 {
			return strings.Join(parts[:len(parts)-1], "-")
		}
	}
	return ""
}

// VMNameForDeployment builds a deterministic VM name for a deployment slot.
func VMNameForDeployment(namespace, deployment string, slot int) string {
	return fmt.Sprintf("vk-%s-%s-%d", namespace, deployment, slot)
}

// VMNameForPod builds a deterministic VM name for a standalone pod.
func VMNameForPod(namespace, podName string) string {
	return fmt.Sprintf("vk-%s-%s", namespace, podName)
}

// ExtractSlotFromVMName parses the trailing slot index from a VM name, returning -1 if absent.
func ExtractSlotFromVMName(vmName string) int {
	idx := strings.LastIndex(vmName, "-")
	if idx < 0 {
		return -1
	}
	n, err := strconv.Atoi(vmName[idx+1:])
	if err != nil {
		return -1
	}
	return n
}

// MainAgentVMName replaces the slot suffix with 0 to derive the main agent name.
func MainAgentVMName(vmName string) string {
	idx := strings.LastIndex(vmName, "-")
	if idx < 0 {
		return vmName
	}
	return vmName[:idx] + "-0"
}

// InferRoleFromVMName determines the role (main or sub-agent) based on the VM name slot.
func InferRoleFromVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) == 0 {
		return RoleMain
	}
	return RoleSubAgent
}

// ConnectionType returns the preferred connection protocol for the given OS and VNC availability.
func ConnectionType(osType string, hasVNCPort bool) string {
	switch {
	case hasVNCPort:
		return "vnc"
	case osType == "android":
		return "adb"
	case osType == "windows":
		return "rdp"
	default:
		return "ssh"
	}
}
