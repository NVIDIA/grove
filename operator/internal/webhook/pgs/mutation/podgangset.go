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
	"k8s.io/apimachinery/pkg/util/intstr"
)

// mutatePodGangSet adds defaults to a PodGangSetSpec.
func mutatePodGangSet(pgs *v1alpha1.PodGangSet) {
	mutatePodGangSetSpec(&pgs.Spec)
}

// mutatePodGangSetSpec adds defaults to the specification of a PodGangSet.
func mutatePodGangSetSpec(spec *v1alpha1.PodGangSetSpec) {

	// default PodGangTemplateSpec
	mutatePodGangTemplateSpec(&spec.Template)

	// default UpdateStrategy
	mutateUpdateStrategy(spec.UpdateStrategy)
}

func mutatePodGangTemplateSpec(spec *v1alpha1.PodGangTemplateSpec) {
	// default PodCliqueTemplateSpec
	mutatePodCliqueTemplateSpec(spec.Cliques)

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

func mutateUpdateStrategy(obj *v1alpha1.GangUpdateStrategy) {
	//default UpdateStrategy
	if obj.Type == "" {
		obj.Type = v1alpha1.GangUpdateStrategyRecreate
	}

	//default RollingUpdateConfig
	if obj.RollingUpdateConfig.MaxSurge == nil {
		*obj.RollingUpdateConfig.MaxSurge = intstr.FromInt(1)
	}

	if obj.RollingUpdateConfig.MaxUnavailable == nil {
		*obj.RollingUpdateConfig.MaxUnavailable = intstr.FromInt(1)
	}
}
