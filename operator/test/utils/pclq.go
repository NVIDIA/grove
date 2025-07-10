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
	"github.com/NVIDIA/grove/operator/internal/component"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

// PodCliqueBuilder is a builder for creating PodClique objects.
// This should primarily be used for tests.
type PodCliqueBuilder struct {
	pgsName         string
	pgsReplicaIndex int32
	pclq            *grovecorev1alpha1.PodClique
}

// NewPodCliqueBuilder creates a new PodCliqueBuilder.
func NewPodCliqueBuilder(pgsName string, pgsUID types.UID, pclqTemplateName, namespace string, pgsReplicaIndex int32) *PodCliqueBuilder {
	return &PodCliqueBuilder{
		pgsName:         pgsName,
		pgsReplicaIndex: pgsReplicaIndex,
		pclq:            createDefaultPodCliqueWithoutPodSpec(pgsName, pgsUID, pclqTemplateName, namespace, pgsReplicaIndex),
	}
}

// WithLabels merges the passed labels with default labels.
// Passed in labels will overwrite default labels with the same keys.
func (b *PodCliqueBuilder) WithLabels(labels map[string]string) *PodCliqueBuilder {
	b.pclq.Labels = lo.Assign(b.pclq.Labels, labels)
	return b
}

// WithReplicas sets the number of replicas for the PodClique.
// Default is set to 1.
func (b *PodCliqueBuilder) WithReplicas(replicas int32) *PodCliqueBuilder {
	b.pclq.Spec.Replicas = replicas
	return b
}

// WithStartsAfter sets the StartsAfter field for the PodClique.
func (b *PodCliqueBuilder) WithStartsAfter(pclqTemplateNames []string) *PodCliqueBuilder {
	pclqDependencies := lo.Map(pclqTemplateNames, func(pclqTemplateName string, _ int) string {
		return grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: b.pgsName, Replica: int(b.pgsReplicaIndex)}, pclqTemplateName)
	})
	b.pclq.Spec.StartsAfter = pclqDependencies
	return b
}

// WithAutoScaleMaxReplicas sets the maximum replicas in ScaleConfig for the PodClique.
func (b *PodCliqueBuilder) WithAutoScaleMaxReplicas(maximum int32) *PodCliqueBuilder {
	b.pclq.Spec.ScaleConfig = &grovecorev1alpha1.AutoScalingConfig{
		MaxReplicas: maximum,
	}
	return b
}

// WithOwnerReference sets the owner reference for the PodClique.
func (b *PodCliqueBuilder) WithOwnerReference(ownerRef metav1.OwnerReference) *PodCliqueBuilder {
	b.pclq.OwnerReferences = []metav1.OwnerReference{ownerRef}
	return b
}

// Build creates a PodClique object.
func (b *PodCliqueBuilder) Build() *grovecorev1alpha1.PodClique {
	_ = b.withDefaultPodSpec()
	return b.pclq
}

func (b *PodCliqueBuilder) withDefaultPodSpec() *PodCliqueBuilder {
	b.pclq.Spec.PodSpec = *NewPodBuilder().Build()
	return b
}

func createDefaultPodCliqueWithoutPodSpec(pgsName string, pgsUID types.UID, pclqTemplateName, namespace string, pgsReplicaIndex int32) *grovecorev1alpha1.PodClique {
	pclqName := grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: int(pgsReplicaIndex)}, pclqTemplateName)
	return &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pclqName,
			Namespace: namespace,
			Labels:    getDefaultLabels(pgsName, pclqName),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         grovecorev1alpha1.SchemeGroupVersion.String(),
					Kind:               grovecorev1alpha1.PodGangSetKind,
					Name:               pgsName,
					UID:                pgsUID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			Replicas: 1,
		},
	}
}

func getDefaultLabels(pgsName, pclqName string) map[string]string {
	pclqComponentLabels := map[string]string{
		grovecorev1alpha1.LabelAppNameKey:   pclqName,
		grovecorev1alpha1.LabelComponentKey: component.NamePGSPodClique,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		pclqComponentLabels,
	)
}
