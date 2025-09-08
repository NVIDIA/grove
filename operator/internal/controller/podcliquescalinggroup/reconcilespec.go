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

package podcliquescalinggroup

import (
	"context"
	"fmt"
	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/internal/component/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"

	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Reconciler) reconcileSpec(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	reconcileStepFns := []ctrlcommon.ReconcileStepFn[grovecorev1alpha1.PodCliqueScalingGroup]{
		r.ensureFinalizer,
		r.recordReconcileStart,
		r.processRollingUpdate,
		r.syncPodCliqueScalingGroupResources,
		r.recordReconcileSuccess,
		r.updateObservedGeneration,
	}

	for _, fn := range reconcileStepFns {
		if stepResult := fn(ctx, logger, pcsg); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteReconcile(ctx, logger, pcsg, &stepResult)
		}
	}
	logger.Info("Finished spec reconciliation flow")
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) ensureFinalizer(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pcsg, constants.FinalizerPodCliqueScalingGroup) {
		logger.Info("Adding finalizer", "finalizerName", constants.FinalizerPodCliqueScalingGroup)
		if err := ctrlutils.AddAndPatchFinalizer(ctx, r.client, pcsg, constants.FinalizerPodCliqueScalingGroup); err != nil {
			logger.Error(err, "failed to add finalizer")
			return ctrlcommon.ReconcileWithErrors("error adding finalizer", fmt.Errorf("failed to add finalizer: %s to PodCliqueScalingGroup: %v: %w", constants.FinalizerPodCliqueScalingGroup, client.ObjectKeyFromObject(pcsg), err))
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileStart(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordStart(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile); err != nil {
		logger.Error(err, "failed to record reconcile start operation")
		return ctrlcommon.ReconcileWithErrors("error recoding reconcile start", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) processRollingUpdate(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	pcsgObjectKey := client.ObjectKeyFromObject(pcsg)
	pgs, err := utils.GetOwnerPodGangSet(ctx, r.client, pcsg.ObjectMeta)
	if err != nil {
		return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("could not get owner PodGangSet for PodCliqueScalingGroup: %v", pcsgObjectKey), err)
	}
	if pgs.Status.RollingUpdateProgress == nil || pgs.Status.RollingUpdateProgress.CurrentlyUpdating == nil {
		// No update has yet been triggered for the PodGangSet. Nothing to do here.
		return ctrlcommon.ContinueReconcile()
	}
	pgsReplicaInUpdating := pgs.Status.RollingUpdateProgress.CurrentlyUpdating.ReplicaIndex
	pgsReplicaIndexStr, ok := pcsg.Labels[apicommon.LabelPodGangSetReplicaIndex]
	if !ok {
		logger.Info("PodGangSet is currently under rolling update. Cannot process pending updates for this PodCliqueScalingGroup as no PodGangSet index label is found")
		return ctrlcommon.ContinueReconcile()
	}
	if pgsReplicaIndexStr != strconv.Itoa(int(pgsReplicaInUpdating)) {
		logger.Info("PodGangSet is currently under rolling update. Skipping processing pending updates for this PodCliqueScalingGroup as it does not belong to the PodGangSet Index in update", "currentlyUpdatingPGSIndex", pgsReplicaInUpdating, "pgsIndexForPCSG", pgsReplicaIndexStr)
		return ctrlcommon.ContinueReconcile()
	}

	// Trigger processing of pending updates for this PCSG. Check if all pending updates for this PCSG and for the PGS GenerationHash
	// has already been completed or are already in-progress. If that is true, then there is nothing more to do.
	// If the rolling update is in-progress for a different PGS GenerationHash, or it has not even been started, then
	// reset the rolling update progress so that it can be restarted.
	if shouldResetOrTriggerRollingUpdate(pgs, pcsg) {
		pcsg.Status.UpdatedReplicas = 0
		pcsg.Status.RollingUpdateProgress = &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
			UpdateStartedAt:          metav1.Now(),
			PodGangSetGenerationHash: *pgs.Status.GenerationHash,
		}
		if err = r.client.Status().Update(ctx, pcsg); err != nil {
			logger.Error(err, "could not update PodCliqueScalingGroup.Status.RollingUpdateProgress")
			return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("could not update PodCliqueScalingGroup.Status.RollingUpdateProgress:  %v", pcsgObjectKey), err)
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func shouldResetOrTriggerRollingUpdate(pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) bool {
	// If processing of rolling update of PCSG for PGS GenerationHash is either completed or in-progress,
	// there is no need to reset or trigger another rolling update of this PCSG for the same PGS GenerationHash.
	// TODO: @renormalize PodGangSetGenerationHash should not be used when the PGS PodCliques are updated.
	// Currently, even changing PGS PodCliques causes rolling update for PCSG.
	if pcsg.Status.RollingUpdateProgress != nil && pcsg.Status.RollingUpdateProgress.PodGangSetGenerationHash == *pgs.Status.GenerationHash {
		return false
	}
	return true
}

func (r *Reconciler) syncPodCliqueScalingGroupResources(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	continueReconcileAndRequeueKinds := make([]component.Kind, 0)
	for _, kind := range getOrderedKindsForSync() {
		operator, err := r.operatorRegistry.GetOperator(kind)
		if err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("error getting operator for kind: %s", kind), err)
		}
		logger.Info("Syncing PodCliqueScalingGroup resource", "kind", kind)
		if err = operator.Sync(ctx, logger, pcsg); err != nil {
			if ctrlutils.ShouldContinueReconcileAndRequeue(err) {
				logger.Info("component has registered a request to requeue post completion of all component syncs", "kind", kind, "message", err.Error())
				continueReconcileAndRequeueKinds = append(continueReconcileAndRequeueKinds, kind)
				continue
			}
			if shouldRequeue := ctrlutils.ShouldRequeueAfter(err); shouldRequeue {
				logger.Info("retrying sync due to component", "kind", kind, "syncRetryInterval", ctrlcommon.ComponentSyncRetryInterval, "message", err.Error())
				return ctrlcommon.ReconcileAfter(ctrlcommon.ComponentSyncRetryInterval, err.Error())
			}
			logger.Error(err, "failed to sync PodCliqueScalingGroup resources", "kind", kind)
			return ctrlcommon.ReconcileWithErrors("error syncing managed resources", fmt.Errorf("failed to sync %s: %w", kind, err))
		}
	}
	if len(continueReconcileAndRequeueKinds) > 0 {
		return ctrlcommon.ReconcileAfter(ctrlcommon.ComponentSyncRetryInterval, fmt.Sprintf("requeueing sync due to component(s) %v after %s", continueReconcileAndRequeueKinds, ctrlcommon.ComponentSyncRetryInterval))
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileSuccess(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile, nil); err != nil {
		logger.Error(err, "failed to record reconcile success operation")
		return ctrlcommon.ReconcileWithErrors("error recording reconcile success", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordIncompleteReconcile(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, errStepResult *ctrlcommon.ReconcileStepResult) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile, errStepResult); err != nil {
		logger.Error(err, "failed to record incomplete reconcile operation")
		// combine all errors
		allErrs := append(errStepResult.GetErrors(), err)
		return ctrlcommon.ReconcileWithErrors("error recording incomplete reconciliation", allErrs...)
	}
	return *errStepResult
}

func (r *Reconciler) updateObservedGeneration(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	original := pcsg.DeepCopy()
	pcsg.Status.ObservedGeneration = &pcsg.Generation
	if err := r.client.Status().Patch(ctx, pcsg, client.MergeFrom(original)); err != nil {
		logger.Error(err, "failed to patch status.ObservedGeneration")
		return ctrlcommon.ReconcileWithErrors("error updating observed generation", err)
	}
	logger.Info("patched status.ObservedGeneration", "ObservedGeneration", pcsg.Generation)
	return ctrlcommon.ContinueReconcile()
}

func getOrderedKindsForSync() []component.Kind {
	return []component.Kind{
		component.KindPodClique,
	}
}
