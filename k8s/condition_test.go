package k8s

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewReadyCondition(t *testing.T) {
	c := NewReadyCondition(7, metav1.ConditionTrue, "AllReady", "2/2 ready")
	if c.Type != ConditionTypeReady {
		t.Errorf("Type = %q, want Ready", c.Type)
	}
	if c.Status != metav1.ConditionTrue {
		t.Errorf("Status = %q", c.Status)
	}
	if c.Reason != "AllReady" {
		t.Errorf("Reason = %q", c.Reason)
	}
	if c.Message != "2/2 ready" {
		t.Errorf("Message = %q", c.Message)
	}
	if c.ObservedGeneration != 7 {
		t.Errorf("ObservedGeneration = %d", c.ObservedGeneration)
	}
	if !c.LastTransitionTime.IsZero() {
		t.Errorf("LastTransitionTime must be left zero so SetStatusCondition preserves the prior timestamp on no-op merges, got %v", c.LastTransitionTime)
	}
}
