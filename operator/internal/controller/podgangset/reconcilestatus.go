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

package podgangset

import (
	"context"
	"fmt"
	"strconv"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileStatus(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	// Calculate available replicas using PCSG-inspired approach
	err := r.mutateReplicas(ctx, logger, pgs)
	if err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to mutate replicas status", err)
	}

	// Update the PodCliqueSet status
	if err = r.client.Status().Update(ctx, pgs); err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to update PodCliqueSet status", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) mutateReplicas(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) error {
	// Set basic replica count
	pgs.Status.Replicas = pgs.Spec.Replicas
	availableReplicas, updatedReplicas, err := r.computeAvailableAndUpdatedReplicas(ctx, logger, pgs)
	if err != nil {
		return fmt.Errorf("could not compute available replicas: %w", err)
	}
	pgs.Status.AvailableReplicas = availableReplicas
	pgs.Status.UpdatedReplicas = updatedReplicas
	return nil
}

// computeAvailableAndUpdatedReplicas calculates the number of available replicas for a PodCliqueSet.
// It checks both standalone PodCliques and PodCliqueScalingGroups to determine availability.
// A replica is considered available if it has all its required components (PCSGs and standalone PCLQs) available.
func (r *Reconciler) computeAvailableAndUpdatedReplicas(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) (int32, int32, error) {
	var (
		availableReplicas int32
		updatedReplicas   int32
		pgsObjectKey      = client.ObjectKeyFromObject(pgs)
	)

	expectedPCSGFQNsPerPGSReplica := componentutils.GetExpectedPCSGFQNsPerPCSReplica(pgs)
	expectedStandAlonePCLQFQNsPerPGSReplica := componentutils.GetExpectedStandAlonePCLQFQNsPerPCSReplica(pgs)

	// Fetch all PCSGs for this PGS
	pcsgs, err := componentutils.GetPCSGsForPCS(ctx, r.client, pgsObjectKey)
	if err != nil {
		return availableReplicas, updatedReplicas, err
	}
	// Filter the PCSGs that belong to the expected set of PCSGs for PGS, this ensures that we do not
	// consider any stray PCSGs that might have been created externally.
	pcsgs = lo.Filter(pcsgs, func(pcsg grovecorev1alpha1.PodCliqueScalingGroup, _ int) bool {
		return lo.Contains(lo.Flatten(lo.Values(expectedPCSGFQNsPerPGSReplica)), pcsg.Name)
	})

	// Fetch all standalone PodCliques for this PGS
	standalonePCLQs, err := componentutils.GetPodCliquesWithParentPCS(ctx, r.client, pgsObjectKey)
	if err != nil {
		return availableReplicas, updatedReplicas, err
	}
	// Filter the PCLQs that belong to the expected set of standalone PCLQs for PGS, this ensures that we do not
	// consider any stray PCLQs that might have been created externally.
	standalonePCLQs = lo.Filter(standalonePCLQs, func(pclq grovecorev1alpha1.PodClique, _ int) bool {
		return lo.Contains(lo.Flatten(lo.Values(expectedStandAlonePCLQFQNsPerPGSReplica)), pclq.Name)
	})

	// Group both resources by PGS replica index
	standalonePCLQsByReplica := componentutils.GroupPCLQsByPCSReplicaIndex(standalonePCLQs)
	pcsgsByReplica := componentutils.GroupPCSGsByPCSReplicaIndex(pcsgs)

	for replicaIndex := 0; replicaIndex < int(pgs.Spec.Replicas); replicaIndex++ {
		replicaIndexStr := strconv.Itoa(replicaIndex)
		replicaStandalonePCLQs := standalonePCLQsByReplica[replicaIndexStr]
		replicaPCSGs := pcsgsByReplica[replicaIndexStr]
		// Check if this PGS replica is available based on all its components
		isReplicaAvailable, isReplicaUpdated := r.computeReplicaStatus(pgs.Status.CurrentGenerationHash, replicaPCSGs,
			replicaStandalonePCLQs, len(expectedPCSGFQNsPerPGSReplica[replicaIndex]), len(expectedStandAlonePCLQFQNsPerPGSReplica[replicaIndex]))
		if isReplicaAvailable {
			availableReplicas++
		}
		if isReplicaUpdated {
			updatedReplicas++
		}
	}

	logger.Info("Calculated available and updated replicas for PGS", "pgs", pgsObjectKey, "availableReplicas", availableReplicas, "updatedReplicas", updatedReplicas, "totalReplicas", pgs.Spec.Replicas)
	return availableReplicas, updatedReplicas, nil
}

func (r *Reconciler) computeReplicaStatus(pgsGenerationHash *string, replicaPCSGs []grovecorev1alpha1.PodCliqueScalingGroup, standalonePCLQs []grovecorev1alpha1.PodClique, expectedPCSGs int, expectedStandalonePCLQs int) (bool, bool) {
	pclqsAvailable, pclqsUpdated := r.computePCLQsStatus(expectedStandalonePCLQs, standalonePCLQs)
	pcsgsAvailable, pcsgsUpdated := r.computePCSGsStatus(pgsGenerationHash, expectedPCSGs, replicaPCSGs)
	return pclqsAvailable && pcsgsAvailable, pclqsUpdated && pcsgsUpdated
}

func (r *Reconciler) computePCLQsStatus(expectedStandalonePCLQs int, existingPCLQs []grovecorev1alpha1.PodClique) (isAvailable, isUpdated bool) {
	nonTerminatedPCLQs := lo.Filter(existingPCLQs, func(pclq grovecorev1alpha1.PodClique, _ int) bool {
		return !k8sutils.IsResourceTerminating(pclq.ObjectMeta)
	})

	isAvailable = len(nonTerminatedPCLQs) == expectedStandalonePCLQs &&
		lo.EveryBy(nonTerminatedPCLQs, func(pclq grovecorev1alpha1.PodClique) bool {
			return pclq.Status.ReadyReplicas >= *pclq.Spec.MinAvailable
		})

	isUpdated = isAvailable && lo.EveryBy(nonTerminatedPCLQs, func(pclq grovecorev1alpha1.PodClique) bool {
		return pclq.Status.UpdatedReplicas >= *pclq.Spec.MinAvailable
	})

	return
}

func (r *Reconciler) computePCSGsStatus(pgsGenerationHash *string, expectedPCSGs int, pcsgs []grovecorev1alpha1.PodCliqueScalingGroup) (isAvailable, isUpdated bool) {
	nonTerminatedPCSGs := lo.Filter(pcsgs, func(pcsg grovecorev1alpha1.PodCliqueScalingGroup, _ int) bool {
		return !k8sutils.IsResourceTerminating(pcsg.ObjectMeta)
	})

	isAvailable = expectedPCSGs == len(nonTerminatedPCSGs) &&
		lo.EveryBy(nonTerminatedPCSGs, func(pcsg grovecorev1alpha1.PodCliqueScalingGroup) bool {
			return pcsg.Status.AvailableReplicas >= *pcsg.Spec.MinAvailable
		})

	isUpdated = isAvailable && lo.EveryBy(nonTerminatedPCSGs, func(pcsg grovecorev1alpha1.PodCliqueScalingGroup) bool {
		return pgsGenerationHash != nil && componentutils.IsPCSGUpdateComplete(&pcsg, *pgsGenerationHash)
	})

	return
}
