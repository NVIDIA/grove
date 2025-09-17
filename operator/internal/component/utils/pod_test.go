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

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestGetPCLQPods validates the retrieval and ownership filtering of pods belonging to a PodClique.
func TestGetPCLQPods(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet name for label generation
		pcsName string
		// PodClique object that owns the pods
		pclq *grovecorev1alpha1.PodClique
		// Existing pods in the cluster
		existingPods []corev1.Pod
		// Expected number of pods that should be returned
		expectedPodCount int
		// Expected names of pods that should be returned
		expectedPodNames []string
		// Whether an error is expected
		wantErr bool
	}{
		{
			// Should return pods that have correct labels and are owned by the PodClique
			name:    "returns owned pods with matching labels",
			pcsName: "test-pcs",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
					UID:       "pclq-uid-123",
				},
			},
			existingPods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "owned-pod-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
							apicommon.LabelPodClique:    "test-pclq",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "grove.io/v1alpha1",
								Kind:       "PodClique",
								Name:       "test-pclq",
								UID:        "pclq-uid-123",
								Controller: ptr.To(true),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "owned-pod-2",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
							apicommon.LabelPodClique:    "test-pclq",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "grove.io/v1alpha1",
								Kind:       "PodClique",
								Name:       "test-pclq",
								UID:        "pclq-uid-123",
								Controller: ptr.To(true),
							},
						},
					},
				},
			},
			expectedPodCount: 2,
			expectedPodNames: []string{"owned-pod-1", "owned-pod-2"},
		},
		{
			// Should filter out pods with matching labels but different owner
			name:    "excludes pods with wrong owner",
			pcsName: "test-pcs",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
					UID:       "pclq-uid-123",
				},
			},
			existingPods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-owner-pod",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPartOfKey:    "test-pcs",
							apicommon.LabelManagedByKey: "grove-operator",
							apicommon.LabelPodClique:    "test-pclq",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "grove.io/v1alpha1",
								Kind:       "PodClique",
								Name:       "other-pclq",
								UID:        "other-uid-456",
								Controller: ptr.To(true),
							},
						},
					},
				},
			},
			expectedPodCount: 0,
			expectedPodNames: []string{},
		},
		{
			// Should filter out pods with correct owner but missing labels
			name:    "excludes pods with missing labels",
			pcsName: "test-pcs",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
					UID:       "pclq-uid-123",
				},
			},
			existingPods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "missing-labels-pod",
						Namespace: "test-ns",
						Labels:    map[string]string{
							// Missing required labels
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "grove.io/v1alpha1",
								Kind:       "PodClique",
								Name:       "test-pclq",
								UID:        "pclq-uid-123",
								Controller: ptr.To(true),
							},
						},
					},
				},
			},
			expectedPodCount: 0,
			expectedPodNames: []string{},
		},
		{
			// Should handle empty pod list gracefully
			name:    "handles empty pod list",
			pcsName: "test-pcs",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
					UID:       "pclq-uid-123",
				},
			},
			existingPods:     []corev1.Pod{},
			expectedPodCount: 0,
			expectedPodNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert []corev1.Pod to []client.Object for the fake client
			objects := make([]client.Object, len(tt.existingPods))
			for i, pod := range tt.existingPods {
				podCopy := pod // Create a copy to avoid pointer issues
				objects[i] = &podCopy
			}

			fakeClient := testutils.SetupFakeClient(objects...)
			ctx := context.Background()

			pods, err := GetPCLQPods(ctx, fakeClient, tt.pcsName, tt.pclq)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, pods, tt.expectedPodCount)

			// Verify pod names match expected
			actualPodNames := make([]string, len(pods))
			for i, pod := range pods {
				actualPodNames[i] = pod.Name
			}
			assert.ElementsMatch(t, tt.expectedPodNames, actualPodNames)
		})
	}
}

// TestAddEnvVarsToContainers validates that environment variables are correctly appended to all containers.
func TestAddEnvVarsToContainers(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Initial containers with existing environment variables
		initialContainers []corev1.Container
		// Environment variables to add to all containers
		envVarsToAdd []corev1.EnvVar
		// Expected final state of containers after adding env vars
		expectedContainers []corev1.Container
	}{
		{
			// Should append new environment variables to containers with existing env vars
			name: "appends env vars to containers with existing env vars",
			initialContainers: []corev1.Container{
				{
					Name: "container1",
					Env: []corev1.EnvVar{
						{Name: "EXISTING_VAR", Value: "existing_value"},
					},
				},
				{
					Name: "container2",
					Env: []corev1.EnvVar{
						{Name: "ANOTHER_VAR", Value: "another_value"},
					},
				},
			},
			envVarsToAdd: []corev1.EnvVar{
				{Name: "NEW_VAR", Value: "new_value"},
				{Name: "SECOND_NEW_VAR", Value: "second_new_value"},
			},
			expectedContainers: []corev1.Container{
				{
					Name: "container1",
					Env: []corev1.EnvVar{
						{Name: "EXISTING_VAR", Value: "existing_value"},
						{Name: "NEW_VAR", Value: "new_value"},
						{Name: "SECOND_NEW_VAR", Value: "second_new_value"},
					},
				},
				{
					Name: "container2",
					Env: []corev1.EnvVar{
						{Name: "ANOTHER_VAR", Value: "another_value"},
						{Name: "NEW_VAR", Value: "new_value"},
						{Name: "SECOND_NEW_VAR", Value: "second_new_value"},
					},
				},
			},
		},
		{
			// Should add environment variables to containers with empty env slice
			name: "adds env vars to containers with no existing env vars",
			initialContainers: []corev1.Container{
				{Name: "container1", Env: []corev1.EnvVar{}},
				{Name: "container2", Env: nil},
			},
			envVarsToAdd: []corev1.EnvVar{
				{Name: "FIRST_VAR", Value: "first_value"},
			},
			expectedContainers: []corev1.Container{
				{
					Name: "container1",
					Env: []corev1.EnvVar{
						{Name: "FIRST_VAR", Value: "first_value"},
					},
				},
				{
					Name: "container2",
					Env: []corev1.EnvVar{
						{Name: "FIRST_VAR", Value: "first_value"},
					},
				},
			},
		},
		{
			// Should handle empty containers slice gracefully
			name:               "handles empty containers slice",
			initialContainers:  []corev1.Container{},
			envVarsToAdd:       []corev1.EnvVar{{Name: "TEST_VAR", Value: "test_value"}},
			expectedContainers: []corev1.Container{},
		},
		{
			// Should handle empty env vars to add gracefully
			name: "handles empty env vars to add",
			initialContainers: []corev1.Container{
				{
					Name: "container1",
					Env: []corev1.EnvVar{
						{Name: "EXISTING_VAR", Value: "existing_value"},
					},
				},
			},
			envVarsToAdd: []corev1.EnvVar{},
			expectedContainers: []corev1.Container{
				{
					Name: "container1",
					Env: []corev1.EnvVar{
						{Name: "EXISTING_VAR", Value: "existing_value"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a deep copy of initial containers to avoid modifying test data
			containers := make([]corev1.Container, len(tt.initialContainers))
			for i, container := range tt.initialContainers {
				containers[i] = corev1.Container{
					Name: container.Name,
					Env:  append([]corev1.EnvVar{}, container.Env...),
				}
			}

			AddEnvVarsToContainers(containers, tt.envVarsToAdd)

			assert.Equal(t, tt.expectedContainers, containers)
		})
	}
}

// TestPodsToObjectNames validates the conversion of pod objects to namespaced object name strings.
func TestPodsToObjectNames(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Input pods to convert to object names
		pods []*corev1.Pod
		// Expected output object name strings in "namespace/name" format
		expectedNames []string
	}{
		{
			// Should convert multiple pods to correct namespace/name format
			name: "converts multiple pods to object names",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "namespace1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: "namespace2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod3",
						Namespace: "namespace1",
					},
				},
			},
			expectedNames: []string{
				"namespace1/pod1",
				"namespace2/pod2",
				"namespace1/pod3",
			},
		},
		{
			// Should handle single pod correctly
			name: "converts single pod to object name",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "single-pod",
						Namespace: "single-namespace",
					},
				},
			},
			expectedNames: []string{"single-namespace/single-pod"},
		},
		{
			// Should handle empty pod slice gracefully
			name:          "handles empty pod slice",
			pods:          []*corev1.Pod{},
			expectedNames: []string{},
		},
		{
			// Should handle pods with same name in different namespaces
			name: "handles pods with same name in different namespaces",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "same-name",
						Namespace: "namespace-a",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "same-name",
						Namespace: "namespace-b",
					},
				},
			},
			expectedNames: []string{
				"namespace-a/same-name",
				"namespace-b/same-name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PodsToObjectNames(tt.pods)
			assert.Equal(t, tt.expectedNames, result)
		})
	}
}
