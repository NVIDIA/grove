package component

import (
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GeneratePodGangName generates a PodGang name based on the PodGangSet name and the replica index.
func GeneratePodGangName(pgsName string, replicaIndex int32) string {
	return fmt.Sprintf("%s-%d", pgsName, replicaIndex)
}

// GeneratePodRoleName generates a Pod role name based on the PodGangSet object metadata.
// This role will be associated to all Pods within a PodGangSet.
func GeneratePodRoleName(pgsObjMeta metav1.ObjectMeta) string {
	return fmt.Sprintf("%s:pgs:%s", v1alpha1.SchemeGroupVersion.Group, pgsObjMeta.Name)
}

// GeneratePodRoleBindingName generates a Pod role binding name based on the PodGangSet object metadata.
func GeneratePodRoleBindingName(pgsObjMeta metav1.ObjectMeta) string {
	return fmt.Sprintf("%s:pgs:%s", v1alpha1.SchemeGroupVersion.Group, pgsObjMeta.Name)
}

// GeneratePodServiceAccountName generates a Pod service account used by all the Pods within a PodGangSet.
func GeneratePodServiceAccountName(pgsObjMeta metav1.ObjectMeta) string {
	return pgsObjMeta.Name
}
