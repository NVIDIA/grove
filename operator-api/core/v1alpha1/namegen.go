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

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GeneratePodGangName generates a PodGang name based on the PodGangSet name and the replica index.
func GeneratePodGangName(pgsName string, pgsReplicaIndex int32) string {
	return fmt.Sprintf("%s-%d", pgsName, pgsReplicaIndex)
}

// GeneratePodRoleName generates a Pod role name based on the PodGangSet object metadata.
// This role will be associated to all Pods within a PodGangSet.
func GeneratePodRoleName(pgsObjMeta metav1.ObjectMeta) string {
	return fmt.Sprintf("%s:pgs:%s", SchemeGroupVersion.Group, pgsObjMeta.Name)
}

// GeneratePodRoleBindingName generates a Pod role binding name based on the PodGangSet object metadata.
func GeneratePodRoleBindingName(pgsObjMeta metav1.ObjectMeta) string {
	return fmt.Sprintf("%s:pgs:%s", SchemeGroupVersion.Group, pgsObjMeta.Name)
}

// GeneratePodServiceAccountName generates a Pod service account used by all the Pods within a PodGangSet.
func GeneratePodServiceAccountName(pgsObjMeta metav1.ObjectMeta) string {
	return pgsObjMeta.Name
}

// GeneratePodCliqueName generates a PodClique name based on the PodGangSet name, replica index, and PodCliqueTemplate name.
func GeneratePodCliqueName(pgsName string, pgsReplicaIndex int32, pclqTemplateName string) string {
	return fmt.Sprintf("%s-%d-%s", pgsName, pgsReplicaIndex, pclqTemplateName)
}

// GeneratePodName generates a Pod name based on the PodClique name and replica index.
func GeneratePodName(pclqName string, pclqReplicaIndex int32) string {
	return fmt.Sprintf("%s-%d", pclqName, pclqReplicaIndex)
}
