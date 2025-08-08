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

// Package testutils provides reusable test utilities and builders for Grove operator testing.
//
// This package contains builders for creating test objects, common setup functions,
// and utility helpers that can be shared across different controller test files.
//
// Example usage:
//
//	pgs := testutils.NewPGSBuilder("test-pgs", "default").
//		WithReplicas(2).
//		WithStandaloneClique("worker").
//		WithScalingGroup("compute", []string{"frontend", "backend"}).
//		Build()
//
//	pcsg := testutils.NewPCSGBuilder("test-pcsg", "default", "test-pgs", 0).
//		WithOptions(testutils.WithPCSGHealthy()).
//		Build()
package testutils

import (
	"strconv"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// ============================================================================
// PodGangSet Builder
// ============================================================================

// PGSBuilder provides a fluent interface for building test PodGangSet objects.
type PGSBuilder struct {
	pgs *grovecorev1alpha1.PodGangSet
}

// NewPGSBuilder creates a new PGSBuilder with basic configuration.
func NewPGSBuilder(name, namespace string) *PGSBuilder {
	return &PGSBuilder{
		pgs: &grovecorev1alpha1.PodGangSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: grovecorev1alpha1.PodGangSetSpec{
				Replicas: 1,
				Template: grovecorev1alpha1.PodGangSetTemplateSpec{
					Cliques:                      []*grovecorev1alpha1.PodCliqueTemplateSpec{},
					PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{},
				},
			},
		},
	}
}

// WithReplicas sets the number of replicas for the PodGangSet.
func (b *PGSBuilder) WithReplicas(replicas int32) *PGSBuilder {
	b.pgs.Spec.Replicas = replicas
	return b
}

// WithStandaloneClique adds a standalone clique (not part of any scaling group).
func (b *PGSBuilder) WithStandaloneClique(name string) *PGSBuilder {
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
func (b *PGSBuilder) WithStandaloneCliqueReplicas(name string, replicas int32) *PGSBuilder {
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
func (b *PGSBuilder) WithScalingGroup(name string, cliqueNames []string) *PGSBuilder {
	return b.WithScalingGroupConfig(name, cliqueNames, 1, 1)
}

// WithScalingGroupConfig adds a scaling group with custom replicas and minAvailable.
func (b *PGSBuilder) WithScalingGroupConfig(name string, cliqueNames []string, replicas, minAvailable int32) *PGSBuilder {
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
		Replicas:     ptr.To(replicas),
		MinAvailable: ptr.To(minAvailable),
	}
	b.pgs.Spec.Template.PodCliqueScalingGroupConfigs = append(b.pgs.Spec.Template.PodCliqueScalingGroupConfigs, pcsgConfig)
	return b
}

// WithLabels adds labels to the PodGangSet.
func (b *PGSBuilder) WithLabels(labels map[string]string) *PGSBuilder {
	if b.pgs.Labels == nil {
		b.pgs.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.pgs.Labels[k] = v
	}
	return b
}

// WithAnnotations adds annotations to the PodGangSet.
func (b *PGSBuilder) WithAnnotations(annotations map[string]string) *PGSBuilder {
	if b.pgs.Annotations == nil {
		b.pgs.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		b.pgs.Annotations[k] = v
	}
	return b
}

// WithObservedGeneration sets the ObservedGeneration field in the PodGangSet status.
func (b *PGSBuilder) WithObservedGeneration(generation int64) *PGSBuilder {
	b.pgs.Status.ObservedGeneration = &generation
	return b
}

// Build returns the constructed PodGangSet.
func (b *PGSBuilder) Build() *grovecorev1alpha1.PodGangSet {
	return b.pgs
}

// ============================================================================
// PodCliqueScalingGroup Builder
// ============================================================================

// PCSGBuilder provides a fluent interface for building test PodCliqueScalingGroup objects.
type PCSGBuilder struct {
	pcsg *grovecorev1alpha1.PodCliqueScalingGroup
}

// NewPCSGBuilder creates a new PCSGBuilder with basic configuration.
func NewPCSGBuilder(name, namespace, pgsName string, replicaIndex int) *PCSGBuilder {
	return &PCSGBuilder{
		pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					grovecorev1alpha1.LabelManagedByKey:           grovecorev1alpha1.LabelManagedByValue,
					grovecorev1alpha1.LabelPartOfKey:              pgsName,
					grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(replicaIndex),
				},
			},
			Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
				Replicas: 1,
			},
			Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{},
		},
	}
}

// WithReplicas sets the number of replicas for the PodCliqueScalingGroup.
func (b *PCSGBuilder) WithReplicas(replicas int32) *PCSGBuilder {
	b.pcsg.Spec.Replicas = replicas
	return b
}

// WithMinAvailable sets the MinAvailable field for the PodCliqueScalingGroup.
func (b *PCSGBuilder) WithMinAvailable(minAvailable int32) *PCSGBuilder {
	b.pcsg.Spec.MinAvailable = ptr.To(minAvailable)
	return b
}

// WithCliqueNames sets the CliqueNames field for the PodCliqueScalingGroup.
func (b *PCSGBuilder) WithCliqueNames(cliqueNames []string) *PCSGBuilder {
	b.pcsg.Spec.CliqueNames = cliqueNames
	return b
}

// WithLabels adds labels to the PodCliqueScalingGroup.
func (b *PCSGBuilder) WithLabels(labels map[string]string) *PCSGBuilder {
	if b.pcsg.Labels == nil {
		b.pcsg.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.pcsg.Labels[k] = v
	}
	return b
}

// WithOwnerReference adds an owner reference to the PodCliqueScalingGroup.
func (b *PCSGBuilder) WithOwnerReference(kind, name, uid string) *PCSGBuilder {
	ownerRef := metav1.OwnerReference{
		Kind: kind,
		Name: name,
		UID:  types.UID("test-uid"),
	}
	if uid != "" {
		ownerRef.UID = types.UID(uid)
	}
	b.pcsg.OwnerReferences = append(b.pcsg.OwnerReferences, ownerRef)
	return b
}

// WithOptions applies option functions to customize the PodCliqueScalingGroup.
func (b *PCSGBuilder) WithOptions(opts ...PCSGOption) *PCSGBuilder {
	for _, opt := range opts {
		opt(b.pcsg)
	}
	return b
}

// Build returns the constructed PodCliqueScalingGroup.
func (b *PCSGBuilder) Build() *grovecorev1alpha1.PodCliqueScalingGroup {
	return b.pcsg
}

// ============================================================================
// PodClique Builder
// ============================================================================

// PodCliqueBuilder provides a fluent interface for building test PodClique objects.
type PodCliqueBuilder struct {
	pclq *grovecorev1alpha1.PodClique
}

// NewPodCliqueBuilder creates a new PodCliqueBuilder with basic configuration.
func NewPodCliqueBuilder(name, namespace, pgsName string, replicaIndex int) *PodCliqueBuilder {
	return &PodCliqueBuilder{
		pclq: &grovecorev1alpha1.PodClique{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					grovecorev1alpha1.LabelManagedByKey:           grovecorev1alpha1.LabelManagedByValue,
					grovecorev1alpha1.LabelPartOfKey:              pgsName,
					grovecorev1alpha1.LabelComponentKey:           common.NamePGSPodClique,
					grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(replicaIndex),
				},
			},
			Spec: grovecorev1alpha1.PodCliqueSpec{
				Replicas: 1,
			},
			Status: grovecorev1alpha1.PodCliqueStatus{},
		},
	}
}

// NewPCSGPodCliqueBuilder creates a PodClique that belongs to a PodCliqueScalingGroup.
func NewPCSGPodCliqueBuilder(name, namespace, pgsName, pcsgName string, pgsReplicaIndex, pcsgReplicaIndex int) *PodCliqueBuilder {
	return &PodCliqueBuilder{
		pclq: &grovecorev1alpha1.PodClique{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					grovecorev1alpha1.LabelManagedByKey:                      grovecorev1alpha1.LabelManagedByValue,
					grovecorev1alpha1.LabelPartOfKey:                         pgsName,
					grovecorev1alpha1.LabelPodCliqueScalingGroup:             pcsgName,
					grovecorev1alpha1.LabelComponentKey:                      common.NamePCSGPodClique,
					grovecorev1alpha1.LabelPodGangSetReplicaIndex:            strconv.Itoa(pgsReplicaIndex),
					grovecorev1alpha1.LabelPodCliqueScalingGroupReplicaIndex: strconv.Itoa(pcsgReplicaIndex),
				},
			},
			Spec: grovecorev1alpha1.PodCliqueSpec{
				Replicas: 1,
			},
			Status: grovecorev1alpha1.PodCliqueStatus{},
		},
	}
}

// WithReplicas sets the number of replicas for the PodClique.
func (b *PodCliqueBuilder) WithReplicas(replicas int32) *PodCliqueBuilder {
	b.pclq.Spec.Replicas = replicas
	return b
}

// WithMinAvailable sets the MinAvailable field for the PodClique.
func (b *PodCliqueBuilder) WithMinAvailable(minAvailable int32) *PodCliqueBuilder {
	b.pclq.Spec.MinAvailable = ptr.To(minAvailable)
	return b
}

// WithLabels adds labels to the PodClique.
func (b *PodCliqueBuilder) WithLabels(labels map[string]string) *PodCliqueBuilder {
	if b.pclq.Labels == nil {
		b.pclq.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.pclq.Labels[k] = v
	}
	return b
}

// WithOwnerReference adds an owner reference to the PodClique.
func (b *PodCliqueBuilder) WithOwnerReference(kind, name, uid string) *PodCliqueBuilder {
	ownerRef := metav1.OwnerReference{
		Kind: kind,
		Name: name,
		UID:  types.UID(""),
	}
	if uid != "" {
		ownerRef.UID = types.UID(uid)
	}

	b.pclq.OwnerReferences = append(b.pclq.OwnerReferences, ownerRef)
	return b
}

// WithOptions applies option functions to customize the PodClique.
func (b *PodCliqueBuilder) WithOptions(opts ...PCLQOption) *PodCliqueBuilder {
	for _, opt := range opts {
		opt(b.pclq)
	}
	return b
}

// Build returns the constructed PodClique.
func (b *PodCliqueBuilder) Build() *grovecorev1alpha1.PodClique {
	return b.pclq
}
