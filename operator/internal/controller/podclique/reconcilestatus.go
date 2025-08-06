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

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileStatus(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	pgsName := componentutils.GetPodGangSetName(pclq.ObjectMeta)

	existingPods, err := componentutils.GetPCLQPods(ctx, r.client, pgsName, pclq)
	if err != nil {
		logger.Error(err, "failed to list pods for PodClique")
		return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("failed to list pods for PodClique: %q", client.ObjectKeyFromObject(pclq)), err)
	}

	podCategories := k8sutils.CategorizePodsByConditionType(logger, existingPods)

	// mutate PodClique Status Replicas, ReadyReplicas, ScheduleGatedReplicas and UpdatedReplicas.
	mutateStatusReplicaCounts(pclq, podCategories, len(existingPods))

	// mutate the conditions only if the PodClique has been successfully reconciled at least once.
	// This prevents prematurely setting incorrect conditions.
	if pclq.Status.ObservedGeneration != nil {
		mutatePodCliqueScheduledCondition(pclq)
		mutateMinAvailableBreachedCondition(pclq,
			len(podCategories[k8sutils.PodHasAtleastOneContainerWithNonZeroExitCode]),
			len(podCategories[k8sutils.PodStartedButNotReady]))
	}

	// mutate the selector that will be used by an autoscaler.
	if err := mutateSelector(pgsName, pclq); err != nil {
		logger.Error(err, "failed to update selector for PodClique")
		return ctrlcommon.ReconcileWithErrors("failed to set selector for PodClique", err)
	}

	// update the PodClique status.
	if err := r.client.Status().Update(ctx, pclq); err != nil {
		logger.Error(err, "failed to update PodClique status")
		return ctrlcommon.ReconcileWithErrors("failed to update PodClique status", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func mutateStatusReplicaCounts(pclq *grovecorev1alpha1.PodClique, podCategories map[corev1.PodConditionType][]*corev1.Pod, numExistingPods int) {
	// mutate the PCLQ status with current number of schedule gated, ready pods and updated pods.
	numNonTerminatingPods := int32(numExistingPods - len(podCategories[k8sutils.TerminatingPod]))
	pclq.Status.Replicas = numNonTerminatingPods
	pclq.Status.ReadyReplicas = int32(len(podCategories[corev1.PodReady]))
	pclq.Status.ScheduleGatedReplicas = int32(len(podCategories[k8sutils.ScheduleGatedPod]))
	pclq.Status.ScheduledReplicas = int32(len(podCategories[corev1.PodScheduled]))
	// TODO: change this when rolling update is implemented
	pclq.Status.UpdatedReplicas = numNonTerminatingPods
}

func mutateSelector(pgsName string, pclq *grovecorev1alpha1.PodClique) error {
	if pclq.Spec.ScaleConfig == nil {
		return nil
	}
	labels := lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelPodClique: pclq.Name,
		},
	)
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return fmt.Errorf("%w: failed to create label selector for PodClique %v", err, client.ObjectKeyFromObject(pclq))
	}
	pclq.Status.Selector = ptr.To(selector.String())
	return nil
}

func mutateMinAvailableBreachedCondition(pclq *grovecorev1alpha1.PodClique, numNotReadyPodsWithContainersInError, numPodsStartedButNotReady int) {
	newCondition := computeMinAvailableBreachedCondition(pclq, numNotReadyPodsWithContainersInError, numPodsStartedButNotReady)
	if k8sutils.HasConditionChanged(pclq.Status.Conditions, newCondition) {
		meta.SetStatusCondition(&pclq.Status.Conditions, newCondition)
	}
}

func computeMinAvailableBreachedCondition(pclq *grovecorev1alpha1.PodClique, numPodsHavingAtleastOneContainerWithNonZeroExitCode, numPodsStartedButNotReady int) metav1.Condition {
	// dereferencing is considered safe as MinAvailable will always be set by the defaulting webhook. If this changes in the future,
	// make sure that you check for nil explicitly.
	minAvailable := int(*pclq.Spec.MinAvailable)
	scheduledReplicas := int(pclq.Status.ScheduledReplicas)
	now := metav1.Now()

	// If the number of scheduled pods is less than the minimum available, then minAvailable is not considered as breached.
	// Consider a case where none of the PodCliques have been scheduled yet, then it should not cause the PodGang to be recreated all the time.
	if scheduledReplicas < minAvailable {
		return metav1.Condition{
			Type:               grovecorev1alpha1.ConditionTypeMinAvailableBreached,
			Status:             metav1.ConditionFalse,
			Reason:             grovecorev1alpha1.ConditionReasonInsufficientScheduledPods,
			Message:            fmt.Sprintf("Insufficient scheduled pods. expected at least: %d, found: %d", minAvailable, scheduledReplicas),
			LastTransitionTime: now,
		}
	}

	readyOrStartingPods := scheduledReplicas - numPodsHavingAtleastOneContainerWithNonZeroExitCode - numPodsStartedButNotReady
	// pclq.Status.ReadyReplicas do not account for Pods which are not yet ready and are in the process of starting/initializing.
	// This allows sufficient time specially for pods that have long-running init containers or slow-to-start main containers.
	// Therefore, we take Pods that are NotReady and at least one of their containers have exited with a non-zero exit code. Kubelet
	// has attempted to start the containers within the Pod at least once and failed. These pods count towards unavailability.
	if readyOrStartingPods < minAvailable {
		return metav1.Condition{
			Type:               grovecorev1alpha1.ConditionTypeMinAvailableBreached,
			Status:             metav1.ConditionTrue,
			Reason:             grovecorev1alpha1.ConditionReasonInsufficientReadyPods,
			Message:            fmt.Sprintf("Insufficient ready or starting pods. expected at least: %d, found: %d", minAvailable, readyOrStartingPods),
			LastTransitionTime: now,
		}
	}
	return metav1.Condition{
		Type:               grovecorev1alpha1.ConditionTypeMinAvailableBreached,
		Status:             metav1.ConditionFalse,
		Reason:             grovecorev1alpha1.ConditionReasonSufficientReadyPods,
		Message:            fmt.Sprintf("Either sufficient ready or starting pods found. expected at least: %d, found: %d", minAvailable, readyOrStartingPods),
		LastTransitionTime: now,
	}
}

func mutatePodCliqueScheduledCondition(pclq *grovecorev1alpha1.PodClique) {
	newCondition := computePodCliqueScheduledCondition(pclq)
	if k8sutils.HasConditionChanged(pclq.Status.Conditions, newCondition) {
		meta.SetStatusCondition(&pclq.Status.Conditions, newCondition)
	}
}

func computePodCliqueScheduledCondition(pclq *grovecorev1alpha1.PodClique) metav1.Condition {
	now := metav1.Now()
	if pclq.Status.ScheduledReplicas < *pclq.Spec.MinAvailable {
		return metav1.Condition{
			Type:               grovecorev1alpha1.ConditionTypePodCliqueScheduled,
			Status:             metav1.ConditionFalse,
			Reason:             grovecorev1alpha1.ConditionReasonInsufficientScheduledPods,
			Message:            fmt.Sprintf("Insufficient scheduled pods. expected at least: %d, found: %d", *pclq.Spec.MinAvailable, pclq.Status.ScheduledReplicas),
			LastTransitionTime: now,
		}
	}
	return metav1.Condition{
		Type:               grovecorev1alpha1.ConditionTypePodCliqueScheduled,
		Status:             metav1.ConditionTrue,
		Reason:             grovecorev1alpha1.ConditionReasonSufficientScheduledPods,
		Message:            fmt.Sprintf("Sufficient scheduled pods found. expected at least: %d, found: %d", *pclq.Spec.MinAvailable, pclq.Status.ScheduledReplicas),
		LastTransitionTime: now,
	}
}
