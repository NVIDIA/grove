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
	"strconv"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	"github.com/NVIDIA/grove/operator/internal/status"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileStatus(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) ctrlcommon.ReconcileStepResult {
	if pgs.Status.ObservedGeneration == nil {
		return ctrlcommon.ContinueReconcile()
	}
	// Set basic replica count
	pgs.Status.Replicas = pgs.Spec.Replicas
	// Calculate available replicas using PCSG-inspired approach
	availableReplicas, err := r.computePGSAvailableReplicas(ctx, logger, pgs)
	if err != nil {
		logger.Error(err, "failed to calculate available replicas")
		return ctrlcommon.ReconcileWithErrors("failed to calculate available replicas", err)
	}
	pgs.Status.AvailableReplicas = availableReplicas

	// Update the PodGangSet status
	if err = r.client.Status().Update(ctx, pgs); err != nil {
		logger.Error(err, "failed to update PodGangSet status")
		return ctrlcommon.ReconcileWithErrors("failed to update PodGangSet status", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) computePGSAvailableReplicas(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) (int32, error) {
	availableReplicas := int32(0)

	// Calculate expected resource counts per replica (same for all replicas)
	expectedStandalonePCLQs, expectedPCSGs := r.computeExpectedResourceCounts(pgs)

	// Fetch all PCSGs for this PGS
	pcsgs, err := componentutils.GetPCSGsForPGS(ctx, r.client, client.ObjectKeyFromObject(pgs))
	if err != nil {
		return 0, err
	}

	// Fetch all standalone PodCliques for this PGS
	standalonePCLQs, err := componentutils.GetPodCliqueForPGSNotInPCSG(ctx, r.client, client.ObjectKeyFromObject(pgs))
	if err != nil {
		return 0, err
	}

	// Group both resources by PGS replica index
	standalonePCLQsByReplica := componentutils.GroupPCLQsByPGSReplicaIndex(standalonePCLQs)
	pcsgsByReplica := componentutils.GroupPCSGsByPGSReplicaIndex(pcsgs)

	for replicaIndex := 0; replicaIndex < int(pgs.Spec.Replicas); replicaIndex++ {
		replicaIndexStr := strconv.Itoa(replicaIndex)

		replicaStandalonePCLQs := standalonePCLQsByReplica[replicaIndexStr]
		replicaPCSGs := pcsgsByReplica[replicaIndexStr]

		// Check if this PGS replica is available based on all its components
		isReplicaAvailable := r.checkPGSReplicaAvailability(logger, pgs, replicaIndex, replicaPCSGs, replicaStandalonePCLQs, expectedPCSGs, expectedStandalonePCLQs)

		if isReplicaAvailable {
			availableReplicas++
		}
	}

	logger.Info("Calculated available replicas for PGS", "pgs", client.ObjectKeyFromObject(pgs), "availableReplicas", availableReplicas, "totalReplicas", pgs.Spec.Replicas)
	return availableReplicas, nil
}

func (r *Reconciler) computeExpectedResourceCounts(pgs *grovecorev1alpha1.PodGangSet) (expectedStandalonePCLQs, expectedPCSGs int) {
	// Count expected PCSGs - this is the number of unique scaling group configs
	expectedPCSGs = len(pgs.Spec.Template.PodCliqueScalingGroupConfigs)

	// Count expected standalone PodCliques - cliques that are NOT part of any scaling group
	expectedStandalonePCLQs = 0
	for _, cliqueSpec := range pgs.Spec.Template.Cliques {
		pcsgConfig := componentutils.FindScalingGroupConfigForClique(pgs.Spec.Template.PodCliqueScalingGroupConfigs, cliqueSpec.Name)
		if pcsgConfig == nil {
			// This clique is not part of any scaling group, so it's standalone
			expectedStandalonePCLQs++
		}
	}

	return expectedStandalonePCLQs, expectedPCSGs
}

func (r *Reconciler) checkPGSReplicaAvailability(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, replicaIndex int, replicaPCSGs []grovecorev1alpha1.PodCliqueScalingGroup, standalonePCLQs []grovecorev1alpha1.PodClique, expectedPCSGs int, expectedStandalonePCLQs int) bool {
	if !status.ValidateReplicaAvailable(
		logger, standalonePCLQs, expectedStandalonePCLQs,
		status.GetPCLQCondition,
		grovecorev1alpha1.PodCliqueKind, replicaIndex,
	) {
		return false
	}

	if !status.ValidateReplicaAvailable(
		logger, replicaPCSGs, expectedPCSGs,
		status.GetPCSGCondition,
		grovecorev1alpha1.PodCliqueScalingGroupKind, replicaIndex,
	) {
		return false
	}

	logger.Info("PGS replica is available", "pgs", client.ObjectKeyFromObject(pgs), "replicaIndex", replicaIndex)
	return true
}
