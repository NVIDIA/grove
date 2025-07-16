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
	"strconv"
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPCSGsForPGSReplicaIndex fetches all PodCliqueScalingGroups for a PodGangSet replica index.
func GetPCSGsForPGSReplicaIndex(ctx context.Context, cl client.Client, pgsObjKey client.ObjectKey, pgsReplicaIndex int) ([]grovecorev1alpha1.PodCliqueScalingGroup, error) {
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	if err := cl.List(ctx,
		pcsgList,
		client.InNamespace(pgsObjKey.Namespace),
		client.MatchingLabels(lo.Assign(
			k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjKey.Name),
			map[string]string{
				grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
			},
		)),
	); err != nil {
		return nil, err
	}
	return pcsgList.Items, nil
}

// GetMinAvailableBreachedPCSGInfo filters PodCliqueScalingGroups that have grovecorev1alpha1.ConditionTypeMinAvailableBreached set to true.
// For each such PodCliqueScalingGroup it returns the name of the PodCliqueScalingGroupand a duration to wait for before terminationDelay is breached.
func GetMinAvailableBreachedPCSGInfo(pcsgs []grovecorev1alpha1.PodCliqueScalingGroup, terminationDelay time.Duration, since time.Time) ([]string, time.Duration) {
	pcsgCandidateNames := make([]string, 0, len(pcsgs))
	waitForDurations := make([]time.Duration, 0, len(pcsgs))
	for _, pcsg := range pcsgs {
		cond := meta.FindStatusCondition(pcsg.Status.Conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached)
		if cond == nil {
			continue
		}
		if cond.Status == metav1.ConditionTrue {
			pcsgCandidateNames = append(pcsgCandidateNames, pcsg.Name)
			waitFor := terminationDelay - since.Sub(cond.LastTransitionTime.Time)
			waitForDurations = append(waitForDurations, waitFor)
		}
	}
	if len(waitForDurations) == 0 {
		return pcsgCandidateNames, 0
	}
	slices.Sort(waitForDurations)
	return pcsgCandidateNames, waitForDurations[0]
}
