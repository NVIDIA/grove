package kubernetes

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
)

func GetDefaultLabelsForPodGangSetManagedResources(pgsName string) map[string]string {
	return map[string]string{
		v1alpha1.LabelManagedByKey: v1alpha1.LabelManagedByValue,
		v1alpha1.LabelPartOfKey:    pgsName,
	}
}
