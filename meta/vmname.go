package meta

import (
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// VMNameForDeployment builds a deterministic VM name from a deployment and slot index.
func VMNameForDeployment(namespace, deployment string, slot int) string {
	return "vk-" + namespace + "-" + deployment + "-" + strconv.Itoa(slot)
}

// VMNameForPod builds a deterministic VM name from a pod name.
func VMNameForPod(namespace, podName string) string {
	return "vk-" + namespace + "-" + podName
}

// AgentVMNamePrefix returns "vk-NAMESPACE-COCOONSET-", the prefix every agent VM name shares.
func AgentVMNamePrefix(namespace, cocoonSet string) string {
	return "vk-" + namespace + "-" + cocoonSet + "-"
}

// ExtractAgentSlot parses the trailing agent slot from vmName, or -1 for a toolbox name such as "vk-NS-CS-db-2".
func ExtractAgentSlot(namespace, cocoonSet, vmName string) int {
	prefix := AgentVMNamePrefix(namespace, cocoonSet)
	suffix, ok := strings.CutPrefix(vmName, prefix)
	if !ok || strings.Contains(suffix, "-") {
		return -1
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || n < 0 {
		return -1
	}
	return n
}

// InferRoleFromAgentSlot maps slot 0 to RoleMain, positive slots to RoleSubAgent, and negatives to RoleToolbox.
func InferRoleFromAgentSlot(slot int) string {
	switch {
	case slot < 0:
		return RoleToolbox
	case slot == 0:
		return RoleMain
	default:
		return RoleSubAgent
	}
}

// RoleForPod derives a pod's role from its CocoonSet owner and VM name.
func RoleForPod(pod *corev1.Pod, vmName string) string {
	cocoonSet := CocoonSetOwnerName(pod.OwnerReferences)
	return InferRoleFromAgentSlot(ExtractAgentSlot(pod.Namespace, cocoonSet, vmName))
}
