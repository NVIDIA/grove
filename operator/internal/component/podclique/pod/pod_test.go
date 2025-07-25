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

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to assert expected environment variables are present
func assertExpectedEnvVars(t *testing.T, container corev1.Container, expectedEnvVars []string) {
	envVarNames := make(map[string]bool)
	for _, env := range container.Env {
		envVarNames[env.Name] = true
	}

	for _, expectedEnv := range expectedEnvVars {
		assert.True(t, envVarNames[expectedEnv], "expected environment variable %s not found in container %s", expectedEnv, container.Name)
	}
}

// Helper function to assert environment variable field paths for Downward API
func assertEnvVarFieldPath(t *testing.T, container corev1.Container, directValueEnvVars []string) {
	// Create a set of env vars that should use direct values
	directValueSet := make(map[string]bool)
	for _, envVar := range directValueEnvVars {
		directValueSet[envVar] = true
	}

	for _, env := range container.Env {
		// Skip env vars that should use direct values
		if directValueSet[env.Name] {
			assert.NotEmpty(t, env.Value, "environment variable %s should have a direct value", env.Name)
			continue
		}

		require.NotNil(t, env.ValueFrom, "environment variable %s should use Downward API FieldRef", env.Name)
		require.NotNil(t, env.ValueFrom.FieldRef, "environment variable %s should use Downward API FieldRef", env.Name)

		// Check specific field paths
		switch env.Name {
		case envVarGrovePGSName:
			assert.Equal(t, "metadata.labels['app.kubernetes.io/part-of']", env.ValueFrom.FieldRef.FieldPath,
				"incorrect field path for %s", env.Name)
		case envVarGrovePGSIndex:
			assert.Equal(t, "metadata.labels['grove.io/podgangset-replica-index']", env.ValueFrom.FieldRef.FieldPath,
				"incorrect field path for %s", env.Name)
		case envVarGrovePCLQName:
			assert.Equal(t, "metadata.labels['grove.io/podclique']", env.ValueFrom.FieldRef.FieldPath,
				"incorrect field path for %s", env.Name)
		}
	}
}

// Helper function to assert replaced environment variables use correct Downward API
func assertReplacedEnvVars(t *testing.T, container corev1.Container, shouldReplace map[string]string) {
	if shouldReplace == nil {
		return
	}

	envVarNames := make(map[string]corev1.EnvVar)
	for _, env := range container.Env {
		envVarNames[env.Name] = env
	}

	for envName, expectedFieldPath := range shouldReplace {
		envVar, found := envVarNames[envName]
		assert.True(t, found, "environment variable %s should exist", envName)
		if found {
			require.NotNil(t, envVar.ValueFrom, "environment variable %s should use Downward API FieldRef", envName)
			require.NotNil(t, envVar.ValueFrom.FieldRef, "environment variable %s should use Downward API FieldRef", envName)
			assert.Equal(t, expectedFieldPath, envVar.ValueFrom.FieldRef.FieldPath,
				"environment variable %s has wrong field path", envName)
		}
	}
}

// Helper function to assert preserved environment variables maintain their values
func assertPreservedEnvVars(t *testing.T, container corev1.Container, shouldPreserve []string) {
	if shouldPreserve == nil {
		return
	}

	envVarNames := make(map[string]corev1.EnvVar)
	for _, env := range container.Env {
		envVarNames[env.Name] = env
	}

	for _, preserveEnv := range shouldPreserve {
		envVar, found := envVarNames[preserveEnv]
		assert.True(t, found, "expected preserved environment variable %s not found in container %s", preserveEnv, container.Name)
		if found {
			assert.NotEmpty(t, envVar.Value, "preserved environment variable %s should have its original value", preserveEnv)
		}
	}
}

// Helper function to assert no duplicate environment variables
func assertNoDuplicateEnvVars(t *testing.T, container corev1.Container) {
	envVarCounts := make(map[string]int)
	for _, env := range container.Env {
		envVarCounts[env.Name]++
	}
	for envName, count := range envVarCounts {
		assert.Equal(t, 1, count, "environment variable %s appears %d times (should be 1)", envName, count)
	}
}

func TestAddGroveEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name               string
		pclq               *grovecorev1alpha1.PodClique
		expectedEnvVars    []string
		unexpectedEnvVars  []string
		directValueEnvVars []string
	}{
		{
			name: "standalone PodClique",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
							},
						},
					},
				},
			},
			expectedEnvVars: []string{
				envVarGrovePGSName,
				envVarGrovePGSIndex,
				envVarGrovePCLQName,
				envVarGroveHeadlessService,
				envVarGrovePodIndex,
			},
			directValueEnvVars: []string{
				envVarGroveHeadlessService,
				envVarGrovePodIndex,
			},
		},
		{
			name: "PCSG member PodClique",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelPodCliqueScalingGroup: "test-pcsg",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
							},
						},
					},
				},
			},
			expectedEnvVars: []string{
				envVarGrovePGSName,
				envVarGrovePGSIndex,
				envVarGrovePCLQName,
				envVarGroveHeadlessService,
				envVarGrovePodIndex,
			},
			directValueEnvVars: []string{
				envVarGroveHeadlessService,
				envVarGrovePodIndex,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: tt.pclq.Spec.PodSpec,
			}

			addGroveEnvironmentVariables(pod, "test-pgs", 0, 0)

			// Check that all containers have the expected environment variables
			for _, container := range pod.Spec.Containers {
				assertExpectedEnvVars(t, container, tt.expectedEnvVars)

				// Check unexpected environment variables are not present
				envVarNames := make(map[string]bool)
				for _, env := range container.Env {
					envVarNames[env.Name] = true
				}
				for _, unexpectedEnv := range tt.unexpectedEnvVars {
					if envVarNames[unexpectedEnv] {
						t.Errorf("unexpected environment variable %s found in container %s", unexpectedEnv, container.Name)
					}
				}

				// Verify Downward API configuration for expected variables
				assertEnvVarFieldPath(t, container, tt.directValueEnvVars)
			}
		})
	}
}

func TestAddGroveEnvironmentVariables_NoDuplicates(t *testing.T) {
	tests := []struct {
		name            string
		pclq            *grovecorev1alpha1.PodClique
		existingEnvVars []corev1.EnvVar
		expectedEnvVars []string
		shouldReplace   map[string]string // env var name -> expected value
		shouldPreserve  []string          // env var names that should be preserved
	}{
		{
			name: "Container with existing Grove env vars - should replace",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
								Env: []corev1.EnvVar{
									{Name: "GROVE_PGS_NAME", Value: "old-pgs-name"},
									{Name: "GROVE_PGS_INDEX", Value: "old-index"},
								},
							},
						},
					},
				},
			},
			expectedEnvVars: []string{
				envVarGrovePGSName,
				envVarGrovePGSIndex,
				envVarGrovePCLQName,
				envVarGroveHeadlessService,
			},
			shouldReplace: map[string]string{
				"GROVE_PGS_NAME":  "metadata.labels['app.kubernetes.io/part-of']",
				"GROVE_PGS_INDEX": "metadata.labels['grove.io/podgangset-replica-index']",
			},
		},
		{
			name: "Container with user env vars - should preserve",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
								Env: []corev1.EnvVar{
									{Name: "USER_VAR", Value: "user-value"},
									{Name: "CUSTOM_CONFIG", Value: "custom-value"},
								},
							},
						},
					},
				},
			},
			expectedEnvVars: []string{
				envVarGrovePGSName,
				envVarGrovePGSIndex,
				envVarGrovePCLQName,
				envVarGroveHeadlessService,
			},
			shouldPreserve: []string{"USER_VAR", "CUSTOM_CONFIG"},
		},
		{
			name: "Container with mixed env vars - should replace Grove, preserve user",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
								Env: []corev1.EnvVar{
									{Name: "USER_VAR", Value: "user-value"},
									{Name: "GROVE_PGS_NAME", Value: "old-pgs-name"},
									{Name: "CUSTOM_CONFIG", Value: "custom-value"},
								},
							},
						},
					},
				},
			},
			expectedEnvVars: []string{
				envVarGrovePGSName,
				envVarGrovePGSIndex,
				envVarGrovePCLQName,
				envVarGroveHeadlessService,
			},
			shouldReplace: map[string]string{
				"GROVE_PGS_NAME": "metadata.labels['app.kubernetes.io/part-of']",
			},
			shouldPreserve: []string{"USER_VAR", "CUSTOM_CONFIG"},
		},
		{
			name: "PCSG PodClique with existing env vars",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pclq",
					Namespace: "test-ns",
					Labels: map[string]string{
						grovecorev1alpha1.LabelPodCliqueScalingGroup: "test-pcsg",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image",
								Env: []corev1.EnvVar{
									{Name: "GROVE_PCSG_NAME", Value: "old-pcsg-name"},
									{Name: "USER_VAR", Value: "user-value"},
								},
							},
						},
					},
				},
			},
			expectedEnvVars: []string{
				envVarGrovePGSName,
				envVarGrovePGSIndex,
				envVarGrovePCLQName,
				envVarGroveHeadlessService,
			},
			shouldReplace:  map[string]string{},
			shouldPreserve: []string{"USER_VAR"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: tt.pclq.Spec.PodSpec,
			}

			addGroveEnvironmentVariables(pod, "test-pgs", 0, 0)

			// Check that all containers have the expected environment variables
			for _, container := range pod.Spec.Containers {
				assertExpectedEnvVars(t, container, tt.expectedEnvVars)
				assertReplacedEnvVars(t, container, tt.shouldReplace)
				assertPreservedEnvVars(t, container, tt.shouldPreserve)
				assertNoDuplicateEnvVars(t, container)
			}
		})
	}
}

func TestAddGroveEnvironmentVariables_Idempotent(t *testing.T) {
	pod := createTestPod()

	// Call addGroveEnvironmentVariables multiple times
	addGroveEnvironmentVariables(pod, "test-pgs", 0, 0)
	envVarsAfterFirst := make([]corev1.EnvVar, len(pod.Spec.Containers[0].Env))
	copy(envVarsAfterFirst, pod.Spec.Containers[0].Env)

	addGroveEnvironmentVariables(pod, "test-pgs", 0, 0)
	envVarsAfterSecond := pod.Spec.Containers[0].Env

	addGroveEnvironmentVariables(pod, "test-pgs", 0, 0)
	envVarsAfterThird := pod.Spec.Containers[0].Env

	// All calls should produce the same result
	assert.Equal(t, envVarsAfterFirst, envVarsAfterSecond, "second call should produce same result")
	assert.Equal(t, envVarsAfterFirst, envVarsAfterThird, "third call should produce same result")

	// Check that no environment variable appears more than once
	assertNoDuplicateEnvVars(t, pod.Spec.Containers[0])
}

func TestAddGroveEnvironmentVariables_EmptyContainers(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
	}

	// Should not panic with empty containers
	addGroveEnvironmentVariables(pod, "test-pgs", 0, 0)
	assert.Empty(t, pod.Spec.Containers)
}

func TestAddGroveEnvironmentVariables_MultipleContainers(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "image1",
				},
				{
					Name:  "container2",
					Image: "image2",
					Env: []corev1.EnvVar{
						{Name: "EXISTING_VAR", Value: "existing-value"},
					},
				},
			},
		},
	}

	addGroveEnvironmentVariables(pod, "test-pgs", 0, 0)

	// Both containers should have Grove environment variables
	expectedEnvVars := []string{
		envVarGrovePGSName,
		envVarGrovePGSIndex,
		envVarGrovePCLQName,
		envVarGroveHeadlessService,
	}

	for _, container := range pod.Spec.Containers {
		assertExpectedEnvVars(t, container, expectedEnvVars)
		assertNoDuplicateEnvVars(t, container)
	}

	// Second container should preserve existing environment variable
	envVarNames := make(map[string]bool)
	for _, env := range pod.Spec.Containers[1].Env {
		envVarNames[env.Name] = true
	}
	assert.True(t, envVarNames["EXISTING_VAR"], "existing environment variable should be preserved")
}

// Helper function to create a test pod with basic configuration
func createTestPod() *corev1.Pod {
	return &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
					Env: []corev1.EnvVar{
						{Name: "USER_VAR", Value: "user-value"},
					},
				},
			},
		},
	}
}
