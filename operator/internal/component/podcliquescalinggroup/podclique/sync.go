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

// Package podclique provides synchronization logic for PodClique resources in PodCliqueScalingGroup.
// This file contains the core sync operations including context preparation and resource management.
package podclique

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// syncContext holds all the necessary data for synchronizing PodClique resources.
// It contains both current state and expected state information needed during sync operations.
type syncContext struct {
	ctx                            context.Context                          // Request context
	pcs                            *grovecorev1alpha1.PodCliqueSet          // Parent PodCliqueSet
	pcsg                           *grovecorev1alpha1.PodCliqueScalingGroup // Target PodCliqueScalingGroup
	existingPCLQs                  []grovecorev1alpha1.PodClique            // Currently existing PodClique resources
	pcsgIndicesToTerminate         []string                                 // PCSG replica indices ready for termination
	pcsgIndicesToRequeue           []string                                 // PCSG replica indices waiting for termination delay
	expectedPCLQFQNsPerPCSGReplica map[int][]string                         // Expected PodClique names per replica index
	expectedPCLQPodTemplateHashMap map[string]string                        // Expected pod template hashes for each PodClique
}

// prepareSyncContext builds the synchronization context with current and expected state information.
// It fetches the parent PodCliqueSet, computes expected resources, and identifies problematic replicas.
func (r _resource) prepareSyncContext(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) (*syncContext, error) {
	var (
		syncCtx = &syncContext{
			ctx:  ctx,
			pcsg: pcsg,
		}
		err error
	)

	// Fetch the parent PodCliqueSet that owns this PodCliqueScalingGroup
	syncCtx.pcs, err = componentutils.GetPodCliqueSet(ctx, r.client, pcsg.ObjectMeta)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeGetPodCliqueSet,
			component.OperationSync,
			fmt.Sprintf("failed to get owner PodCliqueSet for PodCliqueScalingGroup %s", client.ObjectKeyFromObject(pcsg)),
		)
	}

	// Compute expected state and fetch current state
	syncCtx.expectedPCLQFQNsPerPCSGReplica = getExpectedPodCliqueFQNsByPCSGReplica(pcsg)
	syncCtx.existingPCLQs, err = r.getExistingPCLQs(ctx, pcsg)
	if err != nil {
		return nil, err
	}

	// Identify PCSG replica indices with MinAvailable breached and segregate by termination readiness
	// pcsgIndicesToTerminate: replicas past their termination delay
	// pcsgIndicesToRequeue: replicas still within their termination delay
	syncCtx.pcsgIndicesToTerminate, syncCtx.pcsgIndicesToRequeue = getMinAvailableBreachedPCSGIndices(logger, syncCtx.existingPCLQs, syncCtx.pcs.Spec.Template.TerminationDelay.Duration)

	// Pre-compute expected pod template hashes for rolling update detection
	syncCtx.expectedPCLQPodTemplateHashMap = getExpectedPCLQPodTemplateHashMap(syncCtx.pcs, pcsg)

	return syncCtx, nil
}

// runSyncFlow executes the main synchronization logic for PodClique resources.
// It handles scaling, updates, and termination operations in the correct order.
func (r _resource) runSyncFlow(logger logr.Logger, sc *syncContext) error {
	// Handle scale-down: delete excess PodCliques when PCSG replicas have been reduced
	if err := r.triggerDeletionOfExcessPCSGReplicas(logger, sc); err != nil {
		return err
	}

	// Handle scale-up: create missing PodCliques as per the current PCSG configuration
	if err := r.createExpectedPCLQs(logger, sc); err != nil {
		return err
	}

	// Handle different operational modes based on update status
	if !componentutils.IsPCSGUpdateInProgress(sc.pcsg) {
		// Normal mode: process gang termination for unhealthy replicas
		if err := r.processMinAvailableBreachedPCSGReplicas(logger, sc); err != nil {
			if errors.Is(err, errPCCGMinAvailableBreached) {
				logger.Info("Skipping further reconciliation as MinAvailable for the PCSG has been breached. This can potentially trigger PCS replica deletion.")
				return nil
			}
			return err
		}
	} else {
		// Update mode: process rolling updates
		if err := r.processPendingUpdates(logger, sc); err != nil {
			return err
		}
	}

	// Requeue if there are replicas waiting for termination delay to expire
	if len(sc.pcsgIndicesToRequeue) > 0 {
		return groveerr.New(groveerr.ErrCodeRequeueAfter,
			component.OperationSync,
			"Requeuing to re-process PCLQs that have breached MinAvailable but not crossed TerminationDelay",
		)
	}
	return nil
}

// triggerDeletionOfExcessPCSGReplicas handles scale-down operations by deleting excess PodClique replicas.
// It identifies replicas beyond the target count and triggers their deletion.
func (r _resource) triggerDeletionOfExcessPCSGReplicas(logger logr.Logger, sc *syncContext) error {
	existingPCSGReplicas := getExistingNonTerminatingPCSGReplicas(sc.existingPCLQs)
	// Check if the number of existing PodCliques is greater than expected, if so, we need to delete the extra ones.
	diff := existingPCSGReplicas - int(sc.pcsg.Spec.Replicas)
	if diff > 0 {
		pcsgObjectKey := client.ObjectKeyFromObject(sc.pcsg)
		logger.Info("Found more PodCliques than expected, triggering deletion of excess PodCliques", "expected", int(sc.pcsg.Spec.Replicas), "existing", existingPCSGReplicas, "diff", diff)
		reason := "Delete excess PodCliqueScalingGroup replicas"
		replicaIndicesToDelete := computePCSGReplicasToDelete(existingPCSGReplicas, int(sc.pcsg.Spec.Replicas))
		deletionTasks := r.createDeleteTasks(logger, sc.pcs, pcsgObjectKey.Name, replicaIndicesToDelete, reason)
		if err := r.triggerDeletionOfPodCliques(sc.ctx, logger, pcsgObjectKey, deletionTasks); err != nil {
			return err
		}
		return sc.refreshExistingPCLQs(sc.pcsg)
	}
	return nil
}

// getExistingNonTerminatingPCSGReplicas counts the number of active PCSG replicas.
// It excludes replicas that are currently terminating from the count.
func getExistingNonTerminatingPCSGReplicas(existingPCLQs []grovecorev1alpha1.PodClique) int {
	existingIndices := make([]string, 0, len(existingPCLQs))
	for _, pclq := range existingPCLQs {
		if k8sutils.IsResourceTerminating(pclq.ObjectMeta) {
			continue
		}
		pcsgReplicaIndex, ok := pclq.Labels[apicommon.LabelPodCliqueScalingGroupReplicaIndex]
		if !ok {
			continue
		}
		existingIndices = append(existingIndices, pcsgReplicaIndex)
	}
	return len(lo.Uniq(existingIndices))
}

// computePCSGReplicasToDelete calculates which replica indices should be deleted during scale-down.
// It returns the highest-numbered replica indices that exceed the expected count.
func computePCSGReplicasToDelete(existingReplicas, expectedReplicas int) []string {
	indices := make([]string, 0, existingReplicas-expectedReplicas)
	for i := expectedReplicas; i < existingReplicas; i++ {
		indices = append(indices, strconv.Itoa(i))
	}
	return indices
}

// createExpectedPCLQs creates any missing PodClique resources to match the expected state.
// It creates tasks to build PodCliques that should exist but are currently missing.
func (r _resource) createExpectedPCLQs(logger logr.Logger, sc *syncContext) error {
	var tasks []utils.Task
	existingPCLQFQNs := lo.Map(sc.existingPCLQs, func(pclq grovecorev1alpha1.PodClique, _ int) string { return pclq.Name })
	for pcsgReplicaIndex, expectedPCLQNames := range sc.expectedPCLQFQNsPerPCSGReplica {
		for _, pclqFQN := range expectedPCLQNames {
			if slices.Contains(existingPCLQFQNs, pclqFQN) {
				continue
			}
			pclqObjectKey := client.ObjectKey{
				Name:      pclqFQN,
				Namespace: sc.pcsg.Namespace,
			}
			createTask := utils.Task{
				Name: fmt.Sprintf("CreatePodClique-%s", pclqObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreate(ctx, logger, sc.pcs, sc.pcsg, pcsgReplicaIndex, pclqObjectKey)
				},
			}
			tasks = append(tasks, createTask)
		}
	}
	if runResult := utils.RunConcurrently(sc.ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errCodeCreatePodCliques,
			component.OperationSync,
			fmt.Sprintf("Error Create of PodCliques for PodCliqueScalingGroup: %v, run summary: %s", client.ObjectKeyFromObject(sc.pcsg), runResult.GetSummary()),
		)
	}
	return nil
}

// processMinAvailableBreachedPCSGReplicas handles gang termination of unhealthy replicas.
// It ensures the PCSG's minAvailable constraint is respected before terminating replicas.
func (r _resource) processMinAvailableBreachedPCSGReplicas(logger logr.Logger, sc *syncContext) error {
	// If pcsg.spec.minAvailable is breached, then delegate the responsibility to the PodCliqueSet reconciler which after
	// termination delay terminate the PodCliqueSet replica. No further processing is required to be done here.
	minAvailableBreachedPCSGReplicas := len(sc.pcsgIndicesToTerminate) + len(sc.pcsgIndicesToRequeue)
	if int(sc.pcsg.Spec.Replicas)-minAvailableBreachedPCSGReplicas < int(*sc.pcsg.Spec.MinAvailable) {
		return errPCCGMinAvailableBreached
	}
	// If pcsg.spec.minAvailable is not breached but if there is one more PCSG replica for which there is at least one PCLQ that has
	// its minAvailable breached for a duration > terminationDelay then gang terminate such PCSG replicas.
	if len(sc.pcsgIndicesToTerminate) > 0 {
		logger.Info("Identified PodCliqueScalingGroup indices for gang termination", "indices", sc.pcsgIndicesToTerminate)
		reason := fmt.Sprintf("Delete PodCliques %v for PodCliqueScalingGroup %v which have breached MinAvailable longer than TerminationDelay: %s", sc.pcsgIndicesToTerminate, client.ObjectKeyFromObject(sc.pcsg), sc.pcs.Spec.Template.TerminationDelay.Duration)
		pclqGangTerminationTasks := r.createDeleteTasks(logger, sc.pcs, sc.pcsg.Name, sc.pcsgIndicesToTerminate, reason)
		if err := r.triggerDeletionOfPodCliques(sc.ctx, logger, client.ObjectKeyFromObject(sc.pcsg), pclqGangTerminationTasks); err != nil {
			return err
		}
		return groveerr.New(groveerr.ErrCodeRequeueAfter,
			component.OperationSync,
			fmt.Sprintf("Requeuing post gang termination of PodCliqueScalingGroup replicas: %v", pclqGangTerminationTasks),
		)
	}
	return nil
}

// getMinAvailableBreachedPCSGIndices identifies PCSG replicas with MinAvailable violations.
// It separates replicas into those ready for termination and those still within the grace period.
func getMinAvailableBreachedPCSGIndices(logger logr.Logger, existingPCLQs []grovecorev1alpha1.PodClique, terminationDelay time.Duration) (pcsgIndicesToTerminate []string, pcsgIndicesToRequeue []string) {
	now := time.Now()
	// group existing PCLQs by PCSG replica index. These are PCLQs that belong to one replica of PCSG.
	pcsgReplicaIndexPCLQs := componentutils.GroupPCLQsByPCSGReplicaIndex(existingPCLQs)
	// For each PCSG replica check if minAvailable for any constituent PCLQ has been violated. Those PCSG replicas should be marked for termination.
	for pcsgReplicaIndex, pclqs := range pcsgReplicaIndexPCLQs {
		pclqNames, minWaitFor := componentutils.GetMinAvailableBreachedPCLQInfo(pclqs, terminationDelay, now)
		if len(pclqNames) > 0 {
			logger.Info("minAvailable breached for PCLQs", "pcsgReplicaIndex", pcsgReplicaIndex, "pclqNames", pclqNames, "minWaitFor", minWaitFor)
			if minWaitFor <= 0 {
				pcsgIndicesToTerminate = append(pcsgIndicesToTerminate, pcsgReplicaIndex)
			} else {
				pcsgIndicesToRequeue = append(pcsgIndicesToRequeue, pcsgReplicaIndex)
			}
		}
	}
	return
}

// getExpectedPodCliqueFQNsByPCSGReplica computes the expected PodClique names for each PCSG replica.
// It returns a map where keys are replica indices and values are lists of expected PodClique names.
func getExpectedPodCliqueFQNsByPCSGReplica(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) map[int][]string {
	var (
		expectedPCLQFQNs = make(map[int][]string)
	)
	for pcsgReplicaIndex := range int(pcsg.Spec.Replicas) {
		pclqFQNs := lo.Map(pcsg.Spec.CliqueNames, func(cliqueName string, _ int) string {
			return apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: pcsgReplicaIndex,
			}, cliqueName)
		})
		expectedPCLQFQNs[pcsgReplicaIndex] = pclqFQNs
	}
	return expectedPCLQFQNs
}

// getExistingPCLQs retrieves all PodClique resources currently owned by the PodCliqueScalingGroup.
// It uses label selectors to find PodCliques managed by this PCSG.
func (r _resource) getExistingPCLQs(ctx context.Context, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ([]grovecorev1alpha1.PodClique, error) {
	existingPCLQs, err := componentutils.GetPCLQsByOwner(ctx, r.client, constants.KindPodCliqueScalingGroup, client.ObjectKeyFromObject(pcsg), getPodCliqueSelectorLabels(pcsg.ObjectMeta))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodCliquesForPCSG,
			component.OperationSync,
			fmt.Sprintf("Unable to fetch existing PodCliques for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	return existingPCLQs, nil
}

// getExpectedPCLQPodTemplateHashMap computes expected pod template hashes for all PodCliques in the PCSG.
// These hashes are used to detect when rolling updates are needed.
func getExpectedPCLQPodTemplateHashMap(pcs *grovecorev1alpha1.PodCliqueSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) map[string]string {
	pclqFQNToHash := make(map[string]string)
	pcsgPCLQNames := pcsg.Spec.CliqueNames
	for _, pcsgCliqueName := range pcsgPCLQNames {
		pclqTemplateSpec, ok := lo.Find(pcs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return pclqTemplateSpec.Name == pcsgCliqueName
		})
		if !ok {
			continue
		}
		podTemplateHash := componentutils.ComputePCLQPodTemplateHash(pclqTemplateSpec, pcs.Spec.Template.PriorityClassName)
		for pcsgReplicaIndex := range int(pcsg.Spec.Replicas) {
			cliqueFQN := apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: pcsgReplicaIndex,
			}, pcsgCliqueName)
			pclqFQNToHash[cliqueFQN] = podTemplateHash
		}
	}
	return pclqFQNToHash
}

// refreshExistingPCLQs updates the sync context to exclude PodCliques from deleted PCSG replicas.
// This ensures subsequent operations work with a consistent view of existing resources.
func (sc *syncContext) refreshExistingPCLQs(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	revisedExistingPCLQs := make([]grovecorev1alpha1.PodClique, 0, len(sc.existingPCLQs))
	for _, pclq := range sc.existingPCLQs {
		pcsgReplicaIndexStr, ok := pclq.Labels[apicommon.LabelPodCliqueScalingGroupReplicaIndex]
		if !ok {
			continue
		}
		pcsgReplicaIndex, err := strconv.Atoi(pcsgReplicaIndexStr)
		if err != nil {
			return groveerr.WrapError(err,
				errCodeParsePodCliqueScalingGroupReplicaIndex,
				component.OperationSync,
				fmt.Sprintf("invalid pcsg replica index label value found on PodClique: %v", client.ObjectKeyFromObject(&pclq)),
			)
		}
		if pcsgReplicaIndex < int(pcsg.Spec.Replicas) {
			revisedExistingPCLQs = append(revisedExistingPCLQs, pclq)
		}
	}
	sc.existingPCLQs = revisedExistingPCLQs
	return nil
}
