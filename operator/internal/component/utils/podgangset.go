package utils

import (
	"context"
	"fmt"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOwnerPodGangSet gets the owner PodGangSet object.
func GetOwnerPodGangSet(ctx context.Context, cl client.Client, objectMeta metav1.ObjectMeta) (*grovecorev1alpha1.PodGangSet, error) {
	pgsName := GetPodGangSetName(objectMeta)
	if pgsName == nil {
		return nil, fmt.Errorf("label: %s is not present on %v", grovecorev1alpha1.LabelPartOfKey, k8sutils.GetObjectKeyFromObjectMeta(objectMeta))
	}
	pgs := &grovecorev1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *pgsName,
			Namespace: objectMeta.Namespace,
		},
	}
	err := cl.Get(ctx, client.ObjectKeyFromObject(pgs), pgs)
	return pgs, err
}

// GetPodGangSetName retrieves the PodGangSet name from the labels of the given ObjectMeta.
func GetPodGangSetName(objectMeta metav1.ObjectMeta) *string {
	pgsName, ok := objectMeta.GetLabels()[grovecorev1alpha1.LabelPartOfKey]
	if !ok {
		return nil
	}
	return &pgsName
}
