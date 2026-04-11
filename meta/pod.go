package meta

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// IsPodReady reports whether pod carries a PodReady condition set to
// True. Shared across the operator (gating sub-agent creation on the
// main agent's liveness) and any future consumer that needs the same
// check.
func IsPodReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue
	})
}

// IsPodTerminal reports whether pod has reached a phase that will not
// progress without operator intervention. Only PodFailed counts —
// PodSucceeded is left out on purpose because cocoon-managed pods
// are long-running and a Succeeded phase would be a reconciler bug
// we want surfaced as "still running" until delete catches up.
func IsPodTerminal(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return pod.Status.Phase == corev1.PodFailed
}

// IsContainerRunning reports whether any container in pod is in a
// Running state. The cocoon managed pods carry exactly one
// placeholder container so this collapses to "is that container
// running", but the loop keeps the helper generally reusable.
func IsContainerRunning(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return slices.ContainsFunc(pod.Status.ContainerStatuses, func(cs corev1.ContainerStatus) bool {
		return cs.State.Running != nil
	})
}

// PodKey is the canonical "<namespace>/<name>" key every cocoon
// component uses to index pods in in-memory tables.
func PodKey(namespace, name string) string {
	return namespace + "/" + name
}

// IsWindowsPod reports whether pod asks for a Windows guest via the
// OS annotation the operator writes through VMSpec.Apply. The match
// is case-insensitive to tolerate upstream tooling that might mix
// capitalization.
func IsWindowsPod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return strings.EqualFold(pod.Annotations[AnnotationOS], "windows")
}

// PodNodePool returns the cocoon pool a pod requests. Resolution
// order: nodeSelector[cocoonstack.io/pool] -> labels[...] ->
// annotations[...] -> DefaultNodePool. Used by the admission webhook
// for sticky-affinity scoping and by anything else that needs the
// same priority list.
func PodNodePool(pod *corev1.Pod) string {
	if pod == nil {
		return DefaultNodePool
	}
	if v := pod.Spec.NodeSelector[LabelNodePool]; v != "" {
		return v
	}
	if v := pod.Labels[LabelNodePool]; v != "" {
		return v
	}
	if v := pod.Annotations[LabelNodePool]; v != "" {
		return v
	}
	return DefaultNodePool
}
