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

	AnnotationVMID      = "vm.cocoonstack.io/id"
	AnnotationVMName    = "vm.cocoonstack.io/name"
	AnnotationIP        = "vm.cocoonstack.io/ip"
	AnnotationVNCPort   = "vm.cocoonstack.io/vnc-port"
	AnnotationHibernate = "vm.cocoonstack.io/hibernate"
	AnnotationForkFrom  = "vm.cocoonstack.io/fork-from"

	RoleMain     = "main"
	RoleSubAgent = "sub-agent"
	RoleToolbox  = "toolbox"

	ConnTypeVNC = "vnc"
	ConnTypeADB = "adb"
	ConnTypeRDP = "rdp"
	ConnTypeSSH = "ssh"
)

func HasCocoonToleration(tolerations []corev1.Toleration) bool {
	return slices.ContainsFunc(tolerations, func(t corev1.Toleration) bool {
		return t.Key == TolerationKey
	})
}

func IsOwnedByCocoonSet(ownerRefs []metav1.OwnerReference) bool {
	return slices.ContainsFunc(ownerRefs, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindCocoonSet
	})
}

func OwnerDeploymentName(ownerRefs []metav1.OwnerReference) string {
	for _, ref := range ownerRefs {
		if ref.Kind != "ReplicaSet" {
			continue
		}
		if idx := strings.LastIndex(ref.Name, "-"); idx > 0 {
			return ref.Name[:idx]
		}
	}
	return ""
}

func VMNameForDeployment(namespace, deployment string, slot int) string {
	return "vk-" + namespace + "-" + deployment + "-" + strconv.Itoa(slot)
}

func VMNameForPod(namespace, podName string) string {
	return "vk-" + namespace + "-" + podName
}

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

// MainAgentVMName replaces the slot suffix with 0. Non-slot names are returned unchanged.
func MainAgentVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) < 0 {
		return vmName
	}
	idx := strings.LastIndex(vmName, "-")
	return vmName[:idx] + "-0"
}

func InferRoleFromVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) == 0 {
		return RoleMain
	}
	return RoleSubAgent
}

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
