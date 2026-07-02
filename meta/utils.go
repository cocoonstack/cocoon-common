package meta

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// QuantityString returns q.String() or "" when q is nil.
func QuantityString(q *resource.Quantity) string {
	if q == nil {
		return ""
	}
	return q.String()
}

func ensurePodAnnotations(pod *corev1.Pod) map[string]string {
	if pod == nil {
		return nil
	}
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	return pod.Annotations
}

func setIfNotEmpty(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func formatPort(port int32) string {
	if port <= 0 {
		return ""
	}
	return strconv.FormatInt(int64(port), 10)
}
