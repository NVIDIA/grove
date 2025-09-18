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

// Package podclique provides rolling update functionality for PodCliqueScalingGroup resources.
// This file contains the logic for managing rolling updates of PodClique replicas.
package podclique

import (
	"context"
	"fmt"
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// updateWork encapsulates the state analysis needed for rolling updates.
// It categorizes existing replicas by their readiness state to determine update strategy.
type updateWork struct {
	oldPendingReplicaIndices     []int // Replicas that are not yet scheduled
	oldUnavailableReplicaIndices []int // Replicas that are scheduled but not ready
	oldReadyReplicaIndices       []int // Replicas that are ready and candidates for update
}

// replicaState represents the operational state of a PCSG replica
type replicaState int

// Replica state constants for rolling update logic
const (
	replicaStatePending     replicaState = iota // Replica is not yet scheduled
	replicaStateUnAvailable                     // Replica is scheduled but not ready
	replicaStateReady                           // Replica is ready and operational
)

// processPendingUpdates orchestrates the rolling update process for PodCliqueScalingGroup replicas.
// It manages the sequential update of replicas while respecting availability constraints.
func (r _resource) processPendingUpdates(logger logr.Logger, sc *syncContext) error {
	// Analyze current replica states to determine update strategy
	work, err := computePendingUpdateWork(sc)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeComputePendingPodCliqueScalingGroupUpdateWork,
			component.OperationSync,
			fmt.Sprintf("failed to compute pending update work for PodCliqueScalingGroup %v", client.ObjectKeyFromObject(sc.pcsg)))
	}

	// Clean up replicas that are in transitional states (pending/unavailable)
	if err = r.deleteOldPendingAndUnavailableReplicas(logger, sc, work); err != nil {
		return err
	}

	// Check if a replica is currently being updated and wait for completion
	if isAnyReadyReplicaSelectedForUpdate(sc.pcsg) && !isCurrentReplicaUpdateComplete(sc) {
		return groveerr.New(
			groveerr.ErrCodeContinueReconcileAndRequeue,
			component.OperationSync,
			fmt.Sprintf("rolling update of currently selected PCSG replica index: %d is not complete, requeuing", sc.pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Current),
		)
	}

	// Select the next replica for update if available and safe to do so
	var nextReplicaIndexToUpdate *int
	if len(work.oldReadyReplicaIndices) > 0 {
		// Ensure minimum availability is maintained before proceeding
		if sc.pcsg.Status.AvailableReplicas < *sc.pcsg.Spec.MinAvailable {
			return groveerr.New(
				groveerr.ErrCodeContinueReconcileAndRequeue,
				component.OperationSync,
				fmt.Sprintf("available replicas %d lesser than minAvailable %d, requeuing", sc.pcsg.Status.AvailableReplicas, *sc.pcsg.Spec.MinAvailable),
			)
		}
		nextReplicaIndexToUpdate = ptr.To(work.oldReadyReplicaIndices[0])
	}

	// Execute the update for the selected replica
	if nextReplicaIndexToUpdate != nil {
		logger.Info("Selected the next replica to update", "nextReplicaIndexToUpdate", *nextReplicaIndexToUpdate)

		// Update PCSG status to track the replica being updated
		if err := r.updatePCSGStatusWithNextReplicaToUpdate(sc.ctx, logger, sc.pcsg, *nextReplicaIndexToUpdate); err != nil {
			return err
		}

		// Delete the selected replica to trigger recreation with updated configuration
		deleteTask := r.createDeleteTasks(logger, sc.pcs, sc.pcsg.Name, []string{strconv.Itoa(*nextReplicaIndexToUpdate)}, "deleting replica for rolling update")
		if err := r.triggerDeletionOfPodCliques(sc.ctx, logger, client.ObjectKeyFromObject(sc.pcsg), deleteTask); err != nil {
			return err
		}

		// Requeue to allow recreation of the deleted replica
		return groveerr.New(
			groveerr.ErrCodeContinueReconcileAndRequeue,
			component.OperationSync,
			fmt.Sprintf("rolling update of currently selected PCSG replica index: %d is not complete, requeuing", sc.pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Current),
		)
	}

	// All replicas have been updated, mark the rolling update as complete
	return r.markRollingUpdateEnd(sc.ctx, logger, sc.pcsg)
}

// updatePCSGStatusWithNextReplicaToUpdate updates the PCSG status to track the replica currently being updated.
// It records the replica index in the rolling update progress and marks previous replicas as completed.
func (r _resource) updatePCSGStatusWithNextReplicaToUpdate(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, nextReplicaIndexToUpdate int) error {
	patch := client.MergeFrom(pcsg.DeepCopy())

	if pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate == nil {
		pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate = &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{}
	} else {
		pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Completed = append(pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Completed, pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Current)
	}
	pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Current = int32(nextReplicaIndexToUpdate)

	if err := r.client.Status().Patch(ctx, pcsg, patch); err != nil {
		return groveerr.WrapError(
			err,
			errCodeUpdateStatus,
			component.OperationSync,
			fmt.Sprintf("failed to update ready replica selected to update in status of PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	logger.Info("Updated PodCliqueScalingGroup status with new ready replica index selected to update", "nextReplicaIndexToUpdate", nextReplicaIndexToUpdate)
	return nil
}

// markRollingUpdateEnd finalizes the rolling update process by updating the PCSG status.
// It sets the end timestamp and clears the rolling update progress tracking.
func (r _resource) markRollingUpdateEnd(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	patch := client.MergeFrom(pcsg.DeepCopy())

	pcsg.Status.RollingUpdateProgress.UpdateEndedAt = ptr.To(metav1.Now())
	pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate = nil

	if err := r.client.Status().Patch(ctx, pcsg, patch); err != nil {
		return groveerr.WrapError(
			err,
			errCodeUpdateStatus,
			component.OperationSync,
			fmt.Sprintf("failed to mark end of rolling update in status of PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	logger.Info("Marked the end of rolling update for PodCliqueScalingGroup")
	return nil
}

// computePendingUpdateWork analyzes existing replicas to categorize them by state for rolling update planning.
// It identifies which replicas are pending, unavailable, or ready for update.
func computePendingUpdateWork(sc *syncContext) (*updateWork, error) {
	work := &updateWork{}
	existingPCLQsByReplicaIndex := componentutils.GroupPCLQsByPCSGReplicaIndex(sc.existingPCLQs)
	for pcsgReplicaIndex := range int(sc.pcsg.Spec.Replicas) {
		pcsgReplicaIndexStr := strconv.Itoa(pcsgReplicaIndex)
		existingPCSGReplicaPCLQs := existingPCLQsByReplicaIndex[pcsgReplicaIndexStr]
		if isReplicaDeletedOrMarkedForDeletion(sc.pcsg, existingPCSGReplicaPCLQs, pcsgReplicaIndex) {
			continue
		}
		// pcsgReplicaIndex is the currently updating replica
		if sc.pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate != nil &&
			sc.pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Current == int32(pcsgReplicaIndex) {
			continue
		}
		isUpdated, err := isReplicaUpdated(sc.expectedPCLQPodTemplateHashMap, existingPCSGReplicaPCLQs)
		if err != nil {
			return nil, err
		}
		if isUpdated {
			continue
		}
		state := getReplicaState(existingPCSGReplicaPCLQs)
		switch state {
		case replicaStatePending:
			work.oldPendingReplicaIndices = append(work.oldPendingReplicaIndices, pcsgReplicaIndex)
		case replicaStateUnAvailable:
			work.oldUnavailableReplicaIndices = append(work.oldUnavailableReplicaIndices, pcsgReplicaIndex)
		case replicaStateReady:
			work.oldReadyReplicaIndices = append(work.oldReadyReplicaIndices, pcsgReplicaIndex)
		}
	}
	return work, nil
}

// deleteOldPendingAndUnavailableReplicas removes replicas that are in transitional states during rolling updates.
// This cleans up replicas that are not ready and need to be recreated with updated configurations.
func (r _resource) deleteOldPendingAndUnavailableReplicas(logger logr.Logger, sc *syncContext, work *updateWork) error {
	replicaIndicesToDelete := lo.Map(append(work.oldPendingReplicaIndices, work.oldUnavailableReplicaIndices...), func(index int, _ int) string {
		return strconv.Itoa(index)
	})
	deleteTasks := r.createDeleteTasks(logger, sc.pcs, sc.pcsg.Name, replicaIndicesToDelete,
		"delete pending and unavailable PodCliqueScalingGroup replicas for rolling update")
	return r.triggerDeletionOfPodCliques(sc.ctx, logger, client.ObjectKeyFromObject(sc.pcsg), deleteTasks)
}

// isAnyReadyReplicaSelectedForUpdate checks if a replica is currently selected for rolling update.
// Returns true if there's an active rolling update in progress.
func isAnyReadyReplicaSelectedForUpdate(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) bool {
	return pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate != nil
}

// isCurrentReplicaUpdateComplete verifies if the currently updating replica has completed its update.
// It checks that all PodCliques in the replica have the expected template hash and are ready.
func isCurrentReplicaUpdateComplete(sc *syncContext) bool {
	currentlyUpdatingReplicaIndex := int(sc.pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Current)
	existingPCLQsByReplicaIndex := componentutils.GroupPCLQsByPCSGReplicaIndex(sc.existingPCLQs)
	// Get the expected PCLQ PodTemplateHash and compare it against all existing PCLQs for the currently updating replica index.
	expectedPCLQFQNs := sc.expectedPCLQFQNsPerPCSGReplica[currentlyUpdatingReplicaIndex]
	existingPCSGReplicaPCLQs := existingPCLQsByReplicaIndex[strconv.Itoa(currentlyUpdatingReplicaIndex)]
	if len(expectedPCLQFQNs) != len(existingPCSGReplicaPCLQs) {
		return false
	}
	return lo.EveryBy(existingPCSGReplicaPCLQs, func(pclq grovecorev1alpha1.PodClique) bool {
		return pclq.Status.CurrentPodTemplateHash != nil && *pclq.Status.CurrentPodTemplateHash == sc.expectedPCLQPodTemplateHashMap[pclq.Name] &&
			pclq.Status.CurrentPodCliqueSetGenerationHash != nil && *pclq.Status.CurrentPodCliqueSetGenerationHash == *sc.pcs.Status.CurrentGenerationHash &&
			pclq.Status.ReadyReplicas >= *pclq.Spec.MinAvailable
	})
}

// isReplicaUpdated determines if a replica has been updated to the latest configuration.
// It compares the current pod template hashes with the expected ones.
func isReplicaUpdated(expectedPCLQPodTemplateHashes map[string]string, pcsgReplicaPCLQs []grovecorev1alpha1.PodClique) (bool, error) {
	for _, pclq := range pcsgReplicaPCLQs {
		podTemplateHash, ok := pclq.Labels[apicommon.LabelPodTemplateHash]
		if !ok {
			return false, groveerr.ErrMissingPodTemplateHashLabel
		}
		if podTemplateHash != expectedPCLQPodTemplateHashes[pclq.Name] {
			return false, nil
		}
	}
	return true, nil
}

// isReplicaDeletedOrMarkedForDeletion checks if a replica is being deleted or has been deleted.
// It returns true if the replica has no PodCliques or all PodCliques are terminating.
func isReplicaDeletedOrMarkedForDeletion(pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaPCLQs []grovecorev1alpha1.PodClique, _ int) bool {
	if pcsg.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate == nil {
		return false
	}
	if len(pcsgReplicaPCLQs) == 0 {
		return true
	}
	return lo.EveryBy(pcsgReplicaPCLQs, func(pclq grovecorev1alpha1.PodClique) bool {
		return k8sutils.IsResourceTerminating(pclq.ObjectMeta)
	})
}

// getReplicaState determines the operational state of a PCSG replica based on its PodCliques.
// It returns the most restrictive state among all PodCliques in the replica.
func getReplicaState(pcsgReplicaPCLQs []grovecorev1alpha1.PodClique) replicaState {
	for _, pclq := range pcsgReplicaPCLQs {
		if pclq.Status.ScheduledReplicas < *pclq.Spec.MinAvailable {
			return replicaStatePending
		}
		if pclq.Status.ReadyReplicas < *pclq.Spec.MinAvailable {
			return replicaStateUnAvailable
		}
	}
	return replicaStateReady
}
