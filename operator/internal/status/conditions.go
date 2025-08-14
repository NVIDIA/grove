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

package status

import (
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IsScheduled checks if conditions indicate resource is scheduled
func IsScheduled(conditions []metav1.Condition) bool {
	return k8sutils.IsConditionTrue(conditions, grovecorev1alpha1.ConditionTypePodCliqueScheduled)
}

// IsAvailable checks if conditions indicate resource is available
func IsAvailable(conditions []metav1.Condition) bool {
	return k8sutils.IsConditionFalse(conditions, grovecorev1alpha1.ConditionTypeMinAvailableBreached)
}

// GetPCLQCondition extracts conditions from PodClique
func GetPCLQCondition(pclq grovecorev1alpha1.PodClique) []metav1.Condition {
	return pclq.Status.Conditions
}

// GetPCSGCondition extracts conditions from PodCliqueScalingGroup
func GetPCSGCondition(pcsg grovecorev1alpha1.PodCliqueScalingGroup) []metav1.Condition {
	return pcsg.Status.Conditions
}
