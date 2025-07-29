// /*
// Copyright 2024 The Grove Authors.
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

package defaulting

import (
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	defaultTerminationDelay = 30 * time.Second
)

// defaultPodGangSet adds defaults to a PodGangSet.
func defaultPodGangSet(pgs *grovecorev1alpha1.PodGangSet) {
	if utils.IsEmptyStringType(pgs.Namespace) {
		pgs.Namespace = "default"
	}
	defaultPodGangSetSpec(&pgs.Spec)
}

// defaultPodGangSetSpec adds defaults to the specification of a PodGangSet.
func defaultPodGangSetSpec(spec *grovecorev1alpha1.PodGangSetSpec) {
	// default PodGangSetTemplateSpec
	defaultPodGangSetTemplateSpec(&spec.Template)
}

func defaultPodGangSetTemplateSpec(spec *grovecorev1alpha1.PodGangSetTemplateSpec) {
	// default PodCliqueTemplateSpecs
	spec.Cliques = defaultPodCliqueTemplateSpecs(spec.Cliques)
	// default PodCliqueScalingGroupConfigs
	spec.PodCliqueScalingGroupConfigs = defaultPodCliqueScalingGroupConfigs(spec.PodCliqueScalingGroupConfigs)
	if spec.TerminationDelay == nil {
		spec.TerminationDelay = &metav1.Duration{Duration: defaultTerminationDelay}
	}
}

func defaultPodCliqueTemplateSpecs(cliqueSpecs []*grovecorev1alpha1.PodCliqueTemplateSpec) []*grovecorev1alpha1.PodCliqueTemplateSpec {
	defaultedCliqueSpecs := make([]*grovecorev1alpha1.PodCliqueTemplateSpec, 0, len(cliqueSpecs))
	for _, cliqueSpec := range cliqueSpecs {
		defaultedCliqueSpec := cliqueSpec.DeepCopy()
		defaultedCliqueSpec.Spec.PodSpec = *defaultPodSpec(&cliqueSpec.Spec.PodSpec)
		if defaultedCliqueSpec.Spec.Replicas == 0 {
			defaultedCliqueSpec.Spec.Replicas = 1
		}
		if cliqueSpec.Spec.MinAvailable == nil {
			defaultedCliqueSpec.Spec.MinAvailable = ptr.To(cliqueSpec.Spec.Replicas)
		}
		if cliqueSpec.Spec.ScaleConfig != nil {
			if cliqueSpec.Spec.ScaleConfig.MinReplicas == nil {
				defaultedCliqueSpec.Spec.ScaleConfig.MinReplicas = ptr.To(cliqueSpec.Spec.Replicas)
			}
		}
		defaultedCliqueSpecs = append(defaultedCliqueSpecs, defaultedCliqueSpec)
	}
	return defaultedCliqueSpecs
}

func defaultPodCliqueScalingGroupConfigs(scalingGroupConfigs []grovecorev1alpha1.PodCliqueScalingGroupConfig) []grovecorev1alpha1.PodCliqueScalingGroupConfig {
	// Preserve nil when no scaling groups are specified
	if len(scalingGroupConfigs) == 0 {
		return scalingGroupConfigs
	}

	defaultedScalingGroupConfigs := make([]grovecorev1alpha1.PodCliqueScalingGroupConfig, 0, len(scalingGroupConfigs))
	for _, scalingGroupConfig := range scalingGroupConfigs {
		defaultedScalingGroupConfig := scalingGroupConfig.DeepCopy()
		// Replicas is already set by kubebuilder default (API server runs before defaulting webhook)
		if scalingGroupConfig.ScaleConfig != nil {
			if scalingGroupConfig.ScaleConfig.MinReplicas == nil {
				defaultedScalingGroupConfig.ScaleConfig.MinReplicas = ptr.To(*defaultedScalingGroupConfig.Replicas)
			}
		}
		defaultedScalingGroupConfigs = append(defaultedScalingGroupConfigs, *defaultedScalingGroupConfig)
	}
	return defaultedScalingGroupConfigs
}

// defaultPodSpec adds defaults to PodSpec.
func defaultPodSpec(spec *corev1.PodSpec) *corev1.PodSpec {
	defaultedPodSpec := spec.DeepCopy()
	if utils.IsEmptyStringType(defaultedPodSpec.RestartPolicy) {
		defaultedPodSpec.RestartPolicy = corev1.RestartPolicyAlways
	}
	if defaultedPodSpec.TerminationGracePeriodSeconds == nil {
		defaultedPodSpec.TerminationGracePeriodSeconds = ptr.To[int64](30)
	}
	return defaultedPodSpec
}
