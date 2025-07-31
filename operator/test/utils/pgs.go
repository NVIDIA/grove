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

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// PodGangSetBuilder is a builder for PodGangSet objects.
type PodGangSetBuilder struct {
	pgs *grovecorev1alpha1.PodGangSet
}

// NewPodGangSetBuilder creates a new PodGangSetBuilder.
func NewPodGangSetBuilder(name, namespace string) *PodGangSetBuilder {
	return &PodGangSetBuilder{
		pgs: createEmptyPodGangSet(name, namespace),
	}
}

// WithCliqueStartupType sets the StartupType for the PodGangSet.
func (b *PodGangSetBuilder) WithCliqueStartupType(startupType *grovecorev1alpha1.CliqueStartupType) *PodGangSetBuilder {
	b.pgs.Spec.Template.StartupType = startupType
	return b
}

// WithReplicas sets the number of replicas for the PodGangSet.
func (b *PodGangSetBuilder) WithReplicas(replicas int32) *PodGangSetBuilder {
	b.pgs.Spec.Replicas = replicas
	return b
}

// WithPodCliqueParameters is a convenience function that creates a PodCliqueTemplateSpec given the parameters and adds it to the PodGangSet.
func (b *PodGangSetBuilder) WithPodCliqueParameters(name string, replicas int32, startsAfter []string) *PodGangSetBuilder {
	pclqTemplateSpec := NewPodCliqueTemplateSpecBuilder(name).
		WithReplicas(replicas).
		WithStartsAfter(startsAfter).
		Build()
	return b.WithPodCliqueTemplateSpec(pclqTemplateSpec)
}

// WithPodCliqueTemplateSpec sets the PodCliqueTemplateSpec for the PodGangSet.
// Consumers can use PodCliqueBuilder to create a PodCliqueTemplateSpec and then use this method to add it to the PodGangSet.
func (b *PodGangSetBuilder) WithPodCliqueTemplateSpec(pclq *grovecorev1alpha1.PodCliqueTemplateSpec) *PodGangSetBuilder {
	b.pgs.Spec.Template.Cliques = append(b.pgs.Spec.Template.Cliques, pclq)
	return b
}

// WithPodCliqueScalingGroupConfig adds a PodCliqueScalingGroupConfig to the PodGangSet.
func (b *PodGangSetBuilder) WithPodCliqueScalingGroupConfig(config grovecorev1alpha1.PodCliqueScalingGroupConfig) *PodGangSetBuilder {
	b.pgs.Spec.Template.PodCliqueScalingGroupConfigs = append(b.pgs.Spec.Template.PodCliqueScalingGroupConfigs, config)
	return b
}

// Build creates a PodGangSet object.
func (b *PodGangSetBuilder) Build() *grovecorev1alpha1.PodGangSet {
	return b.pgs
}

func createEmptyPodGangSet(name, namespace string) *grovecorev1alpha1.PodGangSet {
	return &grovecorev1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uuid.NewString()),
		},
		Spec: grovecorev1alpha1.PodGangSetSpec{
			Replicas: 1,
		},
	}
}
