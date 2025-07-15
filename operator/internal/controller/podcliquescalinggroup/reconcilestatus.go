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
	pcsgPCLQs, err := componentutils.GetPodCliquesByOwner(ctx,
		r.client,
		grovecorev1alpha1.PodCliqueScalingGroupKind,
		client.ObjectKeyFromObject(pcsg),
		selectorLabels,
	)
	if err != nil {
		return nil, err
	}

	// group PodCliques per PodGang
	podGangPCLQs := componentutils.GroupPCLQsByPodGangName(pcsgPCLQs)
	minAvailableBreachedPodGangNames := make([]string, 0, len(podGangPCLQs))
	for podGangName, pclqs := range podGangPCLQs {
		minAvailableBreached := lo.Reduce(pclqs, func(agg bool, pclq grovecorev1alpha1.PodClique, _ int) bool {
			return agg || k8sutils.IsConditionTrue(pclq.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached)
		}, false)
		if minAvailableBreached {
			minAvailableBreachedPodGangNames = append(minAvailableBreachedPodGangNames, podGangName)
			logger.Info("PodGang has minAvailable breached", "podGangName", podGangName)
		}
	}

	if int(pcsg.Spec.Replicas)-len(minAvailableBreachedPodGangNames) < defaultPCSGMinAvailable {
		return &metav1.Condition{
			Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
			Status:  metav1.ConditionTrue,
			Reason:  "InsufficientReadyPodCliqueScalingGroupReplicas",
			Message: fmt.Sprintf("Insufficient PodCliqueScalingGroup replicas, expected at least: %d, found: %d", defaultPCSGMinAvailable, len(minAvailableBreachedPodGangNames)),
		}, nil
	}

	return &metav1.Condition{
		Type:    grovecorev1alpha1.ConditionTypeMinAvailableBreached,
		Status:  metav1.ConditionFalse,
		Reason:  "SufficientReadyPodCliqueScalingGroupReplicas",
		Message: fmt.Sprintf("Sufficient PodCliqueScalingGroup replicas, expected at least: %d, found: %d", defaultPCSGMinAvailable, len(minAvailableBreachedPodGangNames)),
	}, nil
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
