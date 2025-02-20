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
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// defaultPodGangSet adds defaults to a PodGangSet.
func defaultPodGangSet(pgs *v1alpha1.PodGangSet) {
	if utils.IsEmptyStringType(pgs.ObjectMeta.Namespace) {
		pgs.ObjectMeta.Namespace = "default"
	}
	defaultPodGangSetSpec(&pgs.Spec)
}

// defaultPodGangSetSpec adds defaults to the specification of a PodGangSet.
func defaultPodGangSetSpec(spec *v1alpha1.PodGangSetSpec) {
	// default PodGangTemplateSpec
	defaultPodGangTemplateSpec(&spec.Template)
	// default UpdateStrategy
	defaultUpdateStrategy(spec)
}

func defaultPodGangTemplateSpec(spec *v1alpha1.PodGangTemplateSpec) {
	// default PodCliqueTemplateSpecs
	spec.Cliques = defaultPodCliqueTemplateSpecs(spec.Cliques)
	// default startup type
	if spec.StartupType == nil {
		spec.StartupType = ptr.To(v1alpha1.CliqueStartupTypeInOrder)
	}
	// default NetworkPackStrategy
	if spec.NetworkPackStrategy == nil {
		spec.NetworkPackStrategy = ptr.To(v1alpha1.BestEffort)
	}
}

func defaultUpdateStrategy(spec *v1alpha1.PodGangSetSpec) {
	updateStrategy := spec.UpdateStrategy.DeepCopy()
	if updateStrategy == nil || updateStrategy.RollingUpdateConfig == nil {
		updateStrategy = &v1alpha1.GangUpdateStrategy{
			RollingUpdateConfig: &v1alpha1.RollingUpdateConfiguration{},
		}
	}
	if updateStrategy.RollingUpdateConfig.MaxSurge == nil {
		updateStrategy.RollingUpdateConfig.MaxSurge = ptr.To(intstr.FromInt32(1))
	}
	if updateStrategy.RollingUpdateConfig.MaxUnavailable == nil {
		updateStrategy.RollingUpdateConfig.MaxUnavailable = ptr.To(intstr.FromInt32(1))
	}
	spec.UpdateStrategy = updateStrategy
}

func defaultPodCliqueTemplateSpecs(cliqueSpecs []v1alpha1.PodCliqueTemplateSpec) []v1alpha1.PodCliqueTemplateSpec {
	defaultedCliqueSpecs := make([]v1alpha1.PodCliqueTemplateSpec, 0, len(cliqueSpecs))
	for _, cliqueSpec := range cliqueSpecs {
		defaultedCliqueSpec := cliqueSpec.DeepCopy()
		defaultedCliqueSpec.Spec.PodSpec = *defaultPodSpec(&cliqueSpec.Spec.PodSpec)
		if defaultedCliqueSpec.Spec.Replicas < 1 {
			defaultedCliqueSpec.Spec.Replicas = 1
		}
		if defaultedCliqueSpec.Spec.ScaleConfig != nil && defaultedCliqueSpec.Spec.ScaleConfig.MinReplicas == nil {
			defaultedCliqueSpec.Spec.ScaleConfig.MinReplicas = ptr.To[int32](1)
		}
		defaultedCliqueSpecs = append(defaultedCliqueSpecs, *defaultedCliqueSpec)
	}
	return defaultedCliqueSpecs
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
