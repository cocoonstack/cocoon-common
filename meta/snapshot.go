package meta

import (
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const reservedSnapshotAnnotationPrefix = "cocoonstack."

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
