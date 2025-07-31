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

package hpa

import (
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/utils/ptr"
)

func TestComputeExpectedHPAs(t *testing.T) {
	tests := []struct {
		name     string
		pgs      *grovecorev1alpha1.PodGangSet
		expected []hpaInfo
	}{
		{
			name: "PodClique HPA",
			pgs: testutils.NewPodGangSetBuilder("test-pgs", "default").
				WithReplicas(1).
				WithPodCliqueTemplateSpec(
					testutils.NewPodCliqueTemplateSpecBuilder("test-clique").
						WithReplicas(3).
						WithMinAvailable(1).
						WithScaleConfig(ptr.To(int32(2)), 5).
						Build(),
				).
				Build(),
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueKind,
					targetScaleResourceName: "test-pgs-0-test-clique",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(2)),
						MaxReplicas: 5,
					},
				},
			},
		},
		{
			name: "PodCliqueScalingGroup HPA",
			pgs: testutils.NewPodGangSetBuilder("test-pgs", "default").
				WithReplicas(1).
				WithPodCliqueScalingGroupConfig(grovecorev1alpha1.PodCliqueScalingGroupConfig{
					Name:         "test-sg",
					Replicas:     ptr.To(int32(4)),
					MinAvailable: ptr.To(int32(3)),
					CliqueNames:  []string{"test-clique"},
					ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(2)),
						MaxReplicas: 6,
					},
				}).
				Build(),
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueScalingGroupKind,
					targetScaleResourceName: "test-pgs-0-test-sg",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(2)),
						MaxReplicas: 6,
					},
				},
			},
		},
		{
			name: "Both PodClique and PodCliqueScalingGroup HPAs",
			pgs: testutils.NewPodGangSetBuilder("test-pgs", "default").
				WithReplicas(1).
				WithPodCliqueTemplateSpec(
					testutils.NewPodCliqueTemplateSpecBuilder("individual-clique").
						WithReplicas(2).
						WithMinAvailable(1).
						WithScaleConfig(ptr.To(int32(2)), 4).
						Build(),
				).
				WithPodCliqueScalingGroupConfig(grovecorev1alpha1.PodCliqueScalingGroupConfig{
					Name:         "scaling-group",
					Replicas:     ptr.To(int32(3)),
					MinAvailable: ptr.To(int32(2)),
					CliqueNames:  []string{"sg-clique"},
					ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(1)),
						MaxReplicas: 5,
					},
				}).
				Build(),
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueKind,
					targetScaleResourceName: "test-pgs-0-individual-clique",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(2)),
						MaxReplicas: 4,
					},
				},
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueScalingGroupKind,
					targetScaleResourceName: "test-pgs-0-scaling-group",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(1)),
						MaxReplicas: 5,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &_resource{}
			result := r.computeExpectedHPAs(tt.pgs)

			assert.Equal(t, len(tt.expected), len(result), "Number of HPAs should match")

			for i, expected := range tt.expected {
				assert.Equal(t, expected.targetScaleResourceKind, result[i].targetScaleResourceKind)
				assert.Equal(t, expected.targetScaleResourceName, result[i].targetScaleResourceName)
				assert.Equal(t, expected.scaleConfig, result[i].scaleConfig,
					"scaleConfig should be correctly set for %s", expected.targetScaleResourceName)
			}
		})
	}
}

func TestBuildResource(t *testing.T) {
	tests := []struct {
		name                string
		hpaInfo             hpaInfo
		expectedMinReplicas int32
		expectedMaxReplicas int32
	}{
		{
			name: "Sets HPA spec from scaleConfig",
			hpaInfo: hpaInfo{
				targetScaleResourceKind: grovecorev1alpha1.PodCliqueKind,
				targetScaleResourceName: "test-resource",
				scaleConfig: grovecorev1alpha1.AutoScalingConfig{
					MinReplicas: ptr.To(int32(2)),
					MaxReplicas: 5,
				},
			},
			expectedMinReplicas: 2,
			expectedMaxReplicas: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &_resource{}
			pgs := testutils.NewPodGangSetBuilder("test-pgs", "default").Build()
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}

			// This would normally be set by the scheme, but for testing we can skip it
			err := r.buildResource(pgs, hpa, tt.hpaInfo)

			// We expect an error due to missing scheme, but we can still check the spec
			assert.NotNil(t, err) // Expected due to SetControllerReference failing
			assert.Equal(t, tt.expectedMinReplicas, *hpa.Spec.MinReplicas,
				"MinReplicas should be correctly set")
			assert.Equal(t, tt.expectedMaxReplicas, hpa.Spec.MaxReplicas,
				"MaxReplicas should be correctly set")
			assert.Equal(t, tt.hpaInfo.targetScaleResourceKind, hpa.Spec.ScaleTargetRef.Kind,
				"ScaleTargetRef Kind should be correctly set")
			assert.Equal(t, tt.hpaInfo.targetScaleResourceName, hpa.Spec.ScaleTargetRef.Name,
				"ScaleTargetRef Name should be correctly set")
		})
	}
}
