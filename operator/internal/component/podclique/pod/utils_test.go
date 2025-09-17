/*
Copyright 2025 The Grove Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pod

import (
	"testing"
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TestGetTerminatingAndNonTerminatingPodUIDs tests the utility function that separates
// pod UIDs based on their termination status for expectations store synchronization.
func TestGetTerminatingAndNonTerminatingPodUIDs(t *testing.T) {
	now := metav1.Now()

	tests := []struct {
		// name describes the test scenario
		name string
		// pods are the input pods to categorize
		pods []*corev1.Pod
		// expectedTerminating are UIDs of pods expected to be terminating
		expectedTerminating []types.UID
		// expectedNonTerminating are UIDs of pods expected to be non-terminating
		expectedNonTerminating []types.UID
	}{
		{
			// Empty input should return empty slices
			name:                   "empty input",
			pods:                   []*corev1.Pod{},
			expectedTerminating:    []types.UID{},
			expectedNonTerminating: []types.UID{},
		},
		{
			// All non-terminating pods
			name: "all non-terminating",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "running-pod",
						UID:  "uid-1",
						// No deletion timestamp = non-terminating
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pending-pod",
						UID:  "uid-2",
						// No deletion timestamp = non-terminating
					},
				},
			},
			expectedTerminating:    []types.UID{},
			expectedNonTerminating: []types.UID{"uid-1", "uid-2"},
		},
		{
			// All terminating pods
			name: "all terminating",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "deleting-pod-1",
						UID:               "uid-3",
						DeletionTimestamp: &now,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "deleting-pod-2",
						UID:               "uid-4",
						DeletionTimestamp: &now,
					},
				},
			},
			expectedTerminating:    []types.UID{"uid-3", "uid-4"},
			expectedNonTerminating: []types.UID{},
		},
		{
			// Mixed terminating and non-terminating pods
			name: "mixed pods",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "healthy-pod",
						UID:  "uid-5",
						// No deletion timestamp = non-terminating
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "terminating-pod",
						UID:               "uid-6",
						DeletionTimestamp: &now,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "another-healthy-pod",
						UID:  "uid-7",
						// No deletion timestamp = non-terminating
					},
				},
			},
			expectedTerminating:    []types.UID{"uid-6"},
			expectedNonTerminating: []types.UID{"uid-5", "uid-7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminating, nonTerminating := getTerminatingAndNonTerminatingPodUIDs(tt.pods)

			assert.ElementsMatch(t, tt.expectedTerminating, terminating, "Terminating UIDs should match")
			assert.ElementsMatch(t, tt.expectedNonTerminating, nonTerminating, "Non-terminating UIDs should match")
		})
	}
}

// TestHasPodGangSchedulingGate tests the utility function that checks if a pod
// has the Grove PodGang scheduling gate applied.
func TestHasPodGangSchedulingGate(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pod is the pod to check for scheduling gates
		pod *corev1.Pod
		// expectedHasGate indicates whether the pod should have the gate
		expectedHasGate bool
	}{
		{
			// Pod with no scheduling gates
			name: "no scheduling gates",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					SchedulingGates: []corev1.PodSchedulingGate{},
				},
			},
			expectedHasGate: false,
		},
		{
			// Pod with Grove PodGang scheduling gate
			name: "has grove podgang gate",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					SchedulingGates: []corev1.PodSchedulingGate{
						{Name: podGangSchedulingGate},
					},
				},
			},
			expectedHasGate: true,
		},
		{
			// Pod with other scheduling gates but not Grove PodGang gate
			name: "has other gates but not grove podgang",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					SchedulingGates: []corev1.PodSchedulingGate{
						{Name: "custom-gate"},
						{Name: "another-gate"},
					},
				},
			},
			expectedHasGate: false,
		},
		{
			// Pod with Grove PodGang gate among others
			name: "has grove podgang gate among others",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					SchedulingGates: []corev1.PodSchedulingGate{
						{Name: "custom-gate"},
						{Name: podGangSchedulingGate},
						{Name: "another-gate"},
					},
				},
			},
			expectedHasGate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPodGangSchedulingGate(tt.pod)
			assert.Equal(t, tt.expectedHasGate, result)
		})
	}
}

// TestGetPodCliqueExpectationsStoreKey tests the utility function that creates
// expectations store keys for PodClique resources.
func TestGetPodCliqueExpectationsStoreKey(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// operation is the operation name for the key
		operation string
		// pclqMeta is the PodClique metadata
		pclqMeta metav1.ObjectMeta
		// expectError indicates whether an error should occur
		expectError bool
		// expectedKeyParts are parts that should be in the generated key
		expectedKeyParts []string
	}{
		{
			// Valid metadata should generate proper key
			name:      "valid metadata",
			operation: "sync",
			pclqMeta: metav1.ObjectMeta{
				Name:      "test-pclq",
				Namespace: "default",
			},
			expectError:      false,
			expectedKeyParts: []string{"test-pclq", "default"},
		},
		{
			// Different namespace should generate different key
			name:      "different namespace",
			operation: "delete",
			pclqMeta: metav1.ObjectMeta{
				Name:      "prod-pclq",
				Namespace: "production",
			},
			expectError:      false,
			expectedKeyParts: []string{"prod-pclq", "production"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := getPodCliqueExpectationsStoreKey(logr.Discard(), tt.operation, tt.pclqMeta)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, key)

				// Verify key contains expected parts
				for _, part := range tt.expectedKeyParts {
					assert.Contains(t, key, part)
				}
			}
		})
	}
}

// TestSelectExcessPodsToDelete tests the function that identifies and sorts pods
// for deletion during scale-down operations using DeletionSorter.
func TestSelectExcessPodsToDelete(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	tests := []struct {
		// name describes the test scenario
		name string
		// existingPods are the pods currently in the PodClique
		existingPods []*corev1.Pod
		// desiredReplicas is the target number of replicas
		desiredReplicas int32
		// expectedPodNames are the names of pods that should be selected for deletion
		expectedPodNames []string
	}{
		{
			// No excess pods should return empty list
			name: "no excess pods",
			existingPods: []*corev1.Pod{
				createTestPodForDeletion("pod-1", "node1", corev1.PodRunning, true, now),
			},
			desiredReplicas:  1,
			expectedPodNames: []string{},
		},
		{
			// Excess pods should be selected based on deletion priority
			name: "excess pods - priority based selection",
			existingPods: []*corev1.Pod{
				// Should be selected first: unassigned, pending
				createTestPodForDeletion("unassigned-pending", "", corev1.PodPending, false, now),
				// Should be selected second: assigned, pending
				createTestPodForDeletion("assigned-pending", "node1", corev1.PodPending, false, now),
				// Should be kept: assigned, running, ready
				createTestPodForDeletion("assigned-running-ready", "node2", corev1.PodRunning, true, oneHourAgo),
			},
			desiredReplicas:  1,
			expectedPodNames: []string{"unassigned-pending", "assigned-pending"},
		},
		{
			// More complex scenario with different pod states
			name: "complex excess pod selection",
			existingPods: []*corev1.Pod{
				// Highest priority for deletion: unassigned, pending
				createTestPodForDeletion("delete-me-1", "", corev1.PodPending, false, now),
				// Second highest: assigned, pending
				createTestPodForDeletion("delete-me-2", "node1", corev1.PodPending, false, now),
				// Third: assigned, running, not ready
				createTestPodForDeletion("delete-me-3", "node2", corev1.PodRunning, false, now),
				// Should be kept: assigned, running, ready, older (more stable)
				createTestPodForDeletion("keep-me-1", "node3", corev1.PodRunning, true, oneHourAgo),
				// Should be kept: assigned, running, ready, newer
				createTestPodForDeletion("keep-me-2", "node4", corev1.PodRunning, true, now),
			},
			desiredReplicas:  2,
			expectedPodNames: []string{"delete-me-1", "delete-me-2", "delete-me-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &syncContext{
				existingPCLQPods: tt.existingPods,
				pclq: &grovecorev1alpha1.PodClique{
					Spec: grovecorev1alpha1.PodCliqueSpec{
						Replicas: tt.desiredReplicas,
					},
				},
			}

			candidatePods := selectExcessPodsToDelete(sc, logr.Discard())

			// Extract names from candidate pods
			candidateNames := make([]string, len(candidatePods))
			for i, pod := range candidatePods {
				candidateNames[i] = pod.Name
			}

			assert.Equal(t, tt.expectedPodNames, candidateNames)
		})
	}
}

// TestSyncFlowResult tests the syncFlowResult type and its methods for tracking
// synchronization operation outcomes.
func TestSyncFlowResult(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// setup configures the syncFlowResult for testing
		setup func(*syncFlowResult)
		// testFunc performs the test assertions
		testFunc func(*testing.T, *syncFlowResult)
	}{
		{
			// New result should be empty
			name:  "new result",
			setup: func(sfr *syncFlowResult) {},
			testFunc: func(t *testing.T, sfr *syncFlowResult) {
				assert.False(t, sfr.hasErrors())
				assert.False(t, sfr.hasPendingScheduleGatedPods())
				assert.NoError(t, sfr.getAggregatedError())
			},
		},
		{
			// Result with errors should report correctly
			name: "result with errors",
			setup: func(sfr *syncFlowResult) {
				sfr.recordError(assert.AnError)
				sfr.recordError(assert.AnError)
			},
			testFunc: func(t *testing.T, sfr *syncFlowResult) {
				assert.True(t, sfr.hasErrors())
				assert.Error(t, sfr.getAggregatedError())
				assert.False(t, sfr.hasPendingScheduleGatedPods())
			},
		},
		{
			// Result with pending schedule gated pods should report correctly
			name: "result with pending pods",
			setup: func(sfr *syncFlowResult) {
				sfr.recordPendingScheduleGatedPods([]string{"pod-1", "pod-2"})
			},
			testFunc: func(t *testing.T, sfr *syncFlowResult) {
				assert.False(t, sfr.hasErrors())
				assert.True(t, sfr.hasPendingScheduleGatedPods())
				assert.NoError(t, sfr.getAggregatedError())
			},
		},
		{
			// Result with both errors and pending pods
			name: "result with errors and pending pods",
			setup: func(sfr *syncFlowResult) {
				sfr.recordError(assert.AnError)
				sfr.recordPendingScheduleGatedPods([]string{"pod-3"})
			},
			testFunc: func(t *testing.T, sfr *syncFlowResult) {
				assert.True(t, sfr.hasErrors())
				assert.True(t, sfr.hasPendingScheduleGatedPods())
				assert.Error(t, sfr.getAggregatedError())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &syncFlowResult{}
			tt.setup(result)
			tt.testFunc(t, result)
		})
	}
}

// TestUpdateWork tests the updateWork type and its methods for managing
// rolling update operations.
func TestUpdateWork(t *testing.T) {
	// Create test pods with different template hashes and states
	oldHash := "old-hash-123"
	newHash := "new-hash-456"

	oldReadyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "old-ready-pod",
			UID:  "uid-1",
			Labels: map[string]string{
				apicommon.LabelPodTemplateHash: oldHash,
			},
		},
	}

	oldPendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "old-pending-pod",
			UID:  "uid-2",
			Labels: map[string]string{
				apicommon.LabelPodTemplateHash: oldHash,
			},
		},
	}

	oldUnhealthyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "old-unhealthy-pod",
			UID:  "uid-3",
			Labels: map[string]string{
				apicommon.LabelPodTemplateHash: oldHash,
			},
		},
	}

	newReadyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "new-ready-pod",
			UID:  "uid-4",
			Labels: map[string]string{
				apicommon.LabelPodTemplateHash: newHash,
			},
		},
	}

	work := &updateWork{
		oldTemplateHashReadyPods:     []*corev1.Pod{oldReadyPod},
		oldTemplateHashPendingPods:   []*corev1.Pod{oldPendingPod},
		oldTemplateHashUnhealthyPods: []*corev1.Pod{oldUnhealthyPod},
		newTemplateHashReadyPods:     []*corev1.Pod{newReadyPod},
	}

	t.Run("getPodNamesPendingUpdate", func(t *testing.T) {
		// Test with no deletion expectations
		names := work.getPodNamesPendingUpdate([]types.UID{})
		expectedNames := []string{"old-ready-pod", "old-pending-pod", "old-unhealthy-pod"}
		assert.ElementsMatch(t, expectedNames, names)

		// Test with some deletion expectations
		deletionExpected := []types.UID{"uid-2"} // old-pending-pod
		names = work.getPodNamesPendingUpdate(deletionExpected)
		expectedNames = []string{"old-ready-pod", "old-unhealthy-pod"}
		assert.ElementsMatch(t, expectedNames, names)
	})

	t.Run("getNextPodToUpdate", func(t *testing.T) {
		// Should return the oldest ready pod with old template hash
		nextPod := work.getNextPodToUpdate()
		require.NotNil(t, nextPod)
		assert.Equal(t, "old-ready-pod", nextPod.Name)

		// Test with no ready pods
		emptyWork := &updateWork{
			oldTemplateHashReadyPods: []*corev1.Pod{},
		}
		nextPod = emptyWork.getNextPodToUpdate()
		assert.Nil(t, nextPod)
	})
}
