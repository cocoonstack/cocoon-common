// Package meta defines shared metadata keys and naming rules used across Cocoon components.
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

	LabelNodePool   = "cocoonstack.io/pool"
	DefaultNodePool = "default"
	LabelManagedBy  = "app.kubernetes.io/managed-by"

	AnnotationMode           = "cocoonset.cocoonstack.io/mode"
	AnnotationImage          = "cocoonset.cocoonstack.io/image"
	AnnotationStorage        = "cocoonset.cocoonstack.io/storage"
	AnnotationManaged        = "cocoonset.cocoonstack.io/managed"
	AnnotationOS             = "cocoonset.cocoonstack.io/os"
	AnnotationSnapshotPolicy = "cocoonset.cocoonstack.io/snapshot-policy"
	AnnotationNetwork        = "cocoonset.cocoonstack.io/network"
	AnnotationForcePull      = "cocoonset.cocoonstack.io/force-pull"

	AnnotationVMID       = "vm.cocoonstack.io/id"
	AnnotationVMName     = "vm.cocoonstack.io/name"
	AnnotationIP         = "vm.cocoonstack.io/ip"
	AnnotationVNCPort    = "vm.cocoonstack.io/vnc-port"
	AnnotationHibernate  = "vm.cocoonstack.io/hibernate"
	AnnotationForkFrom   = "vm.cocoonstack.io/fork-from"
	AnnotationConnType   = "vm.cocoonstack.io/conn-type"
	AnnotationBackend    = "vm.cocoonstack.io/backend"
	AnnotationNoDirectIO = "vm.cocoonstack.io/no-direct-io"
	AnnotationProbePort  = "vm.cocoonstack.io/probe-port"

	RoleMain     = "main"
	RoleSubAgent = "sub-agent"
	RoleToolbox  = "toolbox"

	ConnTypeVNC = "vnc"
	ConnTypeADB = "adb"
	ConnTypeRDP = "rdp"
	ConnTypeSSH = "ssh"
)

// HasCocoonToleration reports whether the toleration list includes the virtual-kubelet provider key.
func HasCocoonToleration(tolerations []corev1.Toleration) bool {
	return slices.ContainsFunc(tolerations, func(t corev1.Toleration) bool {
		return t.Key == TolerationKey
	})
}

// IsOwnedByCocoonSet reports whether any owner reference is a CocoonSet.
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
		if before, _, ok := lastCut(ref.Name, "-"); ok {
			return before
		}
	}
	return ""
}

// VMNameForDeployment builds a deterministic VM name from a deployment and slot index.
func VMNameForDeployment(namespace, deployment string, slot int) string {
	return "vk-" + namespace + "-" + deployment + "-" + strconv.Itoa(slot)
}

// VMNameForPod builds a deterministic VM name from a pod name.
func VMNameForPod(namespace, podName string) string {
	return "vk-" + namespace + "-" + podName
}

// ExtractSlotFromVMName parses the trailing slot index from a VM name, or -1 if absent.
func ExtractSlotFromVMName(vmName string) int {
	_, after, ok := lastCut(vmName, "-")
	if !ok {
		return -1
	}
	n, err := strconv.Atoi(after)
	if err != nil {
		return -1
	}
	return n
}

// MainAgentVMName replaces the slot suffix with 0. Non-slot names are returned unchanged.
func MainAgentVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) < 0 {
		return vmName
	}
	before, _, _ := lastCut(vmName, "-")
	return before + "-0"
}

// InferRoleFromVMName returns RoleMain for slot 0, RoleSubAgent otherwise.
func InferRoleFromVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) == 0 {
		return RoleMain
	}
	return RoleSubAgent
}

// ConnectionType returns the connection protocol. A non-empty override
// (typically AnnotationConnType) wins over OS-based inference, so a Linux
// image running xrdp can advertise rdp without faking its OS field.
func ConnectionType(osType string, hasVNCPort bool, override string) string {
	if override != "" {
		return override
	}
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

// lastCut is like strings.Cut but splits at the last occurrence of sep.
func lastCut(s, sep string) (before, after string, found bool) {
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}
