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
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDeletionSorter_Len verifies that DeletionSorter correctly reports its length.
// This is a basic test for the sort.Interface implementation.
func TestDeletionSorter_Len(t *testing.T) {
	tests := []struct {
		// name describes the test case
		name string
		// pods is the slice of pods to sort
		pods []*corev1.Pod
		// expectedLen is the expected length that should be returned
		expectedLen int
	}{
		{
			// Empty slice should return length 0
			name:        "empty slice",
			pods:        []*corev1.Pod{},
			expectedLen: 0,
		},
		{
			// Single pod should return length 1
			name:        "single pod",
			pods:        []*corev1.Pod{createTestPodForDeletion("pod1", "", corev1.PodRunning, true, time.Now())},
			expectedLen: 1,
		},
		{
			// Multiple pods should return correct length
			name: "multiple pods",
			pods: []*corev1.Pod{
				createTestPodForDeletion("pod1", "", corev1.PodRunning, true, time.Now()),
				createTestPodForDeletion("pod2", "node1", corev1.PodPending, false, time.Now().Add(-1*time.Hour)),
			},
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorter := DeletionSorter(tt.pods)
			assert.Equal(t, tt.expectedLen, sorter.Len())
		})
	}
}

// TestDeletionSorter_Swap verifies that DeletionSorter correctly swaps elements.
// This tests the swap functionality required by sort.Interface.
func TestDeletionSorter_Swap(t *testing.T) {
	// Create two different pods for swapping
	pod1 := createTestPodForDeletion("pod1", "node1", corev1.PodRunning, true, time.Now())
	pod2 := createTestPodForDeletion("pod2", "node2", corev1.PodPending, false, time.Now().Add(-1*time.Hour))

	sorter := DeletionSorter([]*corev1.Pod{pod1, pod2})

	// Verify initial order
	assert.Equal(t, "pod1", sorter[0].Name)
	assert.Equal(t, "pod2", sorter[1].Name)

	// Swap elements
	sorter.Swap(0, 1)

	// Verify swapped order
	assert.Equal(t, "pod2", sorter[0].Name)
	assert.Equal(t, "pod1", sorter[1].Name)
}

// TestDeletionSorter_Less tests the core deletion priority logic.
// This verifies that pods are prioritized correctly for deletion based on:
// 1. Unassigned vs assigned nodes
// 2. Pod phase (Pending < Unknown < Running)
// 3. Ready vs not ready status
// 4. Creation timestamp (newer first, with special handling for zero timestamps)
func TestDeletionSorter_Less(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	tests := []struct {
		// name describes the test scenario being tested
		name string
		// podA is the first pod to compare
		podA *corev1.Pod
		// podB is the second pod to compare
		podB *corev1.Pod
		// expectAFirst indicates whether podA should be deleted before podB (Less returns true)
		expectAFirst bool
		// description explains the deletion priority rule being tested
		description string
	}{
		{
			// Unassigned pods should be deleted before assigned pods
			name:         "unassigned vs assigned",
			podA:         createTestPodForDeletion("unassigned", "", corev1.PodPending, false, now),
			podB:         createTestPodForDeletion("assigned", "node1", corev1.PodPending, false, now),
			expectAFirst: true,
			description:  "unassigned pod should be deleted before assigned pod",
		},
		{
			// Both unassigned - phase priority should apply
			name:         "both unassigned - phase priority",
			podA:         createTestPodForDeletion("pending", "", corev1.PodPending, false, now),
			podB:         createTestPodForDeletion("running", "", corev1.PodRunning, false, now),
			expectAFirst: true,
			description:  "pending pod should be deleted before running pod when both unassigned",
		},
		{
			// Phase priority: Pending < Unknown < Running
			name:         "phase priority - pending vs unknown",
			podA:         createTestPodForDeletion("pending", "node1", corev1.PodPending, false, now),
			podB:         createTestPodForDeletion("unknown", "node2", corev1.PodUnknown, false, now),
			expectAFirst: true,
			description:  "pending pod should be deleted before unknown pod",
		},
		{
			// Phase priority: Unknown < Running
			name:         "phase priority - unknown vs running",
			podA:         createTestPodForDeletion("unknown", "node1", corev1.PodUnknown, false, now),
			podB:         createTestPodForDeletion("running", "node2", corev1.PodRunning, false, now),
			expectAFirst: true,
			description:  "unknown pod should be deleted before running pod",
		},
		{
			// Readiness priority: not ready pods deleted before ready pods
			name:         "readiness priority",
			podA:         createTestPodForDeletion("not-ready", "node1", corev1.PodRunning, false, now),
			podB:         createTestPodForDeletion("ready", "node2", corev1.PodRunning, true, now),
			expectAFirst: true,
			description:  "not ready pod should be deleted before ready pod",
		},
		{
			// Creation timestamp priority: zero timestamp pods deleted first
			name:         "zero timestamp vs normal",
			podA:         createTestPodForDeletion("zero-time", "node1", corev1.PodRunning, true, time.Time{}),
			podB:         createTestPodForDeletion("normal-time", "node2", corev1.PodRunning, true, now),
			expectAFirst: true,
			description:  "pod with zero creation timestamp should be deleted before pod with normal timestamp",
		},
		{
			// Creation timestamp priority: newer pods deleted before older pods
			name:         "newer vs older pod",
			podA:         createTestPodForDeletion("newer", "node1", corev1.PodRunning, true, now),
			podB:         createTestPodForDeletion("older", "node2", corev1.PodRunning, true, oneHourAgo),
			expectAFirst: true,
			description:  "newer pod should be deleted before older pod",
		},
		{
			// Complex scenario combining multiple factors
			name:         "complex scenario - assigned ready running vs unassigned not-ready pending",
			podA:         createTestPodForDeletion("unassigned-pending", "", corev1.PodPending, false, now),
			podB:         createTestPodForDeletion("assigned-running", "node1", corev1.PodRunning, true, oneHourAgo),
			expectAFirst: true,
			description:  "unassigned pending pod should be deleted before assigned running pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorter := DeletionSorter([]*corev1.Pod{tt.podA, tt.podB})
			result := sorter.Less(0, 1)
			assert.Equal(t, tt.expectAFirst, result, tt.description)
		})
	}
}

// TestDeletionSorter_FullSort tests the complete sorting behavior by sorting
// a mixed set of pods and verifying the final order matches deletion priority rules.
func TestDeletionSorter_FullSort(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	// Create a mixed set of pods with different priority characteristics
	pods := []*corev1.Pod{
		// Should be last: assigned, running, ready, oldest
		createTestPodForDeletion("assigned-running-ready-old", "node1", corev1.PodRunning, true, twoHoursAgo),
		// Should be first: unassigned, pending, not ready
		createTestPodForDeletion("unassigned-pending", "", corev1.PodPending, false, now),
		// Should be second: unassigned, unknown, not ready
		createTestPodForDeletion("unassigned-unknown", "", corev1.PodUnknown, false, oneHourAgo),
		// Should be third: assigned, pending, not ready
		createTestPodForDeletion("assigned-pending", "node2", corev1.PodPending, false, now),
		// Should be fourth: assigned, running, not ready, newer
		createTestPodForDeletion("assigned-running-not-ready-new", "node3", corev1.PodRunning, false, now),
		// Should be fifth: assigned, running, not ready, older
		createTestPodForDeletion("assigned-running-not-ready-old", "node4", corev1.PodRunning, false, oneHourAgo),
		// Should be sixth: assigned, running, ready, newer
		createTestPodForDeletion("assigned-running-ready-new", "node5", corev1.PodRunning, true, now),
	}

	// Sort using DeletionSorter
	sort.Sort(DeletionSorter(pods))

	// Expected order based on deletion priority rules
	expectedOrder := []string{
		"unassigned-pending",             // 1st: unassigned + pending
		"unassigned-unknown",             // 2nd: unassigned + unknown
		"assigned-pending",               // 3rd: assigned + pending
		"assigned-running-not-ready-new", // 4th: assigned + running + not ready + newer
		"assigned-running-not-ready-old", // 5th: assigned + running + not ready + older
		"assigned-running-ready-new",     // 6th: assigned + running + ready + newer
		"assigned-running-ready-old",     // 7th: assigned + running + ready + oldest
	}

	// Verify the sorting results
	for i, expectedName := range expectedOrder {
		assert.Equal(t, expectedName, pods[i].Name, "Pod at position %d should be %s", i, expectedName)
	}
}

// TestIsPodReady tests the isPodReady helper function used by DeletionSorter.
// This function determines if a pod has a Ready condition with status True.
func TestIsPodReady(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pod is the pod to check for readiness
		pod *corev1.Pod
		// expectedReady indicates whether the pod should be considered ready
		expectedReady bool
	}{
		{
			// Pod with Ready condition True should be ready
			name:          "pod with ready condition true",
			pod:           createTestPodForDeletion("ready-pod", "node1", corev1.PodRunning, true, time.Now()),
			expectedReady: true,
		},
		{
			// Pod with Ready condition False should not be ready
			name:          "pod with ready condition false",
			pod:           createTestPodForDeletion("not-ready-pod", "node1", corev1.PodRunning, false, time.Now()),
			expectedReady: false,
		},
		{
			// Pod with no conditions should not be ready
			name: "pod with no conditions",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "no-conditions-pod"},
				Status:     corev1.PodStatus{Conditions: []corev1.PodCondition{}},
			},
			expectedReady: false,
		},
		{
			// Pod with other conditions but no Ready condition should not be ready
			name: "pod with other conditions but no ready condition",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "other-conditions-pod"},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodInitialized,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expectedReady: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			assert.Equal(t, tt.expectedReady, result)
		})
	}
}

// Helper function to create test pods with specific characteristics for deletion sorting tests.
// This centralizes pod creation logic to ensure consistent test data.
func createTestPodForDeletion(name, nodeName string, phase corev1.PodPhase, ready bool, creationTime time.Time) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}

	// Set creation timestamp if not zero
	if !creationTime.IsZero() {
		pod.CreationTimestamp = metav1.NewTime(creationTime)
	}

	// Add Ready condition if specified
	if ready {
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		}
	} else {
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionFalse,
			},
		}
	}

	return pod
}
