package meta

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// HibernateImportSuffix is appended to a VM name to import a pulled hibernate snapshot beside the live VM.
const HibernateImportSuffix = "-hibernate-import"

// VMNameForDeployment names the agent VM of a CocoonSet slot after its pod, DEPLOYMENT-SLOT.
func VMNameForDeployment(namespace, deployment string, slot int) string {
	return VMNameForPod(namespace, deployment+"-"+strconv.Itoa(slot))
}

// ToolboxPodName names the pod of a CocoonSet toolbox, COCOONSET-TOOLBOX.
func ToolboxPodName(cocoonSet, toolbox string) string {
	return cocoonSet + "-" + toolbox
}

// VMNameForPod builds "vk-NAMESPACE-POD-HASH" with POD's dots as dashes; HASH is 6 hex digits of sha256(NAMESPACE/POD) on the original POD, so the name is unique, dot-free and recomputable.
func VMNameForPod(namespace, podName string) string {
	sum := sha256.Sum256([]byte(namespace + "/" + podName))
	return "vk-" + namespace + "-" + strings.ReplaceAll(podName, ".", "-") + "-" + hex.EncodeToString(sum[:3])
}

// AgentVMNamePrefix returns "vk-NAMESPACE-COCOONSET-" with COCOONSET's dots as dashes, the prefix every agent VM name shares.
func AgentVMNamePrefix(namespace, cocoonSet string) string {
	return "vk-" + namespace + "-" + strings.ReplaceAll(cocoonSet, ".", "-") + "-"
}

// ExtractAgentSlot returns the slot whose VMNameForDeployment is vmName, or -1 for any other name, toolboxes included.
func ExtractAgentSlot(namespace, cocoonSet, vmName string) int {
	rest, ok := strings.CutPrefix(vmName, AgentVMNamePrefix(namespace, cocoonSet))
	if !ok {
		return -1
	}
	slot, _, _ := strings.Cut(rest, "-")
	n, err := strconv.Atoi(slot)
	if err != nil || VMNameForDeployment(namespace, cocoonSet, n) != vmName {
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
