package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cocoonstack/cocoon-common/meta"
)

// DeepCopyObject is the controller-runtime client.Object subset that
// also exposes a typed DeepCopy() method returning the same concrete
// type. Every kubebuilder-generated CRD (and the core k8s types)
// satisfies this shape, so the generic helpers below accept them
// without a hand-rolled DeepCopyObject().(client.Object) cast.
type DeepCopyObject[T any] interface {
	client.Object
	DeepCopy() T
}

// patchMerge applies mutate to obj under a controller-runtime
// MergeFrom patch and sends it through the primary resource. The
// pre-mutation snapshot is captured via T's own typed DeepCopy so
// the patch body contains exactly the delta mutate just applied.
//
// Unexported on purpose: the only caller today is PatchHibernateState
// below, and the exported surface we offer to reconcilers is the
// specialized helpers (PatchStatus, PatchHibernateState). If another
// caller needs a general-purpose primary-resource merge helper,
// export this by renaming.
func patchMerge[T DeepCopyObject[T]](ctx context.Context, cli client.Client, obj T, mutate func(T)) error {
	patch := client.MergeFrom(obj.DeepCopy())
	mutate(obj)
	return cli.Patch(ctx, obj, patch)
}

// PatchStatus is PatchMerge for the /status subresource. Callers
// that just need to rewrite Status fields through a no-op-preserving
// MergeFrom patch should use this so they never have to assemble
// the MergeFrom/DeepCopy boilerplate themselves.
func PatchStatus[T DeepCopyObject[T]](ctx context.Context, cli client.Client, obj T, mutate func(T)) error {
	patch := client.MergeFrom(obj.DeepCopy())
	mutate(obj)
	return cli.Status().Patch(ctx, obj, patch)
}

// PatchHibernateState patches a pod's hibernate annotation through a
// controller-runtime MergeFrom patch. It short-circuits when the pod
// already carries the desired state so callers can invoke it
// unconditionally on every reconcile pass without generating a
// no-op API write.
func PatchHibernateState(ctx context.Context, cli client.Client, pod *corev1.Pod, state bool) error {
	if bool(meta.ReadHibernateState(pod)) == state {
		return nil
	}
	return patchMerge(ctx, cli, pod, func(p *corev1.Pod) {
		meta.HibernateState(state).Apply(p)
	})
}
