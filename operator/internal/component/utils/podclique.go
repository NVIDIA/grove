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
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPCLQsByOwner retrieves PodClique objects that are owned by the specified owner kind and object key, and match the provided selector labels.
func GetPCLQsByOwner(ctx context.Context, cl client.Client, ownerKind string, ownerObjectKey client.ObjectKey, selectorLabels map[string]string) ([]grovecorev1alpha1.PodClique, error) {
	pclqs, err := GetPCLQsMatchingLabels(ctx, cl, ownerObjectKey.Namespace, selectorLabels)
	if err != nil {
		return pclqs, err
	}
	filteredPCLQs := lo.Filter(pclqs, func(pclq grovecorev1alpha1.PodClique, _ int) bool {
		if len(pclq.OwnerReferences) == 0 {
			return false
		}
		return pclq.OwnerReferences[0].Kind == ownerKind && pclq.OwnerReferences[0].Name == ownerObjectKey.Name
	})
	return filteredPCLQs, nil
}

// GetPCLQsByOwnerReplicaIndex retrieves PodClique objects per replica of the owner resource matching provided selector labels.
func GetPCLQsByOwnerReplicaIndex(ctx context.Context, cl client.Client, ownerKind string, ownerObjectKey client.ObjectKey, selectorLabels map[string]string) (map[string][]grovecorev1alpha1.PodClique, error) {
	pclqs, err := GetPCLQsByOwner(ctx, cl, ownerKind, ownerObjectKey, selectorLabels)
	if err != nil {
		return nil, err
	}
	return groupPCLQsByLabel(pclqs, apicommon.LabelPodGangSetReplicaIndex), nil
}

// GetPCLQsMatchingLabels gets all the PodClique's in a given namespace matching selectorLabels.
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

// GroupPCLQsByPodGangName filters PCLQs that have a PodGang label and groups them by the PodGang name.
func GroupPCLQsByPodGangName(pclqs []grovecorev1alpha1.PodClique) map[string][]grovecorev1alpha1.PodClique {
	return groupPCLQsByLabel(pclqs, apicommon.LabelPodGang)
}

// GroupPCLQsByPCSGReplicaIndex filters PCLQs that have a PodCliqueScalingGroupReplicaIndex label and groups them by the PCSG replica.
func GroupPCLQsByPCSGReplicaIndex(pclqs []grovecorev1alpha1.PodClique) map[string][]grovecorev1alpha1.PodClique {
	return groupPCLQsByLabel(pclqs, apicommon.LabelPodCliqueScalingGroupReplicaIndex)
}

// GroupPCLQsByPGSReplicaIndex filters PCLQs that have a PodGangSetReplicaIndex label and groups them by the PGS replica.
func GroupPCLQsByPGSReplicaIndex(pclqs []grovecorev1alpha1.PodClique) map[string][]grovecorev1alpha1.PodClique {
	return groupPCLQsByLabel(pclqs, constants.LabelPodGangSetReplicaIndex)
}

// GetMinAvailableBreachedPCLQInfo filters PodCliques that have grovecorev1alpha1.ConditionTypeMinAvailableBreached set to true.
// For each such PodClique it returns the name of the PodClique a duration to wait for before terminationDelay is breached.
func GetMinAvailableBreachedPCLQInfo(pclqs []grovecorev1alpha1.PodClique, terminationDelay time.Duration, since time.Time) ([]string, time.Duration) {
	pclqCandidateNames := make([]string, 0, len(pclqs))
	waitForDurations := make([]time.Duration, 0, len(pclqs))
	for _, pclq := range pclqs {
		cond := meta.FindStatusCondition(pclq.Status.Conditions, constants.ConditionTypeMinAvailableBreached)
		if cond == nil {
			continue
		}
		if cond.Status == metav1.ConditionTrue {
			pclqCandidateNames = append(pclqCandidateNames, pclq.Name)
			waitFor := terminationDelay - since.Sub(cond.LastTransitionTime.Time)
			waitForDurations = append(waitForDurations, waitFor)
		}
	}
	if len(pclqCandidateNames) == 0 {
		return nil, 0
	}
	slices.Sort(waitForDurations)
	return pclqCandidateNames, waitForDurations[0]
}

// GetPodCliquesWithParentPGS retrieves PodClique objects that are not part of any PodCliqueScalingGroup for the given PodGangSet.
func GetPodCliquesWithParentPGS(ctx context.Context, cl client.Client, pgsObjKey client.ObjectKey) ([]grovecorev1alpha1.PodClique, error) {
	pclqList := &grovecorev1alpha1.PodCliqueList{}
	err := cl.List(ctx,
		pclqList,
		client.InNamespace(pgsObjKey.Namespace),
		client.MatchingLabels(lo.Assign(
			apicommon.GetDefaultLabelsForPodGangSetManagedResources(pgsObjKey.Name),
			map[string]string{
				apicommon.LabelComponentKey: constants.LabelComponentPGSPodCliqueValue,
			},
		)),
	)
	if err != nil {
		return nil, err
	}
	return pclqList.Items, nil
}

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

// GetPCLQPodTemplateHash computes the pod template hash for the PCLQ pod spec.
func GetPCLQPodTemplateHash(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, priorityClassName string) string {
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      pclqTemplateSpec.Labels,
			Annotations: pclqTemplateSpec.Annotations,
		},
		Spec: pclqTemplateSpec.Spec.PodSpec,
	}
	podTemplateSpec.Spec.PriorityClassName = priorityClassName
	return k8sutils.ComputeHash(&podTemplateSpec)
}
