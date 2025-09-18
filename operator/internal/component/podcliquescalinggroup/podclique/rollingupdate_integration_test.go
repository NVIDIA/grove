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
	"context"
	"testing"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testRollingUpdateNamespace = "test-rolling-update"
)

// TestComputePendingUpdateWork verifies that the function correctly analyzes existing replicas
// to categorize them by state for rolling update planning.
func TestComputePendingUpdateWork(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Sync context with existing PodCliques and expected state
		syncContext *syncContext
		// Expected update work categorization
		expectedWork *updateWork
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// No existing PodCliques - no work needed
			name: "no existing podcliques",
			syncContext: &syncContext{
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 2,
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs:                  []grovecorev1alpha1.PodClique{},
				expectedPCLQPodTemplateHashMap: map[string]string{},
			},
			expectedWork: &updateWork{
				oldPendingReplicaIndices:     nil,
				oldUnavailableReplicaIndices: nil,
				oldReadyReplicaIndices:       nil,
			},
			expectError: false,
		},
		{
			// Ready replica that needs update
			name: "ready replica needs update",
			syncContext: &syncContext{
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 1,
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createReadyPodCliqueWithHash("test-pcsg-0-worker", "0", "old-hash"),
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash", // Different hash indicates update needed
				},
			},
			expectedWork: &updateWork{
				oldPendingReplicaIndices:     nil,
				oldUnavailableReplicaIndices: nil,
				oldReadyReplicaIndices:       []int{0}, // Replica 0 is ready but needs update
			},
			expectError: false,
		},
		{
			// Pending replica (not scheduled)
			name: "pending replica not scheduled",
			syncContext: &syncContext{
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 1,
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createPendingPodCliqueWithHash("test-pcsg-0-worker", "0", "old-hash"),
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
				},
			},
			expectedWork: &updateWork{
				oldPendingReplicaIndices:     []int{0}, // Replica 0 is pending
				oldUnavailableReplicaIndices: nil,
				oldReadyReplicaIndices:       nil,
			},
			expectError: false,
		},
		{
			// Unavailable replica (scheduled but not ready)
			name: "unavailable replica scheduled but not ready",
			syncContext: &syncContext{
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 1,
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createUnavailablePodCliqueWithHash("test-pcsg-0-worker", "0", "old-hash"),
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
				},
			},
			expectedWork: &updateWork{
				oldPendingReplicaIndices:     nil,
				oldUnavailableReplicaIndices: []int{0}, // Replica 0 is unavailable
				oldReadyReplicaIndices:       nil,
			},
			expectError: false,
		},
		{
			// Currently updating replica should be skipped
			name: "currently updating replica skipped",
			syncContext: &syncContext{
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 2,
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
							ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
								Current: 0, // Replica 0 is currently updating
							},
						},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createReadyPodCliqueWithHash("test-pcsg-0-worker", "0", "old-hash"),
					createReadyPodCliqueWithHash("test-pcsg-1-worker", "1", "old-hash"),
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
					"test-pcsg-1-worker": "new-hash",
				},
			},
			expectedWork: &updateWork{
				oldPendingReplicaIndices:     nil,
				oldUnavailableReplicaIndices: nil,
				oldReadyReplicaIndices:       []int{1}, // Only replica 1, replica 0 is currently updating
			},
			expectError: false,
		},
		{
			// Already updated replica should be skipped
			name: "already updated replica skipped",
			syncContext: &syncContext{
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 2,
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createReadyPodCliqueWithHash("test-pcsg-0-worker", "0", "new-hash"), // Already updated
					createReadyPodCliqueWithHash("test-pcsg-1-worker", "1", "old-hash"), // Needs update
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
					"test-pcsg-1-worker": "new-hash",
				},
			},
			expectedWork: &updateWork{
				oldPendingReplicaIndices:     nil,
				oldUnavailableReplicaIndices: nil,
				oldReadyReplicaIndices:       []int{1}, // Only replica 1 needs update
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			work, err := computePendingUpdateWork(tt.syncContext)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				if tt.expectedErrorSubstring != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstring, "error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.Equal(t, tt.expectedWork.oldPendingReplicaIndices, work.oldPendingReplicaIndices, "pending replica indices should match")
				assert.Equal(t, tt.expectedWork.oldUnavailableReplicaIndices, work.oldUnavailableReplicaIndices, "unavailable replica indices should match")
				assert.Equal(t, tt.expectedWork.oldReadyReplicaIndices, work.oldReadyReplicaIndices, "ready replica indices should match")
			}
		})
	}
}

// TestIsCurrentReplicaUpdateComplete verifies that the function correctly determines
// if the currently updating replica has completed its update.
func TestIsCurrentReplicaUpdateComplete(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Sync context with current update state
		syncContext *syncContext
		// Whether the update should be considered complete
		expectedComplete bool
	}{
		{
			// Update complete - all PodCliques updated and ready
			name: "update complete all podcliques ready",
			syncContext: &syncContext{
				pcs: &grovecorev1alpha1.PodCliqueSet{
					Status: grovecorev1alpha1.PodCliqueSetStatus{
						CurrentGenerationHash: ptr.To("gen-hash-123"),
					},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
							ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
								Current: 0, // Currently updating replica 0
							},
						},
					},
				},
				expectedPCLQFQNsPerPCSGReplica: map[int][]string{
					0: {"test-pcsg-0-worker"},
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pcsg-0-worker",
							Labels: map[string]string{
								apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
							},
						},
						Spec: grovecorev1alpha1.PodCliqueSpec{
							MinAvailable: ptr.To(int32(1)),
						},
						Status: grovecorev1alpha1.PodCliqueStatus{
							CurrentPodTemplateHash:            ptr.To("new-hash"),     // Updated hash
							CurrentPodCliqueSetGenerationHash: ptr.To("gen-hash-123"), // Updated generation
							ReadyReplicas:                     1,                      // Meets MinAvailable
						},
					},
				},
			},
			expectedComplete: true,
		},
		{
			// Update incomplete - wrong number of PodCliques
			name: "update incomplete wrong number of podcliques",
			syncContext: &syncContext{
				pcs: &grovecorev1alpha1.PodCliqueSet{
					Status: grovecorev1alpha1.PodCliqueSetStatus{
						CurrentGenerationHash: ptr.To("gen-hash-123"),
					},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
							ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
								Current: 0,
							},
						},
					},
				},
				expectedPCLQFQNsPerPCSGReplica: map[int][]string{
					0: {"test-pcsg-0-worker", "test-pcsg-0-master"}, // Expecting 2 PodCliques
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					// Only 1 PodClique exists, but expecting 2
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pcsg-0-worker",
							Labels: map[string]string{
								apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
							},
						},
					},
				},
			},
			expectedComplete: false,
		},
		{
			// Update incomplete - PodClique not ready
			name: "update incomplete podclique not ready",
			syncContext: &syncContext{
				pcs: &grovecorev1alpha1.PodCliqueSet{
					Status: grovecorev1alpha1.PodCliqueSetStatus{
						CurrentGenerationHash: ptr.To("gen-hash-123"),
					},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
							ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
								Current: 0,
							},
						},
					},
				},
				expectedPCLQFQNsPerPCSGReplica: map[int][]string{
					0: {"test-pcsg-0-worker"},
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pcsg-0-worker",
							Labels: map[string]string{
								apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
							},
						},
						Spec: grovecorev1alpha1.PodCliqueSpec{
							MinAvailable: ptr.To(int32(2)),
						},
						Status: grovecorev1alpha1.PodCliqueStatus{
							CurrentPodTemplateHash:            ptr.To("new-hash"),
							CurrentPodCliqueSetGenerationHash: ptr.To("gen-hash-123"),
							ReadyReplicas:                     1, // Does not meet MinAvailable of 2
						},
					},
				},
			},
			expectedComplete: false,
		},
		{
			// Update incomplete - old pod template hash
			name: "update incomplete old pod template hash",
			syncContext: &syncContext{
				pcs: &grovecorev1alpha1.PodCliqueSet{
					Status: grovecorev1alpha1.PodCliqueSetStatus{
						CurrentGenerationHash: ptr.To("gen-hash-123"),
					},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
							ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
								Current: 0,
							},
						},
					},
				},
				expectedPCLQFQNsPerPCSGReplica: map[int][]string{
					0: {"test-pcsg-0-worker"},
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pcsg-0-worker",
							Labels: map[string]string{
								apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
							},
						},
						Spec: grovecorev1alpha1.PodCliqueSpec{
							MinAvailable: ptr.To(int32(1)),
						},
						Status: grovecorev1alpha1.PodCliqueStatus{
							CurrentPodTemplateHash:            ptr.To("old-hash"), // Old hash
							CurrentPodCliqueSetGenerationHash: ptr.To("gen-hash-123"),
							ReadyReplicas:                     1,
						},
					},
				},
			},
			expectedComplete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			complete := isCurrentReplicaUpdateComplete(tt.syncContext)

			// Verify result
			assert.Equal(t, tt.expectedComplete, complete, "update completion status should match expected")
		})
	}
}

// TestDeleteOldPendingAndUnavailableReplicas verifies that the function correctly
// removes replicas that are in transitional states during rolling updates.
func TestDeleteOldPendingAndUnavailableReplicas(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Update work with replicas to delete
		work *updateWork
		// Whether an error is expected
		expectError bool
	}{
		{
			// No replicas to delete
			name: "no replicas to delete",
			work: &updateWork{
				oldPendingReplicaIndices:     []int{},
				oldUnavailableReplicaIndices: []int{},
				oldReadyReplicaIndices:       []int{},
			},
			expectError: false,
		},
		{
			// Delete pending replicas
			name: "delete pending replicas",
			work: &updateWork{
				oldPendingReplicaIndices:     []int{0, 1},
				oldUnavailableReplicaIndices: []int{},
				oldReadyReplicaIndices:       []int{},
			},
			expectError: false,
		},
		{
			// Delete unavailable replicas
			name: "delete unavailable replicas",
			work: &updateWork{
				oldPendingReplicaIndices:     []int{},
				oldUnavailableReplicaIndices: []int{2, 3},
				oldReadyReplicaIndices:       []int{},
			},
			expectError: false,
		},
		{
			// Delete both pending and unavailable replicas
			name: "delete pending and unavailable replicas",
			work: &updateWork{
				oldPendingReplicaIndices:     []int{0},
				oldUnavailableReplicaIndices: []int{1},
				oldReadyReplicaIndices:       []int{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()
			client := testutils.SetupFakeClient()
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Create sync context
			syncCtx := &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testRollingUpdateNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testRollingUpdateNamespace},
				},
			}

			// Execute the function under test
			err := operator.deleteOldPendingAndUnavailableReplicas(logger, syncCtx, tt.work)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
			}
		})
	}
}

// TestUpdatePCSGStatusWithNextReplicaToUpdate verifies that the function correctly
// updates the PCSG status to track the replica currently being updated.
func TestUpdatePCSGStatusWithNextReplicaToUpdate(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Initial PCSG status
		initialPCSG *grovecorev1alpha1.PodCliqueScalingGroup
		// Next replica index to update
		nextReplicaIndex int
		// Whether an error is expected
		expectError bool
	}{
		{
			// First replica to update - no previous progress
			name: "first replica to update",
			initialPCSG: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testRollingUpdateNamespace,
				},
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						// No ReadyReplicaIndicesSelectedToUpdate yet
					},
				},
			},
			nextReplicaIndex: 0,
			expectError:      false,
		},
		{
			// Second replica to update - move previous to completed
			name: "second replica to update",
			initialPCSG: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testRollingUpdateNamespace,
				},
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
							Current:   0,
							Completed: []int32{},
						},
					},
				},
			},
			nextReplicaIndex: 1,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()

			// Create fake client with the PCSG
			objects := []client.Object{tt.initialPCSG}
			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.updatePCSGStatusWithNextReplicaToUpdate(context.Background(), logger, tt.initialPCSG, tt.nextReplicaIndex)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")

				// Verify the status was updated correctly
				assert.NotNil(t, tt.initialPCSG.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate, "should have update progress")
				assert.Equal(t, int32(tt.nextReplicaIndex), tt.initialPCSG.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate.Current, "current replica should be updated")
			}
		})
	}
}

// TestMarkRollingUpdateEnd verifies that the function correctly finalizes
// the rolling update process by updating the PCSG status.
func TestMarkRollingUpdateEnd(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Initial PCSG with rolling update in progress
		initialPCSG *grovecorev1alpha1.PodCliqueScalingGroup
		// Whether an error is expected
		expectError bool
	}{
		{
			// Mark end of rolling update
			name: "mark end of rolling update",
			initialPCSG: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testRollingUpdateNamespace,
				},
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
						ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
							Current:   2,
							Completed: []int32{0, 1},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()

			// Create fake client with the PCSG
			objects := []client.Object{tt.initialPCSG}
			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.markRollingUpdateEnd(context.Background(), logger, tt.initialPCSG)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")

				// Verify the status was updated correctly
				assert.NotNil(t, tt.initialPCSG.Status.RollingUpdateProgress.UpdateEndedAt, "should have end timestamp")
				assert.Nil(t, tt.initialPCSG.Status.RollingUpdateProgress.ReadyReplicaIndicesSelectedToUpdate, "should clear update progress")
			}
		})
	}
}

// Helper functions for creating test PodCliques with different states

// createReadyPodCliqueWithHash creates a PodClique that is ready with a specific hash
func createReadyPodCliqueWithHash(name, replicaIndex, hash string) grovecorev1alpha1.PodClique {
	return grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: replicaIndex,
				apicommon.LabelPodTemplateHash:                   hash,
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
		Status: grovecorev1alpha1.PodCliqueStatus{
			ScheduledReplicas: 1, // Meets MinAvailable
			ReadyReplicas:     1, // Meets MinAvailable
		},
	}
}

// createPendingPodCliqueWithHash creates a PodClique that is pending (not scheduled)
func createPendingPodCliqueWithHash(name, replicaIndex, hash string) grovecorev1alpha1.PodClique {
	return grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: replicaIndex,
				apicommon.LabelPodTemplateHash:                   hash,
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
		Status: grovecorev1alpha1.PodCliqueStatus{
			ScheduledReplicas: 0, // Below MinAvailable - pending
			ReadyReplicas:     0,
		},
	}
}

// createUnavailablePodCliqueWithHash creates a PodClique that is scheduled but not ready
func createUnavailablePodCliqueWithHash(name, replicaIndex, hash string) grovecorev1alpha1.PodClique {
	return grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: replicaIndex,
				apicommon.LabelPodTemplateHash:                   hash,
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
		Status: grovecorev1alpha1.PodCliqueStatus{
			ScheduledReplicas: 1, // Meets MinAvailable for scheduling
			ReadyReplicas:     0, // Below MinAvailable for readiness - unavailable
		},
	}
}

// TestProcessPendingUpdates verifies that the main rolling update orchestrator
// correctly manages the sequential update of replicas while respecting availability constraints.
func TestProcessPendingUpdates(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Sync context with current state
		syncContext *syncContext
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// No replicas need updating
			name: "no replicas need updating",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testRollingUpdateNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testRollingUpdateNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     2,
						MinAvailable: ptr.To(int32(1)),
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createReadyPodCliqueWithHash("test-pcsg-0-worker", "0", "new-hash"),
					createReadyPodCliqueWithHash("test-pcsg-1-worker", "1", "new-hash"),
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash", // Already up to date
					"test-pcsg-1-worker": "new-hash", // Already up to date
				},
			},
			expectError: false,
		},
		{
			// Update in progress - wait for completion
			name: "update in progress wait for completion",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testRollingUpdateNamespace},
					Status: grovecorev1alpha1.PodCliqueSetStatus{
						CurrentGenerationHash: ptr.To("gen-hash-123"),
					},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testRollingUpdateNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     2,
						MinAvailable: ptr.To(int32(1)),
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{
							ReadyReplicaIndicesSelectedToUpdate: &grovecorev1alpha1.PodCliqueScalingGroupReplicaRollingUpdateProgress{
								Current: 0, // Currently updating replica 0
							},
						},
					},
				},
				expectedPCLQFQNsPerPCSGReplica: map[int][]string{
					0: {"test-pcsg-0-worker"},
					1: {"test-pcsg-1-worker"},
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
					"test-pcsg-1-worker": "new-hash",
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					// Replica 0 is still updating (old hash)
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-pcsg-0-worker",
							Labels: map[string]string{
								apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
							},
						},
						Spec: grovecorev1alpha1.PodCliqueSpec{
							MinAvailable: ptr.To(int32(1)),
						},
						Status: grovecorev1alpha1.PodCliqueStatus{
							CurrentPodTemplateHash:            ptr.To("old-hash"), // Still old hash
							CurrentPodCliqueSetGenerationHash: ptr.To("gen-hash-123"),
							ReadyReplicas:                     1,
						},
					},
					createReadyPodCliqueWithHash("test-pcsg-1-worker", "1", "old-hash"),
				},
			},
			expectError:            true,
			expectedErrorSubstring: "rolling update of currently selected PCSG replica index: 0 is not complete",
		},
		{
			// MinAvailable breached - cannot proceed
			name: "minAvailable breached cannot proceed",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testRollingUpdateNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testRollingUpdateNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     2,
						MinAvailable: ptr.To(int32(2)), // All replicas must be available
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						AvailableReplicas:     1, // Only 1 available, but need 2
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createReadyPodCliqueWithHash("test-pcsg-0-worker", "0", "old-hash"),
					createReadyPodCliqueWithHash("test-pcsg-1-worker", "1", "old-hash"),
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
					"test-pcsg-1-worker": "new-hash",
				},
			},
			expectError:            true,
			expectedErrorSubstring: "available replicas 1 lesser than minAvailable 2",
		},
		{
			// Start new update - select first replica
			name: "start new update select first replica",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testRollingUpdateNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testRollingUpdateNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     2,
						MinAvailable: ptr.To(int32(1)),
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						AvailableReplicas:     2, // Both replicas available
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createReadyPodCliqueWithHash("test-pcsg-0-worker", "0", "old-hash"),
					createReadyPodCliqueWithHash("test-pcsg-1-worker", "1", "old-hash"),
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "new-hash",
					"test-pcsg-1-worker": "new-hash",
				},
			},
			expectError:            true,
			expectedErrorSubstring: "rolling update of currently selected PCSG replica index: 0 is not complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()

			// Create fake client with the PCSG and PCS
			objects := []client.Object{tt.syncContext.pcs, tt.syncContext.pcsg}
			for i := range tt.syncContext.existingPCLQs {
				objects = append(objects, &tt.syncContext.existingPCLQs[i])
			}

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.processPendingUpdates(logger, tt.syncContext)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				if tt.expectedErrorSubstring != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstring, "error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "should not return an error")
			}
		})
	}
}
