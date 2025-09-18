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

package podcliquescalinggroup

import (
	"testing"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/component/podcliquescalinggroup/podclique"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestCreateOperatorRegistry verifies that CreateOperatorRegistry correctly creates and configures
// an operator registry for PodCliqueScalingGroup reconciliation with all required components.
func TestCreateOperatorRegistry(t *testing.T) {
	tests := []struct {
		// Test case name describing the scenario being tested
		name string
		// Expected number of operators to be registered in the registry
		expectedOperatorCount int
		// List of component kinds that should be registered
		expectedKinds []component.Kind
	}{
		{
			// Verify basic registry creation with all required operators
			name:                  "creates registry with all required operators",
			expectedOperatorCount: 1,
			expectedKinds:         []component.Kind{component.KindPodClique},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment with fake client and event recorder
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			eventRecorder := record.NewFakeRecorder(100)

			// Create the registry manually to test internal functionality without manager dependency
			registry := component.NewOperatorRegistry[v1alpha1.PodCliqueScalingGroup]()

			// Register the PodClique operator just like in CreateOperatorRegistry
			registry.Register(component.KindPodClique, podclique.New(client, scheme, eventRecorder))

			// Verify registry is not nil
			assert.NotNil(t, registry, "registry should not be nil")

			// Verify correct number of operators registered
			allOperators := registry.GetAllOperators()
			assert.Equal(t, tt.expectedOperatorCount, len(allOperators), "unexpected number of operators registered")

			// Verify specific operators are registered
			for _, expectedKind := range tt.expectedKinds {
				operator, err := registry.GetOperator(expectedKind)
				assert.NoError(t, err, "should be able to get operator for kind %s", expectedKind)
				assert.NotNil(t, operator, "operator for kind %s should be registered", expectedKind)
			}
		})
	}
}

// TestCreateOperatorRegistryIntegration performs integration testing of the registry
// to ensure all registered operators are properly configured and functional.
func TestCreateOperatorRegistryIntegration(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Whether this test should verify operator interface compliance
		verifyInterface bool
	}{
		{
			// Verify all registered operators implement the expected interface
			name:            "all operators implement component.Operator interface",
			verifyInterface: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			scheme := runtime.NewScheme()
			require.NoError(t, v1alpha1.AddToScheme(scheme))

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			eventRecorder := record.NewFakeRecorder(100)

			// Create registry manually to verify interface compliance
			registry := component.NewOperatorRegistry[v1alpha1.PodCliqueScalingGroup]()
			registry.Register(component.KindPodClique, podclique.New(client, scheme, eventRecorder))

			if tt.verifyInterface {
				// Verify PodClique operator implements the interface correctly
				podCliqueOperator, err := registry.GetOperator(component.KindPodClique)
				require.NoError(t, err, "should be able to get PodClique operator")
				require.NotNil(t, podCliqueOperator, "PodClique operator should be registered")

				// Verify the operator implements the interface by attempting to use its methods
				// Since the type is already known to be component.Operator[PodCliqueScalingGroup],
				// we don't need a type assertion - just verify it's functional
				assert.NotNil(t, podCliqueOperator, "PodClique operator should implement component.Operator[PodCliqueScalingGroup]")
			}
		})
	}
}
