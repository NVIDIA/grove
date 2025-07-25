// /*
// Copyright 2024 The Grove Authors.
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
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// IsEmptyStringType returns true if value (which is a string or has an underline type string) is empty or contains only whitespace characters.
func IsEmptyStringType[T ~string](val T) bool {
	return len(strings.TrimSpace(string(val))) == 0
}

// MergeEnvVars merges new environment variables into existing ones, replacing any with the same name.
// This prevents duplicate environment variables and ensures new values take precedence.
func MergeEnvVars(existing []corev1.EnvVar, newEnvVars []corev1.EnvVar) []corev1.EnvVar {
	// Create a map of new environment variables for quick lookup
	newEnvMap := make(map[string]corev1.EnvVar)
	for _, envVar := range newEnvVars {
		newEnvMap[envVar.Name] = envVar
	}

	result := make([]corev1.EnvVar, 0, len(existing)+len(newEnvVars))

	// Process existing environment variables
	for _, envVar := range existing {
		if newEnvVar, exists := newEnvMap[envVar.Name]; exists {
			result = append(result, newEnvVar) // Replace with new value
			delete(newEnvMap, envVar.Name)     // Mark as processed
		} else {
			result = append(result, envVar) // Keep existing
		}
	}

	// Add any remaining new environment variables that weren't replacements
	for _, envVar := range newEnvMap {
		result = append(result, envVar)
	}

	return result
}
