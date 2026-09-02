package k8s

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeepCopyObject is client.Object with a typed DeepCopy() method.
type DeepCopyObject[T any] interface {
	client.Object
	DeepCopy() T
}

// PatchStatus applies mutate under a MergeFrom patch on the /status subresource.
func PatchStatus[T DeepCopyObject[T]](ctx context.Context, cli client.Client, obj T, mutate func(T)) error {
	patch := buildMergePatch(obj, mutate)
	return cli.Status().Patch(ctx, obj, patch)
}

// Patch applies mutate under a MergeFrom patch on the object.
func Patch[T DeepCopyObject[T]](ctx context.Context, cli client.Client, obj T, mutate func(T)) error {
	patch := buildMergePatch(obj, mutate)
	return cli.Patch(ctx, obj, patch)
}

func buildMergePatch[T DeepCopyObject[T]](obj T, mutate func(T)) client.Patch {
	patch := client.MergeFrom(obj.DeepCopy())
	mutate(obj)
	return patch
}
