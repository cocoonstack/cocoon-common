package k8s

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cocoonstack/cocoon-common/meta"
)

// DeepCopyObject is client.Object with a typed DeepCopy() method.
type DeepCopyObject[T any] interface {
	client.Object
	DeepCopy() T
}

// PatchStatus applies mutate under a MergeFrom patch on the /status subresource.
func PatchStatus[T DeepCopyObject[T]](ctx context.Context, cli client.Client, obj T, mutate func(T)) error {
	patch := client.MergeFrom(obj.DeepCopy())
	mutate(obj)
	return cli.Status().Patch(ctx, obj, patch)
}

// PatchHibernateState patches the hibernate annotation, short-circuiting if already at the desired state.
func PatchHibernateState(ctx context.Context, cli client.Client, pod *corev1.Pod, state bool) error {
	if meta.ReadHibernateState(pod) == meta.HibernateState(state) {
		return nil
	}
	return patchMerge(ctx, cli, pod, func(p *corev1.Pod) {
		meta.HibernateState(state).Apply(p)
	})
}

// PatchCocoonSetGeneration stamps the owning CocoonSet's metadata.generation
// onto the pod so vk-cocoon can read it back as lifecycle-observed-generation.
// Short-circuits when the annotation is already correct.
func PatchCocoonSetGeneration(ctx context.Context, cli client.Client, pod *corev1.Pod, generation int64) error {
	want := strconv.FormatInt(generation, 10)
	if pod.Annotations[meta.AnnotationCocoonSetGeneration] == want {
		return nil
	}
	return patchMerge(ctx, cli, pod, func(p *corev1.Pod) {
		meta.StampCocoonSetGeneration(p, generation)
	})
}

// patchMerge applies mutate under a MergeFrom patch on the primary resource.
func patchMerge[T DeepCopyObject[T]](ctx context.Context, cli client.Client, obj T, mutate func(T)) error {
	patch := client.MergeFrom(obj.DeepCopy())
	mutate(obj)
	return cli.Patch(ctx, obj, patch)
}
