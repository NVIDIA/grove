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
	b.pgs.Spec.TemplateSpec.StartupType = startupType
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
	b.pgs.Spec.TemplateSpec.Cliques = append(b.pgs.Spec.TemplateSpec.Cliques, pclq)
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
