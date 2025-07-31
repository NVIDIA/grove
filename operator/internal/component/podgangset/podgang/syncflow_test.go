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

package podgang

import (
	"context"
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// This is a critical test for HPA scaling logic:
// - Tests how PodGangs split when scaling: base vs scaled PodGangs
// - Verifies minAvailable logic works correctly during scale up/down
// - Ensures the first minAvailable replicas stay gang-scheduled together
func TestMinAvailableWithHPAScaling(t *testing.T) {
	tests := []struct {
		name                   string
		minAvailable           *int32
		initialReplicas        int32
		scaledReplicas         int32
		expectedBasePodGang    string
		expectedScaledPodGangs []string
	}{
		{
			name:                "Scale up from 2 to 4 with minAvailable=1",
			minAvailable:        ptr.To(int32(1)),
			initialReplicas:     2,
			scaledReplicas:      4,
			expectedBasePodGang: "test-pgs-0", // Contains replicas 0 to (minAvailable-1) = 0
			expectedScaledPodGangs: []string{
				"test-pgs-0-test-sg-0", // scaled PodGang 0 (scaling group replica 1)
				"test-pgs-0-test-sg-1", // scaled PodGang 1 (scaling group replica 2)
				"test-pgs-0-test-sg-2", // scaled PodGang 2 (scaling group replica 3)
			},
		},
		{
			name:                "Scale up from 3 to 6 with minAvailable=2",
			minAvailable:        ptr.To(int32(2)),
			initialReplicas:     3,
			scaledReplicas:      6,
			expectedBasePodGang: "test-pgs-0", // Contains replicas 0-1
			expectedScaledPodGangs: []string{
				"test-pgs-0-test-sg-0", // scaled PodGang 0 (scaling group replica 2)
				"test-pgs-0-test-sg-1", // scaled PodGang 1 (scaling group replica 3)
				"test-pgs-0-test-sg-2", // scaled PodGang 2 (scaling group replica 4)
				"test-pgs-0-test-sg-3", // scaled PodGang 3 (scaling group replica 5)
			},
		},
		{
			name:                "Scale down from 5 to 3 with minAvailable=1",
			minAvailable:        ptr.To(int32(1)),
			initialReplicas:     5,
			scaledReplicas:      3,
			expectedBasePodGang: "test-pgs-0", // Contains replica 0 (unchanged)
			expectedScaledPodGangs: []string{
				"test-pgs-0-test-sg-0", // scaled PodGang 0 (scaling group replica 1)
				"test-pgs-0-test-sg-1", // scaled PodGang 1 (scaling group replica 2)
				// scaling group replicas 3-4 should be deleted
			},
		},
		{
			name:                   "Scale to exactly minAvailable",
			minAvailable:           ptr.To(int32(2)),
			initialReplicas:        4,
			scaledReplicas:         2,
			expectedBasePodGang:    "test-pgs-0", // Contains replicas 0-1
			expectedScaledPodGangs: []string{
				// No scaled PodGangs when replicas == minAvailable
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

			// Create fake client with both PGS and PCSG using testutils
			fakeClient := testutils.NewTestClientBuilder().
				WithObjects(pgs, pcsg).
				Build()

			// Test the PodGang creation logic
			r := &_resource{client: fakeClient}
			ctx := context.Background()
			logger := logr.Discard()

			// Test scaled PodGang creation - this should read the scaled PCSG
			expectedPodGangs, err := r.getExpectedPodGangsForPCSG(ctx, logger, pgs, 0)
			require.NoError(t, err)

			// Verify scaled PodGangs
			actualScaledPodGangs := make([]string, 0, len(expectedPodGangs))
			for _, pg := range expectedPodGangs {
				actualScaledPodGangs = append(actualScaledPodGangs, pg.fqn)
			}
			assert.Equal(t, tt.expectedScaledPodGangs, actualScaledPodGangs,
				"Scaled PodGangs should match expected after scaling")

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
