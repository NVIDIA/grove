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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestMergeEnvVars(t *testing.T) {
	tests := []struct {
		name       string
		existing   []corev1.EnvVar
		newEnvVars []corev1.EnvVar
		expected   []corev1.EnvVar
	}{
		{
			name:     "Empty existing env vars",
			existing: []corev1.EnvVar{},
			newEnvVars: []corev1.EnvVar{
				{Name: "NEW_VAR", Value: "new_value"},
			},
			expected: []corev1.EnvVar{
				{Name: "NEW_VAR", Value: "new_value"},
			},
		},
		{
			name: "No conflicts between existing and new",
			existing: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing_value"},
			},
			newEnvVars: []corev1.EnvVar{
				{Name: "NEW_VAR", Value: "new_value"},
			},
			expected: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing_value"},
				{Name: "NEW_VAR", Value: "new_value"},
			},
		},
		{
			name: "Replace existing env var with same name",
			existing: []corev1.EnvVar{
				{Name: "SAME_VAR", Value: "old_value"},
			},
			newEnvVars: []corev1.EnvVar{
				{Name: "SAME_VAR", Value: "new_value"},
			},
			expected: []corev1.EnvVar{
				{Name: "SAME_VAR", Value: "new_value"},
			},
		},
		{
			name: "Mix of new and replaced env vars",
			existing: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing_value"},
				{Name: "REPLACE_VAR", Value: "old_value"},
			},
			newEnvVars: []corev1.EnvVar{
				{Name: "REPLACE_VAR", Value: "new_value"},
				{Name: "NEW_VAR", Value: "new_value"},
			},
			expected: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing_value"},
				{Name: "REPLACE_VAR", Value: "new_value"},
				{Name: "NEW_VAR", Value: "new_value"},
			},
		},
		{
			name: "Multiple replacements",
			existing: []corev1.EnvVar{
				{Name: "VAR1", Value: "old_value1"},
				{Name: "VAR2", Value: "old_value2"},
				{Name: "KEEP_VAR", Value: "keep_value"},
			},
			newEnvVars: []corev1.EnvVar{
				{Name: "VAR1", Value: "new_value1"},
				{Name: "VAR2", Value: "new_value2"},
				{Name: "NEW_VAR", Value: "new_value"},
			},
			expected: []corev1.EnvVar{
				{Name: "VAR1", Value: "new_value1"},
				{Name: "VAR2", Value: "new_value2"},
				{Name: "KEEP_VAR", Value: "keep_value"},
				{Name: "NEW_VAR", Value: "new_value"},
			},
		},
		{
			name: "With ValueFrom (Downward API)",
			existing: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing_value"},
				{Name: "REPLACE_VAR", Value: "old_value"},
			},
			newEnvVars: []corev1.EnvVar{
				{
					Name: "REPLACE_VAR",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name: "NEW_VAR",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
			},
			expected: []corev1.EnvVar{
				{Name: "EXISTING_VAR", Value: "existing_value"},
				{
					Name: "REPLACE_VAR",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				},
				{
					Name: "NEW_VAR",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeEnvVars(tt.existing, tt.newEnvVars)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeEnvVars_EmptyInputs(t *testing.T) {
	// Test with empty inputs
	result := MergeEnvVars([]corev1.EnvVar{}, []corev1.EnvVar{})
	assert.Empty(t, result)

	// Test with nil inputs
	result = MergeEnvVars(nil, nil)
	assert.Empty(t, result)
}

func TestMergeEnvVars_PreservesOrder(t *testing.T) {
	// Test that existing variables maintain their relative order
	existing := []corev1.EnvVar{
		{Name: "VAR_A", Value: "a"},
		{Name: "VAR_B", Value: "b"},
		{Name: "VAR_C", Value: "c"},
	}

	newEnvVars := []corev1.EnvVar{
		{Name: "VAR_B", Value: "new_b"}, // Replace middle variable
		{Name: "VAR_D", Value: "d"},     // Add new variable
	}

	result := MergeEnvVars(existing, newEnvVars)

	expected := []corev1.EnvVar{
		{Name: "VAR_A", Value: "a"},
		{Name: "VAR_B", Value: "new_b"}, // Replaced in same position
		{Name: "VAR_C", Value: "c"},
		{Name: "VAR_D", Value: "d"}, // Added at end
	}

	assert.Equal(t, expected, result)
}
