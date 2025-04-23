package utils

import (
	corev1 "k8s.io/api/core/v1"
)

// PodSpecBuilder is a builder for creating Pod objects.
type PodSpecBuilder struct {
	podSpec *corev1.PodSpec
}

// NewPodBuilder creates a new PodSpecBuilder.
func NewPodBuilder() *PodSpecBuilder {
	return &PodSpecBuilder{
		podSpec: createDefaultPodSpec(),
	}
}

func (b *PodSpecBuilder) Build() *corev1.PodSpec {
	return b.podSpec
}

func createDefaultPodSpec() *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:    "test-container",
				Image:   "alpine:3.21",
				Command: []string{"/bin/sh", "-c", "sleep 2m"},
			},
		},
		RestartPolicy: corev1.RestartPolicyAlways,
	}
}
