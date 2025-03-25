package utils

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// AddAndPatchFinalizer uses merge-patch strategy to patch the given object with the given finalizer.
func AddAndPatchFinalizer(ctx context.Context, writer client.Writer, obj client.Object, finalizer string) error {
	return patchFinalizer(ctx,
		writer,
		obj,
		mergeFromWithOptimisticLock,
		controllerutil.AddFinalizer,
		finalizer,
	)
}

// RemoveAndPatchFinalizer uses merge-patch strategy to patch the given object thereby resulting in removal of the given finalizer.
func RemoveAndPatchFinalizer(ctx context.Context, writer client.Writer, obj client.Object, finalizer string) error {
	return client.IgnoreNotFound(
		patchFinalizer(ctx,
			writer,
			obj,
			mergeFromWithOptimisticLock,
			controllerutil.RemoveFinalizer,
			finalizer,
		),
	)
}

func mergeFromWithOptimisticLock(obj client.Object, opts ...client.MergeFromOption) client.Patch {
	return client.MergeFromWithOptions(obj, append(opts, client.MergeFromWithOptimisticLock{})...)
}

// patchFn is a function that returns a client.Patch with the given client.Object as the base object.
// The client.Patch returned is then used to patch the object.
type patchFn func(client.Object, ...client.MergeFromOption) client.Patch

// mutateFn is a function that mutates the object with the given finalizer.
type mutateFn func(client.Object, string) bool

func patchFinalizer(ctx context.Context, writer client.Writer, obj client.Object, patchFunc patchFn, mutateFunc mutateFn, finalizer string) error {
	beforePatch := obj.DeepCopyObject().(client.Object)
	mutateFunc(obj, finalizer)
	return writer.Patch(ctx, obj, patchFunc(beforePatch))
}
