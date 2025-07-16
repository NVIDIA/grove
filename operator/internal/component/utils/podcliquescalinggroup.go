package utils

import (
	"context"
	"strconv"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPCSGsForPGSReplicaIndex fetches all PodCliqueScalingGroups for a PodGangSet replica index.
func GetPCSGsForPGSReplicaIndex(ctx context.Context, cl client.Client, pgsObjKey client.ObjectKey, pgsReplicaIndex int) ([]grovecorev1alpha1.PodCliqueScalingGroup, error) {
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	if err := cl.List(ctx,
		pcsgList,
		client.InNamespace(pgsObjKey.Namespace),
		client.MatchingLabels(lo.Assign(
			k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjKey.Name),
			map[string]string{
				grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
			},
		)),
	); err != nil {
		return nil, err
	}
	return pcsgList.Items, nil
}
