package podclique

import (
	"context"
	"fmt"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Reconciler) triggerDeletionFlow(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	deleteStepFns := []ctrlcommon.ReconcileStepFn[v1alpha1.PodClique]{
		r.deletePodCliqueResources,
		r.verifyNoResourcesAwaitsCleanup,
		r.removeFinalizer,
	}
	for _, fn := range deleteStepFns {
		if stepResult := fn(ctx, logger, pclq); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return stepResult
		}
	}
	logger.Info("PodClique deleted successfully")
	return ctrlcommon.DoNotRequeue()
}

func (r *Reconciler) deletePodCliqueResources(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	operators := r.operatorRegistry.GetAllOperators()
	deleteTasks := make([]utils.Task, 0, len(operators))
	for kind, operator := range operators {
		deleteTasks = append(deleteTasks, utils.Task{
			Name: fmt.Sprintf("delete-%s", kind),
			Fn: func(ctx context.Context) error {
				return operator.Delete(ctx, logger, pclq.ObjectMeta)
			},
		})
	}
	logger.Info("Triggering delete of PodClique resources")
	if errs := utils.RunConcurrently(ctx, deleteTasks); len(errs) > 0 {
		return ctrlcommon.ReconcileWithErrors("error deleting managed resources", errs...)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) verifyNoResourcesAwaitsCleanup(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	// TODO implement me
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) removeFinalizer(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pclq, v1alpha1.FinalizerPodClique) {
		logger.Info("Finalizer not found", "PodClique", pclq)
		return ctrlcommon.DoNotRequeue()
	}
	logger.Info("Removing finalizer", "PodClique", pclq, "finalizerName", v1alpha1.FinalizerPodClique)
	if err := ctrlutils.RemoveAndPatchFinalizer(ctx, r.client, pclq, v1alpha1.FinalizerPodClique); err != nil {
		return ctrlcommon.ReconcileWithErrors("error removing finalizer", fmt.Errorf("failed to remove finalizer: %s from PodClique: %v: %w", v1alpha1.FinalizerPodClique, client.ObjectKeyFromObject(pclq), err))
	}
	return ctrlcommon.ContinueReconcile()
}
