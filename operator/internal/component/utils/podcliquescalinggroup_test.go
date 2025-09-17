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

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestFindScalingGroupConfigForClique validates finding scaling group config that contains a specific clique.
func TestFindScalingGroupConfigForClique(t *testing.T) {
	// Create test scaling group configurations
	scalingGroupConfigs := []grovecorev1alpha1.PodCliqueScalingGroupConfig{
		{
			Name:        "sga",
			CliqueNames: []string{"pca", "pcb"},
		},
		{
			Name:        "sgb",
			CliqueNames: []string{"pcc", "pcd", "pce"},
		},
		{
			Name:        "sgc",
			CliqueNames: []string{"pcf"},
		},
	}

	tests := []struct {
		// Test case description
		name string
		// Scaling group configurations to search in
		configs []grovecorev1alpha1.PodCliqueScalingGroupConfig
		// Clique name to find
		cliqueName string
		// Whether the clique should be found
		expectedFound bool
		// Expected scaling group config name if found
		expectedConfigName string
	}{
		{
			// Should find clique in the first scaling group
			name:               "clique found in first scaling group",
			configs:            scalingGroupConfigs,
			cliqueName:         "pca",
			expectedFound:      true,
			expectedConfigName: "sga",
		},
		{
			// Should find clique in the second scaling group
			name:               "clique found in second scaling group",
			configs:            scalingGroupConfigs,
			cliqueName:         "pcd",
			expectedFound:      true,
			expectedConfigName: "sgb",
		},
		{
			// Should find clique in the third scaling group
			name:               "clique found in third scaling group",
			configs:            scalingGroupConfigs,
			cliqueName:         "pcf",
			expectedFound:      true,
			expectedConfigName: "sgc",
		},
		{
			// Should return nil for non-existent clique
			name:               "clique not found in any scaling group",
			configs:            scalingGroupConfigs,
			cliqueName:         "nonexistent",
			expectedFound:      false,
			expectedConfigName: "",
		},
		{
			// Should handle empty clique name gracefully
			name:               "empty clique name",
			configs:            scalingGroupConfigs,
			cliqueName:         "",
			expectedFound:      false,
			expectedConfigName: "",
		},
		{
			// Should handle empty configs gracefully
			name:               "empty configs",
			configs:            []grovecorev1alpha1.PodCliqueScalingGroupConfig{},
			cliqueName:         "anyClique",
			expectedFound:      false,
			expectedConfigName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := FindScalingGroupConfigForClique(tt.configs, tt.cliqueName)
			assert.Equal(t, tt.expectedFound, config != nil)
			if tt.expectedFound {
				assert.Equal(t, tt.expectedConfigName, config.Name)
			} else {
				// When not found, config should be nil
				assert.Nil(t, config)
			}
		})
	}
}

// TestGenerateDependencyNamesForBasePodGang validates generation of dependency names for PodGang.
func TestGenerateDependencyNamesForBasePodGang(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet specification
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueSet replica index
		pcsReplicaIndex int
		// Parent clique name
		parentCliqueName string
		// Expected dependency names
		expectedNames []string
	}{
		{
			// Should generate names for clique in scaling group based on MinAvailable
			name: "generates names for clique in scaling group",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:         "test-pcsg",
								CliqueNames:  []string{"parent-clique"},
								MinAvailable: ptr.To(int32(2)),
							},
						},
					},
				},
			},
			pcsReplicaIndex:  0,
			parentCliqueName: "parent-clique",
			expectedNames: []string{
				"test-pcs-0-test-pcsg-0-parent-clique",
				"test-pcs-0-test-pcsg-1-parent-clique",
			},
		},
		{
			// Should generate single name for standalone clique
			name: "generates name for standalone clique",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{},
					},
				},
			},
			pcsReplicaIndex:  1,
			parentCliqueName: "standalone-clique",
			expectedNames: []string{
				"test-pcs-1-standalone-clique",
			},
		},
		{
			// Should handle clique not in any scaling group
			name: "handles clique not in scaling group configs",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
							{
								Name:        "other-pcsg",
								CliqueNames: []string{"other-clique"},
							},
						},
					},
				},
			},
			pcsReplicaIndex:  0,
			parentCliqueName: "unrelated-clique",
			expectedNames: []string{
				"test-pcs-0-unrelated-clique",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDependencyNamesForBasePodGang(tt.pcs, tt.pcsReplicaIndex, tt.parentCliqueName)
			assert.ElementsMatch(t, tt.expectedNames, result)
		})
	}
}

// TestGroupPCSGsByPCSReplicaIndex validates grouping of PodCliqueScalingGroups by replica index.
func TestGroupPCSGsByPCSReplicaIndex(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Input PodCliqueScalingGroups to group
		pcsgs []grovecorev1alpha1.PodCliqueScalingGroup
		// Expected grouping by replica index
		expectedGroups map[string][]string // replica index -> pcsg names
	}{
		{
			// Should group PCSGs by their PodCliqueSet replica index label
			name: "groups PCSGs by replica index",
			pcsgs: []grovecorev1alpha1.PodCliqueScalingGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pcsg1",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pcsg2",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pcsg3",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "1",
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pcsg1", "pcsg2"},
				"1": {"pcsg3"},
			},
		},
		{
			// Should exclude PCSGs without replica index label
			name: "excludes PCSGs without replica index label",
			pcsgs: []grovecorev1alpha1.PodCliqueScalingGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pcsg1",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "pcsg2",
						Labels: map[string]string{
							// No replica index label
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pcsg1"},
			},
		},
		{
			// Should handle empty input gracefully
			name:           "handles empty input",
			pcsgs:          []grovecorev1alpha1.PodCliqueScalingGroup{},
			expectedGroups: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupPCSGsByPCSReplicaIndex(tt.pcsgs)

			// Convert result to map[string][]string for easier comparison
			actualGroups := make(map[string][]string)
			for replicaIndex, pcsgs := range result {
				names := make([]string, len(pcsgs))
				for i, pcsg := range pcsgs {
					names[i] = pcsg.Name
				}
				actualGroups[replicaIndex] = names
			}

			assert.Equal(t, tt.expectedGroups, actualGroups)
		})
	}
}

// TestGetPCLQTemplateHashes validates generation of pod template hashes for PodCliques in a scaling group.
func TestGetPCLQTemplateHashes(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet with template specifications
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup to generate hashes for
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected number of hash entries
		expectedHashCount int
		// Expected PodClique FQNs that should have hashes
		expectedFQNs []string
	}{
		{
			// Should generate hashes for all PodClique replicas in scaling group
			name: "generates hashes for all PodClique replicas",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PriorityClassName: "high-priority",
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "clique-a",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									PodSpec: corev1.PodSpec{
										Containers: []corev1.Container{
											{Name: "container1", Image: "image1:latest"},
										},
									},
								},
							},
							{
								Name: "clique-b",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									PodSpec: corev1.PodSpec{
										Containers: []corev1.Container{
											{Name: "container2", Image: "image2:latest"},
										},
									},
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    2,
					CliqueNames: []string{"clique-a", "clique-b"},
				},
			},
			expectedHashCount: 4, // 2 replicas × 2 cliques
			expectedFQNs: []string{
				"test-pcsg-0-clique-a",
				"test-pcsg-0-clique-b",
				"test-pcsg-1-clique-a",
				"test-pcsg-1-clique-b",
			},
		},
		{
			// Should handle clique not found in template specifications
			name: "handles clique not found in template specs",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "existing-clique",
								Spec: grovecorev1alpha1.PodCliqueSpec{},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"nonexistent-clique"},
				},
			},
			expectedHashCount: 0,
			expectedFQNs:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPCLQTemplateHashes(tt.pcs, tt.pcsg)

			assert.Len(t, result, tt.expectedHashCount)

			// Check that expected FQNs are present
			for _, expectedFQN := range tt.expectedFQNs {
				hash, exists := result[expectedFQN]
				assert.True(t, exists, "Expected FQN %s not found in result", expectedFQN)
				assert.NotEmpty(t, hash, "Hash for FQN %s should not be empty", expectedFQN)
			}
		})
	}
}

// TestIsPCSGUpdateInProgress validates detection of PodCliqueScalingGroup rolling update status.
func TestIsPCSGUpdateInProgress(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueScalingGroup object to check
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected result
		expectedInProgress bool
	}{
		{
			// Should return true when update is in progress
			name: "returns true when update is in progress",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						UpdateStartedAt: metav1.Time{},
						UpdateEndedAt:   nil, // Update in progress
					},
				},
			},
			expectedInProgress: true,
		},
		{
			// Should return false when update is completed
			name: "returns false when update is completed",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						UpdateStartedAt: metav1.Time{},
						UpdateEndedAt:   &metav1.Time{},
					},
				},
			},
			expectedInProgress: false,
		},
		{
			// Should return false when no rolling update progress
			name: "returns false when no rolling update progress",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: nil,
				},
			},
			expectedInProgress: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPCSGUpdateInProgress(tt.pcsg)
			assert.Equal(t, tt.expectedInProgress, result)
		})
	}
}

// TestIsPCSGUpdateComplete validates detection of completed PodCliqueScalingGroup rolling update.
func TestIsPCSGUpdateComplete(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueScalingGroup object to check
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected PodCliqueSet generation hash
		pcsGenerationHash string
		// Expected result
		expectedComplete bool
	}{
		{
			// Should return true when generation hashes match
			name: "returns true when generation hashes match",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					CurrentPodCliqueSetGenerationHash: ptr.To("hash123"),
				},
			},
			pcsGenerationHash: "hash123",
			expectedComplete:  true,
		},
		{
			// Should return false when generation hashes don't match
			name: "returns false when generation hashes don't match",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					CurrentPodCliqueSetGenerationHash: ptr.To("hash123"),
				},
			},
			pcsGenerationHash: "hash456",
			expectedComplete:  false,
		},
		{
			// Should return false when current generation hash is nil
			name: "returns false when current generation hash is nil",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					CurrentPodCliqueSetGenerationHash: nil,
				},
			},
			pcsGenerationHash: "hash123",
			expectedComplete:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPCSGUpdateComplete(tt.pcsg, tt.pcsGenerationHash)
			assert.Equal(t, tt.expectedComplete, result)
		})
	}
}

// TestGetPodCliqueFQNsForPCSG validates generation of fully qualified names for PodCliques in a scaling group.
func TestGetPodCliqueFQNsForPCSG(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueScalingGroup to generate FQNs for
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected fully qualified names
		expectedFQNs []string
	}{
		{
			// Should generate FQNs for all replicas and cliques
			name: "generates FQNs for all replicas and cliques",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    2,
					CliqueNames: []string{"clique-a", "clique-b"},
				},
			},
			expectedFQNs: []string{
				"test-pcsg-0-clique-a",
				"test-pcsg-0-clique-b",
				"test-pcsg-1-clique-a",
				"test-pcsg-1-clique-b",
			},
		},
		{
			// Should handle single replica with single clique
			name: "handles single replica with single clique",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "single-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"single-clique"},
				},
			},
			expectedFQNs: []string{
				"single-pcsg-0-single-clique",
			},
		},
		{
			// Should handle zero replicas gracefully
			name: "handles zero replicas",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "zero-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    0,
					CliqueNames: []string{"clique-a"},
				},
			},
			expectedFQNs: []string{},
		},
		{
			// Should handle empty clique names gracefully
			name: "handles empty clique names",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-cliques-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    2,
					CliqueNames: []string{},
				},
			},
			expectedFQNs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPodCliqueFQNsForPCSG(tt.pcsg)
			assert.ElementsMatch(t, tt.expectedFQNs, result)
		})
	}
}

// TestGetPCSGsForPCS validates retrieval of PodCliqueScalingGroups owned by a PodCliqueSet.
func TestGetPCSGsForPCS(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet object key
		pcsObjKey client.ObjectKey
		// Existing PodCliqueScalingGroups in the cluster
		existingPCSGs []grovecorev1alpha1.PodCliqueScalingGroup
		// Expected number of PCSGs to be returned
		expectedCount int
		// Expected names of returned PCSGs
		expectedNames []string
		// Whether an error is expected
		wantErr bool
	}{
		{
			// Should return PCSGs with matching labels
			name: "returns PCSGs with matching labels",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCSGs: []grovecorev1alpha1.PodCliqueScalingGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "matching-pcsg-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "matching-pcsg-2",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
						},
					},
				},
			},
			expectedCount: 2,
			expectedNames: []string{"matching-pcsg-1", "matching-pcsg-2"},
		},
		{
			// Should exclude PCSGs with different labels
			name: "excludes PCSGs with different labels",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCSGs: []grovecorev1alpha1.PodCliqueScalingGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-label-pcsg",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "other-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
						},
					},
				},
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			// Should handle empty list gracefully
			name: "handles empty list",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCSGs: []grovecorev1alpha1.PodCliqueScalingGroup{},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, len(tt.existingPCSGs))
			for i, pcsg := range tt.existingPCSGs {
				pcsgCopy := pcsg
				objects[i] = &pcsgCopy
			}

			fakeClient := testutils.SetupFakeClient(objects...)
			ctx := context.Background()

			result, err := GetPCSGsForPCS(ctx, fakeClient, tt.pcsObjKey)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)

			actualNames := make([]string, len(result))
			for i, pcsg := range result {
				actualNames[i] = pcsg.Name
			}
			assert.ElementsMatch(t, tt.expectedNames, actualNames)
		})
	}
}

// TestGetPCSGsByPCSReplicaIndex validates retrieval and grouping of PodCliqueScalingGroups by replica index.
func TestGetPCSGsByPCSReplicaIndex(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet object key
		pcsObjKey client.ObjectKey
		// Existing PodCliqueScalingGroups in the cluster
		existingPCSGs []grovecorev1alpha1.PodCliqueScalingGroup
		// Expected grouping by replica index
		expectedGroups map[string][]string // replica index -> pcsg names
		// Whether an error is expected
		wantErr bool
	}{
		{
			// Should group PCSGs by their PCS replica index label
			name: "groups PCSGs by PCS replica index",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCSGs: []grovecorev1alpha1.PodCliqueScalingGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pcsg-replica-0-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelManagedByKey:             "grove-operator",
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pcsg-replica-0-2",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelManagedByKey:             "grove-operator",
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pcsg-replica-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelManagedByKey:             "grove-operator",
							apicommon.LabelPodCliqueSetReplicaIndex: "1",
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pcsg-replica-0-1", "pcsg-replica-0-2"},
				"1": {"pcsg-replica-1"},
			},
		},
		{
			// Should exclude PCSGs without replica index label
			name: "excludes PCSGs without replica index label",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCSGs: []grovecorev1alpha1.PodCliqueScalingGroup{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pcsg-with-label",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelManagedByKey:             "grove-operator",
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pcsg-without-label",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
							// No replica index label
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pcsg-with-label"},
			},
		},
		{
			// Should handle empty list gracefully
			name: "handles empty list",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCSGs:  []grovecorev1alpha1.PodCliqueScalingGroup{},
			expectedGroups: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, len(tt.existingPCSGs))
			for i, pcsg := range tt.existingPCSGs {
				pcsgCopy := pcsg
				objects[i] = &pcsgCopy
			}

			fakeClient := testutils.SetupFakeClient(objects...)
			ctx := context.Background()

			result, err := GetPCSGsByPCSReplicaIndex(ctx, fakeClient, tt.pcsObjKey)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Convert result to map[string][]string for easier comparison
			actualGroups := make(map[string][]string)
			for replicaIndex, pcsgs := range result {
				names := make([]string, len(pcsgs))
				for i, pcsg := range pcsgs {
					names[i] = pcsg.Name
				}
				actualGroups[replicaIndex] = names
			}

			assert.Equal(t, tt.expectedGroups, actualGroups)
		})
	}
}

// TestGetPCLQsInPCSGPendingUpdate validates identification of PodCliques requiring updates.
func TestGetPCLQsInPCSGPendingUpdate(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet with template specifications
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup to check for updates
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Existing PodCliques to compare against expected hashes
		existingPCLQs []grovecorev1alpha1.PodClique
		// Expected number of PodCliques pending update
		expectedPendingCount int
	}{
		{
			// Should identify PodCliques with outdated template hashes
			name: "identifies PodCliques with outdated template hashes",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PriorityClassName: "high-priority",
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "clique-a",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									PodSpec: corev1.PodSpec{
										Containers: []corev1.Container{
											{Name: "container1", Image: "image1:v2"}, // Updated image
										},
									},
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"clique-a"},
				},
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pcsg-0-clique-a",
						Labels: map[string]string{
							apicommon.LabelPodTemplateHash: "old-hash-123", // Outdated hash
						},
					},
				},
			},
			expectedPendingCount: 1,
		},
		{
			// Should not identify PodCliques with current template hashes
			name: "excludes PodCliques with current template hashes",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PriorityClassName: "high-priority",
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "clique-a",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									PodSpec: corev1.PodSpec{
										Containers: []corev1.Container{
											{Name: "container1", Image: "image1:latest"},
										},
									},
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"clique-a"},
				},
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pcsg-0-clique-a",
						Labels: map[string]string{
							// Hash will match the computed one since template is the same
							apicommon.LabelPodTemplateHash: ComputePCLQPodTemplateHash(&grovecorev1alpha1.PodCliqueTemplateSpec{
								Name: "clique-a",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									PodSpec: corev1.PodSpec{
										Containers: []corev1.Container{
											{Name: "container1", Image: "image1:latest"},
										},
									},
								},
							}, "high-priority"),
						},
					},
				},
			},
			expectedPendingCount: 0,
		},
		{
			// Should handle empty existing PodCliques list
			name: "handles empty existing PodCliques list",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "clique-a",
								Spec: grovecorev1alpha1.PodCliqueSpec{},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"clique-a"},
				},
			},
			existingPCLQs:        []grovecorev1alpha1.PodClique{},
			expectedPendingCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPCLQsInPCSGPendingUpdate(tt.pcs, tt.pcsg, tt.existingPCLQs)
			assert.Len(t, result, tt.expectedPendingCount)
		})
	}
}
