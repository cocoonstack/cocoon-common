package meta

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	SnapshotStatePushed SnapshotState = "pushed"
	SnapshotStateFailed SnapshotState = "failed"

	reservedSnapshotAnnotationPrefix = "cocoonstack."
)

type SnapshotState string

type SnapshotStatus struct {
	Ref     string        `json:"ref"`
	State   SnapshotState `json:"state"`
	Message string        `json:"message,omitempty"`
}

func (s SnapshotStatus) Annotations() map[string]any {
	raw, _ := json.Marshal(s)
	return map[string]any{AnnotationSnapshotStatus: string(raw)}
}

// ReadSnapshotAnnotations reads the caller's OCI annotations for the pod's pushed snapshots; keys under the cocoonstack. prefix are reserved.
func ReadSnapshotAnnotations(pod *corev1.Pod) (map[string]string, error) {
	raw := pod.Annotations[AnnotationSnapshotAnnotations]
	if raw == "" {
		return nil, nil
	}
	var annotations map[string]string
	if err := json.Unmarshal([]byte(raw), &annotations); err != nil {
		return nil, fmt.Errorf("parse %s: %w", AnnotationSnapshotAnnotations, err)
	}
	for key := range annotations {
		if strings.HasPrefix(key, reservedSnapshotAnnotationPrefix) {
			return nil, fmt.Errorf("%s: key %q is reserved", AnnotationSnapshotAnnotations, key)
		}
	}
	return annotations, nil
}

func ReadSnapshotStatus(pod *corev1.Pod) SnapshotStatus {
	var s SnapshotStatus
	if raw := pod.Annotations[AnnotationSnapshotStatus]; raw != "" && json.Unmarshal([]byte(raw), &s) != nil {
		return SnapshotStatus{}
	}
	return s
}
