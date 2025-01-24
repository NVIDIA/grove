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
	corev1 "k8s.io/api/core/v1"
)

// mutatePodTemplateSpec adds defaults to PodSpec.
func mutatePodTemplateSpec(spec *corev1.PodTemplateSpec) {
	mutatePodSpec(spec.Spec)
}

func mutatePodSpec(spec corev1.PodSpec) {
	// default TerminationGracePeriodSeconds
	if spec.TerminationGracePeriodSeconds == nil {
		*spec.TerminationGracePeriodSeconds = 30
	}
}
