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
	"os"
	"testing"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// TestAddServiceAccountTokenSecretVolume tests the function that adds service account
// token volume required for init container to access Kubernetes API.
func TestAddServiceAccountTokenSecretVolume(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pcsName is the PodCliqueSet name used to generate secret name
		pcsName string
		// existingVolumes are volumes that already exist on the pod
		existingVolumes []corev1.Volume
		// expectedSecretName is the expected name of the secret volume
		expectedSecretName string
	}{
		{
			// Basic volume addition to empty pod
			name:               "add to empty pod",
			pcsName:            "test-pcs",
			existingVolumes:    []corev1.Volume{},
			expectedSecretName: "test-pcs-initc-sa-token-secret",
		},
		{
			// Volume addition to pod with existing volumes
			name:    "add to pod with existing volumes",
			pcsName: "prod-pcs",
			existingVolumes: []corev1.Volume{
				{
					Name: "existing-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			expectedSecretName: "prod-pcs-initc-sa-token-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: tt.existingVolumes,
				},
			}

			initialVolumeCount := len(pod.Spec.Volumes)
			addServiceAccountTokenSecretVolume(tt.pcsName, pod)

			// Verify volume was added
			assert.Len(t, pod.Spec.Volumes, initialVolumeCount+1)

			// Find the added volume
			var addedVolume *corev1.Volume
			for _, vol := range pod.Spec.Volumes {
				if vol.Name == serviceAccountTokenSecretVolumeName {
					addedVolume = &vol
					break
				}
			}

			require.NotNil(t, addedVolume, "Service account token volume should be added")
			assert.Equal(t, serviceAccountTokenSecretVolumeName, addedVolume.Name)

			// Verify volume source is configured correctly
			require.NotNil(t, addedVolume.VolumeSource.Secret, "Volume should have Secret source")
			assert.Equal(t, tt.expectedSecretName, addedVolume.VolumeSource.Secret.SecretName)
			assert.Equal(t, int32(420), *addedVolume.VolumeSource.Secret.DefaultMode)
		})
	}
}

// TestAddPodInfoVolume tests the function that adds downward API volume
// to provide pod metadata to the init container.
func TestAddPodInfoVolume(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// existingVolumes are volumes that already exist on the pod
		existingVolumes []corev1.Volume
	}{
		{
			// Basic volume addition to empty pod
			name:            "add to empty pod",
			existingVolumes: []corev1.Volume{},
		},
		{
			// Volume addition to pod with existing volumes
			name: "add to pod with existing volumes",
			existingVolumes: []corev1.Volume{
				{
					Name: "app-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "app-config",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes: tt.existingVolumes,
				},
			}

			initialVolumeCount := len(pod.Spec.Volumes)
			addPodInfoVolume(pod)

			// Verify volume was added
			assert.Len(t, pod.Spec.Volumes, initialVolumeCount+1)

			// Find the added volume
			var addedVolume *corev1.Volume
			for _, vol := range pod.Spec.Volumes {
				if vol.Name == podInfoVolumeName {
					addedVolume = &vol
					break
				}
			}

			require.NotNil(t, addedVolume, "Pod info volume should be added")
			assert.Equal(t, podInfoVolumeName, addedVolume.Name)

			// Verify downward API configuration
			require.NotNil(t, addedVolume.VolumeSource.DownwardAPI, "Volume should have DownwardAPI source")

			downwardAPI := addedVolume.VolumeSource.DownwardAPI
			require.Len(t, downwardAPI.Items, 2, "Should have 2 downward API items")

			// Verify namespace file configuration
			namespaceItem := downwardAPI.Items[0]
			assert.Equal(t, common.PodNamespaceFileName, namespaceItem.Path)
			assert.Equal(t, "metadata.namespace", namespaceItem.FieldRef.FieldPath)

			// Verify pod gang name file configuration
			podGangItem := downwardAPI.Items[1]
			assert.Equal(t, common.PodGangNameFileName, podGangItem.Path)
			expectedFieldPath := "metadata.labels['" + apicommon.LabelPodGang + "']"
			assert.Equal(t, expectedFieldPath, podGangItem.FieldRef.FieldPath)
		})
	}
}

// TestGetInitContainerImage tests the function that retrieves init container image
// from environment variable with proper error handling.
func TestGetInitContainerImage(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// envValue is the value to set for the environment variable (empty means unset)
		envValue string
		// setEnv indicates whether to set the environment variable
		setEnv bool
		// expectedImage is the expected image string returned
		expectedImage string
		// expectError indicates whether an error should occur
		expectError bool
	}{
		{
			// Valid environment variable should return image
			name:          "valid environment variable",
			envValue:      "gcr.io/grove/init-container",
			setEnv:        true,
			expectedImage: "gcr.io/grove/init-container",
			expectError:   false,
		},
		{
			// Missing environment variable should return error
			name:          "missing environment variable",
			envValue:      "",
			setEnv:        false,
			expectedImage: "",
			expectError:   true,
		},
		{
			// Empty environment variable should return empty string
			name:          "empty environment variable",
			envValue:      "",
			setEnv:        true,
			expectedImage: "",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variable
			originalValue := os.Getenv(envVarInitContainerImage)
			defer func() {
				if originalValue != "" {
					os.Setenv(envVarInitContainerImage, originalValue)
				} else {
					os.Unsetenv(envVarInitContainerImage)
				}
			}()

			if tt.setEnv {
				os.Setenv(envVarInitContainerImage, tt.envValue)
			} else {
				os.Unsetenv(envVarInitContainerImage)
			}

			// Test the function
			image, err := getInitContainerImage()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), envVarInitContainerImage)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedImage, image)
			}
		})
	}
}

// TestGenerateArgsForInitContainer tests the function that creates command line arguments
// for the init container based on StartsAfter dependencies.
func TestGenerateArgsForInitContainer(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pcs is the PodCliqueSet containing templates
		pcs *grovecorev1alpha1.PodCliqueSet
		// pclq is the PodClique with StartsAfter dependencies
		pclq *grovecorev1alpha1.PodClique
		// expectedArgs are the expected command line arguments
		expectedArgs []string
		// expectError indicates whether an error should occur
		expectError bool
	}{
		{
			// No dependencies should return empty args
			name: "no dependencies",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{},
				},
			},
			expectedArgs: []string{},
			expectError:  false,
		},
		{
			// Single dependency should generate correct arg
			name: "single dependency",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "dependency-pclq",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: ptr.To[int32](2),
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"some-prefix.dependency-pclq"},
				},
			},
			expectedArgs: []string{"--podcliques=some-prefix.dependency-pclq:2"},
			expectError:  false,
		},
		{
			// Multiple dependencies should generate multiple args
			name: "multiple dependencies",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "first-dep",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: ptr.To[int32](1),
								},
							},
							{
								Name: "second-dep",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: ptr.To[int32](3),
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"namespace.first-dep", "namespace.second-dep"},
				},
			},
			expectedArgs: []string{
				"--podcliques=namespace.first-dep:1",
				"--podcliques=namespace.second-dep:3",
			},
			expectError: false,
		},
		{
			// Missing dependency template should return error
			name: "missing dependency template",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "existing-dep",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: ptr.To[int32](1),
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"namespace.missing-dep"},
				},
			},
			expectedArgs: nil,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := generateArgsForInitContainer(tt.pcs, tt.pclq)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not present in the templates")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedArgs, args)
			}
		})
	}
}

// TestConfigurePodInitContainer tests the main function that configures init containers
// for pods with startup ordering requirements.
func TestConfigurePodInitContainer(t *testing.T) {
	tests := []struct {
		// name describes the test scenario
		name string
		// pcs is the PodCliqueSet containing templates
		pcs *grovecorev1alpha1.PodCliqueSet
		// pclq is the PodClique being configured
		pclq *grovecorev1alpha1.PodClique
		// setEnvVar indicates whether to set the init container image env var
		setEnvVar bool
		// envValue is the value for the init container image env var
		envValue string
		// expectError indicates whether an error should occur
		expectError bool
		// expectedVolumes are volumes that should be added to the pod
		expectedVolumes []string
		// expectedInitContainers is the number of init containers expected
		expectedInitContainers int
	}{
		{
			// Successful configuration with valid setup
			name: "successful configuration",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "dependency",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: ptr.To[int32](2),
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pclq",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"dependency"},
				},
			},
			setEnvVar:              true,
			envValue:               "gcr.io/grove/init",
			expectError:            false,
			expectedVolumes:        []string{serviceAccountTokenSecretVolumeName, podInfoVolumeName},
			expectedInitContainers: 1,
		},
		{
			// Missing environment variable should cause error
			name: "missing environment variable",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "dependency",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: ptr.To[int32](1),
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"dependency"},
				},
			},
			setEnvVar:              false,
			envValue:               "",
			expectError:            true,
			expectedVolumes:        []string{serviceAccountTokenSecretVolumeName, podInfoVolumeName},
			expectedInitContainers: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variable
			originalValue := os.Getenv(envVarInitContainerImage)
			defer func() {
				if originalValue != "" {
					os.Setenv(envVarInitContainerImage, originalValue)
				} else {
					os.Unsetenv(envVarInitContainerImage)
				}
			}()

			if tt.setEnvVar {
				os.Setenv(envVarInitContainerImage, tt.envValue)
			} else {
				os.Unsetenv(envVarInitContainerImage)
			}

			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					Volumes:        []corev1.Volume{},
					InitContainers: []corev1.Container{},
				},
			}

			err := configurePodInitContainer(tt.pcs, tt.pclq, pod)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify volumes were added
				assert.Len(t, pod.Spec.Volumes, len(tt.expectedVolumes))
				for _, expectedVolName := range tt.expectedVolumes {
					found := false
					for _, vol := range pod.Spec.Volumes {
						if vol.Name == expectedVolName {
							found = true
							break
						}
					}
					assert.True(t, found, "Volume %s should be present", expectedVolName)
				}

				// Verify init container was added
				assert.Len(t, pod.Spec.InitContainers, tt.expectedInitContainers)

				if tt.expectedInitContainers > 0 {
					initContainer := pod.Spec.InitContainers[0]
					assert.Equal(t, initContainerName, initContainer.Name)
					assert.Contains(t, initContainer.Image, tt.envValue)

					// Verify volume mounts
					assert.Len(t, initContainer.VolumeMounts, 2)
					mountNames := make(map[string]bool)
					for _, mount := range initContainer.VolumeMounts {
						mountNames[mount.Name] = true
					}
					assert.True(t, mountNames[podInfoVolumeName])
					assert.True(t, mountNames[serviceAccountTokenSecretVolumeName])
				}
			}
		})
	}
}
