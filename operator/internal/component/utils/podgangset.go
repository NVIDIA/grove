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

// GetExpectedPCSGFQNsForPGS computes the FQNs for all PodCliqueScalingGroups defined in PGS for the given replica.
func GetExpectedPCSGFQNsForPGS(pgs *grovecorev1alpha1.PodCliqueSet) []string {
	pcsgFQNsPerPGSReplica := GetExpectedPCSGFQNsPerPGSReplica(pgs)
	return lo.Flatten(lo.Values(pcsgFQNsPerPGSReplica))
}

// GetPodCliqueFQNsForPGSNotInPCSG computes the FQNs for all PodCliques for all PGS replicas which are not part of any PCSG.
func GetPodCliqueFQNsForPGSNotInPCSG(pgs *grovecorev1alpha1.PodCliqueSet) []string {
	pclqFQNs := make([]string, 0, int(pgs.Spec.Replicas)*len(pgs.Spec.Template.Cliques))
	for pgsReplicaIndex := range int(pgs.Spec.Replicas) {
		pclqFQNs = append(pclqFQNs, GetPodCliqueFQNsForPGSReplicaNotInPCSG(pgs, pgsReplicaIndex)...)
	}
	return pclqFQNs
}

// GetPodCliqueFQNsForPGSReplicaNotInPCSG computes the FQNs for all PodCliques for a PGS replica which are not part of any PCSG.
func GetPodCliqueFQNsForPGSReplicaNotInPCSG(pgs *grovecorev1alpha1.PodCliqueSet, pgsReplicaIndex int) []string {
	pclqNames := make([]string, 0, len(pgs.Spec.Template.Cliques))
	for _, pclqTemplateSpec := range pgs.Spec.Template.Cliques {
		if isStandalonePCLQ(pgs, pclqTemplateSpec.Name) {
			pclqNames = append(pclqNames, common.GeneratePodCliqueName(common.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex}, pclqTemplateSpec.Name))
		}
	}
	return pclqNames
}

// isStandalonePCLQ checks if the PodClique is managed by PodCliqueSet or not
func isStandalonePCLQ(pgs *grovecorev1alpha1.PodCliqueSet, pclqName string) bool {
	return !lo.Reduce(pgs.Spec.Template.PodCliqueScalingGroupConfigs, func(agg bool, pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig, _ int) bool {
		return agg || slices.Contains(pcsgConfig.CliqueNames, pclqName)
	}, false)
}

// GetPodGangSet gets the owner PodCliqueSet object.
func GetPodGangSet(ctx context.Context, cl client.Client, objectMeta metav1.ObjectMeta) (*grovecorev1alpha1.PodCliqueSet, error) {
	pgsName := GetPodGangSetName(objectMeta)
	pgs := &grovecorev1alpha1.PodCliqueSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgsName,
			Namespace: objectMeta.Namespace,
		},
	}
	err := cl.Get(ctx, client.ObjectKeyFromObject(pgs), pgs)
	return pgs, err
}

// GetPodGangSetName retrieves the PodCliqueSet name from the labels of the given ObjectMeta.
// NOTE: It is assumed that all managed objects like PCSG, PCLQ and Pods will always have PGS name as value for grovecorev1alpha1.LabelPartOfKey label.
// It should be ensured that labels that are set by the operator are never removed.
func GetPodGangSetName(objectMeta metav1.ObjectMeta) string {
	pgsName := objectMeta.GetLabels()[common.LabelPartOfKey]
	return pgsName
}

// GetExpectedPCLQNamesGroupByOwner returns the expected unqualified PodClique names which are either owned by PodCliqueSet or PodCliqueScalingGroup.
func GetExpectedPCLQNamesGroupByOwner(pgs *grovecorev1alpha1.PodCliqueSet) (expectedPCLQNamesForPGS []string, expectedPCLQNamesForPCSG []string) {
	pcsgConfigs := pgs.Spec.Template.PodCliqueScalingGroupConfigs
	for _, pcsgConfig := range pcsgConfigs {
		expectedPCLQNamesForPCSG = append(expectedPCLQNamesForPCSG, pcsgConfig.CliqueNames...)
	}
	pgsCliqueNames := lo.Map(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, _ int) string {
		return pclqTemplateSpec.Name
	})
	expectedPCLQNamesForPGS, _ = lo.Difference(pgsCliqueNames, expectedPCLQNamesForPCSG)
	return
}

// GetExpectedPCSGFQNsPerPGSReplica computes the FQNs for all PodCliqueScalingGroups defined in PGS for each replica.
func GetExpectedPCSGFQNsPerPGSReplica(pgs *grovecorev1alpha1.PodCliqueSet) map[int][]string {
	pcsgFQNsByPGSReplica := make(map[int][]string)
	for pgsReplicaIndex := range int(pgs.Spec.Replicas) {
		for _, pcsgConfig := range pgs.Spec.Template.PodCliqueScalingGroupConfigs {
			pcsgName := common.GeneratePodCliqueScalingGroupName(common.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex}, pcsgConfig.Name)
			pcsgFQNsByPGSReplica[pgsReplicaIndex] = append(pcsgFQNsByPGSReplica[pgsReplicaIndex], pcsgName)
		}
	}
	return pcsgFQNsByPGSReplica
}

// GetExpectedStandAlonePCLQFQNsPerPGSReplica computes the FQNs for all standalone PodCliques defined in PGS for each replica.
func GetExpectedStandAlonePCLQFQNsPerPGSReplica(pgs *grovecorev1alpha1.PodCliqueSet) map[int][]string {
	pclqFQNsByPGSReplica := make(map[int][]string)
	for pgsReplicaIndex := range int(pgs.Spec.Replicas) {
		pclqFQNsByPGSReplica[pgsReplicaIndex] = GetPodCliqueFQNsForPGSReplicaNotInPCSG(pgs, pgsReplicaIndex)
	}
	return pclqFQNsByPGSReplica
}
