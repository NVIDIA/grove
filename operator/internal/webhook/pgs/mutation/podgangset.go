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

package mutation

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// defaultPodGangSetSpec adds defaults to the specification of a PodGangSet.
func defaultPodGangSetSpec(spec *v1alpha1.PodGangSetSpec) {

	// default PodGangTemplateSpec
	defaultPodGangTemplateSpec(&spec.Template)

	// default UpdateStrategy
	defaultUpdateStrategy(spec.UpdateStrategy)
}

func defaultPodGangTemplateSpec(spec *v1alpha1.PodGangTemplateSpec) {
	// default PodCliqueTemplateSpec
	defaultPodCliqueTemplateSpec(spec.Cliques)

	// default startup type
	if spec.StartupType == nil {
		*spec.StartupType = v1alpha1.CliqueStartupTypeInOrder
	}

	// default RestartPolicy
	if spec.RestartPolicy == nil {
		*spec.RestartPolicy = v1alpha1.GangRestartPolicyAlways
	}

	// default NetworkPackStrategy
	if spec.NetworkPackStrategy == nil {
		*spec.NetworkPackStrategy = v1alpha1.BestEffort
	}
}

func defaultUpdateStrategy(updateStrategy *v1alpha1.GangUpdateStrategy) {
	//default UpdateStrategy
	if utils.IsEmptyStringType(updateStrategy.Type) {
		updateStrategy.Type = v1alpha1.GangUpdateStrategyRecreate
	}

	if updateStrategy.Type == v1alpha1.GangUpdateStrategyRolling {
		//default RollingUpdateConfig
		if updateStrategy.RollingUpdateConfig.MaxSurge == nil {
			*updateStrategy.RollingUpdateConfig.MaxSurge = intstr.FromInt(1)
		}

		if updateStrategy.RollingUpdateConfig.MaxUnavailable == nil {
			*updateStrategy.RollingUpdateConfig.MaxUnavailable = intstr.FromInt(1)
		}
	}
}

func defaultPodCliqueTemplateSpec(cliqueSpecs []v1alpha1.PodCliqueTemplateSpec) {
	for _, cliqueSpec := range cliqueSpecs {
		// default PodTemplateSpec
		defaultPodTemplateSpec(&cliqueSpec.Spec.Template)
	}
}

// defaultPodTemplateSpec adds defaults to PodSpec.
func defaultPodTemplateSpec(spec *corev1.PodTemplateSpec) {
	if spec.Spec.TerminationGracePeriodSeconds == nil {
		*spec.Spec.TerminationGracePeriodSeconds = 30
	}
}
