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

package podgangset

import (
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/testutils"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test constants
const (
	testNamespace = "test-namespace"
	testPGSName   = "test-pgs"
)

// Helper functions for testing

// setupTestReconciler creates a reconciler with fake client containing the provided objects
func setupTestReconciler(pgs *grovecorev1alpha1.PodGangSet, childObjects []client.Object) *Reconciler {
	allObjects := append([]client.Object{pgs}, childObjects...)
	fakeClient := testutils.SetupFakeClient(allObjects...)
	return &Reconciler{client: fakeClient}
}

// assertAvailableReplicas runs the computePGSAvailableReplicas test and validates the result
func assertAvailableReplicas(t *testing.T, reconciler *Reconciler, pgs *grovecorev1alpha1.PodGangSet, expected int32) {
	available, err := reconciler.computePGSAvailableReplicas(
		testutils.SetupTestContext(),
		testutils.SetupTestLogger(),
		pgs,
	)
	assert.NoError(t, err)
	assert.Equal(t, expected, available, "Available replicas mismatch")
}

// pgsAvailableReplicasTestCase represents a test case for PGS available replicas testing
type pgsAvailableReplicasTestCase struct {
	name              string
	setupPGS          func() *grovecorev1alpha1.PodGangSet
	childResources    func() []client.Object
	expectedAvailable int32
}

// runPGSAvailableReplicasTests executes a set of PGS available replicas test cases
func runPGSAvailableReplicasTests(t *testing.T, testCases []pgsAvailableReplicasTestCase) {
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pgs := tt.setupPGS()
			children := tt.childResources()
			reconciler := setupTestReconciler(pgs, children)

			assertAvailableReplicas(t, reconciler, pgs, tt.expectedAvailable)
		})
	}
}

func TestComputePGSAvailableReplicas(t *testing.T) {
	tests := []pgsAvailableReplicasTestCase{
		{
			name: "all healthy - 2 replicas with standalone and scaling groups",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(2).
					WithStandaloneClique("worker").
					WithScalingGroup("compute", []string{"frontend", "backend"}).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					// Healthy PCSGs
					testutils.NewPCSGBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
					testutils.NewPCSGBuilder("test-pgs-1-compute", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
					// Healthy standalone PodCliques
					testutils.NewPodCliqueBuilder("test-pgs-0-worker", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
					testutils.NewPodCliqueBuilder("test-pgs-1-worker", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
				}
			},
			expectedAvailable: 2,
		},
		{
			name: "mixed health - 1 healthy, 1 unhealthy",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(2).
					WithStandaloneClique("worker").
					WithScalingGroup("compute", []string{"frontend"}).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPCSGBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
					testutils.NewPCSGBuilder("test-pgs-1-compute", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGMinAvailableBreached()).Build(),
					testutils.NewPodCliqueBuilder("test-pgs-0-worker", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
					testutils.NewPodCliqueBuilder("test-pgs-1-worker", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCLQTerminating()).Build(),
				}
			},
			expectedAvailable: 1,
		},
		{
			name: "count mismatch - missing resources",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(1).
					WithStandaloneClique("worker").
					WithScalingGroup("compute", []string{"frontend"}).
					Build()
			},
			childResources: func() []client.Object {
				// Missing PCSG, extra standalone PodClique
				return []client.Object{
					testutils.NewPodCliqueBuilder("test-pgs-0-worker", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
					testutils.NewPodCliqueBuilder("test-pgs-0-extra", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
				}
			},
			expectedAvailable: 0,
		},
		{
			name: "empty configuration",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(1).
					Build()
			},
			childResources:    func() []client.Object { return []client.Object{} },
			expectedAvailable: 1,
		},
		{
			name: "only standalone cliques - all healthy",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(2).
					WithStandaloneClique("worker").
					WithStandaloneClique("monitor").
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder("test-pgs-0-worker", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
					testutils.NewPodCliqueBuilder("test-pgs-0-monitor", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
					testutils.NewPodCliqueBuilder("test-pgs-1-worker", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
					testutils.NewPodCliqueBuilder("test-pgs-1-monitor", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
				}
			},
			expectedAvailable: 2,
		},
		{
			name: "only scaling groups - all healthy",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(2).
					WithScalingGroup("compute", []string{"frontend", "backend"}).
					WithScalingGroup("storage", []string{"database"}).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPCSGBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
					testutils.NewPCSGBuilder("test-pgs-0-storage", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
					testutils.NewPCSGBuilder("test-pgs-1-compute", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
					testutils.NewPCSGBuilder("test-pgs-1-storage", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
				}
			},
			expectedAvailable: 2,
		},
		{
			name: "terminating resources",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(1).
					WithStandaloneClique("worker").
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder("test-pgs-0-worker", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQTerminating()).Build(),
				}
			},
			expectedAvailable: 0,
		},
		{
			name: "unknown condition status",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(1).
					WithScalingGroup("compute", []string{"frontend"}).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPCSGBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGUnknownCondition()).Build(),
				}
			},
			expectedAvailable: 0,
		},
		{
			name: "no conditions set",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(1).
					WithStandaloneClique("worker").
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder("test-pgs-0-worker", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQNoConditions()).Build(),
				}
			},
			expectedAvailable: 0,
		},
		// Edge cases
		{
			name: "no child resources when expected",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(1).
					WithScalingGroup("compute", []string{"frontend"}).
					Build()
			},
			childResources:    func() []client.Object { return []client.Object{} },
			expectedAvailable: 0,
		},
		{
			name: "extra unexpected resources",
			setupPGS: func() *grovecorev1alpha1.PodGangSet {
				return testutils.NewPGSBuilder(testPGSName, testNamespace).
					WithReplicas(1).
					WithStandaloneClique("worker").
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder("test-pgs-0-worker", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCLQAvailable()).Build(),
					testutils.NewPCSGBuilder("test-pgs-0-unexpected", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGHealthy()).Build(),
				}
			},
			expectedAvailable: 1,
		},
	}

	runPGSAvailableReplicasTests(t, tests)
}

func TestComputeExpectedResourceCounts(t *testing.T) {
	tests := []struct {
		name                    string
		pgs                     *grovecorev1alpha1.PodGangSet
		expectedStandalonePCLQs int
		expectedPCSGs           int
	}{
		{
			name: "only standalone cliques",
			pgs: testutils.NewPGSBuilder(testPGSName, testNamespace).
				WithStandaloneClique("worker").
				WithStandaloneClique("monitor").
				Build(),
			expectedStandalonePCLQs: 2,
			expectedPCSGs:           0,
		},
		{
			name: "only scaling groups",
			pgs: testutils.NewPGSBuilder(testPGSName, testNamespace).
				WithScalingGroup("compute", []string{"frontend", "backend"}).
				WithScalingGroup("storage", []string{"database"}).
				Build(),
			expectedStandalonePCLQs: 0,
			expectedPCSGs:           2,
		},
		{
			name: "mixed configuration",
			pgs: testutils.NewPGSBuilder(testPGSName, testNamespace).
				WithStandaloneClique("worker").
				WithScalingGroup("compute", []string{"frontend"}).
				Build(),
			expectedStandalonePCLQs: 1,
			expectedPCSGs:           1,
		},
		{
			name:                    "empty configuration",
			pgs:                     testutils.NewPGSBuilder(testPGSName, testNamespace).Build(),
			expectedStandalonePCLQs: 0,
			expectedPCSGs:           0,
		},
		{
			name: "complex configuration",
			pgs: testutils.NewPGSBuilder(testPGSName, testNamespace).
				WithStandaloneClique("coordinator").
				WithScalingGroup("web", []string{"frontend", "backend"}).
				WithStandaloneClique("monitor").
				WithScalingGroup("data", []string{"database", "cache"}).
				WithStandaloneClique("logger").
				Build(),
			expectedStandalonePCLQs: 3,
			expectedPCSGs:           2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reconciler := &Reconciler{}
			actualStandalone, actualPCSGs := reconciler.computeExpectedResourceCounts(tt.pgs)

			assert.Equal(t, tt.expectedStandalonePCLQs, actualStandalone, "Standalone PodCliques count")
			assert.Equal(t, tt.expectedPCSGs, actualPCSGs, "PCSGs count")
		})
	}
}
