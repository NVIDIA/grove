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
	"github.com/NVIDIA/grove/operator/internal/component"
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

// defaultPCSGMinAvailable is the default value of minAvailable for PCSG. Currently, this is not configurable via the API.
// Once it is made configurable then this constant is no longer required and should be removed.
const defaultPCSGMinAvailable = 1

func (r *Reconciler) reconcileStatus(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	pcsg.Status.Replicas = pcsg.Spec.Replicas

	pgs, err := componentutils.GetOwnerPodGangSet(ctx, r.client, pcsg.ObjectMeta)
	if err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to get owner PodGangSet", err)
	}

	if err = r.mutateMinAvailableBreachedCondition(ctx, logger, pgs.Name, pcsg); err != nil {
		logger.Error(err, "failed to mutate minAvailable breachedCondition")
		return ctrlcommon.ReconcileWithErrors("failed to mutate minAvailable breachedCondition", err)
	}

	if err = mutateSelector(pgs, pcsg); err != nil {
		logger.Error(err, "failed to update selector for PodCliqueScalingGroup")
		return ctrlcommon.ReconcileWithErrors("failed to update selector for PodCliqueScalingGroup", err)
	}

	if err := r.client.Status().Update(ctx, pcsg); err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to update the status with label selector and replicas", err)
	}

	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) mutateMinAvailableBreachedCondition(ctx context.Context, logger logr.Logger, pgsName string, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	newCondition, err := r.computeMinAvailableBreachedCondition(ctx, logger, pgsName, pcsg)
	if err != nil {
		return err
	}
	if k8sutils.HasConditionChanged(pcsg.Status.Conditions, *newCondition) {
		meta.SetStatusCondition(&pcsg.Status.Conditions, *newCondition)
	}
	return nil
}

func (r *Reconciler) computeMinAvailableBreachedCondition(ctx context.Context, logger logr.Logger, pgsName string, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) (*metav1.Condition, error) {
	selectorLabels := lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsg.Name,
			grovecorev1alpha1.LabelComponentKey:          component.NamePCSGPodClique,
		},
	)
	pcsgPCLQs, err := componentutils.GetPCLQsByOwner(ctx,
		r.client,
		grovecorev1alpha1.PodCliqueScalingGroupKind,
		client.ObjectKeyFromObject(pcsg),
		selectorLabels,
	)
	if err != nil {
		return nil, err
	}

	// group PodCliques per PodCliqueScalingGroup
	pcsgReplicaPCLQs := componentutils.GroupPCLQsByPCSGReplicaIndex(pcsgPCLQs)
	minAvailableBreachedPCSGReplicaIndexes := make([]string, 0, pcsg.Spec.Replicas)
	var atleastOnePCSGReplicaIsUnknown bool
	for pcsgReplicaIndex, pclqs := range pcsgReplicaPCLQs {
		pcsgReplicaCondStatus := computeMinAvailableBreachedStatusForPSGReplica(logger, pcsgReplicaIndex, pclqs)
		logger.Info("MinAvailableBreached condition status for PCSG replica", "pcsgReplicaIndex", pcsgReplicaIndex, "conditionStatus", pcsgReplicaCondStatus)
		switch pcsgReplicaCondStatus {
		case metav1.ConditionTrue:
			minAvailableBreachedPCSGReplicaIndexes = append(minAvailableBreachedPCSGReplicaIndexes, pcsgReplicaIndex)
		case metav1.ConditionFalse:
			atleastOnePCSGReplicaIsUnknown = true
		}
	}

	numReadyPCSGReplicas := int(pcsg.Spec.Replicas) - len(minAvailableBreachedPCSGReplicaIndexes)
	if numReadyPCSGReplicas < defaultPCSGMinAvailable {
		return &metav1.Condition{
			Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
			Status:  metav1.ConditionTrue,
			Reason:  "InsufficientReadyPodCliqueScalingGroupReplicas",
			Message: fmt.Sprintf("Insufficient PodCliqueScalingGroup replicas, expected at least: %d, found: %d", defaultPCSGMinAvailable, numReadyPCSGReplicas),
		}, nil
	}

	if atleastOnePCSGReplicaIsUnknown {
		return &metav1.Condition{
			Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
			Status:  metav1.ConditionUnknown,
			Reason:  "PodCliqueScalingGroupReplicasInUnknownState",
			Message: "PodCliqueScalingGroup replicas in unknown state",
		}, nil
	}

	return &metav1.Condition{
		Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
		Status:  metav1.ConditionFalse,
		Reason:  "SufficientReadyPodCliqueScalingGroupReplicas",
		Message: fmt.Sprintf("Sufficient PodCliqueScalingGroup replicas, expected at least: %d, found: %d", defaultPCSGMinAvailable, numReadyPCSGReplicas),
	}, nil
}

func computeMinAvailableBreachedStatusForPSGReplica(logger logr.Logger, pcsgReplicaIndex string, pclqs []grovecorev1alpha1.PodClique) metav1.ConditionStatus {
	var atleastOneStatusIsUnknown bool
	for _, pclq := range pclqs {
		logger.Info("MinAvailable condition status for PCLQ", "pclq", client.ObjectKeyFromObject(&pclq), "status", k8sutils.GetConditionStatus(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached))
		if k8sutils.IsConditionTrue(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached) {
			logger.Info("PodClique has MinAvailableBreached condition set to true", "pcsgReplicaIndex", pcsgReplicaIndex, "pclq", client.ObjectKeyFromObject(&pclq))
			return metav1.ConditionTrue
		}
		if k8sutils.IsResourceTerminating(pclq.ObjectMeta) {
			atleastOneStatusIsUnknown = true
			logger.Info("PCLQ is marked for termination, cannot determine the MinAvailableBreached condition status, assuming Unknown", "pcsgReplicaIndex", pcsgReplicaIndex, "pclq", client.ObjectKeyFromObject(&pclq))
			continue
		}
		if pclq.Status.Conditions == nil ||
			k8sutils.IsConditionUnknown(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached) {
			logger.Info("PodClique has MinAvailableBreached condition is either not set yet or is set to Unknown", "pcsgReplicaIndex", pcsgReplicaIndex, "pclq", client.ObjectKeyFromObject(&pclq))
			atleastOneStatusIsUnknown = true
		}
	}
	return lo.Ternary(atleastOneStatusIsUnknown, metav1.ConditionUnknown, metav1.ConditionFalse)
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
