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
	defaultUpdateStrategy(spec.UpdateStrategy)
}

func defaultPodGangTemplateSpec(spec *v1alpha1.PodGangTemplateSpec) {
	// default PodCliqueTemplateSpecs
	defaultPodCliqueTemplateSpecs(spec.Cliques)
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
	//default RollingUpdateConfig
	if updateStrategy.RollingUpdateConfig != nil {
		if updateStrategy.RollingUpdateConfig.MaxSurge == nil {
			*updateStrategy.RollingUpdateConfig.MaxSurge = intstr.FromInt32(1)
		}
		if updateStrategy.RollingUpdateConfig.MaxUnavailable == nil {
			*updateStrategy.RollingUpdateConfig.MaxUnavailable = intstr.FromInt32(1)
		}
	}
}

func defaultPodCliqueTemplateSpecs(cliqueSpecs []v1alpha1.PodCliqueTemplateSpec) {
	for _, cliqueSpec := range cliqueSpecs {
		// default PodSpec
		defaultPodSpec(&cliqueSpec.Spec.Spec)
		if cliqueSpec.Spec.Replicas < 1 {
			cliqueSpec.Spec.Replicas = 1
		}
		if cliqueSpec.Spec.ScaleConfig != nil {
			*cliqueSpec.Spec.ScaleConfig.MinReplicas = 1
		}
	}
}

// defaultPodSpec adds defaults to PodSpec.
func defaultPodSpec(spec *corev1.PodSpec) {
	if utils.IsEmptyStringType(spec.RestartPolicy) {
		spec.RestartPolicy = corev1.RestartPolicyNever
	}
	if spec.TerminationGracePeriodSeconds == nil {
		*spec.TerminationGracePeriodSeconds = 30
	}
}
