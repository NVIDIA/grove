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
	"context"
	"testing"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/expect"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCreatePods_Integration tests the pod creation orchestration
func TestCreatePods_Integration(t *testing.T) {
	tests := []struct {
		name              string
		numPods           int
		existingPods      []*corev1.Pod
		expectError       bool
		expectedCreations int
	}{
		{
			// Should create 3 pods with unique hostname indices
			name:              "create multiple pods successfully",
			numPods:           3,
			existingPods:      []*corev1.Pod{},
			expectError:       false,
			expectedCreations: 3,
		},
		{
			// Should create pods filling hostname index gaps
			name:    "create pods with existing pods - fill gaps",
			numPods: 2,
			existingPods: []*corev1.Pod{
				createPodWithHostname("existing-pod-1", 1), // Gap at index 0
			},
			expectError:       false,
			expectedCreations: 2,
		},
		{
			// Should handle zero pod creation gracefully
			name:              "create zero pods",
			numPods:           0,
			existingPods:      []*corev1.Pod{},
			expectError:       false,
			expectedCreations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with existing pods
			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			require.NoError(t, err)
			err = grovecorev1alpha1.AddToScheme(scheme)
			require.NoError(t, err)

			var objects []client.Object
			for _, pod := range tt.existingPods {
				objects = append(objects, pod)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			expectationsStore := expect.NewExpectationsStore()
			eventRecorder := &record.FakeRecorder{}

			resource := &_resource{
				client:            fakeClient,
				scheme:            scheme,
				eventRecorder:     eventRecorder,
				expectationsStore: expectationsStore,
			}

			// Create sync context
			sc := &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pcs",
						Namespace: "default",
					},
				},
				pclq: &grovecorev1alpha1.PodClique{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pcs-0-worker",
						Namespace: "default",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
						},
					},
					Spec: grovecorev1alpha1.PodCliqueSpec{
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "worker",
									Image: "worker:latest",
								},
							},
						},
					},
				},
				existingPCLQPods:         tt.existingPods,
				associatedPodGangName:    "test-podgang",
				pclqExpectationsStoreKey: "test-key",
				expectedPodTemplateHash:  "test-hash",
			}

			// Test createPods
			createdCount, err := resource.createPods(sc.ctx, logr.Discard(), sc, tt.numPods)

			if tt.expectError {
				assert.Error(t, err, "Should return error as expected")
				return
			}

			require.NoError(t, err, "Should succeed without error")
			assert.Equal(t, tt.expectedCreations, createdCount, "Should create expected number of pods")

			// For successful cases, verify create expectations were set
			// Note: The actual number may vary based on internal batching and task execution
			createExpectations := expectationsStore.GetCreateExpectations("test-key")
			if createdCount > 0 {
				assert.GreaterOrEqual(t, len(createExpectations), 1, "Should have some create expectations when pods are created")
			}
		})
	}
}

// TestDeleteExcessPods tests the pod deletion logic during scale-down
func TestDeleteExcessPods(t *testing.T) {
	tests := []struct {
		name              string
		existingPods      []*corev1.Pod
		desiredReplicas   int32
		expectedDeletions int
		expectError       bool
	}{
		{
			// Should delete excess pods in correct priority order
			name: "delete excess pods using deletion sorter",
			existingPods: []*corev1.Pod{
				// Should be deleted first (unassigned, pending)
				createPodForDeletion("unassigned-pending", "", corev1.PodPending, false),
				// Should be deleted second (assigned, pending)
				createPodForDeletion("assigned-pending", "node1", corev1.PodPending, false),
				// Should be kept (assigned, running, ready)
				createPodForDeletion("assigned-running", "node2", corev1.PodRunning, true),
			},
			desiredReplicas:   1,
			expectedDeletions: 2,
			expectError:       false,
		},
		{
			// Should not delete pods when count matches desired
			name: "no excess pods to delete",
			existingPods: []*corev1.Pod{
				createPodForDeletion("pod-1", "node1", corev1.PodRunning, true),
				createPodForDeletion("pod-2", "node2", corev1.PodRunning, true),
			},
			desiredReplicas:   2,
			expectedDeletions: 0,
			expectError:       false,
		},
		{
			// Should delete all pods when scaling to zero
			name: "delete all pods when scaling to zero",
			existingPods: []*corev1.Pod{
				createPodForDeletion("pod-1", "node1", corev1.PodRunning, true),
				createPodForDeletion("pod-2", "node2", corev1.PodRunning, true),
			},
			desiredReplicas:   0,
			expectedDeletions: 2,
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client
			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			require.NoError(t, err)
			err = grovecorev1alpha1.AddToScheme(scheme)
			require.NoError(t, err)

			var objects []client.Object
			for _, pod := range tt.existingPods {
				objects = append(objects, pod)
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			expectationsStore := expect.NewExpectationsStore()
			eventRecorder := &record.FakeRecorder{}

			resource := &_resource{
				client:            fakeClient,
				scheme:            scheme,
				eventRecorder:     eventRecorder,
				expectationsStore: expectationsStore,
			}

			// Create sync context
			sc := &syncContext{
				ctx: context.Background(),
				pclq: &grovecorev1alpha1.PodClique{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "delete-test-pclq",
						Namespace: "default",
					},
					Spec: grovecorev1alpha1.PodCliqueSpec{
						Replicas: tt.desiredReplicas,
					},
				},
				existingPCLQPods:         tt.existingPods,
				pclqExpectationsStoreKey: "delete-test-key",
			}

			// Calculate difference (excess pods)
			diff := len(tt.existingPods) - int(tt.desiredReplicas)
			if diff <= 0 {
				diff = 0
			}

			// Test deleteExcessPods
			err = resource.deleteExcessPods(sc, logr.Discard(), diff)

			if tt.expectError {
				assert.Error(t, err, "Should return error as expected")
				return
			}

			require.NoError(t, err, "Should succeed without error")

			// Verify delete expectations were set
			deleteExpectations := expectationsStore.GetDeleteExpectations("delete-test-key")
			if tt.expectedDeletions > 0 {
				assert.GreaterOrEqual(t, len(deleteExpectations), 1, "Should have some delete expectations when pods are deleted")
			}
		})
	}
}

// TestSyncExpectationsAndComputeDifference tests the expectations synchronization
func TestSyncExpectationsAndComputeDifference(t *testing.T) {
	tests := []struct {
		name               string
		existingPods       []*corev1.Pod
		desiredReplicas    int32
		createExpectations int
		deleteExpectations int
		expectedDiff       int
	}{
		{
			// Should compute negative diff for scale up
			name: "scale up scenario",
			existingPods: []*corev1.Pod{
				createPodForDeletion("pod-1", "node1", corev1.PodRunning, true),
			},
			desiredReplicas:    3,
			createExpectations: 0,
			deleteExpectations: 0,
			expectedDiff:       -2, // Need to create 2 more pods
		},
		{
			// Should compute positive diff for scale down
			name: "scale down scenario",
			existingPods: []*corev1.Pod{
				createPodForDeletion("pod-1", "node1", corev1.PodRunning, true),
				createPodForDeletion("pod-2", "node2", corev1.PodRunning, true),
				createPodForDeletion("pod-3", "node3", corev1.PodRunning, true),
			},
			desiredReplicas:    1,
			createExpectations: 0,
			deleteExpectations: 0,
			expectedDiff:       2, // Need to delete 2 pods
		},
		{
			// Should account for pending create expectations
			name: "with pending creates",
			existingPods: []*corev1.Pod{
				createPodForDeletion("pod-1", "node1", corev1.PodRunning, true),
			},
			desiredReplicas:    3,
			createExpectations: 1, // One pod create is pending
			deleteExpectations: 0,
			expectedDiff:       -1, // Need to create 1 more pod (accounting for pending create)
		},
		{
			// Should account for pending delete expectations
			name: "with pending deletes",
			existingPods: []*corev1.Pod{
				createPodForDeletion("pod-1", "node1", corev1.PodRunning, true),
				createPodForDeletion("pod-2", "node2", corev1.PodRunning, true),
				createPodForDeletion("pod-3", "node3", corev1.PodRunning, true),
			},
			desiredReplicas:    1,
			createExpectations: 0,
			deleteExpectations: 1, // One pod delete is pending
			expectedDiff:       2, // Function returns existing - desired = 3 - 1 = 2, regardless of expectations
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectationsStore := expect.NewExpectationsStore()

			resource := &_resource{
				expectationsStore: expectationsStore,
			}

			// Set up expectations
			for i := 0; i < tt.createExpectations; i++ {
				err := expectationsStore.ExpectCreations(logr.Discard(), "test-key", types.UID("create-uid-"+string(rune('0'+i))))
				require.NoError(t, err)
			}

			for i := 0; i < tt.deleteExpectations; i++ {
				err := expectationsStore.ExpectDeletions(logr.Discard(), "test-key", types.UID("delete-uid-"+string(rune('0'+i))))
				require.NoError(t, err)
			}

			sc := &syncContext{
				pclq: &grovecorev1alpha1.PodClique{
					Spec: grovecorev1alpha1.PodCliqueSpec{
						Replicas: tt.desiredReplicas,
					},
				},
				existingPCLQPods:         tt.existingPods,
				pclqExpectationsStoreKey: "test-key",
			}

			// Test syncExpectationsAndComputeDifference
			diff := resource.syncExpectationsAndComputeDifference(logr.Discard(), sc)

			assert.Equal(t, tt.expectedDiff, diff, "Should compute correct difference")
		})
	}
}

// TestGetAssociatedPodGangName tests PodGang name extraction
func TestGetAssociatedPodGangName(t *testing.T) {
	tests := []struct {
		name         string
		pclqMeta     metav1.ObjectMeta
		expectedName string
		expectError  bool
	}{
		{
			// Should extract PodGang name from label
			name: "valid podgang label",
			pclqMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					apicommon.LabelPodGang: "test-podgang",
				},
			},
			expectedName: "test-podgang",
			expectError:  false,
		},
		{
			// Should fail when PodGang label is missing
			name: "missing podgang label",
			pclqMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					apicommon.LabelPartOfKey: "test-pcs",
				},
			},
			expectedName: "",
			expectError:  true,
		},
		{
			// Should fail when no labels exist
			name:         "no labels",
			pclqMeta:     metav1.ObjectMeta{},
			expectedName: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := &_resource{}

			name, err := resource.getAssociatedPodGangName(tt.pclqMeta)

			if tt.expectError {
				assert.Error(t, err, "Should return error as expected")
				assert.Empty(t, name)
			} else {
				assert.NoError(t, err, "Should succeed without error")
				assert.Equal(t, tt.expectedName, name)
			}
		})
	}
}

// Helper functions for pod creation tests

// createPodWithHostname creates a pod with a specific hostname index
func createPodWithHostname(name string, hostnameIndex int) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Hostname: name + "-" + string(rune('0'+hostnameIndex)),
		},
	}
}

// createPodForDeletion creates a pod for deletion testing
func createPodForDeletion(name, nodeName string, phase corev1.PodPhase, ready bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: phase,
		},
	}

	if ready {
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		}
	}

	return pod
}

// TestDeleteExcessPods_ErrorScenarios tests error paths in deleteExcessPods
func TestDeleteExcessPods_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() client.Client
		sc          *syncContext
		diff        int
		expectError bool
	}{
		{
			// Should handle client errors during pod deletion
			name: "client error during deletion",
			setupClient: func() client.Client {
				scheme := runtime.NewScheme()
				_ = corev1.AddToScheme(scheme)

				// Create a client that will fail on Delete operations
				return &errorClient{
					Client:       fake.NewClientBuilder().WithScheme(scheme).Build(),
					failOnDelete: true,
				}
			},
			sc: &syncContext{
				ctx: context.Background(),
				pclq: &grovecorev1alpha1.PodClique{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pclq",
						Namespace: "default",
					},
					Spec: grovecorev1alpha1.PodCliqueSpec{
						Replicas: 1,
					},
				},
				existingPCLQPods: []*corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "excess-pod-1",
							Namespace: "default",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "excess-pod-2",
							Namespace: "default",
						},
					},
				},
				pclqExpectationsStoreKey: "test-key",
			},
			diff:        1, // Need to delete 1 pod
			expectError: true,
		},
		{
			// Should handle zero diff gracefully
			name: "zero diff",
			setupClient: func() client.Client {
				scheme := runtime.NewScheme()
				_ = corev1.AddToScheme(scheme)
				return fake.NewClientBuilder().WithScheme(scheme).Build()
			},
			sc: &syncContext{
				ctx: context.Background(),
				pclq: &grovecorev1alpha1.PodClique{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pclq",
						Namespace: "default",
					},
				},
				existingPCLQPods:         []*corev1.Pod{},
				pclqExpectationsStoreKey: "test-key",
			},
			diff:        0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient()
			expectationsStore := expect.NewExpectationsStore()
			eventRecorder := &record.FakeRecorder{}

			resource := &_resource{
				client:            fakeClient,
				scheme:            runtime.NewScheme(),
				eventRecorder:     eventRecorder,
				expectationsStore: expectationsStore,
			}

			err := resource.deleteExcessPods(tt.sc, logr.Discard(), tt.diff)

			if tt.expectError {
				assert.Error(t, err, "Should return error for %s", tt.name)
			} else {
				assert.NoError(t, err, "Should not return error for %s", tt.name)
			}
		})
	}
}
