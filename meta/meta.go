// Package meta defines shared metadata keys and naming rules used across
// Cocoon controllers, webhooks, dashboards, and providers.
//
// # Annotation namespace migration
//
// Annotation keys are migrating from the legacy `cocoon.cis/*` prefix to
// two more specific subdomains under cocoonstack.io:
//
//   - cocoonset.cocoonstack.io/* — declarative fields mirrored from a
//     CocoonSet spec onto a managed Pod (mode, image, os, storage,
//     snapshot-policy, network, ...).
//   - vm.cocoonstack.io/*        — runtime state observed about the VM
//     backing a Pod (vm-id, vm-name, ip, vnc-port, hibernate, fork-from).
//
// During the migration window cocoon-operator dual-writes both the new
// and the legacy keys on every reconcile, so older provider deployments
// that still read `cocoon.cis/*` keep working until they catch up. Use
// [ReadAnnotation] for reads — it checks the canonical key first and
// falls back to the legacy key automatically — and [WriteAnnotation] /
// [AddLegacyAnnotations] for writes so the legacy key stays populated
// for the duration.
//
// The CRD API group (`cocoon.cis/v1alpha1`) and Pod selector labels
// (`cocoon.cis/cocoonset` etc.) are NOT renamed in this round. Renaming
// the API group requires migrating every existing CocoonSet CR to a new
// group; renaming the labels requires a coordinated selector cutover so
// in-flight Pods are not orphaned. Both will be moved to cocoonstack.io
// in dedicated follow-up rollouts.
package meta

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CRD identification — still on the legacy `cocoon.cis` group pending
// the CRD migration described in the package doc.
const (
	APIVersion    = "cocoon.cis/v1alpha1"
	KindCocoonSet = "CocoonSet"

	TolerationKey = "virtual-kubelet.io/provider"
)

// Selector labels — still on legacy `cocoon.cis/*` because changing them
// requires a coordinated label/selector cutover. See package doc.
const (
	LabelCocoonSet = "cocoon.cis/cocoonset"
	LabelRole      = "cocoon.cis/role"
	LabelSlot      = "cocoon.cis/slot"
)

// Annotation keys mirrored from a CocoonSet spec onto each managed Pod.
// New namespace: cocoonset.cocoonstack.io/*. Use [ReadAnnotation] /
// [WriteAnnotation] / [AddLegacyAnnotations] for backward compatibility
// during the migration.
const (
	AnnotationMode           = "cocoonset.cocoonstack.io/mode"
	AnnotationImage          = "cocoonset.cocoonstack.io/image"
	AnnotationStorage        = "cocoonset.cocoonstack.io/storage"
	AnnotationManaged        = "cocoonset.cocoonstack.io/managed"
	AnnotationOS             = "cocoonset.cocoonstack.io/os"
	AnnotationSnapshotPolicy = "cocoonset.cocoonstack.io/snapshot-policy"
	AnnotationNetwork        = "cocoonset.cocoonstack.io/network"
)

// Annotation keys describing runtime state of the VM backing a managed
// Pod. New namespace: vm.cocoonstack.io/*. Use [ReadAnnotation] /
// [WriteAnnotation] / [AddLegacyAnnotations] for backward compatibility
// during the migration.
const (
	AnnotationVMID      = "vm.cocoonstack.io/id"
	AnnotationVMName    = "vm.cocoonstack.io/name"
	AnnotationIP        = "vm.cocoonstack.io/ip"
	AnnotationVNCPort   = "vm.cocoonstack.io/vnc-port"
	AnnotationHibernate = "vm.cocoonstack.io/hibernate"
	AnnotationForkFrom  = "vm.cocoonstack.io/fork-from"
)

const (
	RoleMain     = "main"
	RoleSubAgent = "sub-agent"
	RoleToolbox  = "toolbox"
)

// legacyAnnotationKeys maps each canonical annotation constant to its
// pre-rename `cocoon.cis/*` equivalent. Reads fall through this map via
// [ReadAnnotation]; writes mirror to it via [WriteAnnotation] /
// [AddLegacyAnnotations] during the migration window.
var legacyAnnotationKeys = map[string]string{
	AnnotationMode:           "cocoon.cis/mode",
	AnnotationImage:          "cocoon.cis/image",
	AnnotationStorage:        "cocoon.cis/storage",
	AnnotationManaged:        "cocoon.cis/managed",
	AnnotationOS:             "cocoon.cis/os",
	AnnotationSnapshotPolicy: "cocoon.cis/snapshot-policy",
	AnnotationNetwork:        "cocoon.cis/network",
	AnnotationVMID:           "cocoon.cis/vm-id",
	AnnotationVMName:         "cocoon.cis/vm-name",
	AnnotationIP:             "cocoon.cis/ip",
	AnnotationVNCPort:        "cocoon.cis/vnc-port",
	AnnotationHibernate:      "cocoon.cis/hibernate",
	AnnotationForkFrom:       "cocoon.cis/fork-from",
}

// LegacyAnnotationKey returns the pre-rename `cocoon.cis/*` equivalent
// of a canonical annotation key, or "" if there is no legacy mapping.
// Used by patch builders that want to mirror a single annotation update
// onto both keys in one PATCH request.
func LegacyAnnotationKey(key string) string {
	return legacyAnnotationKeys[key]
}

// ReadAnnotation returns annotations[key] if present, otherwise the
// value at the legacy `cocoon.cis/*` equivalent. Use this for every
// annotation read during the migration so providers pinned to the old
// keys keep working until they catch up.
func ReadAnnotation(annotations map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	if v, ok := annotations[key]; ok {
		return v
	}
	if legacy := LegacyAnnotationKey(key); legacy != "" {
		return annotations[legacy]
	}
	return ""
}

// WriteAnnotation sets m[key] = value AND its legacy `cocoon.cis/*`
// equivalent (when one exists), so older providers that have not yet
// caught up to the rename can still read the value. No-op when m is nil.
func WriteAnnotation(m map[string]string, key, value string) {
	if m == nil {
		return
	}
	m[key] = value
	if legacy := LegacyAnnotationKey(key); legacy != "" {
		m[legacy] = value
	}
}

// AddLegacyAnnotations walks m and, for every canonical annotation key
// it finds, copies the value to its legacy `cocoon.cis/*` equivalent.
// Convenience for writers that build their annotation map with literal
// canonical keys upfront and want to emit the legacy mirror in one shot
// at the end. No-op when m is nil.
func AddLegacyAnnotations(m map[string]string) {
	if m == nil {
		return
	}
	// Snapshot the (legacy, value) pairs so we don't mutate m during
	// the range — Go map iteration over a mutating map is undefined.
	type pair struct{ legacy, value string }
	var pairs []pair
	for k, v := range m {
		if legacy := LegacyAnnotationKey(k); legacy != "" {
			pairs = append(pairs, pair{legacy, v})
		}
	}
	for _, p := range pairs {
		m[p.legacy] = p.value
	}
}

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
