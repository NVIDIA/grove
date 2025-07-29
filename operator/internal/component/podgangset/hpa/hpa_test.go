package hpa

import (
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestComputeExpectedHPAs(t *testing.T) {
	tests := []struct {
		name     string
		pgs      *grovecorev1alpha1.PodGangSet
		expected []hpaInfo
	}{
		{
			name: "PodClique HPA with explicit minReplicas",
			pgs: &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pgs", Namespace: "default"},
				Spec: grovecorev1alpha1.PodGangSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodGangSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "test-clique",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas:     3,
									MinAvailable: ptr.To(int32(1)),
									ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
										MinReplicas: ptr.To(int32(2)),
										MaxReplicas: 5,
									},
								},
							},
						},
					},
				},
			},
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueKind,
					targetScaleResourceName: "test-pgs-0-test-clique",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(2)),
						MaxReplicas: 5,
					},
					fallbackMinReplicas: 3, // replicas (validation ensures >= minAvailable)
				},
			},
		},
		{
			name: "PodClique HPA without explicit minReplicas",
			pgs: &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pgs", Namespace: "default"},
				Spec: grovecorev1alpha1.PodGangSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodGangSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "test-clique",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas:     3,
									MinAvailable: ptr.To(int32(2)),
									ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
										MaxReplicas: 5,
										// MinReplicas not specified
									},
								},
							},
						},
					},
				},
			},
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueKind,
					targetScaleResourceName: "test-pgs-0-test-clique",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MaxReplicas: 5,
						// MinReplicas not specified
					},
					fallbackMinReplicas: 3, // replicas (validation ensures >= minAvailable)
				},
			},
		},
		{
			name: "PodCliqueScalingGroup HPA with explicit minReplicas",
			pgs: &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pgs", Namespace: "default"},
				Spec: grovecorev1alpha1.PodGangSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodGangSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:         "test-sg",
								Replicas:     ptr.To(int32(4)),
								MinAvailable: ptr.To(int32(3)),
								CliqueNames:  []string{"test-clique"},
								ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
									MinReplicas: ptr.To(int32(2)),
									MaxReplicas: 6,
								},
							},
						},
					},
				},
			},
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueScalingGroupKind,
					targetScaleResourceName: "test-pgs-0-test-sg",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(2)),
						MaxReplicas: 6,
					},
					fallbackMinReplicas: 4, // replicas (validation ensures >= minAvailable)
				},
			},
		},
		{
			name: "PodCliqueScalingGroup HPA without explicit minReplicas",
			pgs: &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pgs", Namespace: "default"},
				Spec: grovecorev1alpha1.PodGangSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodGangSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:         "test-sg",
								Replicas:     ptr.To(int32(4)),
								MinAvailable: ptr.To(int32(1)),
								CliqueNames:  []string{"test-clique"},
								ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
									MaxReplicas: 6,
									// MinReplicas not specified
								},
							},
						},
					},
				},
			},
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueScalingGroupKind,
					targetScaleResourceName: "test-pgs-0-test-sg",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MaxReplicas: 6,
						// MinReplicas not specified
					},
					fallbackMinReplicas: 4, // replicas (validation ensures >= minAvailable)
				},
			},
		},
		{
			name: "Mixed HPAs",
			pgs: &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pgs", Namespace: "default"},
				Spec: grovecorev1alpha1.PodGangSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodGangSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "individual-clique",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas:     2,
									MinAvailable: ptr.To(int32(1)),
									ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
										MaxReplicas: 4,
									},
								},
							},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:         "scaling-group",
								Replicas:     ptr.To(int32(3)),
								MinAvailable: ptr.To(int32(2)),
								CliqueNames:  []string{"sg-clique"},
								ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
									MinReplicas: ptr.To(int32(1)),
									MaxReplicas: 5,
								},
							},
						},
					},
				},
			},
			expected: []hpaInfo{
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueKind,
					targetScaleResourceName: "test-pgs-0-individual-clique",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MaxReplicas: 4,
						// MinReplicas not specified
					},
					fallbackMinReplicas: 2, // replicas (validation ensures >= minAvailable)
				},
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueScalingGroupKind,
					targetScaleResourceName: "test-pgs-0-scaling-group",
					scaleConfig: grovecorev1alpha1.AutoScalingConfig{
						MinReplicas: ptr.To(int32(1)),
						MaxReplicas: 5,
					},
					fallbackMinReplicas: 3, // replicas (validation ensures >= minAvailable)
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
				assert.Equal(t, expected.fallbackMinReplicas, result[i].fallbackMinReplicas,
					"fallbackMinReplicas should be correctly set for %s", expected.targetScaleResourceName)
			}
		})
	}
}

func TestBuildResource(t *testing.T) {
	tests := []struct {
		name                string
		hpaInfo             hpaInfo
		expectedMinReplicas int32
	}{
		{
			name: "Uses explicit MinReplicas from scaleConfig",
			hpaInfo: hpaInfo{
				scaleConfig: grovecorev1alpha1.AutoScalingConfig{
					MinReplicas: ptr.To(int32(2)),
					MaxReplicas: 5,
				},
				fallbackMinReplicas: 3, // replicas (validation ensures >= minAvailable)
			},
			expectedMinReplicas: 2, // Should use explicit MinReplicas
		},
		{
			name: "Uses replicas fallback when MinReplicas is nil",
			hpaInfo: hpaInfo{
				scaleConfig: grovecorev1alpha1.AutoScalingConfig{
					MaxReplicas: 5,
					// MinReplicas not specified
				},
				fallbackMinReplicas: 3, // replicas (validation ensures >= minAvailable)
			},
			expectedMinReplicas: 3, // Should use replicas fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &_resource{}
			pgs := &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pgs", Namespace: "default"},
			}
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}

			// This would normally be set by the scheme, but for testing we can skip it
			err := r.buildResource(pgs, hpa, tt.hpaInfo)

			// We expect an error due to missing scheme, but we can still check MinReplicas
			assert.NotNil(t, err) // Expected due to SetControllerReference failing
			assert.Equal(t, tt.expectedMinReplicas, *hpa.Spec.MinReplicas,
				"MinReplicas should be correctly set")
			assert.Equal(t, tt.hpaInfo.scaleConfig.MaxReplicas, hpa.Spec.MaxReplicas,
				"MaxReplicas should be correctly set")
		})
	}
}
