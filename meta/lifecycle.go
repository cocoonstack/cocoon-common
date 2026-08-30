package meta

import (
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	LifecycleStateCreating    LifecycleState = "creating"
	LifecycleStateReady       LifecycleState = "ready"
	LifecycleStateHibernating LifecycleState = "hibernating"
	LifecycleStateHibernated  LifecycleState = "hibernated"
	LifecycleStateFailed      LifecycleState = "failed"
)

var terminalStates = []LifecycleState{LifecycleStateReady, LifecycleStateHibernated, LifecycleStateFailed}

// LifecycleState is the typed contract for the lifecycle-state annotation vk-cocoon publishes on a Pod.
type LifecycleState string

// IsTerminal reports whether s is a state a client would wait for.
func (s LifecycleState) IsTerminal() bool { return slices.Contains(terminalStates, s) }

// LifecycleStatus is the full triple (state, observed-generation, message).
type LifecycleStatus struct {
	State              LifecycleState
	ObservedGeneration int64
	Message            string
}

// Annotations returns the lifecycle annotation map for a merge patch, where a nil value deletes the key.
func (s LifecycleStatus) Annotations() map[string]any {
	annos := map[string]any{
		AnnotationLifecycleState:              string(s.State),
		AnnotationLifecycleObservedGeneration: strconv.FormatInt(s.ObservedGeneration, 10),
	}
	if s.Message == "" {
		annos[AnnotationLifecycleStateMessage] = nil
	} else {
		annos[AnnotationLifecycleStateMessage] = s.Message
	}
	return annos
}

// Apply writes the status onto pod annotations; an empty message clears the key so a stale reason cannot tail into the next lifecycle.
func (s LifecycleStatus) Apply(pod *corev1.Pod) {
	a := ensurePodAnnotations(pod)
	a[AnnotationLifecycleState] = string(s.State)
	a[AnnotationLifecycleObservedGeneration] = strconv.FormatInt(s.ObservedGeneration, 10)
	if s.Message == "" {
		delete(a, AnnotationLifecycleStateMessage)
	} else {
		a[AnnotationLifecycleStateMessage] = s.Message
	}
}

// Snapshot returns a stable comparison key, NUL-separated so message content cannot collide with another triple.
func (s LifecycleStatus) Snapshot() string {
	return string(s.State) + "\x00" + strconv.FormatInt(s.ObservedGeneration, 10) + "\x00" + s.Message
}

// ReadLifecycleStatus reads the triple from pod annotations.
func ReadLifecycleStatus(pod *corev1.Pod) LifecycleStatus {
	return LifecycleStatus{
		State:              LifecycleState(pod.Annotations[AnnotationLifecycleState]),
		ObservedGeneration: ReadLifecycleObservedGeneration(pod),
		Message:            pod.Annotations[AnnotationLifecycleStateMessage],
	}
}

// ReadLifecycleState reads the lifecycle-state annotation, "" when missing.
func ReadLifecycleState(pod *corev1.Pod) LifecycleState {
	return LifecycleState(pod.Annotations[AnnotationLifecycleState])
}

// ReadLifecycleObservedGeneration reads the observed-generation annotation; missing or unparseable returns 0.
func ReadLifecycleObservedGeneration(pod *corev1.Pod) int64 {
	return readInt64Annotation(pod, AnnotationLifecycleObservedGeneration)
}

// ReadCocoonSetGeneration reads the CocoonSet generation cocoon-operator stamped on the pod.
func ReadCocoonSetGeneration(pod *corev1.Pod) int64 {
	return readInt64Annotation(pod, AnnotationCocoonSetGeneration)
}

// StampCocoonSetGeneration writes the CocoonSet generation onto the pod.
func StampCocoonSetGeneration(pod *corev1.Pod, generation int64) {
	a := ensurePodAnnotations(pod)
	a[AnnotationCocoonSetGeneration] = strconv.FormatInt(generation, 10)
}

func readInt64Annotation(pod *corev1.Pod, key string) int64 {
	raw := pod.Annotations[key]
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
