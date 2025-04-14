package podgangset

import (
	"context"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	"github.com/NVIDIA/grove/operator/internal/utils"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Reconciler) triggerDeletionFlow(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	deleteStepFns := []ctrlcommon.ReconcileStepFn[v1alpha1.PodGangSet]{
		r.recordDeletionStart,
		r.deletePodGangSetResources,
		r.verifyNoResourcesAwaitsCleanup,
		r.removeFinalizer,
	}
	for _, fn := range deleteStepFns {
		if stepResult := fn(ctx, logger, pgs); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteDeletion(ctx, logger, pgs, &stepResult)
		}
	}
	logger.Info("PodGangSet deleted successfully")
	return ctrlcommon.DoNotRequeue()
}

func (r *Reconciler) recordDeletionStart(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordStart(ctx, pgs, v1alpha1.LastOperationTypeDelete); err != nil {
		errMsg := "failed to record deletion start operation"
		logger.Error(err, errMsg, "PodGangSet", pgs)
		return ctrlcommon.ReconcileWithErrors(errMsg, err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) deletePodGangSetResources(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	operators := r.operatorRegistry.GetAllOperators()
	deleteTasks := make([]utils.Task, 0, len(operators))
	for kind, operator := range operators {
		deleteTasks = append(deleteTasks, utils.Task{
			Name: fmt.Sprintf("delete-%s", kind),
			Fn: func(ctx context.Context) error {
				return operator.Delete(ctx, logger, pgs.ObjectMeta)
			},
		})
	}
	logger.Info("Triggering delete of PodGangSet resources")
	if errs := utils.RunConcurrently(ctx, deleteTasks); len(errs) > 0 {
		return ctrlcommon.ReconcileWithErrors("error deleting managed resources", errs...)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) verifyNoResourcesAwaitsCleanup(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	return ctrlutils.VerifyNoResourceAwaitsCleanup(ctx, logger, r.operatorRegistry, pgs)
}

func (r *Reconciler) removeFinalizer(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pgs, v1alpha1.FinalizerPodGangSet) {
		logger.Info("Finalizer not found", "PodGangSet", pgs)
		return ctrlcommon.ContinueReconcile()
	}
	logger.Info("Removing finalizer", "PodGangSet", pgs, "finalizerName", v1alpha1.FinalizerPodGangSet)
	if err := ctrlutils.RemoveAndPatchFinalizer(ctx, r.client, pgs, v1alpha1.FinalizerPodGangSet); err != nil {
		return ctrlcommon.ReconcileWithErrors("error removing finalizer", fmt.Errorf("failed to remove finalizer: %s from PodGangSet: %v: %w", v1alpha1.FinalizerPodGangSet, client.ObjectKeyFromObject(pgs), err))
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordIncompleteDeletion(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet, errResult *ctrlcommon.ReconcileStepResult) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pgs, v1alpha1.LastOperationTypeDelete, errResult); err != nil {
		logger.Error(err, "failed to record deletion completion operation", "PodGangSet", pgs)
		// combine all errors
		allErrs := append(errResult.GetErrors(), err)
		return ctrlcommon.ReconcileWithErrors("error recording incomplete reconciliation", allErrs...)
	}
	return *errResult
}
