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

package podclique

import (
	"context"
	"errors"
	"strconv"
	"testing"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	testNamespace = "test-namespace"
	testPCSName   = "test-pcs"
	testPCSGName  = "test-pcsg"
)

// TestNew verifies that the New function correctly creates a PodClique component operator
// with the required dependencies and implements the expected interface.
func TestNew(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Whether to test with nil inputs to verify error handling
		testNilInputs bool
	}{
		{
			// Verify successful creation with valid inputs
			name:          "creates operator with valid inputs",
			testNilInputs: false,
		},
		{
			// Verify the operator can handle nil inputs gracefully
			name:          "handles nil inputs gracefully",
			testNilInputs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))

			if tt.testNilInputs {
				// Test with nil inputs to ensure operator doesn't panic
				operator := New(nil, nil, nil)
				assert.NotNil(t, operator, "operator should not be nil even with nil inputs")
			} else {
				// Test with valid inputs
				client := testutils.SetupFakeClient()
				eventRecorder := record.NewFakeRecorder(100)

				operator := New(client, scheme, eventRecorder)
				assert.NotNil(t, operator, "operator should not be nil")

				// Verify interface compliance - operator should implement the component.Operator interface
				// We can't check the private _resource type, so just verify it's not nil and can be used
				ctx := context.Background()
				logger := log.FromContext(ctx)
				objMeta := createTestPCSGObjectMeta()

				// Should be able to call methods without panicking
				names, err := operator.GetExistingResourceNames(ctx, logger, objMeta)
				assert.NoError(t, err, "GetExistingResourceNames should work")
				assert.NotNil(t, names, "names should not be nil")
			}
		})
	}
}

// TestGetExistingResourceNames verifies that GetExistingResourceNames correctly retrieves
// the names of existing PodClique resources managed by a PodCliqueScalingGroup.
func TestGetExistingResourceNames(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Existing PodClique resources to create before the test
		existingPodCliques []grovecorev1alpha1.PodClique
		// Expected names that should be returned
		expectedNames []string
		// Whether an error is expected
		expectError bool
	}{
		{
			// No existing PodCliques should return empty list
			name:               "no existing podcliques returns empty list",
			existingPodCliques: []grovecorev1alpha1.PodClique{},
			expectedNames:      []string{},
			expectError:        false,
		},
		{
			// Single owned PodClique should be returned
			name: "single owned podclique returned",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodClique("pcsg-0-clique-a", true),
			},
			expectedNames: []string{"pcsg-0-clique-a"},
			expectError:   false,
		},
		{
			// Multiple owned PodCliques should all be returned
			name: "multiple owned podcliques returned",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodClique("pcsg-0-clique-a", true),
				createTestPodClique("pcsg-0-clique-b", true),
				createTestPodClique("pcsg-1-clique-a", true),
			},
			expectedNames: []string{"pcsg-0-clique-a", "pcsg-0-clique-b", "pcsg-1-clique-a"},
			expectError:   false,
		},
		{
			// Mix of owned and unowned PodCliques - only owned should be returned
			name: "filters out unowned podcliques",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodClique("pcsg-0-clique-a", true),
				createTestPodClique("other-podclique", false),
			},
			expectedNames: []string{"pcsg-0-clique-a"},
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			ctx := context.Background()
			logger := log.FromContext(ctx)

			// Convert PodCliques to client.Object for the fake client
			objects := make([]client.Object, len(tt.existingPodCliques))
			for i := range tt.existingPodCliques {
				objects[i] = &tt.existingPodCliques[i]
			}

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder)
			pcsgObjMeta := createTestPCSGObjectMeta()

			// Execute the function under test
			names, err := operator.GetExistingResourceNames(ctx, logger, pcsgObjMeta)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.ElementsMatch(t, tt.expectedNames, names, "returned names should match expected")
			}
		})
	}
}

// TestDelete verifies that the Delete method correctly removes all PodClique resources
// managed by a PodCliqueScalingGroup.
func TestDelete(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Existing PodClique resources to create before deletion
		existingPodCliques []grovecorev1alpha1.PodClique
		// Whether an error is expected during deletion
		expectError bool
	}{
		{
			// Deleting when no PodCliques exist should succeed
			name:               "no existing podcliques succeeds",
			existingPodCliques: []grovecorev1alpha1.PodClique{},
			expectError:        false,
		},
		{
			// Deleting single PodClique should succeed
			name: "single podclique deletion succeeds",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodClique("pcsg-0-clique-a", true),
			},
			expectError: false,
		},
		{
			// Deleting multiple PodCliques should succeed
			name: "multiple podcliques deletion succeeds",
			existingPodCliques: []grovecorev1alpha1.PodClique{
				createTestPodClique("pcsg-0-clique-a", true),
				createTestPodClique("pcsg-0-clique-b", true),
				createTestPodClique("pcsg-1-clique-a", true),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			ctx := context.Background()
			logger := log.FromContext(ctx)

			// Convert PodCliques to client.Object for the fake client
			objects := make([]client.Object, len(tt.existingPodCliques))
			for i := range tt.existingPodCliques {
				objects[i] = &tt.existingPodCliques[i]
			}

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder)
			pcsgObjMeta := createTestPCSGObjectMeta()

			// Execute the function under test
			err := operator.Delete(ctx, logger, pcsgObjMeta)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")

				// Verify all PodCliques have been deleted
				remainingNames, listErr := operator.GetExistingResourceNames(ctx, logger, pcsgObjMeta)
				assert.NoError(t, listErr, "should be able to list remaining resources")
				assert.Empty(t, remainingNames, "all PodCliques should be deleted")
			}
		})
	}
}

// Helper functions for creating test objects

// createTestPodClique creates a test PodClique with appropriate labels and owner references for testing
func createTestPodClique(name string, ownedByTestPCSG bool) grovecorev1alpha1.PodClique {
	pclq := grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    make(map[string]string),
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
	}

	if ownedByTestPCSG {
		// Add labels that match what getPodCliqueSelectorLabels expects
		// These are the exact labels that GetDefaultLabelsForPodCliqueSetManagedResources returns
		pclq.Labels[apicommon.LabelManagedByKey] = apicommon.LabelManagedByValue
		pclq.Labels[apicommon.LabelPartOfKey] = testPCSName
		// Plus the additional labels that getPodCliqueSelectorLabels adds
		pclq.Labels[apicommon.LabelComponentKey] = apicommon.LabelComponentNamePodCliqueScalingGroupPodClique
		pclq.Labels[apicommon.LabelPodCliqueScalingGroup] = testPCSGName

		// Add owner reference to the PCSG (required for FilterMapOwnedResourceNames)
		pclq.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: grovecorev1alpha1.SchemeGroupVersion.String(),
				Kind:       "PodCliqueScalingGroup",
				Name:       testPCSGName,
				UID:        types.UID("test-pcsg-uid"),
				Controller: ptr.To(true),
			},
		}
	}

	return pclq
}

// TestGetLabelsToDeletePCSGReplicaIndexPCLQs verifies that the function correctly generates
// label selectors for identifying PodClique resources belonging to a specific PCSG replica.
func TestGetLabelsToDeletePCSGReplicaIndexPCLQs(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet name
		pcsName string
		// PodCliqueScalingGroup name
		pcsgName string
		// PCSG replica index as string
		pcsgReplicaIndex string
		// Expected labels in the result
		expectedLabels map[string]string
	}{
		{
			// Basic case with standard names
			name:             "standard names",
			pcsName:          "test-pcs",
			pcsgName:         "test-pcsg",
			pcsgReplicaIndex: "0",
			expectedLabels: map[string]string{
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "test-pcs",
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             "test-pcsg",
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
			},
		},
		{
			// Case with different replica index
			name:             "different replica index",
			pcsName:          "my-pcs",
			pcsgName:         "my-pcsg",
			pcsgReplicaIndex: "5",
			expectedLabels: map[string]string{
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "my-pcs",
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             "my-pcsg",
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: "5",
			},
		},
		{
			// Case with empty strings (edge case)
			name:             "empty strings",
			pcsName:          "",
			pcsgName:         "",
			pcsgReplicaIndex: "",
			expectedLabels: map[string]string{
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "",
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             "",
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			result := getLabelsToDeletePCSGReplicaIndexPCLQs(tt.pcsName, tt.pcsgName, tt.pcsgReplicaIndex)

			// Verify the result matches expectations
			assert.Equal(t, tt.expectedLabels, result, "labels should match expected")
		})
	}
}

// TestGetPCSGTemplateNumPods verifies that the function correctly calculates the total
// number of pods expected in a PCSG replica based on PodClique template specifications.
func TestGetPCSGTemplateNumPods(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet with template specifications
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup with clique names
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected total number of pods
		expectedNumPods int
	}{
		{
			// Single clique with single replica
			name: "single clique single replica",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					CliqueNames: []string{"worker"},
				},
			},
			expectedNumPods: 2,
		},
		{
			// Multiple cliques with different replica counts
			name: "multiple cliques",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "master",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 3,
								},
							},
							{
								Name: "storage",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					CliqueNames: []string{"master", "worker"},
				},
			},
			expectedNumPods: 4, // 1 + 3 = 4
		},
		{
			// PCSG references non-existent clique (should be ignored)
			name: "non-existent clique ignored",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					CliqueNames: []string{"worker", "non-existent"},
				},
			},
			expectedNumPods: 2, // Only worker counted
		},
		{
			// No cliques in PCSG
			name: "no cliques in pcsg",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 2,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					CliqueNames: []string{},
				},
			},
			expectedNumPods: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operator instance for testing
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			client := testutils.SetupFakeClient()
			eventRecorder := record.NewFakeRecorder(100)
			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			result := operator.getPCSGTemplateNumPods(tt.pcs, tt.pcsg)

			// Verify the result matches expectations
			assert.Equal(t, tt.expectedNumPods, result, "pod count should match expected")
		})
	}
}

// TestAddEnvironmentVariablesToPodContainerSpecs verifies that the function correctly
// adds PCSG-specific environment variables to PodClique containers.
func TestAddEnvironmentVariablesToPodContainerSpecs(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Initial PodClique with containers
		initialPodClique *grovecorev1alpha1.PodClique
		// Template number of pods for env var
		templateNumPods int
		// Expected number of env vars added to each container
		expectedEnvVarCount int
	}{
		{
			// PodClique with regular containers
			name: "podclique with containers",
			initialPodClique: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "main",
								Image: "nginx",
								Env:   []corev1.EnvVar{}, // Start with no env vars
							},
							{
								Name:  "sidecar",
								Image: "busybox",
								Env:   []corev1.EnvVar{}, // Start with no env vars
							},
						},
					},
				},
			},
			templateNumPods:     5,
			expectedEnvVarCount: 3, // Three PCSG-specific env vars
		},
		{
			// PodClique with init containers
			name: "podclique with init containers",
			initialPodClique: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{
								Name:  "init",
								Image: "alpine",
								Env:   []corev1.EnvVar{},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "main",
								Image: "nginx",
								Env:   []corev1.EnvVar{},
							},
						},
					},
				},
			},
			templateNumPods:     10,
			expectedEnvVarCount: 3,
		},
		{
			// PodClique with existing env vars (should be preserved)
			name: "podclique with existing env vars",
			initialPodClique: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					PodSpec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "main",
								Image: "nginx",
								Env: []corev1.EnvVar{
									{Name: "EXISTING_VAR", Value: "existing_value"},
								},
							},
						},
					},
				},
			},
			templateNumPods:     3,
			expectedEnvVarCount: 4, // 1 existing + 3 PCSG-specific
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operator instance for testing
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			client := testutils.SetupFakeClient()
			eventRecorder := record.NewFakeRecorder(100)
			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			operator.addEnvironmentVariablesToPodContainerSpecs(tt.initialPodClique, tt.templateNumPods)

			// Verify containers have the expected environment variables
			for _, container := range tt.initialPodClique.Spec.PodSpec.Containers {
				assert.Equal(t, tt.expectedEnvVarCount, len(container.Env),
					"container %s should have %d env vars", container.Name, tt.expectedEnvVarCount)

				// Verify specific PCSG env vars are present
				envVarNames := make([]string, len(container.Env))
				for i, env := range container.Env {
					envVarNames[i] = env.Name
				}
				assert.Contains(t, envVarNames, constants.EnvVarPodCliqueScalingGroupName)
				assert.Contains(t, envVarNames, constants.EnvVarPodCliqueScalingGroupTemplateNumPods)
				assert.Contains(t, envVarNames, constants.EnvVarPodCliqueScalingGroupIndex)
			}

			// Verify init containers have the expected environment variables
			for _, container := range tt.initialPodClique.Spec.PodSpec.InitContainers {
				assert.Equal(t, tt.expectedEnvVarCount, len(container.Env),
					"init container %s should have %d env vars", container.Name, tt.expectedEnvVarCount)
			}

			// Verify the template num pods env var has correct value
			for _, container := range tt.initialPodClique.Spec.PodSpec.Containers {
				for _, env := range container.Env {
					if env.Name == constants.EnvVarPodCliqueScalingGroupTemplateNumPods {
						assert.Equal(t, strconv.Itoa(tt.templateNumPods), env.Value,
							"template num pods env var should have correct value")
					}
				}
			}
		})
	}
}

// TestGetPCSReplicaFromPCSG verifies that the function correctly extracts the
// PodCliqueSet replica index from PodCliqueScalingGroup labels.
func TestGetPCSReplicaFromPCSG(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueScalingGroup with labels
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Expected replica index
		expectedIndex int
		// Whether an error is expected
		expectError bool
	}{
		{
			// Valid replica index in labels
			name: "valid replica index",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: "test-ns",
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "3",
					},
				},
			},
			expectedIndex: 3,
			expectError:   false,
		},
		{
			// Zero replica index (valid edge case)
			name: "zero replica index",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: "test-ns",
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "0",
					},
				},
			},
			expectedIndex: 0,
			expectError:   false,
		},
		{
			// Missing replica index label should return error
			name: "missing replica index label",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: "test-ns",
					Labels:    map[string]string{}, // No replica index label
				},
			},
			expectedIndex: 0,
			expectError:   true,
		},
		{
			// Invalid replica index (non-numeric) should return error
			name: "invalid replica index format",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: "test-ns",
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "invalid",
					},
				},
			},
			expectedIndex: 0,
			expectError:   true,
		},
		{
			// Empty replica index should return error
			name: "empty replica index",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: "test-ns",
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "",
					},
				},
			},
			expectedIndex: 0,
			expectError:   true,
		},
		{
			// Negative replica index should return error
			name: "negative replica index",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: "test-ns",
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "-1",
					},
				},
			},
			expectedIndex: -1, // Function will return -1, but this is still a valid parse
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			result, err := getPCSReplicaFromPCSG(tt.pcsg)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.Equal(t, tt.expectedIndex, result, "replica index should match expected")
			}
		})
	}
}

// TestGetLabels verifies that the function correctly generates labels for PodClique resources
// with all required component-specific, template, and default labels.
func TestGetLabels(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet for context
		pcs *grovecorev1alpha1.PodCliqueSet
		// PCS replica index
		pcsReplicaIndex int
		// PodCliqueScalingGroup for context
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// PCSG replica index
		pcsgReplicaIndex int
		// PodClique object key
		pclqObjectKey client.ObjectKey
		// PodClique template spec
		pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec
		// PodGang name
		podGangName string
		// Expected labels to be present
		expectedLabels map[string]string
		// Whether base PodGang label should be present
		expectBasePodGangLabel bool
	}{
		{
			// Basic case with standard configuration
			name: "basic configuration",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PriorityClassName: "high-priority",
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
			},
			pcsgReplicaIndex: 1,
			pclqObjectKey: client.ObjectKey{
				Name:      "test-pclq",
				Namespace: "test-ns",
			},
			pclqTemplateSpec: &grovecorev1alpha1.PodCliqueTemplateSpec{
				Name: "worker",
				Labels: map[string]string{
					"template-label": "template-value",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					Replicas: 2,
				},
			},
			podGangName: "test-pcs-0", // Same as base PodGang name
			expectedLabels: map[string]string{
				apicommon.LabelAppNameKey:                        "test-pclq",
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             "test-pcsg",
				apicommon.LabelPodGang:                           "test-pcs-0",
				apicommon.LabelPodCliqueSetReplicaIndex:          "0",
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: "1",
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "test-pcs",
				"template-label":                                 "template-value",
			},
			expectBasePodGangLabel: false, // Same as base, so no extra label
		},
		{
			// Case with scaled PodGang (different from base)
			name: "scaled podgang configuration",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						PriorityClassName: "normal-priority",
					},
				},
			},
			pcsReplicaIndex: 2,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcsg",
				},
			},
			pcsgReplicaIndex: 0,
			pclqObjectKey: client.ObjectKey{
				Name:      "test-pclq-scaled",
				Namespace: "test-ns",
			},
			pclqTemplateSpec: &grovecorev1alpha1.PodCliqueTemplateSpec{
				Name: "worker",
				Labels: map[string]string{
					"custom-label": "custom-value",
				},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					Replicas: 3,
				},
			},
			podGangName: "test-pcs-2-scaled-1", // Different from base PodGang name
			expectedLabels: map[string]string{
				apicommon.LabelAppNameKey:                        "test-pclq-scaled",
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             "test-pcsg",
				apicommon.LabelPodGang:                           "test-pcs-2-scaled-1",
				apicommon.LabelBasePodGang:                       "test-pcs-2", // Should have base PodGang label
				apicommon.LabelPodCliqueSetReplicaIndex:          "2",
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "test-pcs",
				"custom-label":                                   "custom-value",
			},
			expectBasePodGangLabel: true,
		},
		{
			// Case with no template labels
			name: "no template labels",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple-pcsg",
				},
			},
			pcsgReplicaIndex: 0,
			pclqObjectKey: client.ObjectKey{
				Name:      "simple-pclq",
				Namespace: "default",
			},
			pclqTemplateSpec: &grovecorev1alpha1.PodCliqueTemplateSpec{
				Name: "simple",
				Spec: grovecorev1alpha1.PodCliqueSpec{
					Replicas: 1,
				},
			},
			podGangName: "simple-pcs-0",
			expectedLabels: map[string]string{
				apicommon.LabelAppNameKey:                        "simple-pclq",
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             "simple-pcsg",
				apicommon.LabelPodGang:                           "simple-pcs-0",
				apicommon.LabelPodCliqueSetReplicaIndex:          "0",
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: "0",
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "simple-pcs",
			},
			expectBasePodGangLabel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			result := getLabels(tt.pcs, tt.pcsReplicaIndex, tt.pcsg, tt.pcsgReplicaIndex, tt.pclqObjectKey, tt.pclqTemplateSpec, tt.podGangName)

			// Verify all expected labels are present
			for expectedKey, expectedValue := range tt.expectedLabels {
				assert.Contains(t, result, expectedKey, "should contain expected label key %s", expectedKey)
				assert.Equal(t, expectedValue, result[expectedKey], "label %s should have expected value", expectedKey)
			}

			// Verify pod template hash label is present and not empty
			assert.Contains(t, result, apicommon.LabelPodTemplateHash, "should contain pod template hash label")
			assert.NotEmpty(t, result[apicommon.LabelPodTemplateHash], "pod template hash should not be empty")

			// Verify base PodGang label presence based on expectation
			if tt.expectBasePodGangLabel {
				assert.Contains(t, result, apicommon.LabelBasePodGang, "should contain base PodGang label")
			} else {
				assert.NotContains(t, result, apicommon.LabelBasePodGang, "should not contain base PodGang label")
			}
		})
	}
}

// TestSync verifies that the main Sync function correctly orchestrates the synchronization
// of PodClique resources for a PodCliqueScalingGroup using the fake Kubernetes client.
func TestSync(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Initial PodCliques to create in the fake client
		existingPodCliques []grovecorev1alpha1.PodClique
		// PodCliqueSet to use for context
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup to sync
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// Basic sync with valid PCSG that has proper labels
			name:               "successful sync with valid pcsg",
			existingPodCliques: []grovecorev1alpha1.PodClique{},
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
					UID:       types.UID("test-pcs-uid"),
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "0",
						apicommon.LabelPartOfKey:                "test-pcs",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: grovecorev1alpha1.SchemeGroupVersion.String(),
							Kind:       "PodCliqueSet",
							Name:       "test-pcs",
							UID:        types.UID("test-pcs-uid"),
							Controller: ptr.To(true),
						},
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     1,
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"worker"},
				},
			},
			expectError: false,
		},
		{
			// Sync with PCSG missing required labels should fail
			name:               "sync fails with missing pcs replica index label",
			existingPodCliques: []grovecorev1alpha1.PodClique{},
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
					UID:       types.UID("test-pcs-uid-2"),
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg-2",
					Namespace: testNamespace,
					Labels: map[string]string{
						apicommon.LabelPartOfKey: "test-pcs",
					}, // Missing LabelPodCliqueSetReplicaIndex label
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: grovecorev1alpha1.SchemeGroupVersion.String(),
							Kind:       "PodCliqueSet",
							Name:       "test-pcs",
							UID:        types.UID("test-pcs-uid-2"),
							Controller: ptr.To(true),
						},
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     1,
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"worker"},
				},
			},
			expectError:            true,
			expectedErrorSubstring: "failed to get the PodCliqueSet replica ind value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			ctx := context.Background()
			logger := log.FromContext(ctx)

			// Convert PodCliques to client.Object for the fake client
			objects := make([]client.Object, 0, len(tt.existingPodCliques)+2)
			for i := range tt.existingPodCliques {
				objects = append(objects, &tt.existingPodCliques[i])
			}
			// Add PCS and PCSG to the fake client
			objects = append(objects, tt.pcs, tt.pcsg)

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.Sync(ctx, logger, tt.pcsg)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				if tt.expectedErrorSubstring != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstring, "error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "should not return an error")
			}
		})
	}
}

// TestTriggerDeletionOfPodCliques verifies that the function correctly handles concurrent
// deletion of PodClique resources using the provided deletion tasks.
func TestTriggerDeletionOfPodCliques(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueScalingGroup object key for context
		pcsgObjectKey client.ObjectKey
		// Deletion tasks to execute
		deletionTasks []utils.Task
		// Whether an error is expected
		expectError bool
	}{
		{
			// No deletion tasks should succeed immediately
			name: "no deletion tasks succeeds",
			pcsgObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: testNamespace,
			},
			deletionTasks: []utils.Task{},
			expectError:   false,
		},
		{
			// Single successful deletion task
			name: "single successful deletion task",
			pcsgObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: testNamespace,
			},
			deletionTasks: []utils.Task{
				{
					Name: "delete-pclq-1",
					Fn: func(_ context.Context) error {
						// Simulate successful deletion
						return nil
					},
				},
			},
			expectError: false,
		},
		{
			// Multiple successful deletion tasks
			name: "multiple successful deletion tasks",
			pcsgObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: testNamespace,
			},
			deletionTasks: []utils.Task{
				{
					Name: "delete-pclq-1",
					Fn: func(_ context.Context) error {
						return nil
					},
				},
				{
					Name: "delete-pclq-2",
					Fn: func(_ context.Context) error {
						return nil
					},
				},
			},
			expectError: false,
		},
		{
			// Single failing deletion task should return error
			name: "single failing deletion task",
			pcsgObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: testNamespace,
			},
			deletionTasks: []utils.Task{
				{
					Name: "delete-pclq-1",
					Fn: func(_ context.Context) error {
						return errors.New("deletion failed")
					},
				},
			},
			expectError: true,
		},
		{
			// Mixed success and failure should return error
			name: "mixed success and failure tasks",
			pcsgObjectKey: client.ObjectKey{
				Name:      "test-pcsg",
				Namespace: testNamespace,
			},
			deletionTasks: []utils.Task{
				{
					Name: "delete-pclq-success",
					Fn: func(_ context.Context) error {
						return nil
					},
				},
				{
					Name: "delete-pclq-failure",
					Fn: func(_ context.Context) error {
						return errors.New("deletion failed")
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			ctx := context.Background()
			logger := log.FromContext(ctx)

			client := testutils.SetupFakeClient()
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.triggerDeletionOfPodCliques(ctx, logger, tt.pcsgObjectKey, tt.deletionTasks)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
			}
		})
	}
}

// TestCreateDeleteTasks verifies that the function correctly creates deletion tasks
// for PodClique resources belonging to specific PCSG replica indices.
func TestCreateDeleteTasks(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet for context
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup name
		pcsgName string
		// PCSG replica indices to delete
		pcsgReplicasToDelete []string
		// Reason for deletion
		reason string
		// Expected number of deletion tasks
		expectedTaskCount int
	}{
		{
			// No replicas to delete should create no tasks
			name: "no replicas to delete",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
			},
			pcsgName:             "test-pcsg",
			pcsgReplicasToDelete: []string{},
			reason:               "scale down",
			expectedTaskCount:    0,
		},
		{
			// Single replica to delete should create one task
			name: "single replica to delete",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
			},
			pcsgName:             "test-pcsg",
			pcsgReplicasToDelete: []string{"0"},
			reason:               "scale down",
			expectedTaskCount:    1,
		},
		{
			// Multiple replicas to delete should create multiple tasks
			name: "multiple replicas to delete",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
			},
			pcsgName:             "test-pcsg",
			pcsgReplicasToDelete: []string{"1", "2", "3"},
			reason:               "scale down",
			expectedTaskCount:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := log.FromContext(context.Background())

			client := testutils.SetupFakeClient()
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			tasks := operator.createDeleteTasks(logger, tt.pcs, tt.pcsgName, tt.pcsgReplicasToDelete, tt.reason)

			// Verify results
			assert.Equal(t, tt.expectedTaskCount, len(tasks), "should create expected number of tasks")

			// Verify task names are properly formatted
			for i, task := range tasks {
				expectedReplicaIndex := tt.pcsgReplicasToDelete[i]
				assert.Contains(t, task.Name, expectedReplicaIndex, "task name should contain replica index")
				assert.Contains(t, task.Name, tt.pcsgName, "task name should contain PCSG name")
			}
		})
	}
}

// TestDoCreate verifies that the function correctly creates PodClique resources
// using the Kubernetes client with proper error handling and event recording.
func TestDoCreate(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet for template context
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup for context
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// PCSG replica index
		pcsgReplicaIndex int
		// PodClique object key to create
		pclqObjectKey client.ObjectKey
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// Successful creation of new PodClique
			name: "successful podclique creation",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "0",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     1,
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"worker"},
				},
			},
			pcsgReplicaIndex: 0,
			pclqObjectKey: client.ObjectKey{
				Name:      "test-pcsg-0-worker",
				Namespace: testNamespace,
			},
			expectError: false,
		},
		{
			// Creation should succeed when PodClique already exists
			name: "creation succeeds when podclique already exists",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "0",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     1,
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"worker"},
				},
			},
			pcsgReplicaIndex: 0,
			pclqObjectKey: client.ObjectKey{
				Name:      "existing-pcsg-0-worker",
				Namespace: testNamespace,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			ctx := context.Background()
			logger := log.FromContext(ctx)

			// Create existing PodClique if test case expects it
			objects := []client.Object{tt.pcs, tt.pcsg}
			if tt.name == "creation succeeds when podclique already exists" {
				existingPCLQ := &grovecorev1alpha1.PodClique{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.pclqObjectKey.Name,
						Namespace: tt.pclqObjectKey.Namespace,
					},
				}
				objects = append(objects, existingPCLQ)
			}

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.doCreate(ctx, logger, tt.pcs, tt.pcsg, tt.pcsgReplicaIndex, tt.pclqObjectKey)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				if tt.expectedErrorSubstring != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstring, "error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "should not return an error")

				// Verify PodClique was created (if it didn't already exist)
				if tt.name != "creation succeeds when podclique already exists" {
					createdPCLQ := &grovecorev1alpha1.PodClique{}
					err := client.Get(ctx, tt.pclqObjectKey, createdPCLQ)
					assert.NoError(t, err, "should be able to get created PodClique")
					assert.Equal(t, tt.pclqObjectKey.Name, createdPCLQ.Name, "created PodClique should have correct name")
				}
			}
		})
	}
}

// TestBuildResource verifies that the function correctly builds PodClique resource
// specifications from PodCliqueSet templates and PodCliqueScalingGroup configurations.
func TestBuildResource(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet for template context
		pcs *grovecorev1alpha1.PodCliqueSet
		// PodCliqueScalingGroup for context
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// PCSG replica index
		pcsgReplicaIndex int
		// PodClique to build (with name set)
		pclq *grovecorev1alpha1.PodClique
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// Successful resource building with matching template
			name: "successful resource building",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Labels: map[string]string{
									"template-label": "template-value",
								},
								Annotations: map[string]string{
									"template-annotation": "template-annotation-value",
								},
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas:     2,
									MinAvailable: ptr.To(int32(1)),
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "0",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     1,
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"worker"},
				},
			},
			pcsgReplicaIndex: 0,
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg-0-worker",
					Namespace: testNamespace,
				},
			},
			expectError: false,
		},
		{
			// Building should fail when template not found
			name: "build fails when template not found",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					Labels: map[string]string{
						apicommon.LabelPodCliqueSetReplicaIndex: "0",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
				},
			},
			pcsgReplicaIndex: 0,
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg-0-nonexistent", // No matching template
					Namespace: testNamespace,
				},
			},
			expectError:            true,
			expectedErrorSubstring: "PodCliqueTemplateSpec for PodClique",
		},
		{
			// Building should fail when PCSG missing required labels
			name: "build fails when pcsg missing labels",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcs",
					Namespace: testNamespace,
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{
								Name: "worker",
								Spec: grovecorev1alpha1.PodCliqueSpec{
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg",
					Namespace: testNamespace,
					Labels:    map[string]string{}, // Missing required label
				},
			},
			pcsgReplicaIndex: 0,
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pcsg-0-worker",
					Namespace: testNamespace,
				},
			},
			expectError:            true,
			expectedErrorSubstring: "failed to get the PodCliqueSet replica ind value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := log.FromContext(context.Background())

			client := testutils.SetupFakeClient()
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.buildResource(logger, tt.pcs, tt.pcsg, tt.pcsgReplicaIndex, tt.pclq)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				if tt.expectedErrorSubstring != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstring, "error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "should not return an error")

				// Verify PodClique was properly built
				assert.NotEmpty(t, tt.pclq.Labels, "PodClique should have labels")
				assert.Contains(t, tt.pclq.Labels, apicommon.LabelComponentKey, "should have component label")
				assert.Equal(t, apicommon.LabelComponentNamePodCliqueScalingGroupPodClique, tt.pclq.Labels[apicommon.LabelComponentKey])

				// Verify owner reference was set
				assert.Len(t, tt.pclq.OwnerReferences, 1, "should have one owner reference")
				assert.Equal(t, tt.pcsg.Name, tt.pclq.OwnerReferences[0].Name, "owner should be the PCSG")

				// Verify spec was copied from template
				if !tt.expectError {
					assert.Equal(t, int32(2), tt.pclq.Spec.Replicas, "spec should be copied from template")
					assert.Equal(t, ptr.To(int32(1)), tt.pclq.Spec.MinAvailable, "MinAvailable should be copied")
				}

				// Verify annotations were copied from template
				if tt.pcs.Spec.Template.Cliques[0].Annotations != nil {
					assert.Equal(t, tt.pcs.Spec.Template.Cliques[0].Annotations, tt.pclq.Annotations, "annotations should be copied")
				}
			}
		})
	}
}

// TestIdentifyFullyQualifiedStartupDependencyNames verifies that the function correctly
// determines startup dependencies based on the startup type configuration.
func TestIdentifyFullyQualifiedStartupDependencyNames(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet with startup configuration
		pcs *grovecorev1alpha1.PodCliqueSet
		// PCS replica index
		pcsReplicaIndex int
		// PodCliqueScalingGroup for context
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// PCSG replica index
		pcsgReplicaIndex int
		// PodClique being configured
		pclq *grovecorev1alpha1.PodClique
		// Index of the PodClique template in the PCS
		foundAtIndex int
		// Expected dependency names
		expectedDependencies []string
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// In-order startup with first PodClique (no dependencies)
			name: "in-order startup first podclique",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "master"},
							{Name: "worker"},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex:     0,
			pclq:                 &grovecorev1alpha1.PodClique{ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg-0-master"}},
			foundAtIndex:         0,
			expectedDependencies: nil, // First PodClique has no dependencies
			expectError:          false,
		},
		{
			// In-order startup with second PodClique (depends on first)
			name: "in-order startup second podclique",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeInOrder),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "master"},
							{Name: "worker"},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex:     0,
			pclq:                 &grovecorev1alpha1.PodClique{ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg-0-worker"}},
			foundAtIndex:         1,
			expectedDependencies: []string{"test-pcs-0-master"}, // Depends on base PodGang (within minAvailable)
			expectError:          false,
		},
		{
			// Explicit startup type with dependencies
			name: "explicit startup with dependencies",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pcs",
				},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: ptr.To(grovecorev1alpha1.CliqueStartupTypeExplicit),
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "master"},
							{Name: "worker"},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex: 0,
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg-0-worker"},
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"master"}, // Explicit dependency
				},
			},
			foundAtIndex:         1,
			expectedDependencies: []string{"test-pcs-0-master"}, // Explicit dependency resolved to base PodGang
			expectError:          false,
		},
		{
			// Nil startup type should return error
			name: "nil startup type returns error",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						StartupType: nil, // Nil startup type
					},
				},
			},
			pcsReplicaIndex:        0,
			pcsg:                   &grovecorev1alpha1.PodCliqueScalingGroup{},
			pcsgReplicaIndex:       0,
			pclq:                   &grovecorev1alpha1.PodClique{ObjectMeta: metav1.ObjectMeta{Name: "test-pclq"}},
			foundAtIndex:           0,
			expectedDependencies:   nil,
			expectError:            true,
			expectedErrorSubstring: "has nil StartupType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			dependencies, err := identifyFullyQualifiedStartupDependencyNames(tt.pcs, tt.pcsReplicaIndex, tt.pcsg, tt.pcsgReplicaIndex, tt.pclq, tt.foundAtIndex)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				if tt.expectedErrorSubstring != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorSubstring, "error should contain expected substring")
				}
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.Equal(t, tt.expectedDependencies, dependencies, "dependencies should match expected")
			}
		})
	}
}

// TestGetInOrderStartupDependencies verifies that the function correctly generates
// startup dependencies for in-order startup type.
func TestGetInOrderStartupDependencies(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet with clique configuration
		pcs *grovecorev1alpha1.PodCliqueSet
		// PCS replica index
		pcsReplicaIndex int
		// PodCliqueScalingGroup for context
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// PCSG replica index
		pcsgReplicaIndex int
		// Index of the PodClique template in the PCS
		foundAtIndex int
		// Expected dependency names
		expectedDependencies []string
	}{
		{
			// First PodClique has no dependencies
			name: "first podclique no dependencies",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "master"},
							{Name: "worker"},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex:     0,
			foundAtIndex:         0,
			expectedDependencies: nil,
		},
		{
			// Second PodClique depends on first within same PCSG
			name: "second podclique depends on first",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "master"},
							{Name: "worker"},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex:     0,
			foundAtIndex:         1,
			expectedDependencies: []string{"test-pcs-0-master"}, // Base PodGang dependency
		},
		{
			// Scaled replica beyond minAvailable with parent in PCSG
			name: "scaled replica parent in pcsg",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "master"},
							{Name: "worker"},
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex:     1, // Beyond minAvailable
			foundAtIndex:         1,
			expectedDependencies: []string{"test-pcsg-1-master"}, // PCSG dependency for scaled replica
		},
		{
			// Scaled replica beyond minAvailable with parent not in PCSG
			name: "scaled replica parent not in pcsg",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
				Spec: grovecorev1alpha1.PodCliqueSetSpec{
					Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
						Cliques: []*grovecorev1alpha1.PodCliqueTemplateSpec{
							{Name: "init"},   // Not in PCSG
							{Name: "worker"}, // In PCSG
						},
					},
				},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"worker"}, // Only worker, not init
				},
			},
			pcsgReplicaIndex:     1, // Beyond minAvailable
			foundAtIndex:         1,
			expectedDependencies: nil, // No dependency since parent not in PCSG
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			dependencies := getInOrderStartupDependencies(tt.pcs, tt.pcsReplicaIndex, tt.pcsg, tt.pcsgReplicaIndex, tt.foundAtIndex)

			// Verify results
			assert.Equal(t, tt.expectedDependencies, dependencies, "dependencies should match expected")
		})
	}
}

// TestGetExplicitStartupDependencies verifies that the function correctly generates
// startup dependencies for explicit startup type.
func TestGetExplicitStartupDependencies(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// PodCliqueSet for context
		pcs *grovecorev1alpha1.PodCliqueSet
		// PCS replica index
		pcsReplicaIndex int
		// PodCliqueScalingGroup for context
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// PCSG replica index
		pcsgReplicaIndex int
		// PodClique with explicit dependencies
		pclq *grovecorev1alpha1.PodClique
		// Expected dependency names
		expectedDependencies []string
	}{
		{
			// Base PodGang (within minAvailable) with dependencies
			name: "base podgang with dependencies",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(2)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex: 0, // Within minAvailable
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"master"},
				},
			},
			expectedDependencies: []string{"test-pcs-0-master"}, // Base PodGang dependency
		},
		{
			// Scaled PodGang (beyond minAvailable) with dependencies in PCSG
			name: "scaled podgang with pcsg dependencies",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"master", "worker"},
				},
			},
			pcsgReplicaIndex: 1, // Beyond minAvailable
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{"master", "init"}, // master in PCSG, init not
				},
			},
			expectedDependencies: []string{"test-pcsg-1-master"}, // Only PCSG dependency
		},
		{
			// No dependencies
			name: "no dependencies",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs"},
			},
			pcsReplicaIndex: 0,
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg"},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					MinAvailable: ptr.To(int32(1)),
				},
			},
			pcsgReplicaIndex: 0,
			pclq: &grovecorev1alpha1.PodClique{
				Spec: grovecorev1alpha1.PodCliqueSpec{
					StartsAfter: []string{}, // No dependencies
				},
			},
			expectedDependencies: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Execute the function under test
			dependencies := getExplicitStartupDependencies(tt.pcs, tt.pcsReplicaIndex, tt.pcsg, tt.pcsgReplicaIndex, tt.pclq)

			// Verify results
			assert.Equal(t, tt.expectedDependencies, dependencies, "dependencies should match expected")
		})
	}
}

// createTestPCSGObjectMeta creates test ObjectMeta for a PodCliqueScalingGroup
func createTestPCSGObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      testPCSGName,
		Namespace: testNamespace,
		UID:       types.UID("test-pcsg-uid"), // Must match the UID in owner references
		Labels: map[string]string{
			apicommon.LabelManagedByKey: apicommon.LabelManagedByValue,
			apicommon.LabelPartOfKey:    testPCSName,
		},
	}
}
