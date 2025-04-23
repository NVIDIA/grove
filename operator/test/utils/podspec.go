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

// Build returns the constructed PodSpec.
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
