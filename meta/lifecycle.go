package meta

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// LifecycleState is the typed contract for the lifecycle-state annotation
// vk-cocoon publishes on a Pod.
type LifecycleState string

const (
	LifecycleStateCreating    LifecycleState = "creating"
	LifecycleStateReady       LifecycleState = "ready"
	LifecycleStateHibernating LifecycleState = "hibernating"
	LifecycleStateHibernated  LifecycleState = "hibernated"
	LifecycleStateFailed      LifecycleState = "failed"
)

// IsTerminal reports whether s is a state a client would wait for.
func (s LifecycleState) IsTerminal() bool {
	switch s {
	case LifecycleStateReady, LifecycleStateHibernated, LifecycleStateFailed:
		return true
	}
	return false
}

// LifecycleStatus is the full triple (state, observed-generation, message).
// Annotations is the source of truth for what gets written; Apply
// consumes the same map in-memory and Snapshot derives a comparison
// key from the same fields.
type LifecycleStatus struct {
	State              LifecycleState
	ObservedGeneration int64
	Message            string
}

// Annotations returns the lifecycle annotation map for s. nil entries
// signal "delete this key" — pass to k8s.AnnotationsMergePatch to wrap
// into a `metadata.annotations` merge-patch body, or iterate directly
// to mutate an in-memory pod (see Apply).
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

// Apply writes Annotations into the pod's annotations, deleting keys
// whose value is nil. Empty message clears the annotation so a stale
// failure reason cannot tail into the next lifecycle.
func (s LifecycleStatus) Apply(pod *corev1.Pod) {
	if pod == nil {
		return
	}
	a := ensurePodAnnotations(pod)
	for key, val := range s.Annotations() {
		if val == nil {
			delete(a, key)
			continue
		}
		a[key] = val.(string)
	}
}

// Snapshot returns a stable comparison key. NUL separator avoids
// collisions with arbitrary message contents.
func (s LifecycleStatus) Snapshot() string {
	return string(s.State) + "\x00" + strconv.FormatInt(s.ObservedGeneration, 10) + "\x00" + s.Message
}

// ReadLifecycleStatus reads the triple from pod annotations.
func ReadLifecycleStatus(pod *corev1.Pod) LifecycleStatus {
	if pod == nil {
		return LifecycleStatus{}
	}
	return LifecycleStatus{
		State:              LifecycleState(pod.Annotations[AnnotationLifecycleState]),
		ObservedGeneration: ReadLifecycleObservedGeneration(pod),
		Message:            pod.Annotations[AnnotationLifecycleStateMessage],
	}
}

// ReadLifecycleState reads the lifecycle-state annotation, "" when missing.
func ReadLifecycleState(pod *corev1.Pod) LifecycleState {
	if pod == nil {
		return ""
	}
	return LifecycleState(pod.Annotations[AnnotationLifecycleState])
}

// ReadLifecycleObservedGeneration reads the observed-generation annotation.
// Missing or unparseable returns 0 — callers treat it as "not observed yet".
func ReadLifecycleObservedGeneration(pod *corev1.Pod) int64 {
	return readInt64Annotation(pod, AnnotationLifecycleObservedGeneration)
}

// ReadCocoonSetGeneration reads the CocoonSet generation stamped by
// cocoon-operator. vk-cocoon writes it back as observed-generation —
// counter-based completion is not subject to wallclock skew.
func ReadCocoonSetGeneration(pod *corev1.Pod) int64 {
	return readInt64Annotation(pod, AnnotationCocoonSetGeneration)
}

// StampCocoonSetGeneration writes the CocoonSet generation onto the pod.
func StampCocoonSetGeneration(pod *corev1.Pod, generation int64) {
	if pod == nil {
		return
	}
	a := ensurePodAnnotations(pod)
	a[AnnotationCocoonSetGeneration] = strconv.FormatInt(generation, 10)
}

// readInt64Annotation parses an int64-valued annotation, returning 0
// when missing or unparseable.
func readInt64Annotation(pod *corev1.Pod, key string) int64 {
	if pod == nil {
		return 0
	}
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
