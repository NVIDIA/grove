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
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// PodGangSetBuilder is a builder for PodCliqueSet objects.
type PodGangSetBuilder struct {
	pgs *grovecorev1alpha1.PodCliqueSet
}

// NewPodCliqueSetBuilder creates a new PodGangSetBuilder.
func NewPodCliqueSetBuilder(name, namespace string, uid types.UID) *PodGangSetBuilder {
	return &PodGangSetBuilder{
		pgs: createEmptyPodGangSet(name, namespace, uid),
	}
}

// WithCliqueStartupType sets the StartupType for the PodCliqueSet.
func (b *PodGangSetBuilder) WithCliqueStartupType(startupType *grovecorev1alpha1.CliqueStartupType) *PodGangSetBuilder {
	b.pgs.Spec.Template.StartupType = startupType
	return b
}

// WithReplicas sets the number of replicas for the PodCliqueSet.
func (b *PodGangSetBuilder) WithReplicas(replicas int32) *PodGangSetBuilder {
	b.pgs.Spec.Replicas = replicas
	return b
}

// WithPodCliqueParameters is a convenience function that creates a PodCliqueTemplateSpec given the parameters and adds it to the PodCliqueSet.
func (b *PodGangSetBuilder) WithPodCliqueParameters(name string, replicas int32, startsAfter []string) *PodGangSetBuilder {
	pclqTemplateSpec := NewPodCliqueTemplateSpecBuilder(name).
		WithReplicas(replicas).
		WithStartsAfter(startsAfter).
		Build()
	return b.WithPodCliqueTemplateSpec(pclqTemplateSpec)
}

// WithPodCliqueTemplateSpec sets the PodCliqueTemplateSpec for the PodCliqueSet.
// Consumers can use PodCliqueBuilder to create a PodCliqueTemplateSpec and then use this method to add it to the PodCliqueSet.
func (b *PodGangSetBuilder) WithPodCliqueTemplateSpec(pclq *grovecorev1alpha1.PodCliqueTemplateSpec) *PodGangSetBuilder {
	b.pgs.Spec.Template.Cliques = append(b.pgs.Spec.Template.Cliques, pclq)
	return b
}

// WithPodCliqueScalingGroupConfig adds a PodCliqueScalingGroupConfig to the PodCliqueSet.
func (b *PodGangSetBuilder) WithPodCliqueScalingGroupConfig(config grovecorev1alpha1.PodCliqueScalingGroupConfig) *PodGangSetBuilder {
	b.pgs.Spec.Template.PodCliqueScalingGroupConfigs = append(b.pgs.Spec.Template.PodCliqueScalingGroupConfigs, config)
	return b
}

// WithPriorityClassName sets the PriorityClassName for the PodCliqueSet.
func (b *PodGangSetBuilder) WithPriorityClassName(priorityClassName string) *PodGangSetBuilder {
	b.pgs.Spec.Template.PriorityClassName = priorityClassName
	return b
}

// WithTerminationDelay sets the TerminationDelay for the PodCliqueSet.
func (b *PodGangSetBuilder) WithTerminationDelay(duration time.Duration) *PodGangSetBuilder {
	b.pgs.Spec.Template.TerminationDelay = &metav1.Duration{Duration: duration}
	return b
}

// WithStandaloneClique adds a standalone clique (not part of any scaling group).
func (b *PodGangSetBuilder) WithStandaloneClique(name string) *PodGangSetBuilder {
	cliqueSpec := &grovecorev1alpha1.PodCliqueTemplateSpec{
		Name: name,
		Spec: grovecorev1alpha1.PodCliqueSpec{
			Replicas: 1,
		},
	}
	b.pgs.Spec.Template.Cliques = append(b.pgs.Spec.Template.Cliques, cliqueSpec)
	return b
}

// WithStandaloneCliqueReplicas adds a standalone clique with specific replica count.
func (b *PodGangSetBuilder) WithStandaloneCliqueReplicas(name string, replicas int32) *PodGangSetBuilder {
	cliqueSpec := &grovecorev1alpha1.PodCliqueTemplateSpec{
		Name: name,
		Spec: grovecorev1alpha1.PodCliqueSpec{
			Replicas: replicas,
		},
	}
	b.pgs.Spec.Template.Cliques = append(b.pgs.Spec.Template.Cliques, cliqueSpec)
	return b
}

// WithScalingGroup adds a scaling group with the specified cliques.
func (b *PodGangSetBuilder) WithScalingGroup(name string, cliqueNames []string) *PodGangSetBuilder {
	return b.WithScalingGroupConfig(name, cliqueNames, 1, 1)
}

// WithScalingGroupConfig adds a scaling group with custom replicas and minAvailable.
func (b *PodGangSetBuilder) WithScalingGroupConfig(name string, cliqueNames []string, replicas, minAvailable int32) *PodGangSetBuilder {
	// Add cliques for the scaling group
	for _, cliqueName := range cliqueNames {
		cliqueSpec := &grovecorev1alpha1.PodCliqueTemplateSpec{
			Name: cliqueName,
			Spec: grovecorev1alpha1.PodCliqueSpec{
				Replicas: 1,
			},
		}
		b.pgs.Spec.Template.Cliques = append(b.pgs.Spec.Template.Cliques, cliqueSpec)
	}

	// Add scaling group config
	pcsgConfig := grovecorev1alpha1.PodCliqueScalingGroupConfig{
		Name:         name,
		CliqueNames:  cliqueNames,
		Replicas:     &replicas,
		MinAvailable: &minAvailable,
	}
	b.pgs.Spec.Template.PodCliqueScalingGroupConfigs = append(b.pgs.Spec.Template.PodCliqueScalingGroupConfigs, pcsgConfig)
	return b
}

// WithPodGangSetGenerationHash sets the CurrentGenerationHash in the PodCliqueSet status.
func (b *PodGangSetBuilder) WithPodGangSetGenerationHash(pgsGenerationHash *string) *PodGangSetBuilder {
	b.pgs.Status.CurrentGenerationHash = pgsGenerationHash
	return b
}

// Build creates a PodCliqueSet object.
func (b *PodGangSetBuilder) Build() *grovecorev1alpha1.PodCliqueSet {
	return b.pgs
}

func createEmptyPodGangSet(name, namespace string, uid types.UID) *grovecorev1alpha1.PodCliqueSet {
	return &grovecorev1alpha1.PodCliqueSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       uid,
		},
		Spec: grovecorev1alpha1.PodCliqueSetSpec{
			Replicas: 1,
		},
	}
}
