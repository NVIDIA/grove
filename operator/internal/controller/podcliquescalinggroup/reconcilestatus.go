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

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/common"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileStatus(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	pgs, err := componentutils.GetOwnerPodGangSet(ctx, r.client, pcsg.ObjectMeta)
	if err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to get owner PodGangSet", err)
	}

	if pcsg.Status.ObservedGeneration != nil {
		pclqsPerPCSGReplica, err := r.getPodCliquesPerPCSGReplica(ctx, pgs.Name, client.ObjectKeyFromObject(pcsg))
		if err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("failed to list PodCliques for PodCliqueScalingGroup: %q", client.ObjectKeyFromObject(pcsg)), err)
		}
		mutateReplicas(logger, pcsg, pclqsPerPCSGReplica)
		mutateMinAvailableBreachedCondition(pcsg)
	}

	if err = mutateSelector(pgs, pcsg); err != nil {
		logger.Error(err, "failed to update selector for PodCliqueScalingGroup")
		return ctrlcommon.ReconcileWithErrors("failed to update selector for PodCliqueScalingGroup", err)
	}

	if err = r.client.Status().Update(ctx, pcsg); err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to update the status with label selector and replicas", err)
	}

	return ctrlcommon.ContinueReconcile()
}

// computeReplicaStatus processes a single PodCliqueScalingGroup replica and returns whether it is scheduled and available.
// It checks if the replica has all expected PodCliques, and if so, it determines
// if the replica is scheduled and available based on the conditions of the PodCliques.
func computeReplicaStatus(logger logr.Logger, expectedPCSGReplicaPCLQSize int, pcsgReplicaIndex string, pclqs []grovecorev1alpha1.PodClique) (bool, bool) {
	nonTerminatedPCLQs := lo.Filter(pclqs, func(pclq grovecorev1alpha1.PodClique, _ int) bool {
		return !k8sutils.IsResourceTerminating(pclq.ObjectMeta)
	})

	if len(nonTerminatedPCLQs) < expectedPCSGReplicaPCLQSize {
		logger.Info("PodCliqueScalingGroup replica does not have all expected PodCliques",
			"pcsgReplicaIndex", pcsgReplicaIndex, "expectedPCSGReplicaPCLQSize", expectedPCSGReplicaPCLQSize, "actualPCLQsCount", len(pclqs))
		return false, false
	}

	var isScheduled, isAvailable bool
	isScheduled = lo.Reduce(nonTerminatedPCLQs, func(agg bool, pclq grovecorev1alpha1.PodClique, _ int) bool {
		return agg && k8sutils.IsConditionTrue(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypePodCliqueScheduled)
	}, true)
	if isScheduled {
		isAvailable = lo.Reduce(nonTerminatedPCLQs, func(agg bool, pclq grovecorev1alpha1.PodClique, _ int) bool {
			return agg && k8sutils.IsConditionFalse(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached)
		}, true)
	}
	logger.Info("PodCliqueScalingGroup replica status", "pcsgReplicaIndex", pcsgReplicaIndex, "isScheduled", isScheduled, "isAvailable", isAvailable)
	return isScheduled, isAvailable
}

func mutateReplicas(logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pclqsPerPCSGReplica map[string][]grovecorev1alpha1.PodClique) {
	pcsg.Status.Replicas = pcsg.Spec.Replicas
	var scheduledReplicas, availableReplicas int32
	for pcsgReplicaIndex, pclqs := range pclqsPerPCSGReplica {
		isScheduled, isAvailable := computeReplicaStatus(logger, len(pcsg.Spec.CliqueNames), pcsgReplicaIndex, pclqs)
		if isScheduled {
			scheduledReplicas++
		}
		if isAvailable {
			availableReplicas++
		}
	}
	logger.Info("Mutating PodCliqueScalingGroup replicas",
		"pcsg", client.ObjectKeyFromObject(pcsg),
		"scheduledReplicas", scheduledReplicas, "availableReplicas", availableReplicas)
	pcsg.Status.ScheduledReplicas = scheduledReplicas
	pcsg.Status.AvailableReplicas = availableReplicas
}

func mutateMinAvailableBreachedCondition(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) {
	newCondition := computeMinAvailableBreachedCondition(pcsg)
	if k8sutils.HasConditionChanged(pcsg.Status.Conditions, newCondition) {
		meta.SetStatusCondition(&pcsg.Status.Conditions, newCondition)
	}
}

func computeMinAvailableBreachedCondition(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) metav1.Condition {
	// its should not be possible to have a PCSG without MinAvailable defined, but if it happens then we assume that the min available is 1
	// to prevent case of passing nil to int conversion
	minAvailable := int(ptr.Deref(pcsg.Spec.MinAvailable, pcsg.Spec.Replicas))
	scheduledReplicas := int(pcsg.Status.ScheduledReplicas)
	if scheduledReplicas < minAvailable {
		return metav1.Condition{
			Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
			Status:  metav1.ConditionFalse,
			Reason:  grovecorev1alpha1.ConditionReasonInsufficientScheduledPCSGReplicas,
			Message: fmt.Sprintf("Insufficient scheduled replicas. expected at least: %d, found: %d", minAvailable, scheduledReplicas),
		}
	}
	availableReplicas := int(pcsg.Status.AvailableReplicas)
	if availableReplicas < minAvailable {
		return metav1.Condition{
			Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
			Status:  metav1.ConditionTrue,
			Reason:  grovecorev1alpha1.ConditionReasonInsufficientAvailablePCSGReplicas,
			Message: fmt.Sprintf("Insufficient PodCliqueScalingGroup ready replicas, expected at least: %d, found: %d", minAvailable, availableReplicas),
		}
	}
	return metav1.Condition{
		Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
		Status:  metav1.ConditionFalse,
		Reason:  grovecorev1alpha1.ConditionReasonSufficientAvailablePCSGReplicas,
		Message: fmt.Sprintf("Sufficient PodCliqueScalingGroup ready replicas, expected at least: %d, found: %d", minAvailable, availableReplicas),
	}
}

func (r *Reconciler) getPodCliquesPerPCSGReplica(ctx context.Context, pgsName string, pcsgObjKey client.ObjectKey) (map[string][]grovecorev1alpha1.PodClique, error) {
	selectorLabels := lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsgObjKey.Name,
			grovecorev1alpha1.LabelComponentKey:          common.NamePCSGPodClique,
		},
	)
	pclqs, err := componentutils.GetPCLQsByOwner(ctx,
		r.client,
		grovecorev1alpha1.PodCliqueScalingGroupKind,
		pcsgObjKey,
		selectorLabels,
	)
	if err != nil {
		return nil, err
	}
	pclqsPerPCSGReplica := componentutils.GroupPCLQsByPCSGReplicaIndex(pclqs)
	return pclqsPerPCSGReplica, nil
}

func mutateSelector(pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	pgsReplicaIndex, err := k8sutils.GetPodGangSetReplicaIndex(pcsg.ObjectMeta)
	if err != nil {
		return err
	}
	matchingPCSGConfig, ok := lo.Find(pgs.Spec.Template.PodCliqueScalingGroupConfigs, func(pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig) bool {
		pcsgFQN := grovecorev1alpha1.GeneratePodCliqueScalingGroupName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex}, pcsgConfig.Name)
		return pcsgFQN == pcsg.Name
	})
	if !ok {
		// This should ideally never happen but if you find a PCSG that is not defined in PGS then just ignore it.
		return nil
	}
	// No ScaleConfig has been defined of this PCSG, therefore there is no need to add a selector in the status.
	if matchingPCSGConfig.ScaleConfig == nil {
		return nil
	}
	labels := lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgs.Name),
		map[string]string{
			grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsg.Name,
		},
	)
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return fmt.Errorf("%w: failed to create label selector for PodCliqueScalingGroup %v", err, client.ObjectKeyFromObject(pcsg))
	}
	pcsg.Status.Selector = ptr.To(selector.String())
	return nil
}
