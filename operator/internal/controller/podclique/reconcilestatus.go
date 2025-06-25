package podclique

import (
	"context"
	"fmt"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
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
	pgsName := k8sutils.GetFirstOwnerName(pclq.ObjectMeta)
	pods, err := r.getPCLQPods(ctx, pgsName, pclq)
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
	if err := updateSelector(pgsName, pclq); err != nil {
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
			grovecorev1alpha1.LabelAppNameKey: pclq.Name,
		},
	)
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return fmt.Errorf("%w: failed to create label selector for PodClique %v", client.ObjectKeyFromObject(pclq))
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

func (r *Reconciler) getPCLQPods(ctx context.Context, pgsName string, pclq *grovecorev1alpha1.PodClique) ([]*corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := r.client.List(ctx,
		podList,
		client.InNamespace(pclq.Namespace),
		client.MatchingLabels(
			lo.Assign(
				k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
				map[string]string{
					grovecorev1alpha1.LabelPodCliqueName: pclq.Name,
				},
			),
		)); err != nil {
		return nil, err
	}
	ownedPods := make([]*corev1.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if metav1.IsControlledBy(&pod, pclq) {
			ownedPods = append(ownedPods, &pod)
		}
	}
	return ownedPods, nil
}
