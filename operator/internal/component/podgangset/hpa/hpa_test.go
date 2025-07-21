package hpa

import (
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/stretchr/testify/assert"
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
									Replicas: 3,
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
					minReplicas:             2, // Should use explicit minReplicas
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
									Replicas: 3,
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
					minReplicas:             3, // Should use template replicas
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
								Name:        "test-sg",
								Replicas:    ptr.To(int32(4)),
								CliqueNames: []string{"test-clique"},
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
					minReplicas:             2, // Should use explicit minReplicas
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
								Name:        "test-sg",
								Replicas:    ptr.To(int32(4)),
								CliqueNames: []string{"test-clique"},
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
					minReplicas:             4, // Should use initial replicas
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
									Replicas: 2,
									ScaleConfig: &grovecorev1alpha1.AutoScalingConfig{
										MaxReplicas: 4,
									},
								},
							},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "scaling-group",
								Replicas:    ptr.To(int32(3)),
								CliqueNames: []string{"sg-clique"},
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
					minReplicas:             2, // Uses template replicas
				},
				{
					targetScaleResourceKind: grovecorev1alpha1.PodCliqueScalingGroupKind,
					targetScaleResourceName: "test-pgs-0-scaling-group",
					minReplicas:             1, // Uses explicit minReplicas
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
				assert.Equal(t, expected.minReplicas, result[i].minReplicas,
					"minReplicas should be correctly computed for %s", expected.targetScaleResourceName)
			}
		})
	}
}
