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

// AgentVMNamePrefix returns the shared prefix every agent VM name for
// (namespace, cocoonSet) starts with: "vk-NAMESPACE-COCOONSET-".
// Use it together with ExtractAgentSlot to disambiguate agent VM names
// from toolbox VM names whose pod-derived name happens to end in -N.
func AgentVMNamePrefix(namespace, cocoonSet string) string {
	return "vk-" + namespace + "-" + cocoonSet + "-"
}

// ExtractAgentSlot parses the trailing slot index from vmName when it
// matches the agent naming convention for (namespace, cocoonSet).
// Returns -1 for any toolbox VM name even when its pod-name suffix is
// numeric (e.g. a toolbox named "db-2" produces vmName
// "vk-NS-CS-db-2" which the legacy ExtractSlotFromVMName would parse
// as slot 2 — that is a misclassification this function is meant to
// prevent).
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
// for (namespace, cocoonSet). Unlike MainAgentVMName it is a constant
// function of its inputs — no parsing — so a toolbox whose name ends
// in -N no longer hijacks the "main agent" rewrite path.
func MainAgentVMNameFor(namespace, cocoonSet string) string {
	return VMNameForDeployment(namespace, cocoonSet, 0)
}

// InferRoleFromAgentSlot returns RoleMain for slot 0 and RoleSubAgent
// for any positive slot. Callers that have already classified a pod as
// an agent (via labels or the AgentVMNamePrefix match) should pass the
// slot they extracted; pods classified as toolboxes belong to
// RoleToolbox and should not be routed through this helper.
func InferRoleFromAgentSlot(slot int) string {
	if slot == 0 {
		return RoleMain
	}
	return RoleSubAgent
}

// ExtractSlotFromVMName parses the trailing slot index from a VM name,
// or -1 if absent.
//
// Deprecated: this helper has no knowledge of agent-versus-toolbox
// naming, so it misclassifies toolbox VM names whose pod-derived
// suffix happens to be numeric (e.g. a toolbox named "db-2" reads as
// slot 2). Prefer ExtractAgentSlot for new code; existing callers that
// know they are looking at agent VM names can keep using this for now.
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

// MainAgentVMName replaces the slot suffix with 0. Non-slot names are
// returned unchanged.
//
// Deprecated: shares the toolbox-collision bug of ExtractSlotFromVMName.
// Prefer MainAgentVMNameFor(namespace, cocoonSet) for new code.
func MainAgentVMName(vmName string) string {
	if ExtractSlotFromVMName(vmName) < 0 {
		return vmName
	}
	before, _, _ := lastCut(vmName, "-")
	return before + "-0"
}

// InferRoleFromVMName returns RoleMain for slot 0, RoleSubAgent otherwise.
//
// Deprecated: shares the toolbox-collision bug of ExtractSlotFromVMName.
// Prefer InferRoleFromAgentSlot(ExtractAgentSlot(ns, cs, vmName)) for
// new code; callers that need to handle toolbox VM names should branch
// on AgentVMNamePrefix first and route toolboxes to RoleToolbox
// explicitly.
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
