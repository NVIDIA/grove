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
	_ "github.com/NVIDIA/grove/operator/test/utils"

	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestGetPodGangSelectorLabels validates generation of selector labels for PodGang objects.
func TestGetPodGangSelectorLabels(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet object metadata to generate labels from
		pcsObjMeta metav1.ObjectMeta
		// Expected label key-value pairs
		expectedLabels map[string]string
	}{
		{
			// Should generate correct labels for PodGang selection
			name: "generates correct selector labels",
			pcsObjMeta: metav1.ObjectMeta{
				Name:      "test-pcs",
				Namespace: "test-ns",
			},
			expectedLabels: map[string]string{
				apicommon.LabelPartOfKey:    "test-pcs",
				apicommon.LabelManagedByKey: "grove-operator",
				apicommon.LabelComponentKey: apicommon.LabelComponentNamePodGang,
			},
		},
		{
			// Should handle PodCliqueSet with different name
			name: "handles different PodCliqueSet name",
			pcsObjMeta: metav1.ObjectMeta{
				Name:      "another-pcs",
				Namespace: "another-ns",
			},
			expectedLabels: map[string]string{
				apicommon.LabelPartOfKey:    "another-pcs",
				apicommon.LabelManagedByKey: "grove-operator",
				apicommon.LabelComponentKey: apicommon.LabelComponentNamePodGang,
			},
		},
		{
			// Should handle empty PodCliqueSet name
			name: "handles empty PodCliqueSet name",
			pcsObjMeta: metav1.ObjectMeta{
				Name:      "",
				Namespace: "test-ns",
			},
			expectedLabels: map[string]string{
				apicommon.LabelPartOfKey:    "",
				apicommon.LabelManagedByKey: "grove-operator",
				apicommon.LabelComponentKey: apicommon.LabelComponentNamePodGang,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPodGangSelectorLabels(tt.pcsObjMeta)
			assert.Equal(t, tt.expectedLabels, result)
		})
	}
}

// TestGetPodGang validates retrieval of PodGang objects by name and namespace.
func TestGetPodGang(t *testing.T) {
	// Setup scheme with grove scheduler types
	scheme := runtime.NewScheme()
	utilruntime.Must(groveschedulerv1alpha1.AddToScheme(scheme))

	tests := []struct {
		// Test case description
		name string
		// PodGang name to retrieve
		podGangName string
		// Namespace to search in
		namespace string
		// Existing PodGang objects in the cluster
		existingPodGangs []groveschedulerv1alpha1.PodGang
		// Expected PodGang name if found
		expectedPodGangName string
		// Whether an error is expected
		wantErr bool
		// Expected error type if error expected
		expectNotFound bool
	}{
		{
			// Should retrieve existing PodGang successfully
			name:        "retrieves existing PodGang",
			podGangName: "test-podgang",
			namespace:   "test-ns",
			existingPodGangs: []groveschedulerv1alpha1.PodGang{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-podgang",
						Namespace: "test-ns",
					},
					Spec: groveschedulerv1alpha1.PodGangSpec{
						PodGroups: []groveschedulerv1alpha1.PodGroup{
							{
								Name:        "test-group",
								MinReplicas: 2,
							},
						},
					},
				},
			},
			expectedPodGangName: "test-podgang",
			wantErr:             false,
		},
		{
			// Should return error for non-existent PodGang
			name:             "returns error for non-existent PodGang",
			podGangName:      "nonexistent-podgang",
			namespace:        "test-ns",
			existingPodGangs: []groveschedulerv1alpha1.PodGang{},
			wantErr:          true,
			expectNotFound:   true,
		},
		{
			// Should return error for PodGang in different namespace
			name:        "returns error for PodGang in different namespace",
			podGangName: "test-podgang",
			namespace:   "test-ns",
			existingPodGangs: []groveschedulerv1alpha1.PodGang{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-podgang",
						Namespace: "other-ns", // Different namespace
					},
				},
			},
			wantErr:        true,
			expectNotFound: true,
		},
		{
			// Should retrieve correct PodGang when multiple exist
			name:        "retrieves correct PodGang among multiple",
			podGangName: "target-podgang",
			namespace:   "test-ns",
			existingPodGangs: []groveschedulerv1alpha1.PodGang{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-podgang",
						Namespace: "test-ns",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "target-podgang",
						Namespace: "test-ns",
					},
					Spec: groveschedulerv1alpha1.PodGangSpec{
						PodGroups: []groveschedulerv1alpha1.PodGroup{
							{
								Name:        "target-group",
								MinReplicas: 3,
							},
						},
					},
				},
			},
			expectedPodGangName: "target-podgang",
			wantErr:             false,
		},
		{
			// Should handle empty PodGang name
			name:        "handles empty PodGang name",
			podGangName: "",
			namespace:   "test-ns",
			existingPodGangs: []groveschedulerv1alpha1.PodGang{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "",
						Namespace: "test-ns",
					},
				},
			},
			expectedPodGangName: "",
			wantErr:             false,
		},
		{
			// Should handle empty namespace
			name:        "handles empty namespace",
			podGangName: "test-podgang",
			namespace:   "",
			existingPodGangs: []groveschedulerv1alpha1.PodGang{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-podgang",
						Namespace: "",
					},
				},
			},
			expectedPodGangName: "test-podgang",
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to client.Object for fake client
			objects := make([]client.Object, len(tt.existingPodGangs))
			for i, podGang := range tt.existingPodGangs {
				podGangCopy := podGang
				objects[i] = &podGangCopy
			}

			// Create fake client with grove scheduler scheme
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			ctx := context.Background()

			result, err := GetPodGang(ctx, fakeClient, tt.podGangName, tt.namespace)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectNotFound {
					// Check that it's a NotFound error
					assert.Contains(t, err.Error(), "not found")
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedPodGangName, result.Name)
			assert.Equal(t, tt.namespace, result.Namespace)
		})
	}
}

// TestGetPodGangWithComplexObjects validates GetPodGang with more complex object structures.
func TestGetPodGangWithComplexObjects(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(groveschedulerv1alpha1.AddToScheme(scheme))

	// Create a complex PodGang with all fields populated
	complexPodGang := &groveschedulerv1alpha1.PodGang{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "complex-podgang",
			Namespace: "complex-ns",
			Labels: map[string]string{
				"app":                       "test-app",
				apicommon.LabelPartOfKey:    "test-pcs",
				apicommon.LabelComponentKey: apicommon.LabelComponentNamePodGang,
			},
			Annotations: map[string]string{
				"description": "Test PodGang with complex structure",
			},
		},
		Spec: groveschedulerv1alpha1.PodGangSpec{
			PodGroups: []groveschedulerv1alpha1.PodGroup{
				{
					Name:        "complex-group",
					MinReplicas: 2,
					PodReferences: []groveschedulerv1alpha1.NamespacedName{
						{Namespace: "complex-ns", Name: "pod-1"},
						{Namespace: "complex-ns", Name: "pod-2"},
					},
				},
			},
		},
		Status: groveschedulerv1alpha1.PodGangStatus{
			Phase: groveschedulerv1alpha1.PodGangPhaseRunning,
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(complexPodGang).
		Build()

	ctx := context.Background()

	// Test retrieval of complex PodGang
	result, err := GetPodGang(ctx, fakeClient, "complex-podgang", "complex-ns")

	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify all fields are preserved
	assert.Equal(t, "complex-podgang", result.Name)
	assert.Equal(t, "complex-ns", result.Namespace)
	assert.Equal(t, "test-app", result.Labels["app"])
	assert.Equal(t, "test-pcs", result.Labels[apicommon.LabelPartOfKey])
	assert.Equal(t, "Test PodGang with complex structure", result.Annotations["description"])
	assert.Len(t, result.Spec.PodGroups, 1)
	assert.Equal(t, "complex-group", result.Spec.PodGroups[0].Name)
	assert.Equal(t, int32(2), result.Spec.PodGroups[0].MinReplicas)
	assert.Len(t, result.Spec.PodGroups[0].PodReferences, 2)
	assert.Equal(t, groveschedulerv1alpha1.PodGangPhaseRunning, result.Status.Phase)
}
