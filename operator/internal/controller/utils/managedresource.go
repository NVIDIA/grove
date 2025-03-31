package utils

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasExpectedOwner checks if the owner references of a resource match the expected owner kind.
func HasExpectedOwner(expectedOwnerKind string, ownerRefs []metav1.OwnerReference) bool {
	if len(ownerRefs) == 0 || len(ownerRefs) > 1 {
		return false
	}
	return ownerRefs[0].Kind == expectedOwnerKind
}

// IsManagedByGrove checks if the pod is managed by Grove by inspecting its labels.
// All grove managed resources will have a standard set of default labels.
func IsManagedByGrove(labels map[string]string) bool {
	if val, ok := labels[v1alpha1.LabelManagedByKey]; ok {
		return v1alpha1.LabelManagedByValue == val
	}
	return false
}
