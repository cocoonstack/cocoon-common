package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionTypeReady is the condition type for overall readiness.
const ConditionTypeReady = "Ready"

// NewReadyCondition builds a Ready condition. LastTransitionTime is left zero
// so apimeta.SetStatusCondition preserves the existing timestamp on no-op updates.
func NewReadyCondition(generation int64, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	}
}
