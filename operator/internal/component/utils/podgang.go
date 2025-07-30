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
	return fmt.Sprintf("%s-%d", pcsgFQN, scaledPodGangIndex)
}

// CreatePodGangNameForPCSGFromFQN generates the PodGang name for a replica of a PodCliqueScalingGroup
// when the PCSG name is already fully qualified (e.g., "simple1-0-sga").
func CreatePodGangNameForPCSGFromFQN(pcsgFQN string, scaledPodGangIndex int) string {
	return fmt.Sprintf("%s-%d", pcsgFQN, scaledPodGangIndex)
}

// DeterminePodGangNameForPodClique determines the correct PodGang name for a PodClique
// based on its owner reference and scaling group configuration. This centralizes the PodGang assignment
// logic so both PodClique and PodGang components use the same rules.
func DeterminePodGangNameForPodClique(pclq *grovecorev1alpha1.PodClique, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int, pcsgReplicaIndex int) string {
	// Check owner reference to determine if this PodClique belongs to a scaling group
	if isOwnedByPodCliqueScalingGroup(pclq) {
		return determinePodGangNameForScalingGroupPodClique(pclq, pgs, pgsReplica, pcsgReplicaIndex)
	}

	// Default: standalone PodClique uses PGS replica PodGang
	return grovecorev1alpha1.GenerateBasePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplica})
}

// determinePodGangNameForScalingGroupPodClique handles PodGang name determination for PodCliques
// that belong to a scaling group. It extracts scaling group configuration and applies minAvailable
// logic to determine whether the PodClique belongs to a base PodGang or scaled PodGang.
func determinePodGangNameForScalingGroupPodClique(pclq *grovecorev1alpha1.PodClique, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int, pcsgReplicaIndex int) string {
	// Extract scaling group name from the PCSG owner's name
	pcsgName := getPodCliqueScalingGroupOwnerName(pclq)
	scalingGroupName := grovecorev1alpha1.ExtractScalingGroupNameFromPCSGFQN(pcsgName, grovecorev1alpha1.ResourceNameReplica{
		Name:    pgs.Name,
		Replica: pgsReplica,
	})

	// Find scaling group configuration and apply minAvailable logic
	pcsgConfig := findScalingGroupConfig(pgs, scalingGroupName)
	return applyMinAvailableLogicForPodGang(pgs, pgsReplica, scalingGroupName, pcsgConfig, pcsgReplicaIndex)
}

// findScalingGroupConfig locates the scaling group configuration by name in the PodGangSet template.
func findScalingGroupConfig(pgs *grovecorev1alpha1.PodGangSet, scalingGroupName string) grovecorev1alpha1.PodCliqueScalingGroupConfig {
	for _, pcsgConfig := range pgs.Spec.Template.PodCliqueScalingGroupConfigs {
		if pcsgConfig.Name == scalingGroupName {
			return pcsgConfig
		}
	}
	// Return empty config if not found (shouldn't happen in normal operation)
	return grovecorev1alpha1.PodCliqueScalingGroupConfig{}
}

// applyMinAvailableLogicForPodGang determines the correct PodGang name based on minAvailable logic.
// Replicas 0..(minAvailable-1) belong to the base PodGang, while replicas minAvailable+ get scaled PodGangs.
func applyMinAvailableLogicForPodGang(pgs *grovecorev1alpha1.PodGangSet, pgsReplica int, scalingGroupName string, pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig, pcsgReplicaIndex int) string {
	// MinAvailable should always be non-nil due to kubebuilder default and defaulting webhook
	minAvailable := *pcsgConfig.MinAvailable

	// Apply the same logic as PodGang creation:
	// Replicas 0..(minAvailable-1) → PGS replica PodGang (base PodGang)
	// Replicas minAvailable+ → Scaled PodGangs (0-based indexing)
	if pcsgReplicaIndex < int(minAvailable) {
		return grovecorev1alpha1.GenerateBasePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplica})
	} else {
		// Convert scaling group replica index to 0-based scaled PodGang index
		scaledPodGangIndex := pcsgReplicaIndex - int(minAvailable)
		return CreatePodGangNameForPCSG(pgs.Name, pgsReplica, scalingGroupName, scaledPodGangIndex)
	}
}

// isOwnedByPodCliqueScalingGroup checks if the PodClique is owned by a PodCliqueScalingGroup
func isOwnedByPodCliqueScalingGroup(pclq *grovecorev1alpha1.PodClique) bool {
	for _, ownerRef := range pclq.GetOwnerReferences() {
		if ownerRef.Kind == string(component.KindPodCliqueScalingGroup) {
			return true
		}
	}
	return false
}

// getPodCliqueScalingGroupOwnerName returns the name of the PodCliqueScalingGroup that owns this PodClique
func getPodCliqueScalingGroupOwnerName(pclq *grovecorev1alpha1.PodClique) string {
	for _, ownerRef := range pclq.GetOwnerReferences() {
		if ownerRef.Kind == string(component.KindPodCliqueScalingGroup) {
			return ownerRef.Name
		}
	}
	return ""
}
