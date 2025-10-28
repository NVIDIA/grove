// /*
// Copyright 2025 The Grove Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package podcliqueset

import (
	"context"
	"fmt"

	"github.com/ai-dynamo/grove/operator/api/common/constants"
	"github.com/ai-dynamo/grove/operator/api/core/v1alpha1"
	ctrlcommon "github.com/ai-dynamo/grove/operator/internal/controller/common"
	ctrlutils "github.com/ai-dynamo/grove/operator/internal/controller/utils"
	"github.com/ai-dynamo/grove/operator/internal/utils"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// triggerDeletionFlow handles the deletion of a PodCliqueSet and its managed resources
func (r *Reconciler) triggerDeletionFlow(ctx context.Context, logger logr.Logger, pcs *v1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	deleteStepFns := []ctrlcommon.ReconcileStepFn[v1alpha1.PodCliqueSet]{
		r.deletePodCliqueSetResources,
		r.verifyNoResourcesAwaitsCleanup,
		r.removeFinalizer,
	}
	for _, fn := range deleteStepFns {
		if stepResult := fn(ctx, logger, pcs); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteDeletion(ctx, logger, pcs, &stepResult)
		}
	}
	logger.Info("PodCliqueSet deleted successfully")
	return ctrlcommon.DoNotRequeue()
}

// deletePodCliqueSetResources triggers concurrent deletion of all managed resources for the PodCliqueSet.
func (r *Reconciler) deletePodCliqueSetResources(ctx context.Context, logger logr.Logger, pcs *v1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	operators := r.operatorRegistry.GetAllOperators()
	deleteTasks := make([]utils.Task, 0, len(operators))
	for kind, operator := range operators {
		deleteTasks = append(deleteTasks, utils.Task{
			Name: fmt.Sprintf("delete-%s", kind),
			Fn: func(ctx context.Context) error {
				return operator.Delete(ctx, logger, pcs.ObjectMeta)
			},
		})
	}
	logger.Info("Triggering delete of PodCliqueSet resources")
	if runResult := utils.RunConcurrently(ctx, logger, deleteTasks); runResult.HasErrors() {
		deletionErr := runResult.GetAggregatedError()
		logger.Error(deletionErr, "Error deleting managed resources", "summary", runResult.GetSummary())
		return ctrlcommon.ReconcileWithErrors("error deleting managed resources", deletionErr)
	}
	return ctrlcommon.ContinueReconcile()
}

// verifyNoResourcesAwaitsCleanup ensures all managed resources have been cleaned up before finalizer removal.
func (r *Reconciler) verifyNoResourcesAwaitsCleanup(ctx context.Context, logger logr.Logger, pcs *v1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	return ctrlutils.VerifyNoResourceAwaitsCleanup(ctx, logger, r.operatorRegistry, pcs.ObjectMeta)
}

// removeFinalizer removes the PodCliqueSet finalizer if present.
func (r *Reconciler) removeFinalizer(ctx context.Context, logger logr.Logger, pcs *v1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pcs, constants.FinalizerPodCliqueSet) {
		logger.Info("Finalizer not found", "PodCliqueSet", pcs)
		return ctrlcommon.ContinueReconcile()
	}
	logger.Info("Removing finalizer", "PodCliqueSet", pcs, "finalizerName", constants.FinalizerPodCliqueSet)
	if err := ctrlutils.RemoveAndPatchFinalizer(ctx, r.client, pcs, constants.FinalizerPodCliqueSet); err != nil {
		return ctrlcommon.ReconcileWithErrors("error removing finalizer", fmt.Errorf("failed to remove finalizer: %s from PodCliqueSet: %v: %w", constants.FinalizerPodCliqueSet, client.ObjectKeyFromObject(pcs), err))
	}
	return ctrlcommon.ContinueReconcile()
}

// recordIncompleteDeletion records errors that occurred during deletion and returns the combined result.
func (r *Reconciler) recordIncompleteDeletion(ctx context.Context, logger logr.Logger, pcs *v1alpha1.PodCliqueSet, errResult *ctrlcommon.ReconcileStepResult) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordErrors(ctx, pcs, errResult); err != nil {
		logger.Error(err, "failed to record deletion completion operation", "PodCliqueSet", pcs)
		// combine all errors
		allErrs := append(errResult.GetErrors(), err)
		return ctrlcommon.ReconcileWithErrors("error recording incomplete reconciliation", allErrs...)
	}
	return *errResult
}
