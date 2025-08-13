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
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test helper functions
func newOwnerReference(kind, name string, isController bool) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "grove.io/v1alpha1",
		Kind:       kind,
		Name:       name,
		UID:        uuid.NewUUID(),
		Controller: ptr.To(isController),
	}
}

func TestHasExpectedOwner(t *testing.T) {
	testCases := []struct {
		description       string
		expectedOwnerKind string
		ownerRefs         []metav1.OwnerReference
		expected          bool
	}{
		{
			description:       "should return true when single owner matches expected kind",
			expectedOwnerKind: "PodGangSet",
			ownerRefs:         []metav1.OwnerReference{newOwnerReference("PodGangSet", "test-pgs", true)},
			expected:          true,
		},
		{
			description:       "should return false when single owner does not match expected kind",
			expectedOwnerKind: "PodGangSet",
			ownerRefs:         []metav1.OwnerReference{newOwnerReference("PodCliqueScalingGroup", "test-pcsg", true)},
			expected:          false,
		},
		{
			description:       "should return false when no owner references exist",
			expectedOwnerKind: "PodGangSet",
			ownerRefs:         []metav1.OwnerReference{},
			expected:          false,
		},
		{
			description:       "should return false when owner references is nil",
			expectedOwnerKind: "PodGangSet",
			ownerRefs:         nil,
			expected:          false,
		},
		{
			description:       "should return false when multiple owner references exist",
			expectedOwnerKind: "PodGangSet",
			ownerRefs: []metav1.OwnerReference{
				newOwnerReference("PodGangSet", "test-pgs", true),
				newOwnerReference("PodCliqueScalingGroup", "test-pcsg", false),
			},
			expected: false,
		},
		{
			description:       "should return true when single owner matches with different case",
			expectedOwnerKind: "PodGangSet",
			ownerRefs:         []metav1.OwnerReference{newOwnerReference("PodGangSet", "test-pgs", false)},
			expected:          true,
		},
		{
			description:       "should return false when kind is empty string",
			expectedOwnerKind: "",
			ownerRefs:         []metav1.OwnerReference{newOwnerReference("PodGangSet", "test-pgs", true)},
			expected:          false,
		},
		{
			description:       "should return false when owner reference kind is empty",
			expectedOwnerKind: "PodGangSet",
			ownerRefs:         []metav1.OwnerReference{newOwnerReference("", "test-pgs", true)},
			expected:          false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := HasExpectedOwner(tc.expectedOwnerKind, tc.ownerRefs)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsManagedByGrove(t *testing.T) {
	testCases := []struct {
		description string
		labels      map[string]string
		expected    bool
	}{
		{
			description: "should return true when managed-by label has correct value",
			labels: map[string]string{
				grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
			},
			expected: true,
		},
		{
			description: "should return false when managed-by label has incorrect value",
			labels: map[string]string{
				grovecorev1alpha1.LabelManagedByKey: "other-operator",
			},
			expected: false,
		},
		{
			description: "should return false when managed-by label is missing",
			labels: map[string]string{
				"app":     "test-app",
				"version": "v1.0",
			},
			expected: false,
		},
		{
			description: "should return false when labels map is empty",
			labels:      map[string]string{},
			expected:    false,
		},
		{
			description: "should return false when labels map is nil",
			labels:      nil,
			expected:    false,
		},
		{
			description: "should return true when managed-by label is correct with other labels",
			labels: map[string]string{
				grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
				"app":                               "test-app",
				"version":                           "v1.0",
				"environment":                       "test",
			},
			expected: true,
		},
		{
			description: "should return false when managed-by label has empty value",
			labels: map[string]string{
				grovecorev1alpha1.LabelManagedByKey: "",
			},
			expected: false,
		},
		{
			description: "should return false when managed-by label has whitespace value",
			labels: map[string]string{
				grovecorev1alpha1.LabelManagedByKey: "  ",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := IsManagedByGrove(tc.labels)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsManagedPodClique(t *testing.T) {
	testCases := []struct {
		description        string
		obj                client.Object
		expectedOwnerKinds []string
		expected           bool
	}{
		{
			description: "should return true when PodClique is managed by Grove with correct owner",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managed-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
					},
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("PodGangSet", "test-pgs", true),
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet"},
			expected:           true,
		},
		{
			description: "should return false when PodClique is not managed by Grove",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unmanaged-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: "other-operator",
					},
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("PodGangSet", "test-pgs", true),
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet"},
			expected:           false,
		},
		{
			description: "should return false when PodClique has wrong owner kind",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wrong-owner-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
					},
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("WrongKind", "test-wrong", true),
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet"},
			expected:           false,
		},
		{
			description: "should return true when PodClique matches one of multiple expected owner kinds",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pcsg-owned-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
					},
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("PodCliqueScalingGroup", "test-pcsg", true),
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet", "PodCliqueScalingGroup"},
			expected:           true,
		},
		{
			description: "should return false when object is not a PodClique",
			obj: &grovecorev1alpha1.PodGangSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pgs",
					Namespace: "different-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
					},
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("PodGangSet", "test-pgs", true),
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet"},
			expected:           false,
		},
		{
			description: "should return false when PodClique has no labels",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-labels-pclq",
					Namespace: "test-ns",
					Labels:    nil,
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("PodGangSet", "test-pgs", true),
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet"},
			expected:           false,
		},
		{
			description: "should return false when PodClique has no owner references",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-owners-pclq",
					Namespace: "another-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet"},
			expected:           false,
		},
		{
			description: "should return false when no expected owner kinds provided",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-expected-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
					},
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("PodGangSet", "test-pgs", true),
					},
				},
			},
			expectedOwnerKinds: []string{},
			expected:           false,
		},
		{
			description: "should return false when PodClique has multiple owners",
			obj: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-owners-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelManagedByKey: grovecorev1alpha1.LabelManagedByValue,
					},
					OwnerReferences: []metav1.OwnerReference{
						newOwnerReference("PodGangSet", "test-pgs", true),
						newOwnerReference("PodCliqueScalingGroup", "test-pcsg", false),
					},
				},
			},
			expectedOwnerKinds: []string{"PodGangSet"},
			expected:           false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := IsManagedPodClique(tc.obj, tc.expectedOwnerKinds...)
			assert.Equal(t, tc.expected, result)
		})
	}
}
