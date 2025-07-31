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

package utils

import (
	"fmt"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetPodGangSelectorLabels creates the label selector to list all the PodGangs for a PodGangSet.
func GetPodGangSelectorLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePodGang,
		})
}

// CreatePodGangNameForPCSG generates the PodGang name for a replica of a PodCliqueScalingGroup.
// This is used for scaled scaling group replicas beyond minAvailable.
func CreatePodGangNameForPCSG(pgsName string, pgsReplicaIndex int, pcsgName string, scaledPodGangIndex int) string {
	pcsgFQN := grovecorev1alpha1.GeneratePodCliqueScalingGroupName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplicaIndex}, pcsgName)
	return CreatePodGangNameFromPCSGFQN(pcsgFQN, scaledPodGangIndex)
}

// CreatePodGangNameFromPCSGFQN generates the PodGang name for a replica of a PodCliqueScalingGroup
// when the PCSG name is already fully qualified (e.g., "simple1-0-sga").
func CreatePodGangNameFromPCSGFQN(pcsgFQN string, scaledPodGangIndex int) string {
	return fmt.Sprintf("%s-%d", pcsgFQN, scaledPodGangIndex)
}

// GeneratePodGangNameForPodCliqueOwnedByPodGangSet generates the PodGang name for a PodClique
// that is directly owned by a PodGangSet (standalone PodClique).
func GeneratePodGangNameForPodCliqueOwnedByPodGangSet(pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex int) string {
	return grovecorev1alpha1.GenerateBasePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex})
}

// GeneratePodGangNameForPodCliqueOwnedByPCSG generates the PodGang name for a PodClique
// that is owned by a PodCliqueScalingGroup, using the PCSG object directly (no config lookup needed).
func GeneratePodGangNameForPodCliqueOwnedByPCSG(pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex int, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int) string {
	// MinAvailable should always be non-nil due to kubebuilder default and defaulting webhook
	minAvailable := *pcsg.Spec.MinAvailable

	// Apply the same logic as PodGang creation:
	// Replicas 0..(minAvailable-1) → PGS replica PodGang (base PodGang)
	// Replicas minAvailable+ → Scaled PodGangs (0-based indexing)
	if pcsgReplicaIndex < int(minAvailable) {
		return grovecorev1alpha1.GenerateBasePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex})
	} else {
		// Convert scaling group replica index to 0-based scaled PodGang index
		scaledPodGangIndex := pcsgReplicaIndex - int(minAvailable)
		// Use the PCSG name directly (it's already the FQN)
		return CreatePodGangNameFromPCSGFQN(pcsg.Name, scaledPodGangIndex)
	}
}
