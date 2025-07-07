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
	"strings"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/samber/lo"
)

// FindMatchingPCSGConfig finds matching PodCliqueScalingGroupConfig defined in PodGangSet which matches PodCliqueScalingGroup fully qualified name.
func FindMatchingPCSGConfig(pgs *grovecorev1alpha1.PodGangSet, pcsgFQN string) grovecorev1alpha1.PodCliqueScalingGroupConfig {
	matchingPCSG, _ := lo.Find(pgs.Spec.Template.PodCliqueScalingGroupConfigs, func(pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig) bool {
		return strings.HasSuffix(pcsgFQN, pcsgConfig.Name)
	})
	return matchingPCSG
}
