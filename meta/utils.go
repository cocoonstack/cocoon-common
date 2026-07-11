package meta

import (
	"strconv"
	"strings"

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

// lastCut is like strings.Cut but splits at the last occurrence of sep.
func lastCut(s, sep string) (before, after string, found bool) {
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}
