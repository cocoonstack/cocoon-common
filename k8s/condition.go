package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionTypeReady is the canonical condition type cocoon
// reconcilers stamp onto CRD status. Every reconciler uses the same
// literal so sharing it here keeps the "Ready" spelling and
// semantics in one place.
const ConditionTypeReady = "Ready"

// NewReadyCondition returns a metav1.Condition with Type=Ready and
// the supplied fields. LastTransitionTime is left zero so that
// apimeta.SetStatusCondition (the idiomatic way to merge conditions
// onto status) preserves the existing transition timestamp on a
// no-op update.
//
// Callers that need a different condition type should build the
// condition inline — this helper only centralizes the Ready path
// because every cocoon CRD already carries a Ready condition.
func NewReadyCondition(generation int64, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               ConditionTypeReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	}
}
