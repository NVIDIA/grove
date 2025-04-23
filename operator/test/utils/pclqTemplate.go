package utils

import (
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/samber/lo"
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

// WithAutoScaleLimits sets the minimum and maximum replicas in ScaleConfig for the PodClique.
func (b *PodCliqueTemplateSpecBuilder) WithAutoScaleLimits(minimum, maximum int32) *PodCliqueTemplateSpecBuilder {
	b.pclqTemplateSpec.Spec.ScaleConfig = &grovecorev1alpha1.AutoScalingConfig{
		MinReplicas: lo.ToPtr(minimum),
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
