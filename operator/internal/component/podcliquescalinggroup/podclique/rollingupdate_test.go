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

package podclique

import (
	"testing"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// TestIsReplicaUpdated verifies that the function correctly determines whether
// a PCSG replica has been updated to match expected pod template hashes.
func TestIsReplicaUpdated(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Expected pod template hashes for PodCliques
		expectedHashes map[string]string
		// PodCliques belonging to the replica being checked
		replicaPCLQs []grovecorev1alpha1.PodClique
		// Whether the replica should be considered updated
		expectedUpdated bool
		// Whether an error is expected
		expectError bool
	}{
		{
			// No PodCliques means replica is considered updated
			name:            "no podcliques is updated",
			expectedHashes:  map[string]string{},
			replicaPCLQs:    []grovecorev1alpha1.PodClique{},
			expectedUpdated: true,
			expectError:     false,
		},
		{
			// Single PodClique with matching hash is updated
			name: "single podclique with matching hash",
			expectedHashes: map[string]string{
				"pcsg-0-worker": "hash123",
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithHash("pcsg-0-worker", "hash123"),
			},
			expectedUpdated: true,
			expectError:     false,
		},
		{
			// Single PodClique with different hash is not updated
			name: "single podclique with different hash",
			expectedHashes: map[string]string{
				"pcsg-0-worker": "hash123",
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithHash("pcsg-0-worker", "hash456"),
			},
			expectedUpdated: false,
			expectError:     false,
		},
		{
			// Multiple PodCliques all matching hashes are updated
			name: "multiple podcliques all matching",
			expectedHashes: map[string]string{
				"pcsg-0-master": "hash123",
				"pcsg-0-worker": "hash456",
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithHash("pcsg-0-master", "hash123"),
				createPodCliqueWithHash("pcsg-0-worker", "hash456"),
			},
			expectedUpdated: true,
			expectError:     false,
		},
		{
			// Multiple PodCliques with one mismatch are not updated
			name: "multiple podcliques with one mismatch",
			expectedHashes: map[string]string{
				"pcsg-0-master": "hash123",
				"pcsg-0-worker": "hash456",
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithHash("pcsg-0-master", "hash123"),
				createPodCliqueWithHash("pcsg-0-worker", "hash999"), // Different hash
			},
			expectedUpdated: false,
			expectError:     false,
		},
		{
			// PodClique without hash label should return error
			name: "podclique without hash label",
			expectedHashes: map[string]string{
				"pcsg-0-worker": "hash123",
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "pcsg-0-worker",
						Labels: map[string]string{}, // No hash label
					},
				},
			},
			expectedUpdated: false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			updated, err := isReplicaUpdated(tt.expectedHashes, tt.replicaPCLQs)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.Equal(t, tt.expectedUpdated, updated, "updated status should match expected")
			}
		})
	}
}

// TestIsReplicaDeletedOrMarkedForDeletion verifies that the function correctly identifies
// replicas that are being deleted or have been deleted.
func TestIsReplicaDeletedOrMarkedForDeletion(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PCSG with rolling update status
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// PodCliques belonging to the replica being checked
		replicaPCLQs []grovecorev1alpha1.PodClique
		// Replica index being checked (not used in current implementation)
		replicaIndex int
		// Whether the replica should be considered deleted/terminating
		expectedDeleted bool
	}{
		{
			// PCSG without rolling update progress should return false
			name: "pcsg without rolling update progress",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: nil, // Explicitly nil
				},
			},
			replicaPCLQs:    []grovecorev1alpha1.PodClique{},
			replicaIndex:    0,
			expectedDeleted: false,
		},
		{
			// No PodCliques means replica is deleted
			name: "no podcliques means deleted",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{},
					},
				},
			},
			replicaPCLQs:    []grovecorev1alpha1.PodClique{},
			replicaIndex:    0,
			expectedDeleted: true,
		},
		{
			// All PodCliques terminating means replica is deleted
			name: "all podcliques terminating",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{},
					},
				},
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createTerminatingPodClique("pcsg-0-master"),
				createTerminatingPodClique("pcsg-0-worker"),
			},
			replicaIndex:    0,
			expectedDeleted: true,
		},
		{
			// Mix of terminating and non-terminating means replica is not deleted
			name: "mixed terminating and non-terminating",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{},
					},
				},
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createTerminatingPodClique("pcsg-0-master"),
				createNonTerminatingPodClique("pcsg-0-worker"),
			},
			replicaIndex:    0,
			expectedDeleted: false,
		},
		{
			// All PodCliques non-terminating means replica is not deleted
			name: "all podcliques non-terminating",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{},
					},
				},
			},
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createNonTerminatingPodClique("pcsg-0-master"),
				createNonTerminatingPodClique("pcsg-0-worker"),
			},
			replicaIndex:    0,
			expectedDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			deleted := isReplicaDeletedOrMarkedForDeletion(tt.pcsg, tt.replicaPCLQs, tt.replicaIndex)

			// Verify result
			assert.Equal(t, tt.expectedDeleted, deleted, "deleted status should match expected")
		})
	}
}

// TestGetReplicaState verifies that the function correctly determines the operational
// state of a PCSG replica based on its constituent PodCliques.
func TestGetReplicaState(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliques belonging to the replica being checked
		replicaPCLQs []grovecorev1alpha1.PodClique
		// Expected replica state
		expectedState replicaState
	}{
		{
			// No PodCliques means replica is ready
			name:          "no podcliques is ready",
			replicaPCLQs:  []grovecorev1alpha1.PodClique{},
			expectedState: replicaStateReady,
		},
		{
			// Single ready PodClique means replica is ready
			name: "single ready podclique",
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithReplicas("pcsg-0-worker", 2, 2, 2),
			},
			expectedState: replicaStateReady,
		},
		{
			// Single PodClique with insufficient scheduled replicas is pending
			name: "single pending podclique",
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithReplicas("pcsg-0-worker", 0, 0, 2), // MinAvailable=2, but 0 scheduled
			},
			expectedState: replicaStatePending,
		},
		{
			// Single PodClique with scheduled but not ready replicas is unavailable
			name: "single unavailable podclique",
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithReplicas("pcsg-0-worker", 1, 2, 2), // MinAvailable=2, 2 scheduled but only 1 ready
			},
			expectedState: replicaStateUnAvailable,
		},
		{
			// Multiple PodCliques all ready means replica is ready
			name: "multiple ready podcliques",
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithReplicas("pcsg-0-master", 1, 1, 1),
				createPodCliqueWithReplicas("pcsg-0-worker", 2, 2, 2),
			},
			expectedState: replicaStateReady,
		},
		{
			// One pending PodClique makes entire replica pending (most restrictive)
			name: "one pending makes replica pending",
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithReplicas("pcsg-0-master", 1, 1, 1), // Ready
				createPodCliqueWithReplicas("pcsg-0-worker", 0, 0, 2), // Pending
			},
			expectedState: replicaStatePending,
		},
		{
			// One unavailable PodClique makes replica unavailable when no pending
			name: "one unavailable makes replica unavailable",
			replicaPCLQs: []grovecorev1alpha1.PodClique{
				createPodCliqueWithReplicas("pcsg-0-master", 1, 1, 1), // Ready
				createPodCliqueWithReplicas("pcsg-0-worker", 1, 2, 2), // Unavailable (scheduled but not ready)
			},
			expectedState: replicaStateUnAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			state := getReplicaState(tt.replicaPCLQs)

			// Verify result
			assert.Equal(t, tt.expectedState, state, "replica state should match expected")
		})
	}
}

// TestIsAnyReadyReplicaSelectedForUpdate verifies that the function correctly
// identifies when a replica is currently selected for rolling update.
func TestIsAnyReadyReplicaSelectedForUpdate(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PCSG with various rolling update states
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Whether a replica should be considered selected for update
		expectedSelected bool
	}{
		{
			// PCSG without rolling update progress has no selected replica
			name: "no rolling update progress",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: nil, // Explicitly nil
				},
			},
			expectedSelected: false,
		},
		{
			// PCSG with rolling update progress but no ready replica selected
			name: "rolling update progress but no selected replica",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: nil,
					},
				},
			},
			expectedSelected: false,
		},
		{
			// PCSG with a ready replica selected for update
			name: "ready replica selected for update",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
							Current: 1,
						},
					},
				},
			},
			expectedSelected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			selected := isAnyReadyReplicaSelectedForUpdate(tt.pcsg)

			// Verify result
			assert.Equal(t, tt.expectedSelected, selected, "selected status should match expected")
		})
	}
}

// Helper functions for creating test objects

// createPodCliqueWithHash creates a PodClique with a specific pod template hash label
func createPodCliqueWithHash(name, hash string) grovecorev1alpha1.PodClique {
	return grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				apicommon.LabelPodTemplateHash: hash,
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
	}
}

// createTerminatingPodClique creates a PodClique that is marked for deletion
func createTerminatingPodClique(name string) grovecorev1alpha1.PodClique {
	now := metav1.Now()
	return grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			DeletionTimestamp: &now,
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
	}
}

// createNonTerminatingPodClique creates a PodClique that is not marked for deletion
func createNonTerminatingPodClique(name string) grovecorev1alpha1.PodClique {
	return grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
	}
}

// createPodCliqueWithReplicas creates a PodClique with specific replica counts and MinAvailable
func createPodCliqueWithReplicas(name string, readyReplicas, scheduledReplicas, minAvailable int32) grovecorev1alpha1.PodClique {
	return grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(minAvailable),
		},
		Status: grovecorev1alpha1.PodCliqueStatus{
			ReadyReplicas:     readyReplicas,
			ScheduledReplicas: scheduledReplicas,
		},
	}
}
