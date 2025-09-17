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
	"context"
	"slices"

	"github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetExpectedPCSGFQNsForPCS computes the fully qualified names for all PodCliqueScalingGroups
// defined in a PodCliqueSet across all replicas.
func GetExpectedPCSGFQNsForPCS(pcs *grovecorev1alpha1.PodCliqueSet) []string {
	pcsgFQNsPerPCSReplica := GetExpectedPCSGFQNsPerPCSReplica(pcs)
	return lo.Flatten(lo.Values(pcsgFQNsPerPCSReplica))
}

// GetPodCliqueFQNsForPCSNotInPCSG computes the fully qualified names for all standalone
// PodCliques (not part of any PodCliqueScalingGroup) across all PodCliqueSet replicas.
func GetPodCliqueFQNsForPCSNotInPCSG(pcs *grovecorev1alpha1.PodCliqueSet) []string {
	pclqFQNs := make([]string, 0, int(pcs.Spec.Replicas)*len(pcs.Spec.Template.Cliques))
	for pcsReplicaIndex := range int(pcs.Spec.Replicas) {
		pclqFQNs = append(pclqFQNs, GetPodCliqueFQNsForPCSReplicaNotInPCSG(pcs, pcsReplicaIndex)...)
	}
	return pclqFQNs
}

// GetPodCliqueFQNsForPCSReplicaNotInPCSG computes the fully qualified names for all
// standalone PodCliques (not part of any PodCliqueScalingGroup) for a specific PodCliqueSet replica.
func GetPodCliqueFQNsForPCSReplicaNotInPCSG(pcs *grovecorev1alpha1.PodCliqueSet, pcsReplicaIndex int) []string {
	pclqNames := make([]string, 0, len(pcs.Spec.Template.Cliques))
	for _, pclqTemplateSpec := range pcs.Spec.Template.Cliques {
		if isStandalonePCLQ(pcs, pclqTemplateSpec.Name) {
			pclqNames = append(pclqNames, common.GeneratePodCliqueName(common.ResourceNameReplica{Name: pcs.Name, Replica: pcsReplicaIndex}, pclqTemplateSpec.Name))
		}
	}
	return pclqNames
}

// isStandalonePCLQ reports whether a PodClique is standalone (directly managed by PodCliqueSet)
// rather than being part of any PodCliqueScalingGroup.
func isStandalonePCLQ(pcs *grovecorev1alpha1.PodCliqueSet, pclqName string) bool {
	return !lo.Reduce(pcs.Spec.Template.PodCliqueScalingGroupConfigs, func(agg bool, pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig, _ int) bool {
		return agg || slices.Contains(pcsgConfig.CliqueNames, pclqName)
	}, false)
}

// GetPodCliqueSet retrieves the owner PodCliqueSet object using the object metadata
// to extract the PodCliqueSet name from labels.
func GetPodCliqueSet(ctx context.Context, cl client.Client, objectMeta metav1.ObjectMeta) (*grovecorev1alpha1.PodCliqueSet, error) {
	pcsName := GetPodCliqueSetName(objectMeta)
	pcs := &grovecorev1alpha1.PodCliqueSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcsName,
			Namespace: objectMeta.Namespace,
		},
	}
	err := cl.Get(ctx, client.ObjectKeyFromObject(pcs), pcs)
	return pcs, err
}

// GetPodCliqueSetName extracts the PodCliqueSet name from object metadata labels.
// All managed objects (PCSG, PCLQ, Pods) are expected to have the PodCliqueSet name
// as the value for the LabelPartOfKey label.
func GetPodCliqueSetName(objectMeta metav1.ObjectMeta) string {
	pcsName := objectMeta.GetLabels()[common.LabelPartOfKey]
	return pcsName
}

// GetExpectedPCLQNamesGroupByOwner categorizes PodClique names by their ownership model.
// Returns two slices: names for standalone PodCliques (owned by PodCliqueSet) and
// names for PodCliques managed by PodCliqueScalingGroups.
func GetExpectedPCLQNamesGroupByOwner(pcs *grovecorev1alpha1.PodCliqueSet) (expectedPCLQNamesForPCS []string, expectedPCLQNamesForPCSG []string) {
	// Collect all clique names that belong to scaling groups
	pcsgConfigs := pcs.Spec.Template.PodCliqueScalingGroupConfigs
	for _, pcsgConfig := range pcsgConfigs {
		expectedPCLQNamesForPCSG = append(expectedPCLQNamesForPCSG, pcsgConfig.CliqueNames...)
	}

	// Get all clique names defined in the PodCliqueSet
	pcsCliqueNames := lo.Map(pcs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, _ int) string {
		return pclqTemplateSpec.Name
	})

	// Standalone cliques are those not part of any scaling group
	expectedPCLQNamesForPCS, _ = lo.Difference(pcsCliqueNames, expectedPCLQNamesForPCSG)
	return
}

// GetExpectedPCSGFQNsPerPCSReplica computes the fully qualified names for all
// PodCliqueScalingGroups defined in a PodCliqueSet, organized by replica index.
func GetExpectedPCSGFQNsPerPCSReplica(pcs *grovecorev1alpha1.PodCliqueSet) map[int][]string {
	pcsgFQNsByPCSReplica := make(map[int][]string)
	for pcsReplicaIndex := range int(pcs.Spec.Replicas) {
		for _, pcsgConfig := range pcs.Spec.Template.PodCliqueScalingGroupConfigs {
			pcsgName := common.GeneratePodCliqueScalingGroupName(common.ResourceNameReplica{Name: pcs.Name, Replica: pcsReplicaIndex}, pcsgConfig.Name)
			pcsgFQNsByPCSReplica[pcsReplicaIndex] = append(pcsgFQNsByPCSReplica[pcsReplicaIndex], pcsgName)
		}
	}
	return pcsgFQNsByPCSReplica
}

// GetExpectedStandAlonePCLQFQNsPerPCSReplica computes the fully qualified names for all
// standalone PodCliques (not part of any PodCliqueScalingGroup) organized by replica index.
func GetExpectedStandAlonePCLQFQNsPerPCSReplica(pcs *grovecorev1alpha1.PodCliqueSet) map[int][]string {
	pclqFQNsByPCSReplica := make(map[int][]string)
	for pcsReplicaIndex := range int(pcs.Spec.Replicas) {
		pclqFQNsByPCSReplica[pcsReplicaIndex] = GetPodCliqueFQNsForPCSReplicaNotInPCSG(pcs, pcsReplicaIndex)
	}
	return pclqFQNsByPCSReplica
}
