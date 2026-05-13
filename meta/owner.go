package meta

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasCocoonTolerationKey reports whether the toleration list includes
// an entry whose Key matches TolerationKey. Operator/Value/Effect are
// ignored — the cocoon-webhook gate is intentionally permissive to
// accept any toleration spelling that targets the virtual-kubelet
// taint. Use a stricter check at the call site if you need to match a
// specific Operator or Effect.
func HasCocoonTolerationKey(tolerations []corev1.Toleration) bool {
	return slices.ContainsFunc(tolerations, func(t corev1.Toleration) bool {
		return t.Key == TolerationKey
	})
}

// IsOwnedByCocoonSet reports whether any owner reference is a CocoonSet.
func IsOwnedByCocoonSet(ownerRefs []metav1.OwnerReference) bool {
	return slices.ContainsFunc(ownerRefs, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindCocoonSet
	})
}

// OwnerDeploymentName extracts the deployment name from a ReplicaSet
// owner reference. Returns ok=false when no ReplicaSet owner is present
// or its name has no recognizable hash suffix — that lets the caller
// distinguish "no owning deployment" from a legitimately empty name
// instead of conflating both into an empty string.
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
