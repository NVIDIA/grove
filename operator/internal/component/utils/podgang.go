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
// This is used for individual scaling group replicas beyond minAvailable.
func CreatePodGangNameForPCSG(pgsName string, pgsReplicaIndex int, pcsgName string, pcsgReplicaIndex int) string {
	pcsgFQN := grovecorev1alpha1.GeneratePodCliqueScalingGroupName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplicaIndex}, pcsgName)
	return fmt.Sprintf("%s-%d", pcsgFQN, pcsgReplicaIndex)
}

// CreatePodGangNameForPCSGFromFQN generates the PodGang name for a replica of a PodCliqueScalingGroup
// when the PCSG name is already fully qualified (e.g., "simple1-0-sga").
func CreatePodGangNameForPCSGFromFQN(pcsgFQN string, pcsgReplicaIndex int) string {
	return fmt.Sprintf("%s-%d", pcsgFQN, pcsgReplicaIndex)
}

// DeterminePodGangNameForPodClique determines the correct PodGang name for a PodClique
// based on the PodGangSet configuration and scaling group membership. This centralizes the PodGang assignment
// logic so both PodClique and PodGang components use the same rules.
func DeterminePodGangNameForPodClique(pgs *grovecorev1alpha1.PodGangSet, pgsReplica int, cliqueName string, scalingGroupName string, sgReplicaIndex int) string {
	// Check if this PodClique belongs to a scaling group
	if scalingGroupName != "" {
		// Find the scaling group configuration to get minAvailable
		for _, pcsgConfig := range pgs.Spec.Template.PodCliqueScalingGroupConfigs {
			if pcsgConfig.Name == scalingGroupName {
				minAvailable := int32(1) // Default
				if pcsgConfig.MinAvailable != nil {
					minAvailable = *pcsgConfig.MinAvailable
				}

				// Apply the same logic as PodGang creation:
				// Replicas 0..(minAvailable-1) → PGS replica PodGang
				// Replicas minAvailable+ → Individual PodGangs (0-based indexing)
				if sgReplicaIndex < int(minAvailable) {
					return grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplica}, nil)
				} else {
					// Convert scaling group replica index to 0-based individual PodGang index
					individualPodGangIndex := sgReplicaIndex - int(minAvailable)
					return CreatePodGangNameForPCSG(pgs.Name, pgsReplica, scalingGroupName, individualPodGangIndex)
				}
			}
		}
	}

	// Default: standalone PodClique uses PGS replica PodGang
	return grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplica}, nil)
}
