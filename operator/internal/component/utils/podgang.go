package utils

import (
	"context"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetExistingPodGangNames(ctx context.Context, cl client.Client, pgs *grovecorev1alpha1.PodGangSet) ([]string, error) {
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(groveschedulerv1alpha1.SchemeGroupVersion.WithKind("PodGang"))
	if err := cl.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(getPodGangSelectorLabels(pgs.ObjectMeta)),
	); err != nil {
		return nil, err
	}
	return k8sutils.FilterMapOwnedResourceNames(pgs.ObjectMeta, objMetaList.Items), nil
}

func getPodGangSelectorLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePodGang,
		})
}
