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
	"fmt"
	"slices"
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPCLQsByOwner retrieves PodClique objects owned by the specified owner kind and object key
// that match the provided selector labels.
func GetPCLQsByOwner(ctx context.Context, cl client.Client, ownerKind string, ownerObjectKey client.ObjectKey, selectorLabels map[string]string) ([]grovecorev1alpha1.PodClique, error) {
	// First get all PodCliques matching the labels in the namespace
	pclqs, err := GetPCLQsMatchingLabels(ctx, cl, ownerObjectKey.Namespace, selectorLabels)
	if err != nil {
		return pclqs, err
	}

	// Filter by owner reference to match the specified owner
	filteredPCLQs := lo.Filter(pclqs, func(pclq grovecorev1alpha1.PodClique, _ int) bool {
		if len(pclq.OwnerReferences) == 0 {
			return false
		}
		return pclq.OwnerReferences[0].Kind == ownerKind && pclq.OwnerReferences[0].Name == ownerObjectKey.Name
	})
	return filteredPCLQs, nil
}

// GetPCLQsByOwnerReplicaIndex retrieves PodClique objects grouped by replica index
// for the specified owner resource matching provided selector labels.
func GetPCLQsByOwnerReplicaIndex(ctx context.Context, cl client.Client, ownerKind string, ownerObjectKey client.ObjectKey, selectorLabels map[string]string) (map[string][]grovecorev1alpha1.PodClique, error) {
	pclqs, err := GetPCLQsByOwner(ctx, cl, ownerKind, ownerObjectKey, selectorLabels)
	if err != nil {
		return nil, err
	}
	return groupPCLQsByLabel(pclqs, apicommon.LabelPodCliqueSetReplicaIndex), nil
}

// GetPCLQsMatchingLabels returns all PodCliques in a namespace that match the selector labels.
func GetPCLQsMatchingLabels(ctx context.Context, cl client.Client, namespace string, selectorLabels map[string]string) ([]grovecorev1alpha1.PodClique, error) {
	podCliqueList := &grovecorev1alpha1.PodCliqueList{}
	if err := cl.List(ctx,
		podCliqueList,
		client.InNamespace(namespace),
		client.MatchingLabels(selectorLabels)); err != nil {
		return nil, err
	}
	return podCliqueList.Items, nil
}

// GroupPCLQsByPodGangName groups PodCliques by their PodGang label value.
// Only PodCliques with the PodGang label are included in the result.
func GroupPCLQsByPodGangName(pclqs []grovecorev1alpha1.PodClique) map[string][]grovecorev1alpha1.PodClique {
	return groupPCLQsByLabel(pclqs, apicommon.LabelPodGang)
}

// GroupPCLQsByPCSGReplicaIndex groups PodCliques by their PodCliqueScalingGroup replica index.
// Only PodCliques with the PodCliqueScalingGroupReplicaIndex label are included.
func GroupPCLQsByPCSGReplicaIndex(pclqs []grovecorev1alpha1.PodClique) map[string][]grovecorev1alpha1.PodClique {
	return groupPCLQsByLabel(pclqs, apicommon.LabelPodCliqueScalingGroupReplicaIndex)
}

// GroupPCLQsByPCSReplicaIndex groups PodCliques by their PodCliqueSet replica index.
// Only PodCliques with the PodCliqueSetReplicaIndex label are included.
func GroupPCLQsByPCSReplicaIndex(pclqs []grovecorev1alpha1.PodClique) map[string][]grovecorev1alpha1.PodClique {
	return groupPCLQsByLabel(pclqs, apicommon.LabelPodCliqueSetReplicaIndex)
}

// GetMinAvailableBreachedPCLQInfo returns PodCliques with MinAvailableBreached condition true
// and calculates the minimum wait duration before termination delay is breached.
// It returns the candidate PodClique names and the shortest wait duration.
func GetMinAvailableBreachedPCLQInfo(pclqs []grovecorev1alpha1.PodClique, terminationDelay time.Duration, since time.Time) ([]string, time.Duration) {
	pclqCandidateNames := make([]string, 0, len(pclqs))
	waitForDurations := make([]time.Duration, 0, len(pclqs))

	// Check each PodClique for MinAvailableBreached condition
	for _, pclq := range pclqs {
		cond := meta.FindStatusCondition(pclq.Status.Conditions, constants.ConditionTypeMinAvailableBreached)
		if cond == nil {
			continue
		}
		if cond.Status == metav1.ConditionTrue {
			pclqCandidateNames = append(pclqCandidateNames, pclq.Name)
			// Calculate remaining wait time before termination delay is breached
			waitFor := terminationDelay - since.Sub(cond.LastTransitionTime.Time)
			waitForDurations = append(waitForDurations, waitFor)
		}
	}

	if len(pclqCandidateNames) == 0 {
		return nil, 0
	}

	// Return the shortest wait duration
	slices.Sort(waitForDurations)
	return pclqCandidateNames, waitForDurations[0]
}

// GetPodCliquesWithParentPCS retrieves standalone PodClique objects that are directly
// owned by a PodCliqueSet (not part of any PodCliqueScalingGroup).
func GetPodCliquesWithParentPCS(ctx context.Context, cl client.Client, pcsObjKey client.ObjectKey) ([]grovecorev1alpha1.PodClique, error) {
	pclqList := &grovecorev1alpha1.PodCliqueList{}
	err := cl.List(ctx,
		pclqList,
		client.InNamespace(pcsObjKey.Namespace),
		client.MatchingLabels(lo.Assign(
			apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsObjKey.Name),
			map[string]string{
				apicommon.LabelComponentKey: apicommon.LabelComponentNamePodCliqueSetPodClique,
			},
		)),
	)
	if err != nil {
		return nil, err
	}
	return pclqList.Items, nil
}

// groupPCLQsByLabel groups PodCliques by the specified label key.
// PodCliques without the label are excluded from the result.
func groupPCLQsByLabel(pclqs []grovecorev1alpha1.PodClique, labelKey string) map[string][]grovecorev1alpha1.PodClique {
	grouped := make(map[string][]grovecorev1alpha1.PodClique)
	for _, pclq := range pclqs {
		labelValue, exists := pclq.Labels[labelKey]
		if !exists {
			continue
		}
		grouped[labelValue] = append(grouped[labelValue], pclq)
	}
	return grouped
}

// ComputePCLQPodTemplateHash computes a hash for the PodClique template specification.
// This hash is used to detect changes in the pod template that require rolling updates.
func ComputePCLQPodTemplateHash(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, priorityClassName string) string {
	// Build a standard PodTemplateSpec from the PodClique template
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      pclqTemplateSpec.Labels,
			Annotations: pclqTemplateSpec.Annotations,
		},
		Spec: pclqTemplateSpec.Spec.PodSpec,
	}
	// Include priority class in the hash computation
	podTemplateSpec.Spec.PriorityClassName = priorityClassName
	return k8sutils.ComputeHash(&podTemplateSpec)
}

// IsPCLQUpdateInProgress reports whether a PodClique is currently undergoing a rolling update.
func IsPCLQUpdateInProgress(pclq *grovecorev1alpha1.PodClique) bool {
	return pclq.Status.RollingUpdateProgress != nil && pclq.Status.RollingUpdateProgress.UpdateEndedAt == nil
}

// IsLastPCLQUpdateCompleted reports whether the last rolling update of a PodClique is completed.
func IsLastPCLQUpdateCompleted(pclq *grovecorev1alpha1.PodClique) bool {
	return pclq.Status.RollingUpdateProgress != nil && pclq.Status.RollingUpdateProgress.UpdateEndedAt != nil
}

// GetExpectedPCLQPodTemplateHash computes the expected pod template hash for a PodClique
// by finding its template specification in the parent PodCliqueSet.
func GetExpectedPCLQPodTemplateHash(pcs *grovecorev1alpha1.PodCliqueSet, pclqObjectMeta metav1.ObjectMeta) (string, error) {
	// Extract the clique name from the PodClique's fully qualified name
	cliqueName, err := utils.GetPodCliqueNameFromPodCliqueFQN(pclqObjectMeta)
	if err != nil {
		return "", err
	}

	// Find the matching template spec in the PodCliqueSet
	matchingPCLQTemplateSpec, ok := lo.Find(pcs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
		return cliqueName == pclqTemplateSpec.Name
	})
	if !ok {
		return "", fmt.Errorf("pod clique template not found for cliqueName: %s", cliqueName)
	}

	return ComputePCLQPodTemplateHash(matchingPCLQTemplateSpec, pcs.Spec.Template.PriorityClassName), nil
}
