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

// lastCut is like strings.Cut but splits at the last occurrence of sep.
func lastCut(s, sep string) (before, after string, found bool) {
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}
