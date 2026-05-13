package meta

import (
	"strconv"
	"strings"
)

// VMNameForDeployment builds a deterministic VM name from a deployment and slot index.
func VMNameForDeployment(namespace, deployment string, slot int) string {
	return "vk-" + namespace + "-" + deployment + "-" + strconv.Itoa(slot)
}

// VMNameForPod builds a deterministic VM name from a pod name.
func VMNameForPod(namespace, podName string) string {
	return "vk-" + namespace + "-" + podName
}

// AgentVMNamePrefix returns "vk-NAMESPACE-COCOONSET-", the prefix every
// agent VM name shares.
func AgentVMNamePrefix(namespace, cocoonSet string) string {
	return "vk-" + namespace + "-" + cocoonSet + "-"
}

// ExtractAgentSlot parses the trailing slot index from vmName when it
// matches the agent naming convention for (namespace, cocoonSet), or
// -1 for any toolbox VM name (e.g. "vk-NS-CS-db-2").
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

// MainAgentVMNameFor returns the VM name of the main (slot 0) agent
// for (namespace, cocoonSet).
func MainAgentVMNameFor(namespace, cocoonSet string) string {
	return VMNameForDeployment(namespace, cocoonSet, 0)
}

// InferRoleFromAgentSlot returns RoleMain for slot 0, RoleSubAgent for
// positive slots, RoleToolbox for slot < 0.
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

// ExtractSlotFromVMName parses the trailing slot index from a VM name,
// or -1 if absent.
//
// Deprecated: misclassifies toolbox names with numeric suffixes (e.g.
// "vk-NS-CS-db-2" → slot 2). Prefer ExtractAgentSlot.
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

// InferRoleFromVMName returns RoleMain for slot 0, RoleSubAgent otherwise.
//
// Deprecated: shares the toolbox-collision bug of ExtractSlotFromVMName.
// Prefer InferRoleFromAgentSlot(ExtractAgentSlot(ns, cs, vmName)).
func InferRoleFromVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) == 0 {
		return RoleMain
	}
	return RoleSubAgent
}

// lastCut is like strings.Cut but splits at the last occurrence of sep.
func lastCut(s, sep string) (before, after string, found bool) {
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}
