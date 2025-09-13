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
	"context"
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test constants
const (
	testNamespace = "test-namespace"
	testPGSName   = "test-pgs"
)

func TestComputePGSAvailableReplicas(t *testing.T) {
	pgsGenerationHash := string(uuid.NewUUID())
	pgsUID := uuid.NewUUID()
	testCases := []struct {
		name              string
		setupPGS          func() *grovecorev1alpha1.PodCliqueSet
		childResources    func() []client.Object
		expectedAvailable int32
	}{
		{
			name: "all healthy - 2 replicas with standalone and scaling groups",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(2).
					WithStandaloneClique("worker").
					WithScalingGroup("compute", []string{"frontend", "backend"}).
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					// Healthy PCSGs
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-1-compute", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
					// Healthy standalone PodCliques
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "worker", testNamespace, 0).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "worker", testNamespace, 1).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
				}
			},
			expectedAvailable: 2,
		},
		{
			name: "mixed health - 1 healthy, 1 unhealthy",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(2).
					WithStandaloneClique("worker").
					WithScalingGroup("compute", []string{"frontend"}).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-1-compute", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGMinAvailableBreached()).Build(),
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "worker", testNamespace, 0).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "worker", testNamespace, 1).
						WithOptions(testutils.WithPCLQTerminating(), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
				}
			},
			expectedAvailable: 1,
		},
		{
			name: "count mismatch - missing resources",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(1).
					WithStandaloneClique("worker").
					WithScalingGroup("compute", []string{"frontend"}).
					Build()
			},
			childResources: func() []client.Object {
				// Missing PCSG, extra standalone PodClique
				return []client.Object{
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "test-pgs-0-worker", testNamespace, 0).
						WithOptions(testutils.WithPCLQAvailable(), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "test-pgs-0-extra", testNamespace, 0).
						WithOptions(testutils.WithPCLQAvailable(), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
				}
			},
			expectedAvailable: 0,
		},
		{
			name: "empty configuration",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(1).
					Build()
			},
			childResources:    func() []client.Object { return []client.Object{} },
			expectedAvailable: 1,
		},
		{
			name: "only standalone cliques - all healthy",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(2).
					WithStandaloneClique("worker").
					WithStandaloneClique("monitor").
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "worker", testNamespace, 0).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "monitor", testNamespace, 0).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "worker", testNamespace, 1).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "monitor", testNamespace, 1).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
				}
			},
			expectedAvailable: 2,
		},
		{
			name: "only scaling groups - all healthy",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(2).
					WithScalingGroup("compute", []string{"frontend", "backend"}).
					WithScalingGroup("storage", []string{"database"}).
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-0-storage", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-1-compute", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-1-storage", testNamespace, testPGSName, 1).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
				}
			},
			expectedAvailable: 2,
		},
		{
			name: "terminating resources",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(1).
					WithStandaloneClique("worker").
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "test-pgs-0-worker", testNamespace, 0).
						WithOptions(testutils.WithPCLQTerminating(), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
				}
			},
			expectedAvailable: 0,
		},
		{
			name: "unknown condition status",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(1).
					WithScalingGroup("compute", []string{"frontend"}).
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-0-compute", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGUnknownCondition()).Build(),
				}
			},
			expectedAvailable: 0,
		},
		{
			name: "no conditions set",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(1).
					WithStandaloneClique("worker").
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "test-pgs-0-worker", testNamespace, 0).
						WithOptions(testutils.WithPCLQNoConditions()).Build(),
				}
			},
			expectedAvailable: 0,
		},
		// Edge cases
		{
			name: "no child resources when expected",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(1).
					WithScalingGroup("compute", []string{"frontend"}).
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources:    func() []client.Object { return []client.Object{} },
			expectedAvailable: 0,
		},
		{
			name: "extra unexpected resources",
			setupPGS: func() *grovecorev1alpha1.PodCliqueSet {
				return testutils.NewPodGangSetBuilder(testPGSName, testNamespace, pgsUID).
					WithReplicas(1).
					WithStandaloneClique("worker").
					WithPodGangSetGenerationHash(&pgsGenerationHash).
					Build()
			},
			childResources: func() []client.Object {
				return []client.Object{
					testutils.NewPodCliqueBuilder(testPGSName, uuid.NewUUID(), "worker", testNamespace, 0).
						WithOptions(testutils.WithPCLQReplicaReadyStatus(1), testutils.WithPCLQCurrentPGSGenerationHash(pgsGenerationHash)).Build(),
					testutils.NewPodCliqueScalingGroupBuilder("test-pgs-0-unexpected", testNamespace, testPGSName, 0).
						WithOptions(testutils.WithPCSGAvailableReplicas(1)).Build(),
				}
			},
			expectedAvailable: 1,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			//t.Parallel()
			pgs := tt.setupPGS()
			existingObjects := []client.Object{pgs}
			existingObjects = append(existingObjects, tt.childResources()...)
			cl := testutils.CreateDefaultFakeClient(existingObjects)
			reconciler := &Reconciler{client: cl}
			// Compute available replicas
			available, _, err := reconciler.computeAvailableAndUpdatedReplicas(context.Background(), logr.Discard(), pgs)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedAvailable, available, "Available replicas mismatch")
		})
	}
}
