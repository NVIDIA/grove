package utils

import (
	"context"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPCLQPods lists all Pods that belong to a PodClique
func GetPCLQPods(ctx context.Context, cl client.Client, pgsName string, pclq *grovecorev1alpha1.PodClique) ([]*corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cl.List(ctx,
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
