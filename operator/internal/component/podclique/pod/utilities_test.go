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

	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/stretchr/testify/assert"
)

// TestGetPodNamesUpdatedInAssociatedPodGang tests the function that extracts
// pod names from PodGang specifications.
func TestGetPodNamesUpdatedInAssociatedPodGang(t *testing.T) {
	tests := []struct {
		name             string
		existingPodGang  *groveschedulerv1alpha1.PodGang
		pclqFQN          string
		expectedPodNames []string
	}{
		{
			// Should return nil when PodGang is nil
			name:             "nil podgang returns empty",
			existingPodGang:  nil,
			pclqFQN:          "test-pclq",
			expectedPodNames: nil,
		},
		{
			// Should return nil when no matching PodGroup found
			name: "podgang without matching podgroup returns empty",
			existingPodGang: &groveschedulerv1alpha1.PodGang{
				Spec: groveschedulerv1alpha1.PodGangSpec{
					PodGroups: []groveschedulerv1alpha1.PodGroup{
						{
							Name: "different-pclq",
							PodReferences: []groveschedulerv1alpha1.NamespacedName{
								{Name: "pod-1", Namespace: "default"},
							},
						},
					},
				},
			},
			pclqFQN:          "test-pclq",
			expectedPodNames: nil,
		},
		{
			// Should return pod names from matching PodGroup
			name: "podgang with matching podgroup returns pod names",
			existingPodGang: &groveschedulerv1alpha1.PodGang{
				Spec: groveschedulerv1alpha1.PodGangSpec{
					PodGroups: []groveschedulerv1alpha1.PodGroup{
						{
							Name: "test-pclq",
							PodReferences: []groveschedulerv1alpha1.NamespacedName{
								{Name: "pod-1", Namespace: "default"},
								{Name: "pod-2", Namespace: "default"},
								{Name: "pod-3", Namespace: "default"},
							},
						},
						{
							Name: "other-pclq",
							PodReferences: []groveschedulerv1alpha1.NamespacedName{
								{Name: "other-pod", Namespace: "default"},
							},
						},
					},
				},
			},
			pclqFQN:          "test-pclq",
			expectedPodNames: []string{"pod-1", "pod-2", "pod-3"},
		},
		{
			// Should return empty slice when PodGroup has no references
			name: "podgang with empty pod references returns empty",
			existingPodGang: &groveschedulerv1alpha1.PodGang{
				Spec: groveschedulerv1alpha1.PodGangSpec{
					PodGroups: []groveschedulerv1alpha1.PodGroup{
						{
							Name:          "test-pclq",
							PodReferences: []groveschedulerv1alpha1.NamespacedName{},
						},
					},
				},
			},
			pclqFQN:          "test-pclq",
			expectedPodNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := &_resource{}

			result := resource.getPodNamesUpdatedInAssociatedPodGang(tt.existingPodGang, tt.pclqFQN)

			assert.Equal(t, tt.expectedPodNames, result, "Should return expected pod names")
		})
	}
}
