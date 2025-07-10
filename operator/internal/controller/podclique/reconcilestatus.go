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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) reconcileStatus(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	pgsName := componentutils.GetPodGangSetName(pclq.ObjectMeta)
	if pgsName == nil {
		logger.Error(nil, "PodClique does not have required label", "label", grovecorev1alpha1.LabelPartOfKey, "pclqObjectKey", client.ObjectKeyFromObject(pclq))
		return ctrlcommon.ReconcileWithErrors(
			"PodClique does not have the required label to determine parent PodGangSet name",
			fmt.Errorf("PodClique %v does not have required label %v", client.ObjectKeyFromObject(pclq), grovecorev1alpha1.LabelPartOfKey))
	}
	pods, err := componentutils.GetPCLQPods(ctx, r.client, *pgsName, pclq)
	if err != nil {
		logger.Error(err, "failed to get pod list for PodClique")
		ctrlcommon.ReconcileWithErrors(fmt.Sprintf("failed to list pods for PodClique %v", client.ObjectKeyFromObject(pclq)), err)
	}

	pclq.Status.Replicas = int32(len(pods))
	readyPods, scheduleGatedPods := getReadyAndScheduleGatedPods(pods)
	pclq.Status.ReadyReplicas = int32(len(readyPods))
	pclq.Status.ScheduleGatedReplicas = int32(len(scheduleGatedPods))

	// TODO: change this when rolling update is implemented
	pclq.Status.UpdatedReplicas = int32(len(pods))
	if err := updateSelector(*pgsName, pclq); err != nil {
		logger.Error(err, "failed to update selector for PodClique")
		ctrlcommon.ReconcileWithErrors("failed to set selector for PodClique", err)
	}

	if err := r.client.Status().Update(ctx, pclq); err != nil {
		logger.Error(err, "failed to update PodClique status")
		return ctrlcommon.ReconcileWithErrors("failed to update PodClique status", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func updateSelector(pgsName string, pclq *grovecorev1alpha1.PodClique) error {
	if pclq.Spec.ScaleConfig == nil {
		return nil
	}

	labels := lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelPodCliqueName: pclq.Name,
		},
	)
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return fmt.Errorf("%w: failed to create label selector for PodClique %v", err, client.ObjectKeyFromObject(pclq))
	}
	pclq.Status.Selector = ptr.To(selector.String())
	return nil
}

func getReadyAndScheduleGatedPods(pods []*corev1.Pod) (readyPods []*corev1.Pod, scheduleGatedPods []*corev1.Pod) {
	for _, pod := range pods {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				readyPods = append(readyPods, pod)
			} else if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Reason == corev1.PodReasonSchedulingGated {
				scheduleGatedPods = append(scheduleGatedPods, pod)
			}
		}
	}
	return
}
