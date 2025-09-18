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
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestGetExpectedPodCliqueFQNsByPCSGReplica verifies that the function correctly computes
// expected PodClique fully qualified names for each PCSG replica.
func TestGetExpectedPodCliqueFQNsByPCSGReplica(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueScalingGroup configuration for testing
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected mapping of replica index to PodClique names
		expectedFQNs map[int][]string
	}{
		{
			// Single replica with one clique should generate one PodClique name
			name: "single replica single clique",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"worker"},
				},
			},
			expectedFQNs: map[int][]string{
				0: {"test-pcsg-0-worker"},
			},
		},
		{
			// Single replica with multiple cliques should generate multiple PodClique names
			name: "single replica multiple cliques",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"master", "worker", "storage"},
				},
			},
			expectedFQNs: map[int][]string{
				0: {"test-pcsg-0-master", "test-pcsg-0-worker", "test-pcsg-0-storage"},
			},
		},
		{
			// Multiple replicas with single clique should generate names for each replica
			name: "multiple replicas single clique",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    3,
					CliqueNames: []string{"worker"},
				},
			},
			expectedFQNs: map[int][]string{
				0: {"test-pcsg-0-worker"},
				1: {"test-pcsg-1-worker"},
				2: {"test-pcsg-2-worker"},
			},
		},
		{
			// Multiple replicas with multiple cliques should generate all combinations
			name: "multiple replicas multiple cliques",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    2,
					CliqueNames: []string{"master", "worker"},
				},
			},
			expectedFQNs: map[int][]string{
				0: {"test-pcsg-0-master", "test-pcsg-0-worker"},
				1: {"test-pcsg-1-master", "test-pcsg-1-worker"},
			},
		},
		{
			// Zero replicas should generate empty map
			name: "zero replicas",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    0,
					CliqueNames: []string{"worker"},
				},
			},
			expectedFQNs: map[int][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			result := getExpectedPodCliqueFQNsByPCSGReplica(tt.pcsg)

			// Verify the result matches expectations
			assert.Equal(t, tt.expectedFQNs, result, "expected FQNs should match")
		})
	}
}

// TestGetExistingNonTerminatingPCSGReplicas verifies that the function correctly counts
// non-terminating PCSG replicas by examining PodClique resources.
func TestGetExistingNonTerminatingPCSGReplicas(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Existing PodClique resources to analyze
		existingPCLQs []grovecorev1alpha1.PodClique
		// Expected count of non-terminating replicas
		expectedCount int
	}{
		{
			// No PodCliques should result in zero replicas
			name:          "no podcliques",
			existingPCLQs: []grovecorev1alpha1.PodClique{},
			expectedCount: 0,
		},
		{
			// Single replica with one PodClique should count as one
			name: "single replica one podclique",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("pcsg-0-worker", "0", false),
			},
			expectedCount: 1,
		},
		{
			// Single replica with multiple PodCliques should still count as one
			name: "single replica multiple podcliques",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("pcsg-0-master", "0", false),
				createTestPodCliqueWithReplicaIndex("pcsg-0-worker", "0", false),
			},
			expectedCount: 1,
		},
		{
			// Multiple replicas should count correctly
			name: "multiple replicas",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("pcsg-0-worker", "0", false),
				createTestPodCliqueWithReplicaIndex("pcsg-1-worker", "1", false),
				createTestPodCliqueWithReplicaIndex("pcsg-2-worker", "2", false),
			},
			expectedCount: 3,
		},
		{
			// Terminating PodCliques should be excluded from count
			name: "mixed terminating and non-terminating",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("pcsg-0-worker", "0", false),
				createTestPodCliqueWithReplicaIndex("pcsg-1-worker", "1", true), // terminating
				createTestPodCliqueWithReplicaIndex("pcsg-2-worker", "2", false),
			},
			expectedCount: 2,
		},
		{
			// PodCliques without replica index label should be ignored
			name: "podcliques without replica index label",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("pcsg-0-worker", "0", false),
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "no-label-podclique",
						Namespace: testNamespace,
						Labels:    map[string]string{}, // No replica index label
					},
				},
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			count := getExistingNonTerminatingPCSGReplicas(tt.existingPCLQs)

			// Verify the count matches expectations
			assert.Equal(t, tt.expectedCount, count, "replica count should match expected")
		})
	}
}

// TestComputePCSGReplicasToDelete verifies that the function correctly determines
// which replica indices should be deleted during scale-down operations.
func TestComputePCSGReplicasToDelete(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Current number of existing replicas
		existingReplicas int
		// Target number of replicas after scale-down
		expectedReplicas int
		// Expected replica indices that should be deleted (as strings)
		expectedIndices []string
	}{
		{
			// No scale-down needed should return empty list
			name:             "no scale down needed",
			existingReplicas: 3,
			expectedReplicas: 3,
			expectedIndices:  []string{},
		},
		{
			// Scale down from 3 to 1 should delete indices 1 and 2
			name:             "scale down from 3 to 1",
			existingReplicas: 3,
			expectedReplicas: 1,
			expectedIndices:  []string{"1", "2"},
		},
		{
			// Scale down from 5 to 2 should delete indices 2, 3, and 4
			name:             "scale down from 5 to 2",
			existingReplicas: 5,
			expectedReplicas: 2,
			expectedIndices:  []string{"2", "3", "4"},
		},
		{
			// Scale down to zero should delete all indices
			name:             "scale down to zero",
			existingReplicas: 2,
			expectedReplicas: 0,
			expectedIndices:  []string{"0", "1"},
		},
		{
			// Already at zero should return empty list
			name:             "already at zero",
			existingReplicas: 0,
			expectedReplicas: 0,
			expectedIndices:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			indices := computePCSGReplicasToDelete(tt.existingReplicas, tt.expectedReplicas)

			// Verify the indices match expectations
			assert.Equal(t, tt.expectedIndices, indices, "indices to delete should match expected")
		})
	}
}

// TestGetMinAvailableBreachedPCSGIndices verifies that the function correctly identifies
// PCSG replica indices that have breached MinAvailable constraints and categorizes them
// by whether they are ready for termination or still within the grace period.
func TestGetMinAvailableBreachedPCSGIndices(t *testing.T) {
	now := time.Now()
	terminationDelay := 5 * time.Minute

	tests := []struct {
		// Test case description
		name string
		// Existing PodClique resources to analyze
		existingPCLQs []grovecorev1alpha1.PodClique
		// Termination delay duration
		terminationDelay time.Duration
		// Expected replica indices ready for termination
		expectedTerminate []string
		// Expected replica indices waiting for termination delay
		expectedRequeue []string
	}{
		{
			// No PodCliques should result in empty lists
			name:              "no podcliques",
			existingPCLQs:     []grovecorev1alpha1.PodClique{},
			terminationDelay:  terminationDelay,
			expectedTerminate: []string{},
			expectedRequeue:   []string{},
		},
		{
			// PodCliques with sufficient replicas should not be marked for termination
			name: "healthy podcliques",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createHealthyPodClique("pcsg-0-worker", "0"),
				createHealthyPodClique("pcsg-1-worker", "1"),
			},
			terminationDelay:  terminationDelay,
			expectedTerminate: []string{},
			expectedRequeue:   []string{},
		},
		{
			// PodCliques with breached MinAvailable but recent breach should be requeued
			name: "recently breached podcliques",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createUnhealthyPodClique("pcsg-0-worker", "0", now.Add(-1*time.Minute)),
			},
			terminationDelay:  terminationDelay,
			expectedTerminate: []string{},
			expectedRequeue:   []string{"0"},
		},
		{
			// PodCliques with breached MinAvailable beyond delay should be terminated
			name: "long breached podcliques",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createUnhealthyPodClique("pcsg-0-worker", "0", now.Add(-10*time.Minute)),
			},
			terminationDelay:  terminationDelay,
			expectedTerminate: []string{"0"},
			expectedRequeue:   []string{},
		},
		{
			// Mix of healthy, recently breached, and long breached PodCliques
			name: "mixed health status",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createHealthyPodClique("pcsg-0-worker", "0"),
				createUnhealthyPodClique("pcsg-1-worker", "1", now.Add(-1*time.Minute)),
				createUnhealthyPodClique("pcsg-2-worker", "2", now.Add(-10*time.Minute)),
			},
			terminationDelay:  terminationDelay,
			expectedTerminate: []string{"2"},
			expectedRequeue:   []string{"1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a test logger that discards output
			logger := logr.Discard()

			// Execute the function under test
			terminate, requeue := getMinAvailableBreachedPCSGIndices(logger, tt.existingPCLQs, tt.terminationDelay)

			// Verify the results match expectations
			assert.ElementsMatch(t, tt.expectedTerminate, terminate, "terminate indices should match expected")
			assert.ElementsMatch(t, tt.expectedRequeue, requeue, "requeue indices should match expected")
		})
	}
}

// Helper functions for creating test objects

// createTestPodCliqueWithReplicaIndex creates a test PodClique with specific replica index and termination status
func createTestPodCliqueWithReplicaIndex(name, replicaIndex string, terminating bool) grovecorev1alpha1.PodClique {
	pclq := grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: replicaIndex,
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
	}

	if terminating {
		// Add deletion timestamp and finalizer to simulate terminating state
		now := metav1.Now()
		pclq.DeletionTimestamp = &now
		pclq.Finalizers = []string{"test-finalizer"}
	}

	return pclq
}

// createHealthyPodClique creates a PodClique that meets its MinAvailable requirements
func createHealthyPodClique(name, replicaIndex string) grovecorev1alpha1.PodClique {
	pclq := createTestPodCliqueWithReplicaIndex(name, replicaIndex, false)
	pclq.Status = grovecorev1alpha1.PodCliqueStatus{
		ReadyReplicas: 1, // Meets MinAvailable of 1
	}
	return pclq
}

// TestGetExpectedPCLQPodTemplateHashMap verifies that the function correctly computes
// expected pod template hashes for all PodCliques in a PCSG.
func TestGetExpectedPCLQPodTemplateHashMap(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet with template specifications
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup with clique names and replicas
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected hash mappings
		expectedHashes map[string]string
	}{
		{
			// Single replica single clique
			name: "single replica single clique",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"worker"},
				},
			},
			expectedHashes: map[string]string{
				"test-pcsg-0-worker": "", // Hash will be computed, we just verify it exists
			},
		},
		{
			// Multiple replicas multiple cliques
			name: "multiple replicas multiple cliques",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "master",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 3,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    2,
					CliqueNames: []string{"master", "worker"},
				},
			},
			expectedHashes: map[string]string{
				"test-pcsg-0-master": "",
				"test-pcsg-0-worker": "",
				"test-pcsg-1-master": "",
				"test-pcsg-1-worker": "",
			},
		},
		{
			// PCSG with clique not in PCS (should be ignored)
			name: "pcsg with non-existent clique",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:    1,
					CliqueNames: []string{"worker", "non-existent"},
				},
			},
			expectedHashes: map[string]string{
				"test-pcsg-0-worker": "", // Only worker should be present
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			result := getExpectedPCLQPodTemplateHashMap(tt.pcs, tt.pcsg)

			// Verify the result has the expected keys
			assert.Equal(t, len(tt.expectedHashes), len(result), "should have expected number of hash entries")

			for expectedKey := range tt.expectedHashes {
				assert.Contains(t, result, expectedKey, "should contain expected key %s", expectedKey)
				assert.NotEmpty(t, result[expectedKey], "hash for %s should not be empty", expectedKey)
			}

			// Verify no unexpected keys are present
			for actualKey := range result {
				assert.Contains(t, tt.expectedHashes, actualKey, "should not contain unexpected key %s", actualKey)
			}
		})
	}
}

// TestGetExistingPCLQs verifies that the function correctly retrieves existing PodClique
// resources owned by a PodCliqueScalingGroup using the Kubernetes client.
func TestGetExistingPCLQs(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Existing PodCliques to create in the fake client
		existingPodCliques []grovecorev1alpha1.PodClique
		// PodCliqueScalingGroup to query for
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected number of PodCliques returned
		expectedCount int
		// Whether an error is expected
		expectError bool
	}{
		{
			// No existing PodCliques should return empty list
			name:               "no existing podcliques",
			existingPodCliques: []grovecorev1alpha1.PodClique{},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
				},
			},
			expectedCount: 0,
			expectError:   false,
		},
		{
			// Single owned PodClique should be returned
			name: "single owned podclique",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodCliqueForPCSG("test-pcsg-0-worker", "0", "test-pcsg", false),
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					UID:       types.UID("test-pcsg-uid"),
					Labels: map[string]string{
						apicommon.LabelPartOfKey: "test-pcs",
					},
				},
			},
			expectedCount: 1,
			expectError:   false,
		},
		{
			// Multiple owned PodCliques should all be returned
			name: "multiple owned podcliques",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodCliqueForPCSG("test-pcsg-0-worker", "0", "test-pcsg", false),
				createTestPodCliqueForPCSG("test-pcsg-0-master", "0", "test-pcsg", false),
				createTestPodCliqueForPCSG("test-pcsg-1-worker", "1", "test-pcsg", false),
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					UID:       types.UID("test-pcsg-uid"),
					Labels: map[string]string{
						apicommon.LabelPartOfKey: "test-pcs",
					},
				},
			},
			expectedCount: 3,
			expectError:   false,
		},
		{
			// Unowned PodCliques should be filtered out
			name: "filters out unowned podcliques",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodCliqueForPCSG("test-pcsg-0-worker", "0", "test-pcsg", false),
				createTestPodCliqueWithReplicaIndex("other-pcsg-0-worker", "0", false), // Not owned - different PCSG
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					UID:       types.UID("test-pcsg-uid"),
					Labels: map[string]string{
						apicommon.LabelPartOfKey: "test-pcs",
					},
				},
			},
			expectedCount: 1,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			ctx := context.Background()

			// Convert PodCliques to client.Object for the fake client
			objects := make([]client.Object, len(tt.existingPodCliques)+1)
			for i := range tt.existingPodCliques {
				objects[i] = &tt.existingPodCliques[i]
			}
			objects[len(tt.existingPodCliques)] = tt.pcsg

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			result, err := operator.getExistingPCLQs(ctx, tt.pcsg)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.Equal(t, tt.expectedCount, len(result), "should return expected number of PodCliques")
			}
		})
	}
}

// TestRefreshExistingPCLQs verifies that the syncContext correctly filters existing
// PodCliques to only include those within the current PCSG replica count.
func TestRefreshExistingPCLQs(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Initial existing PodCliques in sync context
		existingPCLQs []grovecorev1alpha1.PodClique
		// PodCliqueScalingGroup with current replica count
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected number of PodCliques after refresh
		expectedCount int
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// All PodCliques within replica count should be kept
			name: "all podcliques within replica count",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("test-pcsg-0-worker", "0", true),
				createTestPodCliqueWithReplicaIndex("test-pcsg-1-worker", "1", true),
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas: 3, // Both replicas 0 and 1 are within count
				},
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			// PodCliques beyond replica count should be filtered out
			name: "filters out podcliques beyond replica count",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("test-pcsg-0-worker", "0", true),
				createTestPodCliqueWithReplicaIndex("test-pcsg-1-worker", "1", true),
				createTestPodCliqueWithReplicaIndex("test-pcsg-2-worker", "2", true),
				createTestPodCliqueWithReplicaIndex("test-pcsg-3-worker", "3", true),
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas: 2, // Only replicas 0 and 1 should be kept
				},
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			// PodCliques without replica index label should be skipped
			name: "skips podcliques without replica index label",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueWithReplicaIndex("test-pcsg-0-worker", "0", true),
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pcsg-no-index-worker",
						Namespace: testNamespace,
						Labels: map[string]string{
							// Missing LabelPodCliqueScalingGroupReplicaIndex
							apicommon.LabelPodCliqueScalingGroup: "test-pcsg",
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas: 2,
				},
			},
			expectedCount: 1, // Only the one with valid label
			expectError:   false,
		},
		{
			// Invalid replica index should return error
			name: "error on invalid replica index",
			existingPCLQs: []grovecorev1alpha1.PodClique{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pcsg-invalid-worker",
						Namespace: testNamespace,
						Labels: map[string]string{
							apicommon.LabelPodCliqueScalingGroupReplicaIndex: "invalid",
							apicommon.LabelPodCliqueScalingGroup:             "test-pcsg",
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas: 2,
				},
			},
			expectedCount:          0,
			expectError:            true,
			expectedErrorSubstring: "invalid pcsg replica index label value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create sync context with existing PodCliques
			sc := &syncContext{
				existingPCLQs: tt.existingPCLQs,
			}

			// Execute the function under test
			err := sc.refreshExistingPCLQs(tt.pcsg)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				if tt.expectedErrorSubstring != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstring, "error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.Equal(t, tt.expectedCount, len(sc.existingPCLQs), "should have expected number of PodCliques after refresh")
			}
		})
	}
}

// createUnhealthyPodClique creates a PodClique that has breached MinAvailable at a specific time
func createUnhealthyPodClique(name, replicaIndex string, breachTime time.Time) grovecorev1alpha1.PodClique {
	pclq := createTestPodCliqueWithReplicaIndex(name, replicaIndex, false)
	pclq.Status = grovecorev1alpha1.PodCliqueStatus{
		ReadyReplicas: 0, // Breaches MinAvailable of 1
		Conditions: []metav1.Condition{
			{
				Type:               constants.ConditionTypeMinAvailableBreached,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(breachTime),
			},
		},
	}
	return pclq
}

// createTestPodCliqueForPCSG creates a test PodClique with proper labels for PCSG ownership testing
func createTestPodCliqueForPCSG(name, replicaIndex, pcsgName string, terminating bool) grovecorev1alpha1.PodClique {
	pclq := grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels: map[string]string{
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: replicaIndex,
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "test-pcs", // Assuming PCS name is test-pcs
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             pcsgName,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: grovecorev1alpha1.SchemeGroupVersion.String(),
					Kind:       "PodCliqueScalingGroup",
					Name:       pcsgName,
					UID:        types.UID("test-pcsg-uid"),
					Controller: ptr.To(true),
				},
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
	}

	if terminating {
		// Add deletion timestamp and finalizer to simulate terminating state
		now := metav1.Now()
		pclq.DeletionTimestamp = &now
		pclq.Finalizers = []string{"test-finalizer"}
	}

	return pclq
}
