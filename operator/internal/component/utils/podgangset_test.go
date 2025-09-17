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
	"context"
	"testing"

	"github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestGetExpectedPCSGFQNsForPCS validates generation of fully qualified names for all PCSGs in a PodCliqueSet.
func TestGetExpectedPCSGFQNsForPCS(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet specification
		pcs *grovecorev1alpha1.PodCliqueSet
		// Expected fully qualified names for all PCSGs
		expectedFQNs []string
	}{
		{
			// Should generate FQNs for all PCSG configs across all PCS replicas
			name: "generates FQNs for all PCSG configs across replicas",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{Name: "pcsg-a"},
							{Name: "pcsg-b"},
						},
					},
				},
			},
			expectedFQNs: []string{
				"test-pcs-0-pcsg-a",
				"test-pcs-0-pcsg-b",
				"test-pcs-1-pcsg-a",
				"test-pcs-1-pcsg-b",
			},
		},
		{
			// Should handle single replica with single PCSG
			name: "handles single replica with single PCSG",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "single-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{Name: "single-pcsg"},
						},
					},
				},
			},
			expectedFQNs: []string{
				"single-pcs-0-single-pcsg",
			},
		},
		{
			// Should handle empty PCSG configs
			name: "handles empty PCSG configs",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{},
					},
				},
			},
			expectedFQNs: []string{},
		},
		{
			// Should handle zero replicas
			name: "handles zero replicas",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "zero-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 0,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{Name: "pcsg-a"},
						},
					},
				},
			},
			expectedFQNs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExpectedPCSGFQNsForPCS(tt.pcs)
			assert.ElementsMatch(t, tt.expectedFQNs, result)
		})
	}
}

// TestGetPodCliqueFQNsForPCSNotInPCSG validates generation of FQNs for standalone PodCliques.
func TestGetPodCliqueFQNsForPCSNotInPCSG(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet specification
		pcs *grovecorev1alpha1.PodCliqueSet
		// Expected FQNs for standalone PodCliques
		expectedFQNs []string
	}{
		{
			// Should generate FQNs for cliques not in any PCSG
			name: "generates FQNs for standalone cliques",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "standalone-clique-a"},
							{Name: "standalone-clique-b"},
							{Name: "pcsg-clique"}, // This one is in PCSG
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "test-pcsg",
								CliqueNames: []string{"pcsg-clique"},
							},
						},
					},
				},
			},
			expectedFQNs: []string{
				"test-pcs-0-standalone-clique-a",
				"test-pcs-0-standalone-clique-b",
				"test-pcs-1-standalone-clique-a",
				"test-pcs-1-standalone-clique-b",
			},
		},
		{
			// Should handle all cliques being in PCSGs
			name: "handles all cliques in PCSGs",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "clique-a"},
							{Name: "clique-b"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "pcsg-1",
								CliqueNames: []string{"clique-a", "clique-b"},
							},
						},
					},
				},
			},
			expectedFQNs: []string{},
		},
		{
			// Should handle no cliques
			name: "handles no cliques",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{},
					},
				},
			},
			expectedFQNs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPodCliqueFQNsForPCSNotInPCSG(tt.pcs)
			assert.ElementsMatch(t, tt.expectedFQNs, result)
		})
	}
}

// TestGetPodCliqueFQNsForPCSReplicaNotInPCSG validates generation of FQNs for standalone PodCliques in a specific replica.
func TestGetPodCliqueFQNsForPCSReplicaNotInPCSG(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet specification
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueSet replica index
		pcsReplicaIndex int
		// Expected FQNs for standalone PodCliques in the replica
		expectedFQNs []string
	}{
		{
			// Should generate FQNs for standalone cliques in specific replica
			name: "generates FQNs for standalone cliques in replica",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "standalone-clique-a"},
							{Name: "standalone-clique-b"},
							{Name: "pcsg-clique"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "test-pcsg",
								CliqueNames: []string{"pcsg-clique"},
							},
						},
					},
				},
			},
			pcsReplicaIndex: 1,
			expectedFQNs: []string{
				"test-pcs-1-standalone-clique-a",
				"test-pcs-1-standalone-clique-b",
			},
		},
		{
			// Should handle replica index 0
			name: "handles replica index 0",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "standalone-clique"},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			expectedFQNs: []string{
				"test-pcs-0-standalone-clique",
			},
		},
		{
			// Should handle no standalone cliques
			name: "handles no standalone cliques",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "pcsg-clique"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "test-pcsg",
								CliqueNames: []string{"pcsg-clique"},
							},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			expectedFQNs:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPodCliqueFQNsForPCSReplicaNotInPCSG(tt.pcs, tt.pcsReplicaIndex)
			assert.ElementsMatch(t, tt.expectedFQNs, result)
		})
	}
}

// TestGetPodCliqueSet validates retrieval of PodCliqueSet using object metadata.
func TestGetPodCliqueSet(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Object metadata to extract PCS name from
		objectMeta metav1.ObjectMeta
		// Existing PodCliqueSets in the cluster
		existingPCSs []grovecorev1alpha1.PodCliqueSet
		// Expected PodCliqueSet name
		expectedPCSName string
		// Whether an error is expected
		wantErr bool
		// Whether NotFound error is expected
		expectNotFound bool
	}{
		{
			// Should retrieve PodCliqueSet using part-of label
			name: "retrieves PodCliqueSet using part-of label",
			objectMeta: metav1.ObjectMeta{
				Name:      "some-object",
				Namespace: "test-ns",
				Labels: map[string]string{
					common.LabelPartOfKey: "test-pcs",
				},
			},
			existingPCSs: []grovecorev1alpha1.PodCliqueSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pcs",
						Namespace: "test-ns",
					},
					Spec: grovecorev1alpha1.PodCliqueSetSpec{
						Replicas: 2,
					},
				},
			},
			expectedPCSName: "test-pcs",
			wantErr:         false,
		},
		{
			// Should return error for non-existent PodCliqueSet
			name: "returns error for non-existent PodCliqueSet",
			objectMeta: metav1.ObjectMeta{
				Name:      "some-object",
				Namespace: "test-ns",
				Labels: map[string]string{
					common.LabelPartOfKey: "nonexistent-pcs",
				},
			},
			existingPCSs:   []grovecorev1alpha1.PodCliqueSet{},
			wantErr:        true,
			expectNotFound: true,
		},
		{
			// Should handle missing part-of label
			name: "handles missing part-of label",
			objectMeta: metav1.ObjectMeta{
				Name:      "some-object",
				Namespace: "test-ns",
				Labels:    map[string]string{},
			},
			existingPCSs: []grovecorev1alpha1.PodCliqueSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "",
						Namespace: "test-ns",
					},
				},
			},
			expectedPCSName: "",
			wantErr:         false,
		},
		{
			// Should retrieve correct PCS when multiple exist
			name: "retrieves correct PCS among multiple",
			objectMeta: metav1.ObjectMeta{
				Name:      "some-object",
				Namespace: "test-ns",
				Labels: map[string]string{
					common.LabelPartOfKey: "target-pcs",
				},
			},
			existingPCSs: []grovecorev1alpha1.PodCliqueSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-pcs",
						Namespace: "test-ns",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "target-pcs",
						Namespace: "test-ns",
					},
					Spec: grovecorev1alpha1.PodCliqueSetSpec{
						Replicas: 3,
					},
				},
			},
			expectedPCSName: "target-pcs",
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, len(tt.existingPCSs))
			for i, pcs := range tt.existingPCSs {
				pcsCopy := pcs
				objects[i] = &pcsCopy
			}

			fakeClient := testutils.SetupFakeClient(objects...)
			ctx := context.Background()

			result, err := GetPodCliqueSet(ctx, fakeClient, tt.objectMeta)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectNotFound {
					assert.Contains(t, err.Error(), "not found")
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedPCSName, result.Name)
			assert.Equal(t, tt.objectMeta.Namespace, result.Namespace)
		})
	}
}

// TestGetPodCliqueSetName validates extraction of PodCliqueSet name from object metadata.
func TestGetPodCliqueSetName(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Object metadata to extract PCS name from
		objectMeta metav1.ObjectMeta
		// Expected PodCliqueSet name
		expectedPCSName string
	}{
		{
			// Should extract PCS name from part-of label
			name: "extracts PCS name from part-of label",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.LabelPartOfKey: "test-pcs",
				},
			},
			expectedPCSName: "test-pcs",
		},
		{
			// Should handle missing labels
			name: "handles missing labels",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
			},
			expectedPCSName: "",
		},
		{
			// Should handle empty labels map
			name: "handles empty labels map",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{},
			},
			expectedPCSName: "",
		},
		{
			// Should handle missing part-of label specifically
			name: "handles missing part-of label",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"other-label": "other-value",
				},
			},
			expectedPCSName: "",
		},
		{
			// Should handle empty part-of label value
			name: "handles empty part-of label value",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.LabelPartOfKey: "",
				},
			},
			expectedPCSName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPodCliqueSetName(tt.objectMeta)
			assert.Equal(t, tt.expectedPCSName, result)
		})
	}
}

// TestGetExpectedPCLQNamesGroupByOwner validates categorization of PodClique names by ownership model.
func TestGetExpectedPCLQNamesGroupByOwner(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet specification
		pcs *grovecorev1alpha1.PodCliqueSet
		// Expected standalone PodClique names (owned by PCS)
		expectedPCSNames []string
		// Expected PCSG-managed PodClique names
		expectedPCSGNames []string
	}{
		{
			// Should categorize cliques correctly by ownership
			name: "categorizes cliques by ownership",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "standalone-clique-a"},
							{Name: "standalone-clique-b"},
							{Name: "pcsg-clique-a"},
							{Name: "pcsg-clique-b"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "pcsg-1",
								CliqueNames: []string{"pcsg-clique-a", "pcsg-clique-b"},
							},
						},
					},
				},
			},
			expectedPCSNames:  []string{"standalone-clique-a", "standalone-clique-b"},
			expectedPCSGNames: []string{"pcsg-clique-a", "pcsg-clique-b"},
		},
		{
			// Should handle all cliques being standalone
			name: "handles all cliques being standalone",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "clique-a"},
							{Name: "clique-b"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{},
					},
				},
			},
			expectedPCSNames:  []string{"clique-a", "clique-b"},
			expectedPCSGNames: []string{},
		},
		{
			// Should handle all cliques being in PCSGs
			name: "handles all cliques being in PCSGs",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "clique-a"},
							{Name: "clique-b"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "pcsg-1",
								CliqueNames: []string{"clique-a"},
							},
							{
								Name:        "pcsg-2",
								CliqueNames: []string{"clique-b"},
							},
						},
					},
				},
			},
			expectedPCSNames:  []string{},
			expectedPCSGNames: []string{"clique-a", "clique-b"},
		},
		{
			// Should handle no cliques
			name: "handles no cliques",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{},
					},
				},
			},
			expectedPCSNames:  []string{},
			expectedPCSGNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcsNames, pcsgNames := GetExpectedPCLQNamesGroupByOwner(tt.pcs)

			assert.ElementsMatch(t, tt.expectedPCSNames, pcsNames)
			assert.ElementsMatch(t, tt.expectedPCSGNames, pcsgNames)
		})
	}
}

// TestGetExpectedPCSGFQNsPerPCSReplica validates generation of PCSG FQNs organized by replica index.
func TestGetExpectedPCSGFQNsPerPCSReplica(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet specification
		pcs *grovecorev1alpha1.PodCliqueSet
		// Expected PCSG FQNs organized by replica index
		expectedFQNsByReplica map[int][]string
	}{
		{
			// Should generate FQNs organized by replica index
			name: "generates FQNs organized by replica index",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{Name: "pcsg-a"},
							{Name: "pcsg-b"},
						},
					},
				},
			},
			expectedFQNsByReplica: map[int][]string{
				0: {"test-pcs-0-pcsg-a", "test-pcs-0-pcsg-b"},
				1: {"test-pcs-1-pcsg-a", "test-pcs-1-pcsg-b"},
			},
		},
		{
			// Should handle single replica
			name: "handles single replica",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "single-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 1,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{Name: "pcsg-a"},
						},
					},
				},
			},
			expectedFQNsByReplica: map[int][]string{
				0: {"single-pcs-0-pcsg-a"},
			},
		},
		{
			// Should handle empty PCSG configs
			name: "handles empty PCSG configs",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{},
					},
				},
			},
			expectedFQNsByReplica: map[int][]string{},
		},
		{
			// Should handle zero replicas
			name: "handles zero replicas",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "zero-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 0,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{Name: "pcsg-a"},
						},
					},
				},
			},
			expectedFQNsByReplica: map[int][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExpectedPCSGFQNsPerPCSReplica(tt.pcs)
			assert.Equal(t, tt.expectedFQNsByReplica, result)
		})
	}
}

// TestGetExpectedStandAlonePCLQFQNsPerPCSReplica validates generation of standalone PCLQ FQNs organized by replica index.
func TestGetExpectedStandAlonePCLQFQNsPerPCSReplica(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet specification
		pcs *grovecorev1alpha1.PodCliqueSet
		// Expected standalone PCLQ FQNs organized by replica index
		expectedFQNsByReplica map[int][]string
	}{
		{
			// Should generate standalone PCLQ FQNs organized by replica index
			name: "generates standalone PCLQ FQNs organized by replica index",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "standalone-clique-a"},
							{Name: "standalone-clique-b"},
							{Name: "pcsg-clique"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "test-pcsg",
								CliqueNames: []string{"pcsg-clique"},
							},
						},
					},
				},
			},
			expectedFQNsByReplica: map[int][]string{
				0: {"test-pcs-0-standalone-clique-a", "test-pcs-0-standalone-clique-b"},
				1: {"test-pcs-1-standalone-clique-a", "test-pcs-1-standalone-clique-b"},
			},
		},
		{
			// Should handle all cliques being in PCSGs
			name: "handles all cliques being in PCSGs",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 2,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "pcsg-clique"},
						},
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "test-pcsg",
								CliqueNames: []string{"pcsg-clique"},
							},
						},
					},
				},
			},
			expectedFQNsByReplica: map[int][]string{
				0: {},
				1: {},
			},
		},
		{
			// Should handle zero replicas
			name: "handles zero replicas",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Replicas: 0,
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "standalone-clique"},
						},
					},
				},
			},
			expectedFQNsByReplica: map[int][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetExpectedStandAlonePCLQFQNsPerPCSReplica(tt.pcs)
			assert.Equal(t, tt.expectedFQNsByReplica, result)
		})
	}
}
