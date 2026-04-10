// Package meta defines shared metadata keys and naming rules used across
// Cocoon controllers, webhooks, dashboards, and providers.
//
// All identifiers live under two cocoonstack.io subdomains:
//
//   - cocoonset.cocoonstack.io/* — CocoonSet CRD group, Pod selector
//     labels, and the declarative annotation fields mirrored from a
//     CocoonSet spec onto a managed Pod.
//   - vm.cocoonstack.io/*        — runtime state observed about the VM
//     backing a Pod (id, name, ip, vnc-port, hibernate, fork-from).
package meta

import (
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	APIVersion    = "cocoonset.cocoonstack.io/v1"
	KindCocoonSet = "CocoonSet"

	TolerationKey = "virtual-kubelet.io/provider"

	LabelCocoonSet = "cocoonset.cocoonstack.io/name"
	LabelRole      = "cocoonset.cocoonstack.io/role"
	LabelSlot      = "cocoonset.cocoonstack.io/slot"

	// Cluster-wide selector for the cocoon node pool a workload
	// targets. Used by the webhook for affinity scoping and by
	// vk-cocoon to know which node the binary is registering as.
	LabelNodePool = "cocoonstack.io/pool"

	// DefaultNodePool is the cocoon-pool name used when a CocoonSet
	// (or pod) does not specify one explicitly.
	DefaultNodePool = "default"

	// LabelManagedBy is the standard k8s "app.kubernetes.io/managed-by"
	// key the cocoonstack components stamp on resources they own
	// (per-pool affinity ConfigMaps, etc.) so cleanup tooling can
	// recognize them.
	LabelManagedBy = "app.kubernetes.io/managed-by"

	AnnotationMode           = "cocoonset.cocoonstack.io/mode"
	AnnotationImage          = "cocoonset.cocoonstack.io/image"
	AnnotationStorage        = "cocoonset.cocoonstack.io/storage"
	AnnotationManaged        = "cocoonset.cocoonstack.io/managed"
	AnnotationOS             = "cocoonset.cocoonstack.io/os"
	AnnotationSnapshotPolicy = "cocoonset.cocoonstack.io/snapshot-policy"
	AnnotationNetwork        = "cocoonset.cocoonstack.io/network"

	AnnotationVMID      = "vm.cocoonstack.io/id"
	AnnotationVMName    = "vm.cocoonstack.io/name"
	AnnotationIP        = "vm.cocoonstack.io/ip"
	AnnotationVNCPort   = "vm.cocoonstack.io/vnc-port"
	AnnotationHibernate = "vm.cocoonstack.io/hibernate"
	AnnotationForkFrom  = "vm.cocoonstack.io/fork-from"

	RoleMain     = "main"
	RoleSubAgent = "sub-agent"
	RoleToolbox  = "toolbox"

	// Connection protocol identifiers returned by ConnectionType.
	// Sharing them as constants keeps callers (operator status,
	// glance, vk-cocoon) and the function in lock-step.
	ConnTypeVNC = "vnc"
	ConnTypeADB = "adb"
	ConnTypeRDP = "rdp"
	ConnTypeSSH = "ssh"
)

// HasCocoonToleration reports whether any toleration matches the virtual-kubelet provider key.
func HasCocoonToleration(tolerations []corev1.Toleration) bool {
	return slices.ContainsFunc(tolerations, func(t corev1.Toleration) bool {
		return t.Key == TolerationKey
	})
}

// IsOwnedByCocoonSet reports whether any of the supplied owner
// references points at a CocoonSet. Webhook + operator + vk-cocoon
// all need this so it lives next to the rest of the meta helpers
// rather than each one re-implementing the loop.
func IsOwnedByCocoonSet(ownerRefs []metav1.OwnerReference) bool {
	return slices.ContainsFunc(ownerRefs, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindCocoonSet
	})
}

// OwnerDeploymentName extracts the deployment name from a ReplicaSet owner reference.
func OwnerDeploymentName(ownerRefs []metav1.OwnerReference) string {
	for _, ref := range ownerRefs {
		if ref.Kind != "ReplicaSet" {
			continue
		}
		// ReplicaSet names are `<deployment>-<pod-template-hash>`; strip
		// the trailing hash. Reject names with no prefix before the dash.
		if idx := strings.LastIndex(ref.Name, "-"); idx > 0 {
			return ref.Name[:idx]
		}
	}
	return ""
}

// VMNameForDeployment builds a deterministic VM name for a deployment slot.
func VMNameForDeployment(namespace, deployment string, slot int) string {
	// Concat is two allocations cheaper than fmt.Sprintf on this hot path.
	return "vk-" + namespace + "-" + deployment + "-" + strconv.Itoa(slot)
}

// VMNameForPod builds a deterministic VM name for a standalone pod.
func VMNameForPod(namespace, podName string) string {
	return "vk-" + namespace + "-" + podName
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
// Names with no parseable slot suffix (e.g. pod-style names produced by
// VMNameForPod) are returned unchanged so a stray dash inside the name
// can never be mistaken for a slot separator.
func MainAgentVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) < 0 {
		return vmName
	}
	idx := strings.LastIndex(vmName, "-")
	return vmName[:idx] + "-0"
}

// InferRoleFromVMName determines the role (main or sub-agent) based on the VM name slot.
func InferRoleFromVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) == 0 {
		return RoleMain
	}
	return RoleSubAgent
}

// ConnectionType returns the preferred connection protocol for the
// given OS and VNC availability.
func ConnectionType(osType string, hasVNCPort bool) string {
	switch {
	case hasVNCPort:
		return ConnTypeVNC
	case osType == "android":
		return ConnTypeADB
	case osType == "windows":
		return ConnTypeRDP
	default:
		return ConnTypeSSH
	}
}
