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
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestGetPCLQsByOwner validates retrieval of PodCliques owned by a specific resource.
func TestGetPCLQsByOwner(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Owner kind (e.g., "PodCliqueScalingGroup")
		ownerKind string
		// Object key for the owner resource
		ownerObjectKey client.ObjectKey
		// Selector labels to match PodCliques
		selectorLabels map[string]string
		// Existing PodCliques in the cluster
		existingPCLQs []grovecorev1alpha1.PodClique
		// Expected number of PodCliques to be returned
		expectedCount int
		// Expected names of returned PodCliques
		expectedNames []string
		// Whether an error is expected
		wantErr bool
	}{
		{
			// Should return PodCliques owned by the specified resource with matching labels
			name:      "returns owned PodCliques with matching labels",
			ownerKind: "PodCliqueScalingGroup",
			ownerObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: "test-ns",
			},
			selectorLabels: map[string]string{
				apicommon.LabelPartOfKey: "test-pcs",
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "owned-pclq-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey: "test-pcs",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "PodCliqueScalingGroup",
								Name: "test-pcsg",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "owned-pclq-2",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey: "test-pcs",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "PodCliqueScalingGroup",
								Name: "test-pcsg",
							},
						},
					},
				},
			},
			expectedCount: 2,
			expectedNames: []string{"owned-pclq-1", "owned-pclq-2"},
		},
		{
			// Should exclude PodCliques with wrong owner
			name:      "excludes PodCliques with different owner",
			ownerKind: "PodCliqueScalingGroup",
			ownerObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: "test-ns",
			},
			selectorLabels: map[string]string{
				apicommon.LabelPartOfKey: "test-pcs",
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-owner-pclq",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey: "test-pcs",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "PodCliqueScalingGroup",
								Name: "other-pcsg",
							},
						},
					},
				},
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			// Should handle no owner references gracefully
			name:      "handles PodCliques without owner references",
			ownerKind: "PodCliqueScalingGroup",
			ownerObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: "test-ns",
			},
			selectorLabels: map[string]string{
				apicommon.LabelPartOfKey: "test-pcs",
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-owner-pclq",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey: "test-pcs",
						},
						// No owner references
					},
				},
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, len(tt.existingPCLQs))
			for i, pclq := range tt.existingPCLQs {
				pclqCopy := pclq
				objects[i] = &pclqCopy
			}

			fakeClient := testutils.SetupFakeClient(objects...)
			ctx := context.Background()

			result, err := GetPCLQsByOwner(ctx, fakeClient, tt.ownerKind, tt.ownerObjectKey, tt.selectorLabels)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)

			actualNames := make([]string, len(result))
			for i, pclq := range result {
				actualNames[i] = pclq.Name
			}
			assert.ElementsMatch(t, tt.expectedNames, actualNames)
		})
	}
}

// TestGroupPCLQsByPodGangName validates grouping of PodCliques by their PodGang label.
func TestGroupPCLQsByPodGangName(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Input PodCliques to group
		pclqs []grovecorev1alpha1.PodClique
		// Expected grouping by PodGang name
		expectedGroups map[string][]string // gang name -> pclq names
	}{
		{
			// Should group PodCliques by their PodGang label value
			name: "groups PodCliques by PodGang label",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq1",
						Labels: map[string]string{
							apicommon.LabelPodGang: "gang-a",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq2",
						Labels: map[string]string{
							apicommon.LabelPodGang: "gang-a",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq3",
						Labels: map[string]string{
							apicommon.LabelPodGang: "gang-b",
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"gang-a": {"pclq1", "pclq2"},
				"gang-b": {"pclq3"},
			},
		},
		{
			// Should exclude PodCliques without PodGang label
			name: "excludes PodCliques without PodGang label",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq1",
						Labels: map[string]string{
							apicommon.LabelPodGang: "gang-a",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "pclq2",
						Labels: map[string]string{
							// No PodGang label
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"gang-a": {"pclq1"},
			},
		},
		{
			// Should handle empty input gracefully
			name:           "handles empty input",
			pclqs:          []grovecorev1alpha1.PodClique{},
			expectedGroups: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupPCLQsByPodGangName(tt.pclqs)

			// Convert result to map[string][]string for easier comparison
			actualGroups := make(map[string][]string)
			for gangName, pclqs := range result {
				names := make([]string, len(pclqs))
				for i, pclq := range pclqs {
					names[i] = pclq.Name
				}
				actualGroups[gangName] = names
			}

			assert.Equal(t, tt.expectedGroups, actualGroups)
		})
	}
}

// TestGetMinAvailableBreachedPCLQInfo validates detection of PodCliques with MinAvailableBreached condition.
func TestGetMinAvailableBreachedPCLQInfo(t *testing.T) {
	baseTime := time.Now()
	terminationDelay := 30 * time.Second

	tests := []struct {
		// Test case description
		name string
		// Input PodCliques to check
		pclqs []grovecorev1alpha1.PodClique
		// Termination delay duration
		terminationDelay time.Duration
		// Reference time for calculating wait duration
		since time.Time
		// Expected candidate PodClique names
		expectedCandidates []string
		// Expected minimum wait duration
		expectedWaitDuration time.Duration
	}{
		{
			// Should identify PodCliques with MinAvailableBreached condition true
			name: "identifies PodCliques with MinAvailableBreached condition",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pclq1"},
					Status: grovecorev1alpha1.PodCliqueStatus{
						Conditions: []metav1.Condition{
							{
								Type:               constants.ConditionTypeMinAvailableBreached,
								Status:             metav1.ConditionTrue,
								LastTransitionTime: metav1.NewTime(baseTime.Add(-10 * time.Second)),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pclq2"},
					Status: grovecorev1alpha1.PodCliqueStatus{
						Conditions: []metav1.Condition{
							{
								Type:               constants.ConditionTypeMinAvailableBreached,
								Status:             metav1.ConditionTrue,
								LastTransitionTime: metav1.NewTime(baseTime.Add(-5 * time.Second)),
							},
						},
					},
				},
			},
			terminationDelay:     terminationDelay,
			since:                baseTime,
			expectedCandidates:   []string{"pclq1", "pclq2"},
			expectedWaitDuration: 20 * time.Second, // shortest wait (30s - 10s)
		},
		{
			// Should exclude PodCliques with MinAvailableBreached condition false
			name: "excludes PodCliques with condition false",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pclq1"},
					Status: grovecorev1alpha1.PodCliqueStatus{
						Conditions: []metav1.Condition{
							{
								Type:               constants.ConditionTypeMinAvailableBreached,
								Status:             metav1.ConditionFalse,
								LastTransitionTime: metav1.NewTime(baseTime.Add(-10 * time.Second)),
							},
						},
					},
				},
			},
			terminationDelay:     terminationDelay,
			since:                baseTime,
			expectedCandidates:   nil,
			expectedWaitDuration: 0,
		},
		{
			// Should handle PodCliques without the condition
			name: "handles PodCliques without MinAvailableBreached condition",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "pclq1"},
					Status: grovecorev1alpha1.PodCliqueStatus{
						Conditions: []metav1.Condition{
							// Different condition type
							{
								Type:   "SomeOtherCondition",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			terminationDelay:     terminationDelay,
			since:                baseTime,
			expectedCandidates:   nil,
			expectedWaitDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates, waitDuration := GetMinAvailableBreachedPCLQInfo(tt.pclqs, tt.terminationDelay, tt.since)

			assert.ElementsMatch(t, tt.expectedCandidates, candidates)
			assert.Equal(t, tt.expectedWaitDuration, waitDuration)
		})
	}
}

// TestComputePCLQPodTemplateHash validates hash computation for PodClique templates.
func TestComputePCLQPodTemplateHash(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodClique template specification
		pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec
		// Priority class name to include in hash
		priorityClassName string
		// Whether the hash should be consistent for same input
		shouldBeConsistent bool
	}{
		{
			// Should generate consistent hash for identical template specs
			name: "generates consistent hash for identical specs",
			pclqTemplateSpec: &grovecorev1alpha1.PodCliqueTemplateSpec{
				Name: "test-clique",
				Labels: map[string]string{
					"test-label": "test-value",
				},
				Annotations: map[string]string{
					"test-annotation": "test-value",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image:latest",
							},
						},
					},
				},
			},
			priorityClassName:  "high-priority",
			shouldBeConsistent: true,
		},
		{
			// Should handle empty template spec
			name: "handles empty template spec",
			pclqTemplateSpec: &grovecorev1alpha1.PodCliqueTemplateSpec{
				Name: "empty-clique",
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{},
				},
			},
			priorityClassName:  "",
			shouldBeConsistent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := ComputePCLQPodTemplateHash(tt.pclqTemplateSpec, tt.priorityClassName)
			hash2 := ComputePCLQPodTemplateHash(tt.pclqTemplateSpec, tt.priorityClassName)

			// Hash should not be empty
			assert.NotEmpty(t, hash1)

			if tt.shouldBeConsistent {
				// Hash should be consistent for same input
				assert.Equal(t, hash1, hash2)
			}

			// Hash should be different with different priority class
			if tt.priorityClassName != "" {
				hash3 := ComputePCLQPodTemplateHash(tt.pclqTemplateSpec, "different-priority")
				assert.NotEqual(t, hash1, hash3)
			}
		})
	}
}

// TestIsPCLQUpdateInProgress validates detection of PodClique rolling update status.
func TestIsPCLQUpdateInProgress(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodClique object to check
		pclq *grovecorev1alpha1.PodClique
		// Expected result
		expectedInProgress bool
	}{
		{
			// Should return true when update is in progress
			name: "returns true when update is in progress",
			pclq: &grovecorev1alpha1.PodClique{
				Status: grovecorev1alpha1.PodCliqueStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueRollingUpdateProgress{
						UpdateStartedAt: metav1.Time{Time: time.Now()},
						UpdateEndedAt:   nil, // Update in progress
					},
				},
			},
			expectedInProgress: true,
		},
		{
			// Should return false when update is completed
			name: "returns false when update is completed",
			pclq: &grovecorev1alpha1.PodClique{
				Status: grovecorev1alpha1.PodCliqueStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueRollingUpdateProgress{
						UpdateStartedAt: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
						UpdateEndedAt:   &metav1.Time{Time: time.Now()},
					},
				},
			},
			expectedInProgress: false,
		},
		{
			// Should return false when no rolling update progress
			name: "returns false when no rolling update progress",
			pclq: &grovecorev1alpha1.PodClique{
				Status: grovecorev1alpha1.PodCliqueStatus{
					RollingUpdateProgress: nil,
				},
			},
			expectedInProgress: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPCLQUpdateInProgress(tt.pclq)
			assert.Equal(t, tt.expectedInProgress, result)
		})
	}
}

// TestIsLastPCLQUpdateCompleted validates detection of completed PodClique rolling update.
func TestIsLastPCLQUpdateCompleted(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodClique object to check
		pclq *grovecorev1alpha1.PodClique
		// Expected result
		expectedCompleted bool
	}{
		{
			// Should return true when last update is completed
			name: "returns true when last update is completed",
			pclq: &grovecorev1alpha1.PodClique{
				Status: grovecorev1alpha1.PodCliqueStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueRollingUpdateProgress{
						UpdateStartedAt: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
						UpdateEndedAt:   &metav1.Time{Time: time.Now()},
					},
				},
			},
			expectedCompleted: true,
		},
		{
			// Should return false when update is in progress
			name: "returns false when update is in progress",
			pclq: &grovecorev1alpha1.PodClique{
				Status: grovecorev1alpha1.PodCliqueStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueRollingUpdateProgress{
						UpdateStartedAt: metav1.Time{Time: time.Now()},
						UpdateEndedAt:   nil,
					},
				},
			},
			expectedCompleted: false,
		},
		{
			// Should return false when no rolling update progress
			name: "returns false when no rolling update progress",
			pclq: &grovecorev1alpha1.PodClique{
				Status: grovecorev1alpha1.PodCliqueStatus{
					RollingUpdateProgress: nil,
				},
			},
			expectedCompleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLastPCLQUpdateCompleted(tt.pclq)
			assert.Equal(t, tt.expectedCompleted, result)
		})
	}
}

// TestGetPCLQsByOwnerReplicaIndex validates retrieval and grouping of PodCliques by replica index.
func TestGetPCLQsByOwnerReplicaIndex(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Owner kind (e.g., "PodCliqueScalingGroup")
		ownerKind string
		// Object key for the owner resource
		ownerObjectKey client.ObjectKey
		// Selector labels to match PodCliques
		selectorLabels map[string]string
		// Existing PodCliques in the cluster
		existingPCLQs []grovecorev1alpha1.PodClique
		// Expected grouping by replica index
		expectedGroups map[string][]string // replica index -> pclq names
		// Whether an error is expected
		wantErr bool
	}{
		{
			// Should group PodCliques by replica index for specific owner
			name:      "groups PodCliques by replica index for owner",
			ownerKind: "PodCliqueScalingGroup",
			ownerObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: "test-ns",
			},
			selectorLabels: map[string]string{
				apicommon.LabelPartOfKey: "test-pcs",
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pclq-replica-0",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "PodCliqueScalingGroup",
								Name: "test-pcsg",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pclq-replica-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelPodCliqueSetReplicaIndex: "1",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "PodCliqueScalingGroup",
								Name: "test-pcsg",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pclq-replica-0-2",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "PodCliqueScalingGroup",
								Name: "test-pcsg",
							},
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pclq-replica-0", "pclq-replica-0-2"},
				"1": {"pclq-replica-1"},
			},
		},
		{
			// Should handle empty result gracefully
			name:      "handles no matching PodCliques",
			ownerKind: "PodCliqueScalingGroup",
			ownerObjectKey: client.ObjectKey{
				Name:      "nonexistent-pcsg",
				Namespace: "test-ns",
			},
			selectorLabels: map[string]string{
				apicommon.LabelPartOfKey: "test-pcs",
			},
			existingPCLQs:  []grovecorev1alpha1.PodClique{},
			expectedGroups: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, len(tt.existingPCLQs))
			for i, pclq := range tt.existingPCLQs {
				pclqCopy := pclq
				objects[i] = &pclqCopy
			}

			fakeClient := testutils.SetupFakeClient(objects...)
			ctx := context.Background()

			result, err := GetPCLQsByOwnerReplicaIndex(ctx, fakeClient, tt.ownerKind, tt.ownerObjectKey, tt.selectorLabels)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Convert result to map[string][]string for easier comparison
			actualGroups := make(map[string][]string)
			for replicaIndex, pclqs := range result {
				names := make([]string, len(pclqs))
				for i, pclq := range pclqs {
					names[i] = pclq.Name
				}
				actualGroups[replicaIndex] = names
			}

			assert.Equal(t, tt.expectedGroups, actualGroups)
		})
	}
}

// TestGroupPCLQsByPCSGReplicaIndex validates grouping of PodCliques by PodCliqueScalingGroup replica index.
func TestGroupPCLQsByPCSGReplicaIndex(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Input PodCliques to group
		pclqs []grovecorev1alpha1.PodClique
		// Expected grouping by PCSG replica index
		expectedGroups map[string][]string // replica index -> pclq names
	}{
		{
			// Should group PodCliques by their PCSG replica index label
			name: "groups PodCliques by PCSG replica index",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq1",
						Labels: map[string]string{
							apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq2",
						Labels: map[string]string{
							apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq3",
						Labels: map[string]string{
							apicommon.LabelPodCliqueScalingGroupReplicaIndex: "1",
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pclq1", "pclq2"},
				"1": {"pclq3"},
			},
		},
		{
			// Should exclude PodCliques without PCSG replica index label
			name: "excludes PodCliques without PCSG replica index label",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq1",
						Labels: map[string]string{
							apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "pclq2",
						Labels: map[string]string{
							// No PCSG replica index label
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pclq1"},
			},
		},
		{
			// Should handle empty input gracefully
			name:           "handles empty input",
			pclqs:          []grovecorev1alpha1.PodClique{},
			expectedGroups: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupPCLQsByPCSGReplicaIndex(tt.pclqs)

			// Convert result to map[string][]string for easier comparison
			actualGroups := make(map[string][]string)
			for replicaIndex, pclqs := range result {
				names := make([]string, len(pclqs))
				for i, pclq := range pclqs {
					names[i] = pclq.Name
				}
				actualGroups[replicaIndex] = names
			}

			assert.Equal(t, tt.expectedGroups, actualGroups)
		})
	}
}

// TestGroupPCLQsByPCSReplicaIndex validates grouping of PodCliques by PodCliqueSet replica index.
func TestGroupPCLQsByPCSReplicaIndex(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Input PodCliques to group
		pclqs []grovecorev1alpha1.PodClique
		// Expected grouping by PCS replica index
		expectedGroups map[string][]string // replica index -> pclq names
	}{
		{
			// Should group PodCliques by their PCS replica index label
			name: "groups PodCliques by PCS replica index",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq1",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq2",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq3",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "1",
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pclq1", "pclq2"},
				"1": {"pclq3"},
			},
		},
		{
			// Should exclude PodCliques without PCS replica index label
			name: "excludes PodCliques without PCS replica index label",
			pclqs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pclq1",
						Labels: map[string]string{
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "pclq2",
						Labels: map[string]string{
							// No PCS replica index label
						},
					},
				},
			},
			expectedGroups: map[string][]string{
				"0": {"pclq1"},
			},
		},
		{
			// Should handle empty input gracefully
			name:           "handles empty input",
			pclqs:          []grovecorev1alpha1.PodClique{},
			expectedGroups: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GroupPCLQsByPCSReplicaIndex(tt.pclqs)

			// Convert result to map[string][]string for easier comparison
			actualGroups := make(map[string][]string)
			for replicaIndex, pclqs := range result {
				names := make([]string, len(pclqs))
				for i, pclq := range pclqs {
					names[i] = pclq.Name
				}
				actualGroups[replicaIndex] = names
			}

			assert.Equal(t, tt.expectedGroups, actualGroups)
		})
	}
}

// TestGetPodCliquesWithParentPCS validates retrieval of standalone PodCliques owned by a PodCliqueSet.
func TestGetPodCliquesWithParentPCS(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet object key
		pcsObjKey client.ObjectKey
		// Existing PodCliques in the cluster
		existingPCLQs []grovecorev1alpha1.PodClique
		// Expected number of PodCliques to be returned
		expectedCount int
		// Expected names of returned PodCliques
		expectedNames []string
		// Whether an error is expected
		wantErr bool
	}{
		{
			// Should return standalone PodCliques with correct labels
			name: "returns standalone PodCliques with matching labels",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "standalone-pclq-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
							apicommon.LabelComponentKey: apicommon.LabelComponentNamePodCliqueSetPodClique,
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "standalone-pclq-2",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
							apicommon.LabelComponentKey: apicommon.LabelComponentNamePodCliqueSetPodClique,
						},
					},
				},
			},
			expectedCount: 2,
			expectedNames: []string{"standalone-pclq-1", "standalone-pclq-2"},
		},
		{
			// Should exclude PodCliques with different component label
			name: "excludes PodCliques with different component label",
			pcsObjKey: client.ObjectKey{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pcsg-pclq",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
							apicommon.LabelComponentKey: "different-component",
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
			existingPCLQs: []grovecorev1alpha1.PodClique{},
			expectedCount: 0,
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]client.Object, len(tt.existingPCLQs))
			for i, pclq := range tt.existingPCLQs {
				pclqCopy := pclq
				objects[i] = &pclqCopy
			}

			fakeClient := testutils.SetupFakeClient(objects...)
			ctx := context.Background()

			result, err := GetPodCliquesWithParentPCS(ctx, fakeClient, tt.pcsObjKey)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)

			actualNames := make([]string, len(result))
			for i, pclq := range result {
				actualNames[i] = pclq.Name
			}
			assert.ElementsMatch(t, tt.expectedNames, actualNames)
		})
	}
}

// TestGetExpectedPCLQPodTemplateHash validates computation of expected pod template hash for PodClique.
func TestGetExpectedPCLQPodTemplateHash(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet containing template specifications
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodClique object metadata to extract clique name from
		pclqObjectMeta metav1.ObjectMeta
		// Expected hash value (if deterministic)
		expectValidHash bool
		// Whether an error is expected
		wantErr bool
		// Expected error message if error expected
		expectedErrMsg string
	}{
		{
			// Should compute hash for existing clique template
			name: "computes hash for existing clique template",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PriorityClassName: "high-priority",
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "test-clique",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									PodSpec: corev1.PodSpec{
										Containers: []corev1.Container{
											{Name: "test-container", Image: "test:latest"},
										},
									},
								},
							},
						},
					},
				},
			},
			pclqObjectMeta: metav1.ObjectMeta{
				Name: "test-pcs-0-test-clique",
				Labels: map[string]string{
					apicommon.LabelPartOfKey:                "test-pcs",
					apicommon.LabelPodCliqueSetReplicaIndex: "0",
				},
			},
			expectValidHash: true,
		},
		{
			// Should return error for non-existent clique template
			name: "returns error for non-existent clique template",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
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
			pclqObjectMeta: metav1.ObjectMeta{
				Name: "test-pcs-0-nonexistent-clique",
				Labels: map[string]string{
					apicommon.LabelPartOfKey:                "test-pcs",
					apicommon.LabelPodCliqueSetReplicaIndex: "0",
				},
			},
			wantErr:        true,
			expectedErrMsg: "pod clique template not found for cliqueName: nonexistent-clique",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := GetExpectedPCLQPodTemplateHash(tt.pcs, tt.pclqObjectMeta)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)

			if tt.expectValidHash {
				assert.NotEmpty(t, hash)
			}
		})
	}
}
