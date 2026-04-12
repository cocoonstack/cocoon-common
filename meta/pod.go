package meta

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// IsPodReady reports whether pod has a PodReady=True condition.
func IsPodReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue
	})
}

// IsPodTerminal reports whether pod is in PodFailed phase.
func IsPodTerminal(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return pod.Status.Phase == corev1.PodFailed
}

// IsContainerRunning reports whether any container in pod is in a Running state.
func IsContainerRunning(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return slices.ContainsFunc(pod.Status.ContainerStatuses, func(cs corev1.ContainerStatus) bool {
		return cs.State.Running != nil
	})
}

func PodKey(namespace, name string) string {
	return namespace + "/" + name
}

func IsWindowsPod(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	return strings.EqualFold(pod.Annotations[AnnotationOS], "windows")
}

// PodNodePool returns the cocoon pool from nodeSelector, labels, annotations, or DefaultNodePool.
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
