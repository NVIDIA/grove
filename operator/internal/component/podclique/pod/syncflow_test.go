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

package pod

import (
	"context"
	"fmt"
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCheckAndRemovePodSchedulingGates_MinAvailableAware(t *testing.T) {
	tests := []struct {
		name                string
		podGangName         string
		basePodGangExists   bool
		basePodGangReady    bool
		podHasGate          bool
		podInPodGang        bool
		expectedGateRemoved bool
		expectedSkippedPods int
		description         string
	}{
		{
			name:                "Base PodGang pod - gates removed immediately",
			podGangName:         "simple1-0",
			basePodGangExists:   true,
			basePodGangReady:    false, // Irrelevant for base PodGang
			podHasGate:          true,
			podInPodGang:        true,
			expectedGateRemoved: true,
			expectedSkippedPods: 0,
			description:         "Base PodGang pods should have gates removed immediately",
		},
		{
			name:                "Individual PodGang pod - base not ready",
			podGangName:         "simple1-0-sga-2",
			basePodGangExists:   true,
			basePodGangReady:    false,
			podHasGate:          true,
			podInPodGang:        true,
			expectedGateRemoved: false,
			expectedSkippedPods: 1,
			description:         "Individual PodGang pods should keep gates when base not ready",
		},
		{
			name:                "Individual PodGang pod - base ready",
			podGangName:         "simple1-0-sga-2",
			basePodGangExists:   true,
			basePodGangReady:    true,
			podHasGate:          true,
			podInPodGang:        true,
			expectedGateRemoved: true,
			expectedSkippedPods: 0,
			description:         "Individual PodGang pods should have gates removed when base ready",
		},
		{
			name:                "Individual PodGang pod - base missing",
			podGangName:         "simple1-0-sga-3",
			basePodGangExists:   false,
			basePodGangReady:    false,
			podHasGate:          true,
			podInPodGang:        true,
			expectedGateRemoved: false,
			expectedSkippedPods: 1,
			description:         "Individual PodGang pods should keep gates when base PodGang missing",
		},
		{
			name:                "Pod not in PodGang yet",
			podGangName:         "simple1-0-sga-2",
			basePodGangExists:   true,
			basePodGangReady:    true,
			podHasGate:          true,
			podInPodGang:        false,
			expectedGateRemoved: false,
			expectedSkippedPods: 1,
			description:         "Pods not yet in PodGang should keep gates regardless",
		},
		{
			name:                "Pod without gate",
			podGangName:         "simple1-0-sga-2",
			basePodGangExists:   true,
			basePodGangReady:    true,
			podHasGate:          false,
			podInPodGang:        true,
			expectedGateRemoved: false,
			expectedSkippedPods: 0,
			description:         "Pods without gates should be ignored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test objects
			pod := createTestPod(tt.podGangName, tt.podHasGate, tt.podInPodGang)
			var objects []client.Object
			objects = append(objects, pod)

			if tt.basePodGangExists {
				basePodGang := createTestBasePodGang(tt.podGangName, tt.basePodGangReady)
				objects = append(objects, basePodGang)

				if tt.basePodGangReady {
					// Add ready PodClique for base PodGang
					basePclq := createTestPodClique("simple1-0-pcb", 2, 2) // ReadyReplicas >= MinAvailable
					objects = append(objects, basePclq)
				} else {
					// Add not-ready PodClique for base PodGang
					basePclq := createTestPodClique("simple1-0-pcb", 2, 1) // ReadyReplicas < MinAvailable
					objects = append(objects, basePclq)
				}
			}

			// Create fake client
			scheme := runtime.NewScheme()
			corev1.AddToScheme(scheme)
			grovecorev1alpha1.AddToScheme(scheme)
			groveschedulerv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			// Create a test PodClique for the sync context
			testPclq := &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "default",
				},
			}

			// Create resource and sync context
			r := &_resource{client: fakeClient}
			sc := &syncContext{
				ctx:                           context.Background(),
				pclq:                          testPclq,
				existingPCLQPods:              []*corev1.Pod{pod},
				podNamesUpdatedInPCLQPodGangs: []string{},
			}

			if tt.podInPodGang {
				sc.podNamesUpdatedInPCLQPodGangs = []string{pod.Name}
			}

			// Test the gate removal logic
			skippedPods, err := r.checkAndRemovePodSchedulingGates(sc, logr.Discard())
			require.NoError(t, err)

			// Verify results
			assert.Len(t, skippedPods, tt.expectedSkippedPods, tt.description)

			// Check if gate was actually removed
			updatedPod := &corev1.Pod{}
			err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(pod), updatedPod)
			require.NoError(t, err)

			hasGateAfter := hasPodGangSchedulingGate(updatedPod)
			if tt.expectedGateRemoved {
				assert.False(t, hasGateAfter, "Pod should not have scheduling gate after removal")
			} else if tt.podHasGate {
				assert.True(t, hasGateAfter, "Pod should still have scheduling gate")
			}
		})
	}
}

func TestCheckAndRemovePodSchedulingGates_ConcurrentExecution(t *testing.T) {
	// Test that multiple pods can have their gates removed concurrently without race conditions

	// Create multiple pods with gates for base PodGang (should all be processed)
	pods := []*corev1.Pod{}
	objects := []client.Object{}

	for i := 0; i < 5; i++ {
		pod := createTestPod("simple1-0", true, true) // Base PodGang, has gate, in PodGang
		pod.Name = fmt.Sprintf("test-pod-%d", i)
		pods = append(pods, pod)
		objects = append(objects, pod)
	}

	// Create fake client
	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)
	grovecorev1alpha1.AddToScheme(scheme)
	groveschedulerv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	// Create test PodClique for the sync context
	testPclq := &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pclq",
			Namespace: "default",
		},
	}

	// Create resource and sync context
	r := &_resource{client: fakeClient}
	sc := &syncContext{
		ctx:                           context.Background(),
		pclq:                          testPclq,
		existingPCLQPods:              pods,
		podNamesUpdatedInPCLQPodGangs: []string{},
	}

	// All pods are in PodGang
	for _, pod := range pods {
		sc.podNamesUpdatedInPCLQPodGangs = append(sc.podNamesUpdatedInPCLQPodGangs, pod.Name)
	}

	// Test the gate removal logic
	skippedPods, err := r.checkAndRemovePodSchedulingGates(sc, logr.Discard())
	require.NoError(t, err)
	assert.Empty(t, skippedPods, "No pods should be skipped for base PodGang")

	// Verify all pods had their gates removed
	for i, originalPod := range pods {
		updatedPod := &corev1.Pod{}
		err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(originalPod), updatedPod)
		require.NoError(t, err)

		hasGateAfter := hasPodGangSchedulingGate(updatedPod)
		assert.False(t, hasGateAfter, "Pod %d should not have scheduling gate after removal", i)
	}
}

func TestIsIndividualPodGang(t *testing.T) {
	tests := []struct {
		name        string
		podGangName string
		expected    bool
	}{
		{
			name:        "Base PodGang",
			podGangName: "simple1-0",
			expected:    false,
		},
		{
			name:        "Individual PodGang with sga",
			podGangName: "simple1-0-sga-2",
			expected:    true,
		},
		{
			name:        "Individual PodGang with different scaling group",
			podGangName: "test-pgs-1-mysg-5",
			expected:    false, // doesn't contain "-sga-"
		},
		{
			name:        "Invalid name format",
			podGangName: "simple1",
			expected:    false,
		},
		{
			name:        "Individual PodGang with many segments",
			podGangName: "complex-name-0-sga-10",
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isIndividualPodGang(tt.podGangName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractBasePodGangName(t *testing.T) {
	tests := []struct {
		name                    string
		individualPodGangName   string
		expectedBasePodGangName string
	}{
		{
			name:                    "Simple individual PodGang",
			individualPodGangName:   "simple1-0-sga-2",
			expectedBasePodGangName: "simple1-0",
		},
		{
			name:                    "Complex individual PodGang",
			individualPodGangName:   "complex-name-1-sga-5",
			expectedBasePodGangName: "complex-name-1",
		},
		{
			name:                    "No sga pattern",
			individualPodGangName:   "simple1-0-other-2",
			expectedBasePodGangName: "simple1-0-other-2", // Returns original
		},
		{
			name:                    "Multiple dashes before sga",
			individualPodGangName:   "multi-part-name-0-sga-1",
			expectedBasePodGangName: "multi-part-name-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBasePodGangName(tt.individualPodGangName)
			assert.Equal(t, tt.expectedBasePodGangName, result)
		})
	}
}

func TestIsBasePodGangReady(t *testing.T) {
	tests := []struct {
		name              string
		basePodGangExists bool
		podCliques        []testPodClique
		expectedReady     bool
		description       string
	}{
		{
			name:              "Base PodGang ready - all PodCliques meet MinAvailable",
			basePodGangExists: true,
			podCliques: []testPodClique{
				{name: "simple1-0-pcb", minAvailable: 2, readyReplicas: 2},
				{name: "simple1-0-pcc", minAvailable: 1, readyReplicas: 3},
			},
			expectedReady: true,
			description:   "All PodCliques meet their MinAvailable requirements",
		},
		{
			name:              "Base PodGang not ready - one PodClique below MinAvailable",
			basePodGangExists: true,
			podCliques: []testPodClique{
				{name: "simple1-0-pcb", minAvailable: 2, readyReplicas: 2},
				{name: "simple1-0-pcc", minAvailable: 3, readyReplicas: 2}, // Below MinAvailable
			},
			expectedReady: false,
			description:   "One PodClique below MinAvailable makes base PodGang not ready",
		},
		{
			name:              "Base PodGang missing",
			basePodGangExists: false,
			podCliques:        []testPodClique{},
			expectedReady:     false,
			description:       "Missing base PodGang should return false",
		},
		{
			name:              "Base PodGang ready - single PodClique",
			basePodGangExists: true,
			podCliques: []testPodClique{
				{name: "simple1-0-pcb", minAvailable: 1, readyReplicas: 1},
			},
			expectedReady: true,
			description:   "Single PodClique meeting MinAvailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object

			if tt.basePodGangExists {
				basePodGang := createTestBasePodGangWithPodCliques(tt.podCliques)
				objects = append(objects, basePodGang)

				// Add corresponding PodCliques
				for _, pclq := range tt.podCliques {
					podClique := createTestPodClique(pclq.name, pclq.minAvailable, pclq.readyReplicas)
					objects = append(objects, podClique)
				}
			}

			// Create fake client
			scheme := runtime.NewScheme()
			grovecorev1alpha1.AddToScheme(scheme)
			groveschedulerv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			// Test the readiness check
			result := isBasePodGangReady(context.Background(), fakeClient, logr.Discard(), "default", "simple1-0-sga-2")
			assert.Equal(t, tt.expectedReady, result, tt.description)
		})
	}
}

// Test helper types and functions

type testPodClique struct {
	name          string
	minAvailable  int32
	readyReplicas int32
}

func createTestPod(podGangName string, hasGate bool, inPodGang bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{},
		},
		Spec: corev1.PodSpec{},
	}

	if hasGate {
		pod.Spec.SchedulingGates = []corev1.PodSchedulingGate{
			{Name: podGangSchedulingGate},
		}
	}

	if inPodGang {
		pod.Labels[grovecorev1alpha1.LabelPodGangName] = podGangName
	}

	return pod
}

func createTestBasePodGang(individualPodGangName string, ready bool) *groveschedulerv1alpha1.PodGang {
	basePodGangName := extractBasePodGangName(individualPodGangName)

	podGroups := []groveschedulerv1alpha1.PodGroup{
		{
			Name:        "simple1-0-pcb",
			MinReplicas: 2,
		},
	}

	if !ready {
		// Add another PodGroup to make it more complex
		podGroups = append(podGroups, groveschedulerv1alpha1.PodGroup{
			Name:        "simple1-0-pcc",
			MinReplicas: 3,
		})
	}

	return &groveschedulerv1alpha1.PodGang{
		ObjectMeta: metav1.ObjectMeta{
			Name:      basePodGangName,
			Namespace: "default",
		},
		Spec: groveschedulerv1alpha1.PodGangSpec{
			PodGroups: podGroups,
		},
	}
}

func createTestBasePodGangWithPodCliques(podCliques []testPodClique) *groveschedulerv1alpha1.PodGang {
	podGroups := make([]groveschedulerv1alpha1.PodGroup, len(podCliques))
	for i, pclq := range podCliques {
		podGroups[i] = groveschedulerv1alpha1.PodGroup{
			Name:        pclq.name,
			MinReplicas: pclq.minAvailable,
		}
	}

	return &groveschedulerv1alpha1.PodGang{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple1-0",
			Namespace: "default",
		},
		Spec: groveschedulerv1alpha1.PodGangSpec{
			PodGroups: podGroups,
		},
	}
}

func createTestPodClique(name string, minAvailable, readyReplicas int32) *grovecorev1alpha1.PodClique {
	return &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(minAvailable),
		},
		Status: grovecorev1alpha1.PodCliqueStatus{
			ReadyReplicas: readyReplicas,
		},
	}
}
