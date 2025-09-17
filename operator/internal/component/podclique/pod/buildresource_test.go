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
	"fmt"
	"os"
	"testing"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

// TestBuildResource tests the buildResource method that constructs a pod resource
// from PodClique specifications with proper metadata, labels, and configuration.
func TestBuildResource(t *testing.T) {
	// Setup scheme for controller reference setting
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = grovecorev1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	tests := []struct {
		// name describes the test scenario
		name string
		// pcs is the PodCliqueSet for the test
		pcs *grovecorev1alpha1.PodCliqueSet
		// pclq is the PodClique for the test
		pclq *grovecorev1alpha1.PodClique
		// podGangName is the PodGang name for the pod
		podGangName string
		// podIndex is the index of the pod within the PodClique
		podIndex int
		// setInitEnvVar indicates whether to set the init container env var
		setInitEnvVar bool
		// initEnvValue is the value for the init container env var
		initEnvValue string
		// expectError indicates whether an error should occur
		expectError bool
		// expectedSchedulingGates are gates that should be applied to the pod
		expectedSchedulingGates []string
		// expectedEnvVars are environment variables that should be added
		expectedEnvVars []string
		// expectedInitContainers is the number of init containers expected
		expectedInitContainers int
	}{
		{
			// Basic pod without startup ordering
			name: "basic pod without startup ordering",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpcs",
					Namespace: "default",
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testpcs-0-basic",
					Namespace: "default",
					Labels: map[string]string{
						"app":                       "test-app",
						apicommon.LabelPartOfKey:    "testpcs",
						apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
					},
					Annotations: map[string]string{
						"test-annotation": "test-value",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "app",
								Image: "nginx:latest",
							},
						},
					},
					StartsAfter: []string{}, // No dependencies
				},
			},
			podGangName:             "test-podgang",
			podIndex:                0,
			setInitEnvVar:           false,
			initEnvValue:            "",
			expectError:             false,
			expectedSchedulingGates: []string{podGangSchedulingGate},
			expectedEnvVars: []string{
				constants.EnvVarPodCliqueSetName,
				constants.EnvVarPodCliqueSetIndex,
				constants.EnvVarPodCliqueName,
				constants.EnvVarHeadlessService,
				constants.EnvVarPodIndex,
			},
			expectedInitContainers: 0,
		},
		{
			// Pod with startup ordering should include init container
			name: "pod with startup ordering",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "orderedpcs",
					Namespace: "default",
				},
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
								Image: "worker:v1.0",
							},
						},
					},
					StartsAfter: []string{"dependency-pclq"},
				},
			},
			podGangName:             "ordered-podgang",
			podIndex:                2,
			setInitEnvVar:           true,
			initEnvValue:            "gcr.io/grove/init",
			expectError:             false,
			expectedSchedulingGates: []string{podGangSchedulingGate},
			expectedEnvVars: []string{
				constants.EnvVarPodCliqueSetName,
				constants.EnvVarPodCliqueSetIndex,
				constants.EnvVarPodCliqueName,
				constants.EnvVarHeadlessService,
				constants.EnvVarPodIndex,
			},
			expectedInitContainers: 1,
		},
		{
			// Pod with startup ordering but missing init container env var should fail
			name: "startup ordering with missing init env var",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "failpcs",
					Namespace: "default",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "dep",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									MinAvailable: ptr.To[int32](1),
								},
							},
						},
					},
				},
			},
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "failpcs-0-fail",
					Namespace: "default",
					Labels: map[string]string{
						apicommon.LabelPartOfKey:    "failpcs",
						apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
					},
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
			podGangName:             "fail-podgang",
			podIndex:                0,
			setInitEnvVar:           false,
			initEnvValue:            "",
			expectError:             true,
			expectedSchedulingGates: []string{podGangSchedulingGate},
			expectedEnvVars:         []string{},
			expectedInitContainers:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variable for init container if needed
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

			// Create resource instance
			resource := &_resource{
				scheme: scheme,
			}

			// Create empty pod to be built
			pod := &corev1.Pod{}

			// Test buildResource
			err := resource.buildResource(tt.pcs, tt.pclq, tt.podGangName, pod, tt.podIndex)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify basic metadata
			expectedGenerateName := tt.pclq.Name + "-"
			assert.Equal(t, expectedGenerateName, pod.GenerateName)
			assert.Equal(t, tt.pclq.Namespace, pod.Namespace)

			// Verify controller reference was set
			require.Len(t, pod.OwnerReferences, 1)
			ownerRef := pod.OwnerReferences[0]
			assert.Equal(t, tt.pclq.Name, ownerRef.Name)
			assert.True(t, *ownerRef.Controller)

			// Verify labels include required Grove labels
			require.NotNil(t, pod.Labels)
			assert.Equal(t, tt.pclq.Name, pod.Labels[apicommon.LabelPodClique])
			assert.Equal(t, tt.podGangName, pod.Labels[apicommon.LabelPodGang])

			// Verify user labels and annotations are preserved
			if tt.pclq.Labels != nil {
				for key, value := range tt.pclq.Labels {
					assert.Equal(t, value, pod.Labels[key])
				}
			}
			if tt.pclq.Annotations != nil {
				assert.Equal(t, tt.pclq.Annotations, pod.Annotations)
			}

			// Verify scheduling gates
			require.Len(t, pod.Spec.SchedulingGates, len(tt.expectedSchedulingGates))
			for i, expectedGate := range tt.expectedSchedulingGates {
				assert.Equal(t, expectedGate, pod.Spec.SchedulingGates[i].Name)
			}

			// Verify pod spec is copied from PodClique
			assert.Len(t, pod.Spec.Containers, len(tt.pclq.Spec.PodSpec.Containers))
			for i, container := range pod.Spec.Containers {
				originalContainer := tt.pclq.Spec.PodSpec.Containers[i]
				assert.Equal(t, originalContainer.Name, container.Name)
				assert.Equal(t, originalContainer.Image, container.Image)

				// Verify Grove environment variables were added
				envVarNames := make(map[string]bool)
				for _, env := range container.Env {
					envVarNames[env.Name] = true
				}
				for _, expectedEnv := range tt.expectedEnvVars {
					assert.True(t, envVarNames[expectedEnv], "Environment variable %s should be present", expectedEnv)
				}
			}

			// Verify hostname and subdomain are set
			expectedHostname := tt.pclq.Name + "-" + fmt.Sprintf("%d", tt.podIndex)
			assert.Equal(t, expectedHostname, pod.Spec.Hostname)
			assert.NotEmpty(t, pod.Spec.Subdomain)

			// Verify init containers
			assert.Len(t, pod.Spec.InitContainers, tt.expectedInitContainers)
			if tt.expectedInitContainers > 0 {
				initContainer := pod.Spec.InitContainers[0]
				assert.Equal(t, initContainerName, initContainer.Name)
				assert.Contains(t, initContainer.Image, tt.initEnvValue)
			}

			// Verify volumes for init container if present
			if tt.expectedInitContainers > 0 {
				volumeNames := make(map[string]bool)
				for _, vol := range pod.Spec.Volumes {
					volumeNames[vol.Name] = true
				}
				assert.True(t, volumeNames[serviceAccountTokenSecretVolumeName])
				assert.True(t, volumeNames[podInfoVolumeName])
			}
		})
	}
}

// TestBuildResource_InvalidPCSReplicaIndex tests error handling when PodClique name
// doesn't contain valid PodCliqueSet replica index information.
func TestBuildResource_InvalidPCSReplicaIndex(t *testing.T) {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = grovecorev1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	resource := &_resource{
		scheme: scheme,
	}

	// Create PodClique with invalid name format
	pcs := &grovecorev1alpha1.PodCliqueSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpcs",
			Namespace: "default",
		},
	}

	pclq := &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-name-format", // Missing proper replica index format
			Namespace: "default",
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test",
						Image: "test:latest",
					},
				},
			},
		},
	}

	pod := &corev1.Pod{}

	// This should fail due to invalid name format
	err = resource.buildResource(pcs, pclq, "test-podgang", pod, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extracting PCS replica index")
}
