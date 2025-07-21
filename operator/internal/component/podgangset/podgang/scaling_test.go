package podgang

import (
	"context"
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMinAvailableWithHPAScaling(t *testing.T) {
	tests := []struct {
		name                       string
		minAvailable               *int32
		initialReplicas            int32
		scaledReplicas             int32
		expectedBasePodGang        string
		expectedIndividualPodGangs []string
	}{
		{
			name:                "Scale up from 2 to 4 with minAvailable=1",
			minAvailable:        ptr.To(int32(1)),
			initialReplicas:     2,
			scaledReplicas:      4,
			expectedBasePodGang: "test-pgs-0", // Contains replicas 0 to (minAvailable-1) = 0
			expectedIndividualPodGangs: []string{
				"test-pgs-0-test-sg-0", // individual PodGang 0 (scaling group replica 1)
				"test-pgs-0-test-sg-1", // individual PodGang 1 (scaling group replica 2)
				"test-pgs-0-test-sg-2", // individual PodGang 2 (scaling group replica 3)
			},
		},
		{
			name:                "Scale up from 3 to 6 with minAvailable=2",
			minAvailable:        ptr.To(int32(2)),
			initialReplicas:     3,
			scaledReplicas:      6,
			expectedBasePodGang: "test-pgs-0", // Contains replicas 0-1
			expectedIndividualPodGangs: []string{
				"test-pgs-0-test-sg-0", // individual PodGang 0 (scaling group replica 2)
				"test-pgs-0-test-sg-1", // individual PodGang 1 (scaling group replica 3)
				"test-pgs-0-test-sg-2", // individual PodGang 2 (scaling group replica 4)
				"test-pgs-0-test-sg-3", // individual PodGang 3 (scaling group replica 5)
			},
		},
		{
			name:                "Scale down from 5 to 3 with minAvailable=1",
			minAvailable:        ptr.To(int32(1)),
			initialReplicas:     5,
			scaledReplicas:      3,
			expectedBasePodGang: "test-pgs-0", // Contains replica 0 (unchanged)
			expectedIndividualPodGangs: []string{
				"test-pgs-0-test-sg-0", // individual PodGang 0 (scaling group replica 1)
				"test-pgs-0-test-sg-1", // individual PodGang 1 (scaling group replica 2)
				// scaling group replicas 3-4 should be deleted
			},
		},
		{
			name:                       "Scale to exactly minAvailable",
			minAvailable:               ptr.To(int32(2)),
			initialReplicas:            4,
			scaledReplicas:             2,
			expectedBasePodGang:        "test-pgs-0", // Contains replicas 0-1
			expectedIndividualPodGangs: []string{
				// No individual PodGangs when replicas == minAvailable
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test PodGangSet
			pgs := &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pgs",
					Namespace: "default",
					UID:       "test-uid-123",
				},
				Spec: grovecorev1alpha1.PodGangSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodGangSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "test-clique",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas:     2,
									MinAvailable: ptr.To(int32(2)),
								},
							},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:         "test-sg",
								Replicas:     &tt.scaledReplicas, // This simulates HPA scaling
								MinAvailable: tt.minAvailable,
								CliqueNames:  []string{"test-clique"},
							},
						},
					},
				},
			}

			// Create test PodCliqueScalingGroup (simulates what HPA would create)
			pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pgs-0-test-sg",
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/managed-by":      "grove-operator",
						"app.kubernetes.io/part-of":         "test-pgs",
						"grove.io/podgangset-replica-index": "0",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "grove.io/v1alpha1",
							Kind:       "PodGangSet",
							Name:       "test-pgs",
							UID:        "test-uid-123",
							Controller: ptr.To(true),
						},
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     tt.scaledReplicas, // This is what HPA modifies
					MinAvailable: tt.minAvailable,
					CliqueNames:  []string{"test-clique"},
				},
			}

			// Create fake client with both PGS and PCSG
			scheme := runtime.NewScheme()
			grovecorev1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(pgs, pcsg).
				Build()

			// Test the PodGang creation logic
			r := &_resource{client: fakeClient}
			ctx := context.Background()
			logger := logr.Discard()

			// Test individual PodGang creation - this should read the scaled PCSG
			expectedPodGangs, err := r.getExpectedPodGangsForPCSG(ctx, logger, pgs, 0)
			require.NoError(t, err)

			// Verify individual PodGangs
			actualIndividualPodGangs := make([]string, len(expectedPodGangs))
			for i, pg := range expectedPodGangs {
				actualIndividualPodGangs[i] = pg.fqn
			}
			assert.Equal(t, tt.expectedIndividualPodGangs, actualIndividualPodGangs,
				"Individual PodGangs should match expected after scaling")

			// Test base PodGang logic - this should be independent of scaling
			sc := &syncContext{pgs: pgs}
			basePodGangs := getExpectedPodGangForPGSReplicas(sc)
			require.Len(t, basePodGangs, 1, "Should have exactly one base PodGang")
			assert.Equal(t, tt.expectedBasePodGang, basePodGangs[0].fqn,
				"Base PodGang name should be correct and unchanged by scaling")

			// Verify base PodGang only contains replicas 0 to (minAvailable-1)
			minAvail := int32(1)
			if tt.minAvailable != nil {
				minAvail = *tt.minAvailable
			}
			expectedBasePodCliques := int(minAvail)
			actualBasePodCliques := len(basePodGangs[0].pclqs)
			assert.Equal(t, expectedBasePodCliques, actualBasePodCliques,
				"Base PodGang should only contain PodCliques for replicas 0 to (minAvailable-1)")
		})
	}
}
