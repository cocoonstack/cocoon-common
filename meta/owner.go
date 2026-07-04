package meta

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasCocoonTolerationKey reports whether tolerations include an entry
// whose Key matches TolerationKey. Operator/Value/Effect are ignored —
// the cocoon-webhook gate is intentionally permissive.
func HasCocoonTolerationKey(tolerations []corev1.Toleration) bool {
	return slices.ContainsFunc(tolerations, func(t corev1.Toleration) bool {
		return t.Key == TolerationKey
	})
}

// IsOwnedByCocoonSet reports whether any owner reference is a CocoonSet.
func IsOwnedByCocoonSet(ownerRefs []metav1.OwnerReference) bool {
	return CocoonSetOwnerName(ownerRefs) != ""
}

// CocoonSetOwnerName returns the name of the CocoonSet owner reference, or
// "" if none is present.
func CocoonSetOwnerName(ownerRefs []metav1.OwnerReference) string {
	for _, ref := range ownerRefs {
		if ref.Kind == KindCocoonSet {
			return ref.Name
		}
	}
	return ""
}

// OwnerDeploymentName extracts the deployment name from a ReplicaSet
// owner reference. Returns ok=false when absent or unparseable.
func OwnerDeploymentName(ownerRefs []metav1.OwnerReference) (string, bool) {
	for _, ref := range ownerRefs {
		if ref.Kind != "ReplicaSet" {
			continue
		}
		if before, _, ok := lastCut(ref.Name, "-"); ok {
			return before, true
		}
	}
	return "", false
}
