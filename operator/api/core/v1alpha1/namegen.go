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
)

// ResourceNameReplica is a type that holds a resource name and its replica index.
type ResourceNameReplica struct {
	// Name is the name of the resource.
	Name string
	// Replica is the index of the replica within the resource.
	Replica int
}

// GenerateHeadlessServiceName generates a headless service name based on the PodGangSet name and replica index.
func GenerateHeadlessServiceName(pgsNameReplica ResourceNameReplica) string {
	return fmt.Sprintf("%s-%d", pgsNameReplica.Name, pgsNameReplica.Replica)
}

// GeneratePodRoleName generates a Pod role name based on the PodGangSet name.
// This role will be associated to all Pods within a PodGangSet.
func GeneratePodRoleName(pgsName string) string {
	return fmt.Sprintf("%s:pgs:%s", SchemeGroupVersion.Group, pgsName)
}

// GeneratePodRoleBindingName generates a Pod role binding name based on the PodGangSet name.
func GeneratePodRoleBindingName(pgsName string) string {
	return fmt.Sprintf("%s:pgs:%s", SchemeGroupVersion.Group, pgsName)
}

// GeneratePodServiceAccountName generates a Pod service account used by all the Pods within a PodGangSet.
func GeneratePodServiceAccountName(pgsName string) string {
	return pgsName
}

// GeneratePodCliqueName generates a PodClique name based on the PodGangSet name, replica index, and PodCliqueTemplate name.
func GeneratePodCliqueName(pgsNameReplica ResourceNameReplica, pclqTemplateName string) string {
	return fmt.Sprintf("%s-%d-%s", pgsNameReplica.Name, pgsNameReplica.Replica, pclqTemplateName)
}

// GeneratePodName generates a Pod name based on the PodClique name and replica index.
func GeneratePodName(pclqNameReplica ResourceNameReplica) string {
	return fmt.Sprintf("%s-%d", pclqNameReplica.Name, pclqNameReplica.Replica)
}

// GeneratePodCliqueScalingGroupName generates a PodCliqueScalingGroup name based on the PodGangSet name, replica index and PodCliqueScalingGroup name.
// PodCliqueScalingGroup name is only guaranteed to be unique within the PodGangSet, so it is prefixed with the PodGangSet name and its replica index.
func GeneratePodCliqueScalingGroupName(pgsNameReplica ResourceNameReplica, pclqScalingGroupName string) string {
	return fmt.Sprintf("%s-%d-%s", pgsNameReplica.Name, pgsNameReplica.Replica, pclqScalingGroupName)
}

// GeneratePodGangName generates a PodGang name based on pgs and pcsg name and replicas.
func GeneratePodGangName(pgsNameReplica ResourceNameReplica, pcsgNameReplica *ResourceNameReplica) string {
	if pcsgNameReplica == nil {
		return fmt.Sprintf("%s-%d", pgsNameReplica.Name, pgsNameReplica.Replica)
	} else {
		return fmt.Sprintf("%s%d", GeneratePCSGPodGangNamePrefix(pgsNameReplica, pcsgNameReplica.Name), pcsgNameReplica.Replica)
	}
}

// GeneratePCSGPodGangNamePrefix generates a PodGang name prefix for Podgangs created due to PCSG replica scale-out.
func GeneratePCSGPodGangNamePrefix(pgsNameReplica ResourceNameReplica, pcsgName string) string {
	return fmt.Sprintf("%s-%d-%s-", pgsNameReplica.Name, pgsNameReplica.Replica, pcsgName)
}
