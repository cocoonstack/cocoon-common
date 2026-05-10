package meta

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// LifecycleState is the typed contract for the lifecycle-state annotation
// vk-cocoon publishes on a Pod. Clients poll the annotation triple
// (state, observed-generation, message) for non-stale completion detection.
type LifecycleState string

const (
	LifecycleStateCreating    LifecycleState = "creating"
	LifecycleStateReady       LifecycleState = "ready"
	LifecycleStateHibernating LifecycleState = "hibernating"
	LifecycleStateHibernated  LifecycleState = "hibernated"
	LifecycleStateFailed      LifecycleState = "failed"
)

// LifecycleStatus is the full triple (state, observed-generation, message)
// vk-cocoon writes atomically. Apply, PatchPayload, and Snapshot keep the
// in-memory pod, the apiserver patch body, and the drift-comparison key
// in sync — callers that touch one path go through this struct so they
// cannot diverge.
type LifecycleStatus struct {
	State              LifecycleState
	ObservedGeneration int64
	Message            string
}

// IsTerminal reports whether s is one of the terminal states a client
// would wait for (ready, hibernated, failed). Transient states
// (creating, hibernating) return false.
func (s LifecycleState) IsTerminal() bool {
	switch s {
	case LifecycleStateReady, LifecycleStateHibernated, LifecycleStateFailed:
		return true
	}
	return false
}

// Apply writes the LifecycleStatus into the pod's annotations. An empty
// message removes the message annotation so a stale failure reason from
// a prior lifecycle does not tail along into the next one.
func (s LifecycleStatus) Apply(pod *corev1.Pod) {
	if pod == nil {
		return
	}
	a := ensurePodAnnotations(pod)
	a[AnnotationLifecycleState] = string(s.State)
	a[AnnotationLifecycleObservedGeneration] = strconv.FormatInt(s.ObservedGeneration, 10)
	if s.Message == "" {
		delete(a, AnnotationLifecycleStateMessage)
	} else {
		a[AnnotationLifecycleStateMessage] = s.Message
	}
}

// PatchPayload returns the strategic-merge value map for s. Empty
// message uses a nil entry so the apiserver deletes the key — the
// payload mirrors what Apply does in-memory.
func (s LifecycleStatus) PatchPayload() map[string]any {
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

// Snapshot returns a stable comparison key. Uses NUL as separator so
// arbitrary message contents (error strings can contain '|', '\n', etc.)
// cannot collide with the join.
func (s LifecycleStatus) Snapshot() string {
	return string(s.State) + "\x00" + strconv.FormatInt(s.ObservedGeneration, 10) + "\x00" + s.Message
}

// ReadLifecycleStatus returns the LifecycleStatus stored in the pod's
// annotations. Missing or unparseable observed-generation falls back to 0.
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

// ReadLifecycleState returns the lifecycle-state annotation on the pod,
// or an empty string when the annotation is missing.
func ReadLifecycleState(pod *corev1.Pod) LifecycleState {
	if pod == nil {
		return ""
	}
	return LifecycleState(pod.Annotations[AnnotationLifecycleState])
}

// ReadLifecycleObservedGeneration returns the observed-generation paired
// with the lifecycle-state annotation. Returns 0 when missing or
// unparseable so callers treat absence as "not observed yet".
func ReadLifecycleObservedGeneration(pod *corev1.Pod) int64 {
	if pod == nil {
		return 0
	}
	raw := pod.Annotations[AnnotationLifecycleObservedGeneration]
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// ReadCocoonSetGeneration returns the owning CocoonSet's metadata.generation
// as stamped by cocoon-operator on the Pod. Returns 0 when missing.
// vk-cocoon writes this back as lifecycle-observed-generation when a state
// transition completes, giving clients a counter-based completion signal
// that is not subject to wallclock skew.
func ReadCocoonSetGeneration(pod *corev1.Pod) int64 {
	if pod == nil {
		return 0
	}
	raw := pod.Annotations[AnnotationCocoonSetGeneration]
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// StampCocoonSetGeneration writes generation onto the pod's annotation
// map (allocating it if nil). The companion to ReadCocoonSetGeneration.
func StampCocoonSetGeneration(pod *corev1.Pod, generation int64) {
	if pod == nil {
		return
	}
	a := ensurePodAnnotations(pod)
	a[AnnotationCocoonSetGeneration] = strconv.FormatInt(generation, 10)
}
