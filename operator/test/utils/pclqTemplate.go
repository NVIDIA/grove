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
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
)

// PodCliqueTemplateSpecBuilder is a builder for creating PodCliqueTemplateSpec objects.
type PodCliqueTemplateSpecBuilder struct {
	pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec
}

// NewPodCliqueTemplateSpecBuilder creates a new PodCliqueTemplateSpecBuilder.
func NewPodCliqueTemplateSpecBuilder(name string) *PodCliqueTemplateSpecBuilder {
	return &PodCliqueTemplateSpecBuilder{
		pclqTemplateSpec: createDefaultPodCliqueTemplateSpec(name),
	}
}

// WithReplicas sets the number of replicas for the PodCliqueTemplateSpec.
func (b *PodCliqueTemplateSpecBuilder) WithReplicas(replicas int32) *PodCliqueTemplateSpecBuilder {
	b.pclqTemplateSpec.Spec.Replicas = replicas
	return b
}

// WithLabels sets the labels for the PodCliqueTemplateSpec.
func (b *PodCliqueTemplateSpecBuilder) WithLabels(labels map[string]string) *PodCliqueTemplateSpecBuilder {
	b.pclqTemplateSpec.Labels = labels
	return b
}

// WithStartsAfter sets the StartsAfter field for the PodCliqueTemplateSpec.
func (b *PodCliqueTemplateSpecBuilder) WithStartsAfter(startsAfter []string) *PodCliqueTemplateSpecBuilder {
	b.pclqTemplateSpec.Spec.StartsAfter = startsAfter
	return b
}

// WithAutoScaleMaxReplicas sets the maximum replicas in ScaleConfig for the PodClique.
func (b *PodCliqueTemplateSpecBuilder) WithAutoScaleMaxReplicas(maximum int32) *PodCliqueTemplateSpecBuilder {
	b.pclqTemplateSpec.Spec.ScaleConfig = &grovecorev1alpha1.AutoScalingConfig{
		MaxReplicas: maximum,
	}
	return b
}

// Build creates a PodCliqueTemplateSpec object.
func (b *PodCliqueTemplateSpecBuilder) Build() *grovecorev1alpha1.PodCliqueTemplateSpec {
	b.withDefaultPodSpec()
	return b.pclqTemplateSpec
}

func (b *PodCliqueTemplateSpecBuilder) withDefaultPodSpec() *PodCliqueTemplateSpecBuilder {
	b.pclqTemplateSpec.Spec.PodSpec = *NewPodBuilder().Build()
	return b
}

func createDefaultPodCliqueTemplateSpec(name string) *grovecorev1alpha1.PodCliqueTemplateSpec {
	return &grovecorev1alpha1.PodCliqueTemplateSpec{
		Name: name,
		Spec: grovecorev1alpha1.PodCliqueSpec{
			Replicas: 1,
		},
	}
}
