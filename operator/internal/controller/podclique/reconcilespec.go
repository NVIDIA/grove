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

package podclique

import (
	"context"
	"fmt"
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Reconciler) reconcileSpec(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	log := logger.WithValues("operation", "specReconcile")
	reconcileStepFns := []ctrlcommon.ReconcileStepFn[grovecorev1alpha1.PodClique]{
		r.ensureFinalizer,
		r.recordReconcileStart,
		r.processRollingUpdate,
		r.syncPCLQResources,
		r.recordReconcileSuccess,
		r.updateObservedGeneration,
	}

	for _, fn := range reconcileStepFns {
		if stepResult := fn(ctx, log, pclq); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteReconcile(ctx, logger, pclq, &stepResult)
		}
	}
	log.Info("Finished spec reconciliation flow", "PodClique", client.ObjectKeyFromObject(pclq))
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) ensureFinalizer(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pclq, constants.FinalizerPodClique) {
		logger.Info("Adding finalizer", "PodClique", client.ObjectKeyFromObject(pclq), "finalizerName", constants.FinalizerPodClique)
		if err := ctrlutils.AddAndPatchFinalizer(ctx, r.client, pclq, constants.FinalizerPodClique); err != nil {
			return ctrlcommon.ReconcileWithErrors("error adding finalizer", err)
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileStart(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordStart(ctx, pclq, grovecorev1alpha1.LastOperationTypeReconcile); err != nil {
		logger.Error(err, "failed to record reconcile start operation")
		return ctrlcommon.ReconcileWithErrors("error recoding reconcile start", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) processRollingUpdate(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	pclqObjectKey := client.ObjectKeyFromObject(pclq)
	pgs, err := componentutils.GetPodGangSet(ctx, r.client, pclq.ObjectMeta)
	if err != nil {
		return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("could not get owner PodGangSet for PodClique: %v", pclqObjectKey), err)
	}

	if pgsHasNoActiveRollingUpdate(pgs) {
		return ctrlcommon.ContinueReconcile()
	}
	shouldEvaluatePCLQForUpdates, err := shouldCheckPendingUpdatesForPCLQ(logger, pgs, pclq)
	if err != nil {
		return ctrlcommon.ReconcileWithErrors("error checking if PodClique should be evaluated for pending updates", err)
	}
	if !shouldEvaluatePCLQForUpdates {
		return ctrlcommon.ContinueReconcile()
	}

	if shouldResetOrTriggerRollingUpdate(pgs, pclq) {
		logger.Info("PodGangSet has a new generation hash. Initializing or resetting rolling update for PodClique", "PodGangSetGenerationHash", *pgs.Status.CurrentGenerationHash, "CurrentPodGangSetGenerationHash", pclq.Status.CurrentPodGangSetGenerationHash, "isPCLQUpdateInProgress", componentutils.IsPCLQUpdateInProgress(pclq), "isLastPCLQUpdateCompleted", componentutils.IsLastPCLQUpdateCompleted(pclq))
		if err = r.initOrResetRollingUpdate(ctx, pgs, pclq); err != nil {
			return ctrlcommon.ReconcileWithErrors("could not initialize rolling update", err)
		}
	}

	return ctrlcommon.ContinueReconcile()
}

func pgsHasNoActiveRollingUpdate(pgs *grovecorev1alpha1.PodGangSet) bool {
	return pgs.Status.CurrentGenerationHash == nil || pgs.Status.RollingUpdateProgress == nil || pgs.Status.RollingUpdateProgress.CurrentlyUpdating == nil
}

func shouldCheckPendingUpdatesForPCLQ(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique) (bool, error) {
	// Only if PCLQ does not belong to any PCSG should an update be triggered for the PCLQ. For PCLQs that belong to
	// a PCSG, the PCSG controller will handle the updates by deleting the PCLQ resources instead of updating PCLQ pods
	// individually.
	if !componentutils.IsStandalonePCLQ(pgs, pclq.Name) {
		return false, nil
	}

	// check if this PCLQ belongs to PGS index that is currently getting updated.
	pgsReplicaInUpdating := pgs.Status.RollingUpdateProgress.CurrentlyUpdating.ReplicaIndex
	pgsReplicaIndexStr, ok := pclq.Labels[apicommon.LabelPodGangSetReplicaIndex]
	if !ok {
		return false, fmt.Errorf("could not determine PodGangSet index for this PodClique %v. Required label %s is missing", client.ObjectKeyFromObject(pclq), apicommon.LabelPodGangSetReplicaIndex)
	}
	if pgsReplicaIndexStr != strconv.Itoa(int(pgsReplicaInUpdating)) {
		logger.Info("PodGangSet is currently under rolling update. Skipping processing update for this PodClique as it does not belong to the PodGangSet Index currently being updated", "currentlyUpdatingPGSIndex", pgsReplicaInUpdating, "pgsIndexForPCLQ", pgsReplicaIndexStr)
		return false, nil
	}

	return true, nil
}

func shouldResetOrTriggerRollingUpdate(pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique) bool {
	// PCLQ has never been updated yet and PGS has a new generation hash.
	firstEverUpdateRequired := pclq.Status.RollingUpdateProgress == nil && pclq.Status.CurrentPodGangSetGenerationHash != nil && *pgs.Status.CurrentGenerationHash != *pclq.Status.CurrentPodGangSetGenerationHash
	if firstEverUpdateRequired {
		return true
	}

	// PCLQ is undergoing a rolling update for a different PGS generation hash
	// Irrespective of whether the pod template hash has changed or not, the in-progress update is stale and needs to be
	// reset in order to set the correct rollingUpdateProgress.PodGangSetGenerationHash
	inProgressPCLQUpdateNotStale := componentutils.IsPCLQUpdateInProgress(pclq) && pclq.Status.RollingUpdateProgress.PodGangSetGenerationHash == *pgs.Status.CurrentGenerationHash
	// PCLQ had an update in the past but that was for an older PGS generation hash.
	lastCompletedUpdateIsNotStale := componentutils.IsLastPCLQUpdateCompleted(pclq) && pclq.Status.RollingUpdateProgress.PodGangSetGenerationHash == *pgs.Status.CurrentGenerationHash
	if inProgressPCLQUpdateNotStale || lastCompletedUpdateIsNotStale {
		return false
	}

	return true
}

func (r *Reconciler) initOrResetRollingUpdate(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique) error {
	podTemplateHash, err := componentutils.GetPCLQPodTemplateHash(pgs, pclq.ObjectMeta)
	if err != nil {
		return fmt.Errorf("could not update PodClique %s status with rolling update progress: %w", client.ObjectKeyFromObject(pclq), err)
	}
	// reset and start the rolling update
	patch := client.MergeFrom(pclq.DeepCopy())
	pclq.Status.RollingUpdateProgress = &grovecorev1alpha1.PodCliqueRollingUpdateProgress{
		UpdateStartedAt:          metav1.Now(),
		PodGangSetGenerationHash: *pgs.Status.CurrentGenerationHash,
		PodTemplateHash:          podTemplateHash,
	}
	pclq.Status.CurrentPodGangSetGenerationHash = pgs.Status.CurrentGenerationHash
	pclq.Status.CurrentPodTemplateHash = &podTemplateHash
	// reset the updated replicas count to 0 so that the rolling update can start afresh.
	pclq.Status.UpdatedReplicas = 0
	if err = r.client.Status().Patch(ctx, pclq, patch); err != nil {
		return fmt.Errorf("failed to update PodClique %s status with rolling update progress: %w", client.ObjectKeyFromObject(pclq), err)
	}
	return nil
}

func (r *Reconciler) syncPCLQResources(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	for _, kind := range getOrderedKindsForSync() {
		operator, err := r.operatorRegistry.GetOperator(kind)
		if err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("error getting operator for kind: %s", kind), err)
		}
		logger.Info("Syncing PodClique resources", "kind", kind)
		if err = operator.Sync(ctx, logger, pclq); err != nil {
			if shouldRequeue := ctrlutils.ShouldRequeueAfter(err); shouldRequeue {
				logger.Info("retrying sync due to component", "kind", kind, "syncRetryInterval", ctrlcommon.ComponentSyncRetryInterval, "message", err.Error())
				return ctrlcommon.ReconcileAfter(ctrlcommon.ComponentSyncRetryInterval, err.Error())
			}
			logger.Error(err, "failed to sync PodClique resources", "kind", kind)
			return ctrlcommon.ReconcileWithErrors("error syncing managed resources", fmt.Errorf("failed to sync %s: %w", kind, err))
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileSuccess(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pclq, grovecorev1alpha1.LastOperationTypeReconcile, nil); err != nil {
		logger.Error(err, "failed to record reconcile success operation")
		return ctrlcommon.ReconcileWithErrors("error recording reconcile success", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) updateObservedGeneration(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	original := pclq.DeepCopy()
	pclq.Status.ObservedGeneration = &pclq.Generation
	if err := r.client.Status().Patch(ctx, pclq, client.MergeFrom(original)); err != nil {
		logger.Error(err, "failed to patch status.ObservedGeneration")
		return ctrlcommon.ReconcileWithErrors("error updating observed generation", err)
	}
	logger.Info("patched status.ObservedGeneration", "ObservedGeneration", pclq.Generation)
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordIncompleteReconcile(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique, errResult *ctrlcommon.ReconcileStepResult) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pclq, grovecorev1alpha1.LastOperationTypeReconcile, errResult); err != nil {
		logger.Error(err, "failed to record incomplete reconcile operation")
		// combine all errors
		allErrs := append(errResult.GetErrors(), err)
		return ctrlcommon.ReconcileWithErrors("error recording incomplete reconciliation", allErrs...)
	}
	return *errResult
}

func getOrderedKindsForSync() []component.Kind {
	return []component.Kind{
		component.KindPod,
	}
}
