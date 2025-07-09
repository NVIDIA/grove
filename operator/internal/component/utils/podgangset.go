package utils

import (
	"context"
	"fmt"
	"github.com/samber/lo"

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

// GetExpectedPCLQNamesGroupByOwner returns the expected unqualified PodClique names which are either owned by PodGangSet or PodCliqueScalingGroup.
func GetExpectedPCLQNamesGroupByOwner(pgs *grovecorev1alpha1.PodGangSet) (expectedPCLQNamesForPGS []string, expectedPCLQNamesForPCSG []string) {
	pcsgConfigs := pgs.Spec.Template.PodCliqueScalingGroupConfigs
	for _, pcsgConfig := range pcsgConfigs {
		expectedPCLQNamesForPCSG = append(expectedPCLQNamesForPCSG, pcsgConfig.CliqueNames...)
	}
	pgsCliqueNames := lo.Map(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, _ int) string {
		return pclqTemplateSpec.Name
	})
	expectedPCLQNamesForPGS, _ = lo.Difference(pgsCliqueNames, expectedPCLQNamesForPCSG)
	return
}
