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

// GenerateHeadlessServiceAddress generates a headless service address based on the PodGangSet name, replica index, and namespace.
// The address is in the format: <headless-service-name>.<namespace>.svc.cluster.local
func GenerateHeadlessServiceAddress(pgsNameReplica ResourceNameReplica, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", GenerateHeadlessServiceName(pgsNameReplica), namespace)
}

// GeneratePodRoleName generates a Pod role name.
// This role will be associated to an init container within each Pod for a PodGangSet.
// The init container is created by the operator and is responsible for ensuring start-up order amongst PodCliques.
func GeneratePodRoleName(pgsName string) string {
	return fmt.Sprintf("%s:pgs:%s", SchemeGroupVersion.Group, pgsName)
}

// GeneratePodRoleBindingName generates a role binding name. The role binding will bind the
// role to the service account that are created for the init container responsible for ensuring start-up order amongst PodCliques.
func GeneratePodRoleBindingName(pgsName string) string {
	return fmt.Sprintf("%s:pgs:%s", SchemeGroupVersion.Group, pgsName)
}

// GeneratePodServiceAccountName generates a Pod service account used by all the init containers
// within the PodGangSet (one per pod) that are responsible for ensuring start-up order amongst PodCliques.
func GeneratePodServiceAccountName(pgsName string) string {
	return pgsName
}

// GenerateInitContainerSATokenSecretName generates a Secret name containing a service account token that will be mounted onto the init container
// responsible for ensuring start-up order amongst PodCliques.
func GenerateInitContainerSATokenSecretName(pgsName string) string {
	return fmt.Sprintf("%s-initc-sa-token-secret", pgsName)
}

// GeneratePodCliqueName generates a PodClique name based on the PodGangSet name, replica index, and PodCliqueTemplate name.
func GeneratePodCliqueName(ownerNameReplica ResourceNameReplica, pclqTemplateName string) string {
	return fmt.Sprintf("%s-%d-%s", ownerNameReplica.Name, ownerNameReplica.Replica, pclqTemplateName)
}

// GeneratePodCliqueScalingGroupName generates a PodCliqueScalingGroup name based on the PodGangSet name, replica index and PodCliqueScalingGroup name.
// PodCliqueScalingGroup name is only guaranteed to be unique within the PodGangSet, so it is prefixed with the PodGangSet name and its replica index.
func GeneratePodCliqueScalingGroupName(pgsNameReplica ResourceNameReplica, pclqScalingGroupName string) string {
	return fmt.Sprintf("%s-%d-%s", pgsNameReplica.Name, pgsNameReplica.Replica, pclqScalingGroupName)
}

// GenerateBasePodGangName generates a base PodGang name for a PodGangSet replica.
// This is used for PodGangs that are not part of scaled scaling group replicas.
func GenerateBasePodGangName(pgsNameReplica ResourceNameReplica) string {
	return fmt.Sprintf("%s-%d", pgsNameReplica.Name, pgsNameReplica.Replica)
}

// CreatePodGangNameFromPCSGFQN generates the PodGang name for a replica of a PodCliqueScalingGroup
// when the PCSG name is already fully qualified.
func CreatePodGangNameFromPCSGFQN(pcsgFQN string, scaledPodGangIndex int) string {
	return fmt.Sprintf("%s-%d", pcsgFQN, scaledPodGangIndex)
}

// GeneratePodGangNameForPodCliqueOwnedByPodGangSet generates the PodGang name for a PodClique
// that is directly owned by a PodGangSet.
func GeneratePodGangNameForPodCliqueOwnedByPodGangSet(pgs *PodGangSet, pgsReplicaIndex int) string {
	return GenerateBasePodGangName(ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex})
}

// GeneratePodGangNameForPodCliqueOwnedByPCSG generates the PodGang name for a PodClique
// that is owned by a PodCliqueScalingGroup, using the PCSG object directly (no config lookup needed).
func GeneratePodGangNameForPodCliqueOwnedByPCSG(pgs *PodGangSet, pgsReplicaIndex int, pcsg *PodCliqueScalingGroup, pcsgReplicaIndex int) string {
	// MinAvailable should always be non-nil due to kubebuilder default and defaulting webhook
	minAvailable := *pcsg.Spec.MinAvailable

	// Apply the same logic as PodGang creation:
	// Replicas 0..(minAvailable-1) → PGS replica PodGang (base PodGang)
	// Replicas minAvailable+ → Scaled PodGangs (0-based indexing)
	if pcsgReplicaIndex < int(minAvailable) {
		return GenerateBasePodGangName(ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex})
	} else {
		// Convert scaling group replica index to 0-based scaled PodGang index
		scaledPodGangIndex := pcsgReplicaIndex - int(minAvailable)
		// Use the PCSG name directly (it's already the FQN)
		return CreatePodGangNameFromPCSGFQN(pcsg.Name, scaledPodGangIndex)
	}
}

// ExtractScalingGroupNameFromPCSGFQN extracts the scaling group name from a PodCliqueScalingGroup FQN.
// For example, "simple1-0-sga" with pgsNameReplica="simple1-0" returns "sga".
func ExtractScalingGroupNameFromPCSGFQN(pcsgFQN string, pgsNameReplica ResourceNameReplica) string {
	prefix := fmt.Sprintf("%s-%d-", pgsNameReplica.Name, pgsNameReplica.Replica)
	return pcsgFQN[len(prefix):]
}
