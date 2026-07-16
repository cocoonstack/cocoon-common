package meta

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestHasCocoonTolerationKey(t *testing.T) {
	tolerations := []corev1.Toleration{{Key: TolerationKey}}
	if !HasCocoonTolerationKey(tolerations) {
		t.Fatalf("expected toleration to be detected")
	}
	if HasCocoonTolerationKey(nil) {
		t.Errorf("expected nil tolerations to be rejected")
	}
	if HasCocoonTolerationKey([]corev1.Toleration{{Key: "other"}}) {
		t.Errorf("expected unrelated toleration to be rejected")
	}
}
