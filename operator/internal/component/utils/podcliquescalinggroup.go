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

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FindScalingGroupConfigForClique searches for the scaling group configuration
// that contains the specified clique name in its CliqueNames list.
// Returns the matching config or nil if not found.
func FindScalingGroupConfigForClique(scalingGroupConfigs []grovecorev1alpha1.PodCliqueScalingGroupConfig, cliqueName string) *grovecorev1alpha1.PodCliqueScalingGroupConfig {
	pcsgConfig, ok := lo.Find(scalingGroupConfigs, func(pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig) bool {
		return slices.Contains(pcsgConfig.CliqueNames, cliqueName)
	})
	if !ok {
		return nil
	}
	return &pcsgConfig
}

// GetPCSGsForPCS retrieves all PodCliqueScalingGroups owned by a PodCliqueSet.
func GetPCSGsForPCS(ctx context.Context, cl client.Client, pcsObjKey client.ObjectKey) ([]grovecorev1alpha1.PodCliqueScalingGroup, error) {
	pcsgList, err := doGetPCSGsForPCS(ctx, cl, pcsObjKey, nil)
	if err != nil {
		return nil, err
	}
	return pcsgList.Items, nil
}

// doGetPCSGsForPCS is a helper function that lists PodCliqueScalingGroups for a PodCliqueSet
// with optional additional matching labels.
func doGetPCSGsForPCS(ctx context.Context, cl client.Client, pcsObjKey client.ObjectKey, matchingLabels map[string]string) (*grovecorev1alpha1.PodCliqueScalingGroupList, error) {
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	if err := cl.List(ctx,
		pcsgList,
		client.InNamespace(pcsObjKey.Namespace),
		client.MatchingLabels(lo.Assign(
			apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsObjKey.Name),
			matchingLabels,
		)),
	); err != nil {
		return nil, err
	}
	return pcsgList, nil
}

// GenerateDependencyNamesForBasePodGang generates the fully qualified names of PodCliques
// that serve as dependencies for a base PodGang, considering scaling group configurations.
func GenerateDependencyNamesForBasePodGang(pcs *grovecorev1alpha1.PodCliqueSet, pcsReplicaIndex int, parentCliqueName string) []string {
	parentPCLQNames := make([]string, 0)
	pcsgConfig := FindScalingGroupConfigForClique(pcs.Spec.Template.PodCliqueScalingGroupConfigs, parentCliqueName)

	if pcsgConfig != nil {
		// Generate FQNs for minAvailable number of PodCliques in the scaling group
		pcsgFQN := apicommon.GeneratePodCliqueScalingGroupName(apicommon.ResourceNameReplica{Name: pcs.Name, Replica: pcsReplicaIndex}, pcsgConfig.Name)
		for pcsgReplicaIndex := range int(*pcsgConfig.MinAvailable) {
			parentPCLQNames = append(parentPCLQNames, apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pcsgFQN, Replica: pcsgReplicaIndex}, parentCliqueName))
		}
	} else {
		// Generate standalone PodClique FQN if not part of any scaling group
		parentPCLQNames = append(parentPCLQNames, apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pcs.Name, Replica: pcsReplicaIndex}, parentCliqueName))
	}
	return parentPCLQNames
}

// GroupPCSGsByPCSReplicaIndex groups PodCliqueScalingGroups by their PodCliqueSet replica index.
// Only PCSGs with the PodCliqueSetReplicaIndex label are included.
func GroupPCSGsByPCSReplicaIndex(pcsgs []grovecorev1alpha1.PodCliqueScalingGroup) map[string][]grovecorev1alpha1.PodCliqueScalingGroup {
	return groupPCSGsByLabel(pcsgs, apicommon.LabelPodCliqueSetReplicaIndex)
}

// groupPCSGsByLabel groups PodCliqueScalingGroups by the specified label key.
// PCSGs without the label are excluded from the result.
func groupPCSGsByLabel(pcsgs []grovecorev1alpha1.PodCliqueScalingGroup, label string) map[string][]grovecorev1alpha1.PodCliqueScalingGroup {
	result := make(map[string][]grovecorev1alpha1.PodCliqueScalingGroup)
	for _, pcsg := range pcsgs {
		labelValue, exists := pcsg.Labels[label]
		if !exists {
			continue
		}
		result[labelValue] = append(result[labelValue], pcsg)
	}
	return result
}

// GetPCSGsByPCSReplicaIndex retrieves and groups PodCliqueScalingGroups by their
// PodCliqueSet replica index. Returns a map where keys are replica indices and
// values are slices of PodCliqueScalingGroup objects.
func GetPCSGsByPCSReplicaIndex(ctx context.Context, cl client.Client, pcsObjKey client.ObjectKey) (map[string][]grovecorev1alpha1.PodCliqueScalingGroup, error) {
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	if err := cl.List(ctx,
		pcsgList,
		client.InNamespace(pcsObjKey.Namespace),
		client.MatchingLabels(apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsObjKey.Name)),
	); err != nil {
		return nil, err
	}

	// Group PCSGs by their PodCliqueSet replica index
	pcsgsByPCSReplicaIndex := make(map[string][]grovecorev1alpha1.PodCliqueScalingGroup)
	for _, pcsg := range pcsgList.Items {
		pcsReplicaIndex, ok := pcsg.Labels[apicommon.LabelPodCliqueSetReplicaIndex]
		if !ok {
			continue
		}
		pcsgsByPCSReplicaIndex[pcsReplicaIndex] = append(pcsgsByPCSReplicaIndex[pcsReplicaIndex], pcsg)
	}
	return pcsgsByPCSReplicaIndex, nil
}

// GetPCLQTemplateHashes generates pod template hashes for all PodCliques in a
// PodCliqueScalingGroup. Returns a map where keys are PodClique FQNs and values are template hashes.
func GetPCLQTemplateHashes(pcs *grovecorev1alpha1.PodCliqueSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) map[string]string {
	// Find template specs for all cliques in this scaling group
	pclqTemplateSpecs := make([]*grovecorev1alpha1.PodCliqueTemplateSpec, 0, len(pcsg.Spec.CliqueNames))
	for _, cliqueName := range pcsg.Spec.CliqueNames {
		pclqTemplateSpec, ok := lo.Find(pcs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return cliqueName == pclqTemplateSpec.Name
		})
		if !ok {
			continue
		}
		pclqTemplateSpecs = append(pclqTemplateSpecs, pclqTemplateSpec)
	}

	// Generate hashes for each replica of each clique
	cliqueTemplateSpecHashes := make(map[string]string, len(pclqTemplateSpecs))
	for pcsgReplicaIndex := range int(pcsg.Spec.Replicas) {
		for _, pclqTemplateSpec := range pclqTemplateSpecs {
			pclqFQN := apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pcsg.Name, Replica: pcsgReplicaIndex}, pclqTemplateSpec.Name)
			cliqueTemplateSpecHashes[pclqFQN] = ComputePCLQPodTemplateHash(pclqTemplateSpec, pcs.Spec.Template.PriorityClassName)
		}
	}
	return cliqueTemplateSpecHashes
}

// GetPCLQsInPCSGPendingUpdate identifies PodClique FQNs that require updates by comparing
// their current pod template hash labels with expected hashes from the PodCliqueSet specification.
func GetPCLQsInPCSGPendingUpdate(pcs *grovecorev1alpha1.PodCliqueSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, existingPCLQs []grovecorev1alpha1.PodClique) []string {
	pclqFQNsPendingUpdate := make([]string, 0, len(existingPCLQs))
	expectedPCLQPodTemplateHashes := GetPCLQTemplateHashes(pcs, pcsg)

	// Compare existing pod template hashes with expected ones
	for _, existingPCLQ := range existingPCLQs {
		existingPodTemplateHash := existingPCLQ.Labels[apicommon.LabelPodTemplateHash]
		expectedPodTemplateHash := expectedPCLQPodTemplateHashes[existingPCLQ.Name]
		if existingPodTemplateHash != expectedPodTemplateHash {
			pclqFQNsPendingUpdate = append(pclqFQNsPendingUpdate, expectedPodTemplateHash)
		}
	}
	return pclqFQNsPendingUpdate
}

// IsPCSGUpdateInProgress reports whether a PodCliqueScalingGroup is currently undergoing a rolling update.
func IsPCSGUpdateInProgress(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) bool {
	return pcsg.Status.RollingUpdateProgress != nil && pcsg.Status.RollingUpdateProgress.UpdateEndedAt == nil
}

// IsPCSGUpdateComplete reports whether the rolling update of a PodCliqueScalingGroup is complete
// by comparing the current generation hash with the expected one.
func IsPCSGUpdateComplete(pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsGenerationHash string) bool {
	return pcsg.Status.CurrentPodCliqueSetGenerationHash != nil && *pcsg.Status.CurrentPodCliqueSetGenerationHash == pcsGenerationHash
}

// GetPodCliqueFQNsForPCSG generates fully qualified names for all PodCliques
// owned by a PodCliqueScalingGroup across all replicas and clique names.
func GetPodCliqueFQNsForPCSG(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) []string {
	pclqFQNsInPCSG := make([]string, 0, len(pcsg.Spec.CliqueNames)*int(pcsg.Spec.Replicas))
	for replicaIndex := range int(pcsg.Spec.Replicas) {
		for _, cliqueName := range pcsg.Spec.CliqueNames {
			pclqFQNsInPCSG = append(pclqFQNsInPCSG, apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: replicaIndex,
			}, cliqueName))
		}
	}
	return pclqFQNsInPCSG
}
