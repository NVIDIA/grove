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
	"os"
	"testing"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/expect"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCreatePodCreationTask tests the function that creates pod creation tasks
// for concurrent pod creation operations.
func TestCreatePodCreationTask(t *testing.T) {
	// Setup scheme
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = grovecorev1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	tests := []struct {
		// name describes the test scenario
		name string
		// pcs is the PodCliqueSet for the task
		pcs *grovecorev1alpha1.PodCliqueSet
		// pclq is the PodClique for the task
		pclq *grovecorev1alpha1.PodClique
		// podGangName is the PodGang name
		podGangName string
		// taskIndex is the task index for naming
		taskIndex int
		// podHostNameIndex is the pod hostname index
		podHostNameIndex int
		// setInitEnvVar indicates whether to set init container env var
		setInitEnvVar bool
		// initEnvValue is the init container env var value
		initEnvValue string
		// expectTaskError indicates whether the task execution should fail
		expectTaskError bool
		// expectPodCreated indicates whether a pod should be created
		expectPodCreated bool
	}{
		{
			// Successful pod creation without init container
			name: "successful pod creation without init container",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpcs",
					Namespace: "default",
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpcs-0-worker",
					Namespace: "default",
					Labels: map[string]string{
						apicommon.LabelPartOfKey:    "testpcs",
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
					StartsAfter: []string{}, // No dependencies
				},
			},
			podGangName:      "test-podgang",
			taskIndex:        0,
			podHostNameIndex: 0,
			setInitEnvVar:    false,
			initEnvValue:     "",
			expectTaskError:  false,
			expectPodCreated: true,
		},
		{
			// Successful pod creation with init container
			name: "successful pod creation with init container",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "orderedpcs",
					Namespace: "default",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "dependency",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: &[]int32{1}[0],
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "orderedpcs-1-worker",
					Namespace: "default",
					Labels: map[string]string{
						apicommon.LabelPartOfKey:    "orderedpcs",
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
					StartsAfter: []string{"dependency"},
				},
			},
			podGangName:      "ordered-podgang",
			taskIndex:        1,
			podHostNameIndex: 2,
			setInitEnvVar:    true,
			initEnvValue:     "gcr.io/grove/init",
			expectTaskError:  false,
			expectPodCreated: true,
		},
		{
			// Failed pod creation due to missing init container env var
			name: "failed pod creation - missing init env var",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fail-pcs",
					Namespace: "default",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "dep",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: &[]int32{1}[0],
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fail-pcs-0-fail-pclq",
					Namespace: "default",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "fail",
								Image: "fail:latest",
							},
						},
					},
					StartsAfter: []string{"dep"},
				},
			},
			podGangName:      "fail-podgang",
			taskIndex:        0,
			podHostNameIndex: 0,
			setInitEnvVar:    false,
			initEnvValue:     "",
			expectTaskError:  true,
			expectPodCreated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variable for init container
			originalValue := os.Getenv(envVarInitContainerImage)
			defer func() {
				if originalValue != "" {
					os.Setenv(envVarInitContainerImage, originalValue)
				} else {
					os.Unsetenv(envVarInitContainerImage)
				}
			}()

			if tt.setInitEnvVar {
				os.Setenv(envVarInitContainerImage, tt.initEnvValue)
			} else {
				os.Unsetenv(envVarInitContainerImage)
			}

			// Create fake client
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			expectationsStore := expect.NewExpectationsStore()
			eventRecorder := &record.FakeRecorder{}

			// Create resource
			resource := &_resource{
				client:            fakeClient,
				scheme:            scheme,
				eventRecorder:     eventRecorder,
				expectationsStore: expectationsStore,
			}

			// Create the task
			task := resource.createPodCreationTask(
				logr.Discard(),
				tt.pcs,
				tt.pclq,
				tt.podGangName,
				"test-expectations-key",
				tt.taskIndex,
				tt.podHostNameIndex,
			)

			// Verify task name
			expectedTaskName := "CreatePod-" + tt.pclq.Name + "-" + string(rune('0'+tt.taskIndex))
			assert.Equal(t, expectedTaskName, task.Name)

			// Execute the task
			ctx := context.Background()
			err := task.Fn(ctx)

			if tt.expectTaskError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectPodCreated {
				// Verify pod was created
				podList := &corev1.PodList{}
				err = fakeClient.List(ctx, podList, client.InNamespace(tt.pclq.Namespace))
				require.NoError(t, err)
				assert.Len(t, podList.Items, 1)

				pod := podList.Items[0]
				assert.Contains(t, pod.Name, tt.pclq.Name)
				assert.Equal(t, tt.pclq.Namespace, pod.Namespace)

				// Verify controller reference
				require.Len(t, pod.OwnerReferences, 1)
				assert.Equal(t, tt.pclq.Name, pod.OwnerReferences[0].Name)
			}
		})
	}
}

// TestCreatePodDeletionTask tests the function that creates pod deletion tasks
// for concurrent pod deletion operations.
func TestCreatePodDeletionTask(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// podExists indicates whether the pod exists before deletion
		podExists bool
		// expectTaskError indicates whether the task should fail
		expectTaskError bool
		// expectPodDeleted indicates whether the pod should be deleted
		expectPodDeleted bool
	}{
		{
			// Successful pod deletion when pod exists
			name:             "successful pod deletion",
			podExists:        true,
			expectTaskError:  false,
			expectPodDeleted: true,
		},
		{
			// Pod deletion when pod doesn't exist should succeed (idempotent)
			name:             "pod deletion when pod not found",
			podExists:        false,
			expectTaskError:  false,
			expectPodDeleted: false, // Already deleted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup scheme
			scheme := runtime.NewScheme()
			schemeErr := corev1.AddToScheme(scheme)
			require.NoError(t, schemeErr)

			// Create test pod
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					UID:       types.UID("test-uid-123"),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: "test:latest",
						},
					},
				},
			}

			// Create test PodClique
			pclq := &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "default",
				},
			}

			// Create fake client with or without the pod
			var objects []client.Object
			if tt.podExists {
				objects = append(objects, pod)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			expectationsStore := expect.NewExpectationsStore()
			eventRecorder := &record.FakeRecorder{}

			// Create resource
			resource := &_resource{
				client:            fakeClient,
				scheme:            scheme,
				eventRecorder:     eventRecorder,
				expectationsStore: expectationsStore,
			}

			// Create the task
			task := resource.createPodDeletionTask(
				logr.Discard(),
				pclq,
				pod,
				"test-expectations-key",
			)

			// Verify task name
			expectedTaskName := "DeletePod-" + pod.Name
			assert.Equal(t, expectedTaskName, task.Name)

			// Execute the task
			ctx := context.Background()
			err := task.Fn(ctx)

			if tt.expectTaskError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectPodDeleted {
				// Verify pod has deletion timestamp (fake client doesn't actually delete)
				updatedPod := &corev1.Pod{}
				err = fakeClient.Get(ctx, client.ObjectKeyFromObject(pod), updatedPod)
				if err == nil {
					// If pod still exists, it should have deletion timestamp
					assert.NotNil(t, updatedPod.DeletionTimestamp)
				} else {
					// Pod might be fully deleted, which is also valid
					assert.True(t, apierrors.IsNotFound(err))
				}
			}
		})
	}
}

// TestCreatePodDeletionTasks tests the convenience function that creates multiple
// pod deletion tasks for batch operations.
func TestCreatePodDeletionTasks(t *testing.T) {
	// Create test pods
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-1",
				Namespace: "default",
				UID:       types.UID("uid-1"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-2",
				Namespace: "default",
				UID:       types.UID("uid-2"),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-3",
				Namespace: "default",
				UID:       types.UID("uid-3"),
			},
		},
	}

	// Create test PodClique
	pclq := &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pclq",
			Namespace: "default",
		},
	}

	// Create resource
	resource := &_resource{
		client:            fake.NewClientBuilder().Build(),
		eventRecorder:     &record.FakeRecorder{},
		expectationsStore: expect.NewExpectationsStore(),
	}

	// Test creating multiple deletion tasks
	tasks := resource.createPodDeletionTasks(
		logr.Discard(),
		pclq,
		pods,
		"test-expectations-key",
	)

	// Verify correct number of tasks created
	assert.Len(t, tasks, len(pods))

	// Verify each task has correct name and corresponds to correct pod
	expectedTaskNames := []string{
		"DeletePod-pod-1",
		"DeletePod-pod-2",
		"DeletePod-pod-3",
	}

	for i, task := range tasks {
		assert.Equal(t, expectedTaskNames[i], task.Name)
	}
}

// TestCreatePodCreationTask_ErrorHandling tests error scenarios in pod creation tasks
// including API server errors and invalid configurations.
func TestCreatePodCreationTask_ErrorHandling(t *testing.T) {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = grovecorev1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	// For testing error handling, we'll test the buildResource error path
	// by creating an invalid PodClique name format
	expectationsStore := expect.NewExpectationsStore()
	eventRecorder := &record.FakeRecorder{}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	resource := &_resource{
		client:            fakeClient,
		scheme:            scheme,
		eventRecorder:     eventRecorder,
		expectationsStore: expectationsStore,
	}

	pcs := &grovecorev1alpha1.PodCliqueSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "error-pcs",
			Namespace: "default",
		},
	}

	// Create PodClique with invalid name format that will cause buildResource to fail
	pclq := &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-name-format", // Missing proper replica index format
			Namespace: "default",
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "error",
						Image: "error:latest",
					},
				},
			},
		},
	}

	task := resource.createPodCreationTask(
		logr.Discard(),
		pcs,
		pclq,
		"error-podgang",
		"error-expectations-key",
		0,
		0,
	)

	// Execute the task - should fail due to invalid PodClique name format
	ctx := context.Background()
	err = task.Fn(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build Pod resource")
}
