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
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestNew verifies that the New constructor correctly initializes the pod operator
// with all required dependencies and returns a properly configured component operator.
func TestNew(t *testing.T) {
	// Create mock dependencies
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = grovecorev1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	eventRecorder := &record.FakeRecorder{}
	expectationsStore := expect.NewExpectationsStore()

	// Test the constructor
	operator := New(fakeClient, scheme, eventRecorder, expectationsStore)

	// Verify the operator is properly initialized
	assert.NotNil(t, operator, "operator should not be nil")

	// Type assertion to access internal fields for verification
	resource, ok := operator.(*_resource)
	require.True(t, ok, "operator should be of type *_resource")

	assert.Equal(t, fakeClient, resource.client, "client should be set correctly")
	assert.Equal(t, scheme, resource.scheme, "scheme should be set correctly")
	assert.Equal(t, eventRecorder, resource.eventRecorder, "event recorder should be set correctly")
	assert.Equal(t, expectationsStore, resource.expectationsStore, "expectations store should be set correctly")
}

// TestGetExistingResourceNames tests the method that retrieves names of existing pods
// for a given PodClique. It should return only pods that are controlled by the PodClique.
func TestGetExistingResourceNames(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pclqMeta is the PodClique metadata used for the test
		pclqMeta metav1.ObjectMeta
		// existingPods are the pods that exist in the cluster before the test
		existingPods []corev1.Pod
		// expectedPodNames are the pod names that should be returned
		expectedPodNames []string
		// expectError indicates whether an error should occur
		expectError bool
	}{
		{
			// No existing pods should return empty list
			name: "no existing pods",
			pclqMeta: metav1.ObjectMeta{
				Name:      "test-pclq",
				Namespace: "default",
				UID:       "pclq-uid-123",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "test-pcs",
						Kind: "PodCliqueSet",
					},
				},
			},
			existingPods:     []corev1.Pod{},
			expectedPodNames: []string{},
			expectError:      false,
		},
		{
			// Only controlled pods should be returned
			name: "mixed controlled and uncontrolled pods",
			pclqMeta: metav1.ObjectMeta{
				Name:      "test-pclq",
				Namespace: "default",
				UID:       "pclq-uid-123",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "test-pcs",
						Kind: "PodCliqueSet",
					},
				},
			},
			existingPods: []corev1.Pod{
				// This pod is controlled by the PodClique
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "controlled-pod-1",
						Namespace: "default",
						Labels: map[string]string{
							apicommon.LabelPodClique:    "test-pclq",
							apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
							apicommon.LabelPartOfKey:    "test-pcs",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "core.grove.nvidia.com/v1alpha1",
								Kind:       "PodClique",
								Name:       "test-pclq",
								UID:        "pclq-uid-123",
								Controller: &[]bool{true}[0],
							},
						},
					},
				},
				// This pod has matching labels but different owner
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "different-owner-pod",
						Namespace: "default",
						Labels: map[string]string{
							apicommon.LabelPodClique:    "test-pclq",
							apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
							apicommon.LabelPartOfKey:    "test-pcs",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "core.grove.nvidia.com/v1alpha1",
								Kind:       "PodClique",
								Name:       "other-pclq",
								UID:        "other-uid-456",
								Controller: &[]bool{true}[0],
							},
						},
					},
				},
				// This pod has no matching labels
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unrelated-pod",
						Namespace: "default",
						Labels: map[string]string{
							"app": "some-app",
						},
					},
				},
			},
			expectedPodNames: []string{"controlled-pod-1"},
			expectError:      false,
		},
		{
			// Multiple controlled pods should all be returned
			name: "multiple controlled pods",
			pclqMeta: metav1.ObjectMeta{
				Name:      "multi-pod-pclq",
				Namespace: "test-ns",
				UID:       "multi-uid-789",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "multi-pcs",
						Kind: "PodCliqueSet",
					},
				},
			},
			existingPods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPodClique:    "multi-pod-pclq",
							apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
							apicommon.LabelPartOfKey:    "multi-pcs",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "core.grove.nvidia.com/v1alpha1",
								Kind:       "PodClique",
								Name:       "multi-pod-pclq",
								UID:        "multi-uid-789",
								Controller: &[]bool{true}[0],
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-2",
						Namespace: "test-ns",
						Labels: map[string]string{
							apicommon.LabelPodClique:    "multi-pod-pclq",
							apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
							apicommon.LabelPartOfKey:    "multi-pcs",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "core.grove.nvidia.com/v1alpha1",
								Kind:       "PodClique",
								Name:       "multi-pod-pclq",
								UID:        "multi-uid-789",
								Controller: &[]bool{true}[0],
							},
						},
					},
				},
			},
			expectedPodNames: []string{"pod-1", "pod-2"},
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with existing pods
			scheme := runtime.NewScheme()
			err := corev1.AddToScheme(scheme)
			require.NoError(t, err)

			var objects []client.Object
			for i := range tt.existingPods {
				objects = append(objects, &tt.existingPods[i])
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

			// Create the operator
			operator := &_resource{
				client: fakeClient,
				scheme: scheme,
			}

			// Test GetExistingResourceNames
			ctx := context.Background()
			podNames, err := operator.GetExistingResourceNames(ctx, logr.Discard(), tt.pclqMeta)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.ElementsMatch(t, tt.expectedPodNames, podNames)
			}
		})
	}
}

// TestDelete verifies the Delete method properly removes all pods belonging to a PodClique
// and cleans up the expectations store.
func TestDelete(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pclqMeta is the PodClique metadata for the test
		pclqMeta metav1.ObjectMeta
		// existingPods are pods that exist before deletion
		existingPods []corev1.Pod
		// expectError indicates whether an error should occur
		expectError bool
	}{
		{
			// Deleting with no existing pods should succeed
			name: "no existing pods",
			pclqMeta: metav1.ObjectMeta{
				Name:      "empty-pclq",
				Namespace: "default",
				UID:       "empty-uid",
			},
			existingPods: []corev1.Pod{},
			expectError:  false,
		},
		{
			// Should delete only pods with matching labels
			name: "delete matching pods only",
			pclqMeta: metav1.ObjectMeta{
				Name:      "target-pclq",
				Namespace: "default",
				UID:       "target-uid",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "owner-pcs",
						Kind: "PodCliqueSet",
					},
				},
			},
			existingPods: []corev1.Pod{
				// This pod should be deleted (matching labels)
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "target-pod-1",
						Namespace: "default",
						Labels: map[string]string{
							apicommon.LabelPodClique:    "target-pclq",
							apicommon.LabelPartOfKey:    "owner-pcs",
							apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
						},
					},
				},
				// This pod should not be deleted (different PodClique)
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-pod",
						Namespace: "default",
						Labels: map[string]string{
							apicommon.LabelPodClique:    "other-pclq",
							apicommon.LabelPartOfKey:    "owner-pcs",
							apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
						},
					},
				},
			},
			expectError: false,
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
			for i := range tt.existingPods {
				objects = append(objects, &tt.existingPods[i])
			}

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			expectationsStore := expect.NewExpectationsStore()

			// Create the operator
			operator := &_resource{
				client:            fakeClient,
				scheme:            scheme,
				eventRecorder:     &record.FakeRecorder{},
				expectationsStore: expectationsStore,
			}

			// Test Delete method
			ctx := context.Background()
			err = operator.Delete(ctx, logr.Discard(), tt.pclqMeta)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify that matching pods were deleted by checking remaining pods
				podList := &corev1.PodList{}
				err = fakeClient.List(ctx, podList, client.InNamespace(tt.pclqMeta.Namespace))
				require.NoError(t, err)

				// Count pods that should have been deleted vs remaining
				for _, pod := range podList.Items {
					if pod.Labels[apicommon.LabelPodClique] == tt.pclqMeta.Name {
						t.Errorf("Pod %s with matching PodClique label should have been deleted", pod.Name)
					}
				}
			}
		})
	}
}

// TestGetSelectorLabelsForPods tests the function that creates label selectors
// for finding pods belonging to a specific PodClique.
func TestGetSelectorLabelsForPods(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pclqMeta is the PodClique metadata for the test
		pclqMeta metav1.ObjectMeta
		// expectedLabels are the labels that should be returned for selection
		expectedLabels map[string]string
	}{
		{
			// Basic PodClique without owner should include default labels
			name: "basic podclique",
			pclqMeta: metav1.ObjectMeta{
				Name:      "basic-pclq",
				Namespace: "default",
			},
			expectedLabels: map[string]string{
				apicommon.LabelPodClique:    "basic-pclq",
				apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:    "", // Empty when no owner
			},
		},
		{
			// PodClique with owner should include owner name in labels
			name: "podclique with owner",
			pclqMeta: metav1.ObjectMeta{
				Name:      "owned-pclq",
				Namespace: "test-ns",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name: "parent-pcs",
						Kind: "PodCliqueSet",
					},
				},
			},
			expectedLabels: map[string]string{
				apicommon.LabelPodClique:    "owned-pclq",
				apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:    "parent-pcs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := getSelectorLabelsForPods(tt.pclqMeta)

			// Verify all expected labels are present
			for key, expectedValue := range tt.expectedLabels {
				actualValue, exists := labels[key]
				assert.True(t, exists, "Label %s should exist", key)
				assert.Equal(t, expectedValue, actualValue, "Label %s should have value %s", key, expectedValue)
			}
		})
	}
}

// TestGetLabels tests the function that constructs complete label sets for pods
// including Grove-specific labels and user-provided labels.
func TestGetLabels(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pclqMeta is the PodClique metadata for the test
		pclqMeta metav1.ObjectMeta
		// pcsName is the PodCliqueSet name
		pcsName string
		// podGangName is the PodGang name
		podGangName string
		// pcsReplicaIndex is the PodCliqueSet replica index
		pcsReplicaIndex int
		// expectedLabels are the labels that should be included
		expectedLabels map[string]string
	}{
		{
			// Basic label construction with minimal inputs
			name: "basic labels",
			pclqMeta: metav1.ObjectMeta{
				Name:      "test-pclq",
				Namespace: "default",
			},
			pcsName:         "test-pcs",
			podGangName:     "test-podgang",
			pcsReplicaIndex: 0,
			expectedLabels: map[string]string{
				apicommon.LabelPodClique:                "test-pclq",
				apicommon.LabelPodCliqueSetReplicaIndex: "0",
				apicommon.LabelPodGang:                  "test-podgang",
				apicommon.LabelManagedByKey:             apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                "test-pcs",
			},
		},
		{
			// Labels should include user-provided labels from PodClique
			name: "with user labels",
			pclqMeta: metav1.ObjectMeta{
				Name:      "labeled-pclq",
				Namespace: "default",
				Labels: map[string]string{
					"app":         "my-app",
					"version":     "v1.0.0",
					"environment": "test",
				},
			},
			pcsName:         "labeled-pcs",
			podGangName:     "labeled-podgang",
			pcsReplicaIndex: 2,
			expectedLabels: map[string]string{
				apicommon.LabelPodClique:                "labeled-pclq",
				apicommon.LabelPodCliqueSetReplicaIndex: "2",
				apicommon.LabelPodGang:                  "labeled-podgang",
				apicommon.LabelManagedByKey:             apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                "labeled-pcs",
				"app":                                   "my-app",
				"version":                               "v1.0.0",
				"environment":                           "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := getLabels(tt.pclqMeta, tt.pcsName, tt.podGangName, tt.pcsReplicaIndex)

			// Verify all expected labels are present with correct values
			for key, expectedValue := range tt.expectedLabels {
				actualValue, exists := labels[key]
				assert.True(t, exists, "Label %s should exist", key)
				assert.Equal(t, expectedValue, actualValue, "Label %s should have value %s", key, expectedValue)
			}

			// Verify no unexpected labels were added (except for Grove defaults)
			expectedCount := len(tt.expectedLabels)
			actualCount := len(labels)
			assert.Equal(t, expectedCount, actualCount, "Should have exactly %d labels", expectedCount)
		})
	}
}

// TestConfigurePodHostname tests the function that sets pod hostname and subdomain
// for service discovery within the PodClique's headless service.
func TestConfigurePodHostname(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pcsName is the PodCliqueSet name
		pcsName string
		// pcsReplicaIndex is the PodCliqueSet replica index
		pcsReplicaIndex int
		// pclqName is the PodClique name
		pclqName string
		// podIndex is the pod index within the PodClique
		podIndex int
		// expectedHostname is the expected pod hostname
		expectedHostname string
		// expectedSubdomain is the expected pod subdomain
		expectedSubdomain string
	}{
		{
			// Basic hostname configuration
			name:              "basic hostname",
			pcsName:           "test-pcs",
			pcsReplicaIndex:   0,
			pclqName:          "test-pclq",
			podIndex:          0,
			expectedHostname:  "test-pclq-0",
			expectedSubdomain: "test-pcs-0",
		},
		{
			// Different indices should produce different hostnames
			name:              "different indices",
			pcsName:           "prod-pcs",
			pcsReplicaIndex:   2,
			pclqName:          "worker-pclq",
			podIndex:          5,
			expectedHostname:  "worker-pclq-5",
			expectedSubdomain: "prod-pcs-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{}

			configurePodHostname(tt.pcsName, tt.pcsReplicaIndex, tt.pclqName, pod, tt.podIndex)

			assert.Equal(t, tt.expectedHostname, pod.Spec.Hostname)
			assert.Equal(t, tt.expectedSubdomain, pod.Spec.Subdomain)
		})
	}
}
