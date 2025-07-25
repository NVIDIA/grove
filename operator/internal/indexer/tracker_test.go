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

package indexer

import (
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Test utilities
func createDummyHostnameIndexTestPod(name, hostname string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Hostname: hostname,
		},
	}
}

func createTestPodsWithIndices(indices []int) ([]*corev1.Pod, map[string]int) {
	pods := make([]*corev1.Pod, len(indices))
	podIndexMap := make(map[string]int)
	for i, idx := range indices {
		name := "pod-" + string(rune('a'+i))
		pods[i] = createDummyHostnameIndexTestPod(name, "test-clique-"+strconv.Itoa(idx))
		podIndexMap[name] = idx
	}
	return pods, podIndexMap
}

// ==================== indexTracker Tests ====================

func TestNewIndexManager_Empty(t *testing.T) {
	manager := NewPodIndexTracker(nil, 5)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.podIndexMap)
	assert.Equal(t, -1, manager.highestIndex)
}

func TestIndexManager_LoadExistingPodIndex(t *testing.T) {
	name1 := "pod-1"
	name2 := "pod-2"

	// Create manager with existing pod indices
	indexedPods := map[string]int{
		name1: 0,
		name2: 1,
	}
	manager := NewPodIndexTracker(indexedPods, 5)

	// Next available should be 2
	assert.Equal(t, 2, manager.assignAvailableIndex("test-pod"), "Next available should be 2")
}

func TestIndexManager_HoleFilling(t *testing.T) {
	tests := []struct {
		name          string
		usedIndices   []int
		expectedIndex int
		description   string
	}{
		{
			name:          "empty manager",
			usedIndices:   []int{},
			expectedIndex: 0,
			description:   "should return 0 for empty manager",
		},
		{
			name:          "continuous from 0",
			usedIndices:   []int{0, 1, 2},
			expectedIndex: 3,
			description:   "should return next after continuous sequence",
		},
		{
			name:          "gap at beginning",
			usedIndices:   []int{1, 2, 3},
			expectedIndex: 0,
			description:   "should return 0 when no continuous sequence from 0",
		},
		{
			name:          "continuous with gap",
			usedIndices:   []int{0, 1, 3, 4},
			expectedIndex: 2,
			description:   "should extend continuous sequence from 0",
		},
		{
			name:          "large gap",
			usedIndices:   []int{0, 1, 2, 100, 200},
			expectedIndex: 3,
			description:   "should continue from highest continuous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load used indices
			indexedPods := make(map[string]int)
			for i, index := range tt.usedIndices {
				name := "pod-" + string(rune('a'+i))
				indexedPods[name] = index
			}
			manager := NewPodIndexTracker(indexedPods, 10)

			actualIndex := manager.assignAvailableIndex("test-pod")
			assert.Equal(t, tt.expectedIndex, actualIndex, tt.description)
		})
	}
}

// ==================== extractIndexFromHostname Tests ====================

func TestExtractIndexFromHostname_Valid(t *testing.T) {
	tests := []struct {
		hostname string
		expected int
	}{
		{"pod-0", 0},
		{"test-clique-5", 5},
		{"complex-name-with-dashes-42", 42},
		{"single-1", 1},
		{"prefix-123", 123},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			index, err := extractIndexFromHostname(tt.hostname)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, index)
		})
	}
}

func TestExtractIndexFromHostname_Invalid(t *testing.T) {
	tests := []struct {
		hostname    string
		description string
	}{
		{"", "empty hostname"},
		{"no-dash", "hostname without dash"},
		{"pod", "hostname without index"},
		{"pod-abc", "non-numeric index"},
		{"prefix-", "empty index"},
		{"prefix-12a", "partially numeric index"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			_, err := extractIndexFromHostname(tt.hostname)
			assert.Error(t, err, "should return error for: %s", tt.description)
		})
	}
}

// ==================== NewPodIndexTracker Tests ====================

func TestNewIndexManager_EmptyPods(t *testing.T) {
	im := NewPodIndexTracker(map[string]int{}, 5)

	assert.NotNil(t, im)
	assert.NotNil(t, im.podIndexMap)
	assert.Equal(t, -1, im.highestIndex)
}

// ==================== GetIndex Tests ====================

func TestGetIndex_NewPods_HoleFilling(t *testing.T) {
	tests := []struct {
		name            string
		existingIndices []int
		expectedIndex   int
		description     string
	}{
		{
			name:            "fill hole at beginning",
			existingIndices: []int{1, 2, 3},
			expectedIndex:   0,
			description:     "should assign index 0 when it's available",
		},
		{
			name:            "fill first hole in middle",
			existingIndices: []int{0, 2, 4},
			expectedIndex:   1,
			description:     "should extend continuous sequence from 0 to 1",
		},
		{
			name:            "fill multiple holes - choose lowest",
			existingIndices: []int{1, 3, 5},
			expectedIndex:   0,
			description:     "should assign index 0 (lowest available)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, podIndexMap := createTestPodsWithIndices(tt.existingIndices)
			im := NewPodIndexTracker(podIndexMap, 10)

			newPod := createDummyHostnameIndexTestPod("new-pod", "irrelevant")
			actualIndex, err := im.GetIndex(newPod)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedIndex, actualIndex, tt.description)
		})
	}
}

func TestGetIndex_ExistingPods(t *testing.T) {
	existingPods, podIndexMap := createTestPodsWithIndices([]int{0, 2, 4})
	im := NewPodIndexTracker(podIndexMap, 10)

	// Get index for existing pod - should return same index
	existingPod := existingPods[1] // Pod with index 2
	index, err := im.GetIndex(existingPod)
	require.NoError(t, err)
	assert.Equal(t, 2, index)

	// Call again - should be idempotent
	index2, err2 := im.GetIndex(existingPod)
	require.NoError(t, err2)
	assert.Equal(t, 2, index2)
	assert.Equal(t, index, index2, "should return same index on repeated calls")
}

func TestGetIndex_NilPod(t *testing.T) {
	im := NewPodIndexTracker(map[string]int{}, 5)

	index, err := im.GetIndex(nil)
	assert.Error(t, err, "should return error for nil pod")
	assert.Equal(t, -1, index, "should return -1 for nil pod")
}

func TestGetIndex_SequentialAssignment(t *testing.T) {
	// Starting with indices [0, 3, 5] - continuous sequence is just [0]
	_, podIndexMap := createTestPodsWithIndices([]int{0, 3, 5})
	im := NewPodIndexTracker(podIndexMap, 10)

	// Add new pods sequentially - should extend continuous sequence from 0
	// Starting with [0, 3, 5], next available should be 1 (extend continuous from 0)

	// First new pod should extend continuous sequence to [0, 1]
	pod1 := createDummyHostnameIndexTestPod("new-pod-0", "irrelevant")
	index1, err := im.GetIndex(pod1)
	require.NoError(t, err)
	assert.Equal(t, 1, index1, "First pod should get index 1")

	// Second new pod should extend continuous sequence to [0, 1, 2]
	pod2 := createDummyHostnameIndexTestPod("new-pod-1", "irrelevant")
	index2, err := im.GetIndex(pod2)
	require.NoError(t, err)
	assert.Equal(t, 2, index2, "Second pod should get index 2")

	// Third new pod should extend continuous sequence but index 3 is taken,
	// so it should get index 4 (first available after extending sequence)
	pod3 := createDummyHostnameIndexTestPod("new-pod-2", "irrelevant")
	index3, err := im.GetIndex(pod3)
	require.NoError(t, err)
	assert.Equal(t, 4, index3, "Third pod should get index 4 (first available after 3 is taken)")

	// Next pod should get next available index
	// After assignments [0, 1, 2, 3, 4, 5], next should be 6
	pod := createDummyHostnameIndexTestPod("overflow-pod", "irrelevant")
	actualIndex, err := im.GetIndex(pod)
	require.NoError(t, err, "should succeed and assign next available index")
	assert.Equal(t, 6, actualIndex, "should assign index 6 (next available after 5)")

	// Final indices: [0, 1, 2, 3, 4, 5, 6]
	// The continuous sequence is [0, 1, 2], so highestContinuous should be 2
}

func TestGetIndex_ContinuousAssignment(t *testing.T) {
	// Create manager with continuous indices [0,1,2] - next available should be 3
	podIndexMap := map[string]int{
		"pod-a": 0,
		"pod-b": 1,
		"pod-c": 2,
	}
	im := NewPodIndexTracker(podIndexMap, 10)

	// Add new pod - should get next available index
	newPod := createDummyHostnameIndexTestPod("new-pod", "irrelevant")
	index, err := im.GetIndex(newPod)
	require.NoError(t, err)
	assert.Equal(t, 3, index, "should assign next available index 3 (after continuous [0,1,2])")
}

// ==================== Integration Tests ====================

func TestIndexManager_ComplexScenario(t *testing.T) {
	// Complex scenario: algorithm prioritizes extending continuous sequence from 0
	// Start with pods at indices [0, 2] - continuous sequence is [0], gap at 1
	_, podIndexMap := createTestPodsWithIndices([]int{0, 2})
	im := NewPodIndexTracker(podIndexMap, 10)

	// First new pod should extend continuous sequence from [0] to [0, 1]
	pod1 := createDummyHostnameIndexTestPod("pod1", "irrelevant")
	index1, err := im.GetIndex(pod1)
	require.NoError(t, err)
	assert.Equal(t, 1, index1, "first new pod should get index 1")

	// Second new pod should continue extending but index 2 is taken, so gets next available
	pod2 := createDummyHostnameIndexTestPod("pod2", "irrelevant")
	index2, err2 := im.GetIndex(pod2)
	require.NoError(t, err2)
	assert.Equal(t, 3, index2, "second new pod should get index 3 (2 is taken)")

	// Third new pod should get next available index
	pod3 := createDummyHostnameIndexTestPod("pod3", "irrelevant")
	index3, err3 := im.GetIndex(pod3)
	require.NoError(t, err3, "third new pod should succeed")
	assert.Equal(t, 4, index3, "should assign next available index 4")

	// Verify existing assignments are still consistent
	check1, _ := im.GetIndex(pod1)
	assert.Equal(t, 1, check1, "pod1 should still return index 1")
	check2, _ := im.GetIndex(pod2)
	assert.Equal(t, 3, check2, "pod2 should still return index 3")
	check3, _ := im.GetIndex(pod3)
	assert.Equal(t, 4, check3, "pod3 should still return index 4")
}

func TestIndexManager_ContinuousAssignment(t *testing.T) {
	// Create manager with no initial assignments
	im := NewPodIndexTracker(map[string]int{}, 5)

	// Add pods sequentially
	pod0 := createDummyHostnameIndexTestPod("pod0", "irrelevant")
	pod1 := createDummyHostnameIndexTestPod("pod1", "irrelevant")

	idx0, err0 := im.GetIndex(pod0)
	require.NoError(t, err0)
	assert.Equal(t, 0, idx0)
	idx1, err1 := im.GetIndex(pod1)
	require.NoError(t, err1)
	assert.Equal(t, 1, idx1)

	// Next pod should continue the sequence
	pod2 := createDummyHostnameIndexTestPod("pod2", "irrelevant")
	idx2, err2 := im.GetIndex(pod2)
	require.NoError(t, err2)
	assert.Equal(t, 2, idx2, "should assign next sequential index")
}

func TestGetIndex_EmptyManager(t *testing.T) {
	im := NewPodIndexTracker(map[string]int{}, 5)

	pod := createDummyHostnameIndexTestPod("test-pod", "irrelevant")
	index, err := im.GetIndex(pod)
	require.NoError(t, err, "should succeed and assign first index")
	assert.Equal(t, 0, index, "should assign index 0")
}

// createTestPodWithHostname creates a test pod with the specified name, uid, and hostname
func createTestPodWithHostname(name, hostname string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			Hostname: hostname,
		},
	}
}

// createTestLogger creates a simple logger for testing
func createTestLogger() logr.Logger {
	return log.Log.WithName("test")
}

// assertIndexManagerMappings verifies the PodIndexTracker has the expected pod name to index mappings
func assertIndexManagerMappings(t *testing.T, im *PodIndexTracker, expectedMappings map[string]int) {
	// We can't directly access PodIndexTracker internals, so we test via GetIndex behavior
	// Create test pods with the expected names and verify they return the expected indices
	for name, expectedIndex := range expectedMappings {
		testPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		actualIndex, err := im.GetIndex(testPod)
		require.NoError(t, err)
		assert.Equal(t, expectedIndex, actualIndex, "Pod name %s should have index %d", name, expectedIndex)
	}
}

// ==================== Happy Path Tests ====================

func TestInitIndexManager_ValidPods(t *testing.T) {
	logger := createTestLogger()

	// Create test pods with valid hostnames
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-a", "test-clique-0"),
		createTestPodWithHostname("pod-b", "test-clique-2"),
		createTestPodWithHostname("pod-c", "test-clique-4"),
	}

	// Initialize PodIndexTracker with expected size 5
	im := InitIndexManger(pods, 10, logger)

	// Verify mappings
	expectedMappings := map[string]int{
		"pod-a": 0,
		"pod-b": 2,
		"pod-c": 4,
	}
	assertIndexManagerMappings(t, im, expectedMappings)
}

func TestInitIndexManager_EmptyPods(t *testing.T) {
	logger := createTestLogger()

	// Initialize PodIndexTracker with empty pod list
	im := InitIndexManger([]*corev1.Pod{}, 10, logger)

	// Verify no mappings exist (try to get index for new pod should get next available)
	newPod := createTestPodWithHostname("new-pod", "irrelevant")
	index, err := im.GetIndex(newPod)
	require.NoError(t, err)
	assert.Equal(t, 0, index, "should get first available index in empty manager")
}

func TestInitIndexManager_SparseIndices(t *testing.T) {
	logger := createTestLogger()

	// Create test pods with non-consecutive indices
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-a", "test-clique-0"),
		createTestPodWithHostname("pod-b", "test-clique-2"),
		createTestPodWithHostname("pod-c", "test-clique-4"),
	}

	// Initialize PodIndexTracker with expected size 6
	im := InitIndexManger(pods, 10, logger)

	// Verify mappings
	expectedMappings := map[string]int{
		"pod-a": 0,
		"pod-b": 2,
		"pod-c": 4,
	}
	assertIndexManagerMappings(t, im, expectedMappings)

	// Verify that holes can be filled (index 1 should be available)
	newPod := createTestPodWithHostname("new-pod", "irrelevant")
	index, err := im.GetIndex(newPod)
	require.NoError(t, err)
	assert.Equal(t, 1, index, "should fill hole at index 1")
}

// ==================== Parameter Edge Case Tests ====================

func TestInitIndexManager_ValidPods_NoLimits(t *testing.T) {
	logger := createTestLogger()

	// Create test pods with valid hostnames
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-a", "test-clique-0"),
		createTestPodWithHostname("pod-b", "test-clique-1"),
	}

	// Initialize PodIndexTracker
	im := InitIndexManger(pods, 10, logger)

	// All valid pods should be mapped
	expectedMappings := map[string]int{
		"pod-a": 0,
		"pod-b": 1,
	}
	assertIndexManagerMappings(t, im, expectedMappings)

	// Try to get index for new pod - should succeed
	newPod := createTestPodWithHostname("new-pod", "irrelevant")
	index, err := im.GetIndex(newPod)
	require.NoError(t, err, "should succeed")
	assert.Equal(t, 2, index, "should assign next available index")
}

func TestInitIndexManager_LargeExpectedSize(t *testing.T) {
	logger := createTestLogger()

	// Create test pods with valid hostnames
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-a", "test-clique-0"),
		createTestPodWithHostname("pod-b", "test-clique-1"),
	}

	// Initialize PodIndexTracker with large expected size
	im := InitIndexManger(pods, 10, logger)

	// Verify mappings work correctly
	expectedMappings := map[string]int{
		"pod-a": 0,
		"pod-b": 1,
	}
	assertIndexManagerMappings(t, im, expectedMappings)
}

// ==================== Input Validation Tests ====================

func TestInitIndexManager_InvalidHostnames(t *testing.T) {
	logger := createTestLogger()

	// Create test pods with mix of valid and invalid hostnames
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-valid", "test-clique-0"),
		createTestPodWithHostname("pod-invalid1", "invalid-hostname"),
		createTestPodWithHostname("pod-invalid2", "pod-abc"),
		createTestPodWithHostname("pod-valid2", "test-clique-2"),
	}

	// Initialize PodIndexTracker
	im := InitIndexManger(pods, 10, logger)

	// Only valid pods should be mapped
	expectedMappings := map[string]int{
		"pod-valid":  0,
		"pod-valid2": 2,
	}
	assertIndexManagerMappings(t, im, expectedMappings)
}

func TestInitIndexManager_EmptyHostnames(t *testing.T) {
	logger := createTestLogger()

	// Create test pods with empty hostnames
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-empty1", ""),
		createTestPodWithHostname("pod-valid", "test-clique-1"),
		createTestPodWithHostname("pod-empty2", ""),
	}

	// Initialize PodIndexTracker
	im := InitIndexManger(pods, 10, logger)

	// Only pod with valid hostname should be mapped
	expectedMappings := map[string]int{
		"pod-valid": 1,
	}
	assertIndexManagerMappings(t, im, expectedMappings)
}

func TestInitIndexManager_MixedValidInvalidIndices(t *testing.T) {
	logger := createTestLogger()

	// Create test pods with mix of valid and invalid hostnames
	validPod1 := createTestPodWithHostname("pod-valid", "test-clique-1")
	validPod2 := createTestPodWithHostname("pod-valid2", "test-clique-2")
	validPod3 := createTestPodWithHostname("pod-high-index", "test-clique-10") // High index should work now

	pods := []*corev1.Pod{
		validPod1,
		createTestPodWithHostname("pod-invalid", "test-clique-negative"), // This will fail hostname parsing
		validPod3,
		validPod2,
	}

	// Initialize PodIndexTracker
	im := InitIndexManger(pods, 10, logger)

	// All valid pods should be mapped, regardless of index value
	index1, err1 := im.GetIndex(validPod1)
	require.NoError(t, err1)
	assert.Equal(t, 1, index1, "pod-valid should have index 1")

	index2, err2 := im.GetIndex(validPod2)
	require.NoError(t, err2)
	assert.Equal(t, 2, index2, "pod-valid2 should have index 2")

	index3, err3 := im.GetIndex(validPod3)
	require.NoError(t, err3)
	assert.Equal(t, 10, index3, "pod-high-index should have index 10")

	// Verify that a new pod gets the next available index (0)
	newPod := createTestPodWithHostname("new-pod", "irrelevant")
	newIndex, err := im.GetIndex(newPod)
	require.NoError(t, err)
	assert.Equal(t, 0, newIndex, "new pod should get next available index (0)")
}

// ==================== Edge Case Tests ====================

func TestInitIndexManager_DuplicateIndices(t *testing.T) {
	// During initialization with duplicate indices, the behavior is:
	// 1. Each pod gets processed and assigns its index
	// 2. Later pods with same index remove the previous pod's mapping ("last wins")
	// 3. When GetIndex is called later, removed pods get new indices
	logger := createTestLogger()

	// Create test pods with duplicate indices
	pod1 := createTestPodWithHostname("pod-first", "test-clique-1")
	pod2 := createTestPodWithHostname("pod-second", "test-clique-1") // Same index
	pod3 := createTestPodWithHostname("pod-third", "test-clique-2")

	pods := []*corev1.Pod{pod1, pod2, pod3}

	// Initialize PodIndexTracker
	im := InitIndexManger(pods, 10, logger)

	index2, err2 := im.GetIndex(pod2)
	require.NoError(t, err2)
	assert.Equal(t, 0, index2, "second pod should have index 0")

	// Test the pod that should not have been overwritten
	index3, err3 := im.GetIndex(pod3)
	require.NoError(t, err3)
	assert.Equal(t, 2, index3, "third pod should have index 2")

	// Verify that the first pod gets the next available index
	// After "last wins" processing: pod2 has index 1, pod3 has index 2, pod1 was removed
	// When pod1 calls GetIndex(), it gets the next available index
	// Since there's no continuous sequence from 0, it gets index 0
	index1, err1 := im.GetIndex(pod1)
	require.NoError(t, err1)
	assert.Equal(t, 1, index1, "first pod should get index 1")
}

func TestInitIndexManager_NilPods(t *testing.T) {
	logger := createTestLogger()

	// Create pod slice with nil pods
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-valid1", "test-clique-0"),
		nil, // Nil pod
		createTestPodWithHostname("pod-valid2", "test-clique-2"),
		nil, // Another nil pod
	}

	// Initialize PodIndexTracker (should handle nil pods gracefully)
	im := InitIndexManger(pods, 10, logger)

	// Only valid pods should be mapped
	expectedMappings := map[string]int{
		"pod-valid1": 0,
		"pod-valid2": 2,
	}
	assertIndexManagerMappings(t, im, expectedMappings)
}

func TestInitIndexManager_MixedScenario(t *testing.T) {
	logger := createTestLogger()

	// Create a realistic mixed scenario
	pods := []*corev1.Pod{
		createTestPodWithHostname("pod-valid1", "test-clique-0"),
		createTestPodWithHostname("pod-empty", ""),            // Empty hostname
		createTestPodWithHostname("pod-invalid", "malformed"), // Invalid hostname
		createTestPodWithHostname("pod-valid2", "test-clique-1"),
		createTestPodWithHostname("pod-oob", "test-clique-10"), // Out of bounds
		nil, // Nil pod
		createTestPodWithHostname("pod-valid3", "test-clique-2"),
	}

	// Initialize PodIndexTracker with expected size 5
	im := InitIndexManger(pods, 10, logger)

	// Only valid, in-bounds pods should be mapped
	expectedMappings := map[string]int{
		"pod-valid1": 0,
		"pod-valid2": 1,
		"pod-valid3": 2,
	}
	assertIndexManagerMappings(t, im, expectedMappings)

	// Verify next available index is 3
	newPod := createTestPodWithHostname("new-pod", "irrelevant")
	index, err := im.GetIndex(newPod)
	require.NoError(t, err)
	assert.Equal(t, 3, index, "next available index should be 3")
}
