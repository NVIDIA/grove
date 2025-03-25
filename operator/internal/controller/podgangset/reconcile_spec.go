package podgangset

import (
	"context"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Reconciler) reconcileSpec(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	rLog := logger.WithValues("operation", "spec-reconcile")
	reconcileStepFns := []ctrlcommon.ReconcileStepFn[v1alpha1.PodGangSet]{
		r.recordReconcileStart,
		r.ensureFinalizer,
		r.syncPodGangSetResources,
		r.recordReconcileSuccess,
	}

	for _, fn := range reconcileStepFns {
		if stepResult := fn(ctx, rLog, pgs); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteReconcile(ctx, logger, pgs, &stepResult)
		}
	}
	logger.Info("Finished spec reconciliation flow", "PodGangSet", client.ObjectKeyFromObject(pgs))
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileStart(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordStart(ctx, pgs, v1alpha1.LastOperationTypeReconcile); err != nil {
		logger.Error(err, "failed to record reconcile start operation", "PodGangSet", client.ObjectKeyFromObject(pgs))
		return ctrlcommon.ReconcileWithErrors(err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) ensureFinalizer(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pgs, v1alpha1.FinalizerPodGangSet) {
		logger.Info("Adding finalizer", "PodGangSet", client.ObjectKeyFromObject(pgs), "finalizerName", v1alpha1.FinalizerPodGangSet)
		if err := ctrlutils.AddAndPatchFinalizer(ctx, r.client, pgs, v1alpha1.FinalizerPodGangSet); err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Errorf("failed to add finalizer: %s to PodGangSet: %v: %w", v1alpha1.FinalizerPodGangSet, client.ObjectKeyFromObject(pgs), err))
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) syncPodGangSetResources(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	for _, kind := range getOrderedKindsForSync() {
		operator, err := r.operatorRegistry.GetOperator(kind)
		if err != nil {
			return ctrlcommon.ReconcileWithErrors(err)
		}
		if err = operator.Sync(ctx, logger, pgs); err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Errorf("failed to sync %s: %w", kind, err))
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileSuccess(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pgs, v1alpha1.LastOperationTypeReconcile, nil); err != nil {
		logger.Error(err, "failed to record reconcile success operation", "PodGangSet", client.ObjectKeyFromObject(pgs))
		return ctrlcommon.ReconcileWithErrors(err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordIncompleteReconcile(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet, errResult *ctrlcommon.ReconcileStepResult) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pgs, v1alpha1.LastOperationTypeReconcile, errResult); err != nil {
		logger.Error(err, "failed to record incomplete reconcile operation", "PodGangSet", client.ObjectKeyFromObject(pgs))
		// combine all errors
		allErrs := append(errResult.GetErrors(), err)
		return ctrlcommon.ReconcileWithErrors(allErrs...)
	}
	return *errResult
}

func getOrderedKindsForSync() []component.Kind {
	return []component.Kind{
		component.KindPodClique,
	}
}
