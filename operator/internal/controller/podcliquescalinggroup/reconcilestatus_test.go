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
	"context"
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test helpers
func buildHealthyClique(name string) grovecorev1alpha1.PodClique {
	return *testutils.NewPodCliqueBuilder(name, "test-ns", "test-pgs", 0).
		WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build()
}

func buildScheduledClique(name string) grovecorev1alpha1.PodClique {
	return *testutils.NewPodCliqueBuilder(name, "test-ns", "test-pgs", 0).
		WithOptions(testutils.WithPCLQScheduledButBreached()).Build()
}

func buildFailedClique(name string) grovecorev1alpha1.PodClique {
	return *testutils.NewPodCliqueBuilder(name, "test-ns", "test-pgs", 0).
		WithOptions(testutils.WithPCLQNotScheduled()).Build()
}

func buildTerminatingClique(name string) grovecorev1alpha1.PodClique {
	return *testutils.NewPodCliqueBuilder(name, "test-ns", "test-pgs", 0).
		WithOptions(testutils.WithPCLQTerminating()).Build()
}

func buildTestPCSG(replicas int32) *grovecorev1alpha1.PodCliqueScalingGroup {
	return testutils.NewPCSGBuilder("test-pcsg", "test-ns", "test-pgs", 0).
		WithReplicas(replicas).
		WithCliqueNames([]string{"frontend", "backend"}).Build()
}

func assertCondition(t *testing.T, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, expectBreached bool) {
	var condition *metav1.Condition
	for i := range pcsg.Status.Conditions {
		if pcsg.Status.Conditions[i].Type == "MinAvailableBreached" {
			condition = &pcsg.Status.Conditions[i]
			break
		}
	}

	require.NotNil(t, condition, "MinAvailableBreached condition should exist")
	isBreached := condition.Status == metav1.ConditionTrue
	assert.Equal(t, expectBreached, isBreached, "condition breach status mismatch")
}

// ============================================================================
// Unit Tests
// ============================================================================

func TestComputeReplicaStatus(t *testing.T) {
	logger := testutils.SetupTestLogger()

	tests := []struct {
		name          string
		expectedSize  int
		cliques       []grovecorev1alpha1.PodClique
		wantScheduled bool
		wantAvailable bool
	}{
		{
			name:          "healthy replica",
			expectedSize:  2,
			cliques:       []grovecorev1alpha1.PodClique{buildHealthyClique("frontend"), buildHealthyClique("backend")},
			wantScheduled: true,
			wantAvailable: true,
		},
		{
			name:          "incomplete replica",
			expectedSize:  3,
			cliques:       []grovecorev1alpha1.PodClique{buildHealthyClique("frontend")},
			wantScheduled: false,
			wantAvailable: false,
		},
		{
			name:          "scheduling failed",
			expectedSize:  2,
			cliques:       []grovecorev1alpha1.PodClique{buildHealthyClique("frontend"), buildFailedClique("backend")},
			wantScheduled: false,
			wantAvailable: false,
		},
		{
			name:          "scheduled but unavailable",
			expectedSize:  2,
			cliques:       []grovecorev1alpha1.PodClique{buildHealthyClique("frontend"), buildScheduledClique("backend")},
			wantScheduled: true,
			wantAvailable: false,
		},
		{
			name:          "terminating cliques ignored",
			expectedSize:  2,
			cliques:       []grovecorev1alpha1.PodClique{buildHealthyClique("frontend"), buildHealthyClique("backend"), buildTerminatingClique("old-backend")},
			wantScheduled: true,
			wantAvailable: true,
		},
		{
			name:          "insufficient non-terminated",
			expectedSize:  2,
			cliques:       []grovecorev1alpha1.PodClique{buildHealthyClique("frontend"), buildTerminatingClique("backend")},
			wantScheduled: false,
			wantAvailable: false,
		},
		{
			name:          "all terminated cliques",
			expectedSize:  2,
			cliques:       []grovecorev1alpha1.PodClique{buildTerminatingClique("frontend"), buildTerminatingClique("backend")},
			wantScheduled: false,
			wantAvailable: false,
		},
		{
			name:          "mixed terminated and healthy",
			expectedSize:  3,
			cliques:       []grovecorev1alpha1.PodClique{buildHealthyClique("frontend"), buildHealthyClique("backend"), buildHealthyClique("worker"), buildTerminatingClique("old-worker")},
			wantScheduled: true,
			wantAvailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduled, available := computeReplicaStatus(logger, tt.expectedSize, "0", tt.cliques)

			assert.Equal(t, tt.wantScheduled, scheduled, "scheduled mismatch")
			assert.Equal(t, tt.wantAvailable, available, "available mismatch")
		})
	}
}

func TestComputeMinAvailableBreachedCondition(t *testing.T) {
	tests := []struct {
		name         string
		replicas     int32
		minAvailable *int32
		scheduled    int32
		available    int32
		wantStatus   metav1.ConditionStatus
		wantReason   string
	}{
		{
			name:       "sufficient replicas",
			replicas:   3,
			scheduled:  3,
			available:  3,
			wantStatus: metav1.ConditionFalse,
			wantReason: "SufficientAvailablePodCliqueScalingGroupReplicas",
		},
		{
			name:         "custom minAvailable met",
			replicas:     5,
			minAvailable: ptr.To(int32(2)),
			scheduled:    3,
			available:    3,
			wantStatus:   metav1.ConditionFalse,
			wantReason:   "SufficientAvailablePodCliqueScalingGroupReplicas",
		},
		{
			name:         "insufficient scheduled",
			replicas:     3,
			minAvailable: ptr.To(int32(2)),
			scheduled:    1,
			available:    1,
			wantStatus:   metav1.ConditionFalse,
			wantReason:   "InsufficientScheduledPodCliqueScalingGroupReplicas",
		},
		{
			name:         "insufficient available",
			replicas:     3,
			minAvailable: ptr.To(int32(2)),
			scheduled:    3,
			available:    1,
			wantStatus:   metav1.ConditionTrue,
			wantReason:   "InsufficientAvailablePodCliqueScalingGroupReplicas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     tt.replicas,
					MinAvailable: tt.minAvailable,
				},
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					ScheduledReplicas: tt.scheduled,
					AvailableReplicas: tt.available,
				},
			}

			condition := computeMinAvailableBreachedCondition(pcsg)

			assert.Equal(t, "MinAvailableBreached", condition.Type)
			assert.Equal(t, tt.wantStatus, condition.Status)
			assert.Equal(t, tt.wantReason, condition.Reason)
		})
	}
}

func TestMutateReplicas(t *testing.T) {
	logger := testutils.SetupTestLogger()

	tests := []struct {
		name           string
		pcsg           *grovecorev1alpha1.PodCliqueScalingGroup
		replicaCliques map[string][]grovecorev1alpha1.PodClique
		wantScheduled  int32
		wantAvailable  int32
	}{
		{
			name: "healthy replica",
			pcsg: buildTestPCSG(1),
			replicaCliques: map[string][]grovecorev1alpha1.PodClique{
				"0": {buildHealthyClique("frontend"), buildHealthyClique("backend")},
			},
			wantScheduled: 1,
			wantAvailable: 1,
		},
		{
			name: "mixed states",
			pcsg: buildTestPCSG(3),
			replicaCliques: map[string][]grovecorev1alpha1.PodClique{
				"0": {buildHealthyClique("frontend-0"), buildHealthyClique("backend-0")},
				"1": {buildScheduledClique("frontend-1"), buildScheduledClique("backend-1")},
				"2": {buildFailedClique("frontend-2"), buildFailedClique("backend-2")},
			},
			wantScheduled: 2,
			wantAvailable: 1,
		},
		{
			name:           "no replicas",
			pcsg:           buildTestPCSG(2),
			replicaCliques: map[string][]grovecorev1alpha1.PodClique{},
			wantScheduled:  0,
			wantAvailable:  0,
		},
		{
			name: "with terminating cliques",
			pcsg: buildTestPCSG(2),
			replicaCliques: map[string][]grovecorev1alpha1.PodClique{
				"0": {buildHealthyClique("frontend-0"), buildHealthyClique("backend-0")},
				"1": {buildHealthyClique("frontend-1"), buildTerminatingClique("backend-1")},
			},
			wantScheduled: 1, // only replica 0 has sufficient non-terminated cliques
			wantAvailable: 1, // only replica 0 is available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutateReplicas(logger, tt.pcsg, tt.replicaCliques)

			assert.Equal(t, tt.wantScheduled, tt.pcsg.Status.ScheduledReplicas)
			assert.Equal(t, tt.wantAvailable, tt.pcsg.Status.AvailableReplicas)
			assert.Equal(t, tt.pcsg.Spec.Replicas, tt.pcsg.Status.Replicas)
		})
	}
}

func TestMutateMinAvailableBreachedCondition(t *testing.T) {
	tests := []struct {
		name       string
		pcsg       *grovecorev1alpha1.PodCliqueScalingGroup
		wantStatus metav1.ConditionStatus
	}{
		{
			name: "condition becomes breached",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     3,
					MinAvailable: ptr.To(int32(2)),
				},
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					ScheduledReplicas: 3,
					AvailableReplicas: 1,
				},
			},
			wantStatus: metav1.ConditionTrue,
		},
		{
			name: "condition stays healthy",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     3,
					MinAvailable: ptr.To(int32(2)),
				},
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					ScheduledReplicas: 3,
					AvailableReplicas: 3,
				},
			},
			wantStatus: metav1.ConditionFalse,
		},
		{
			name: "new condition added",
			pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     2,
					MinAvailable: ptr.To(int32(1)),
				},
				Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
					ScheduledReplicas: 2,
					AvailableReplicas: 2,
					Conditions:        []metav1.Condition{},
				},
			},
			wantStatus: metav1.ConditionFalse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutateMinAvailableBreachedCondition(tt.pcsg)
			assertCondition(t, tt.pcsg, tt.wantStatus == metav1.ConditionTrue)
		})
	}
}

func TestGetPodCliquesPerPCSGReplica(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		objects      []client.Object
		wantReplicas int
	}{
		{
			name: "find expected cliques",
			objects: []client.Object{
				testutils.NewPCSGPodCliqueBuilder("test-pgs-0-frontend-0", "test-ns", "test-pgs", "test-pcsg", 0, 0).
					WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").Build(),
				testutils.NewPCSGPodCliqueBuilder("test-pgs-0-backend-0", "test-ns", "test-pgs", "test-pcsg", 0, 0).
					WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").Build(),
				testutils.NewPCSGPodCliqueBuilder("test-pgs-0-frontend-1", "test-ns", "test-pgs", "test-pcsg", 0, 1).
					WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").Build(),
			},
			wantReplicas: 2,
		},
		{
			name:         "no cliques found",
			objects:      []client.Object{},
			wantReplicas: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := testutils.SetupFakeClient(tt.objects...)
			reconciler := &Reconciler{client: fakeClient}
			objKey := client.ObjectKey{Name: "test-pcsg", Namespace: "test-ns"}

			result, err := reconciler.getPodCliquesPerPCSGReplica(ctx, "test-pgs", objKey)

			require.NoError(t, err)
			assert.Len(t, result, tt.wantReplicas)
		})
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestReconcileStatus(t *testing.T) {
	ctx := context.Background()
	logger := testutils.SetupTestLogger()

	tests := []struct {
		name          string
		setup         func() (*grovecorev1alpha1.PodCliqueScalingGroup, *grovecorev1alpha1.PodGangSet, []client.Object)
		wantAvailable int32
		wantScheduled int32
		wantBreached  bool
	}{
		{
			name: "happy path",
			setup: func() (*grovecorev1alpha1.PodCliqueScalingGroup, *grovecorev1alpha1.PodGangSet, []client.Object) {
				pcsg := testutils.NewPCSGBuilder("test-pcsg", "test-ns", "test-pgs", 0).
					WithReplicas(2).
					WithCliqueNames([]string{"frontend", "backend"}).
					WithOptions(testutils.WithPCSGObservedGeneration(1)).Build()
				pgs := testutils.NewPGSBuilder("test-pgs", "test-ns").Build()
				cliques := []client.Object{
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-frontend-0", "test-ns", "test-pgs", "test-pcsg", 0, 0).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-backend-0", "test-ns", "test-pgs", "test-pcsg", 0, 0).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-frontend-1", "test-ns", "test-pgs", "test-pcsg", 0, 1).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-backend-1", "test-ns", "test-pgs", "test-pcsg", 0, 1).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
				}
				return pcsg, pgs, cliques
			},
			wantAvailable: 2,
			wantScheduled: 2,
			wantBreached:  false,
		},
		{
			name: "mixed replica states",
			setup: func() (*grovecorev1alpha1.PodCliqueScalingGroup, *grovecorev1alpha1.PodGangSet, []client.Object) {
				pcsg := testutils.NewPCSGBuilder("test-pcsg", "test-ns", "test-pgs", 0).
					WithReplicas(3).
					WithCliqueNames([]string{"worker"}).
					WithMinAvailable(2).
					WithOptions(testutils.WithPCSGObservedGeneration(1)).Build()
				pgs := testutils.NewPGSBuilder("test-pgs", "test-ns").Build()
				cliques := []client.Object{
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-worker-0", "test-ns", "test-pgs", "test-pcsg", 0, 0).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-worker-1", "test-ns", "test-pgs", "test-pcsg", 0, 1).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledButBreached()).Build(),
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-worker-2", "test-ns", "test-pgs", "test-pcsg", 0, 2).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQNotScheduled()).Build(),
				}
				return pcsg, pgs, cliques
			},
			wantAvailable: 1,
			wantScheduled: 2,
			wantBreached:  true,
		},
		{
			name: "with terminating cliques",
			setup: func() (*grovecorev1alpha1.PodCliqueScalingGroup, *grovecorev1alpha1.PodGangSet, []client.Object) {
				pcsg := testutils.NewPCSGBuilder("test-pcsg", "test-ns", "test-pgs", 0).
					WithReplicas(2).
					WithCliqueNames([]string{"frontend", "backend"}).
					WithOptions(testutils.WithPCSGObservedGeneration(1)).Build()
				pgs := testutils.NewPGSBuilder("test-pgs", "test-ns").Build()
				cliques := []client.Object{
					// Replica 0: healthy
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-frontend-0", "test-ns", "test-pgs", "test-pcsg", 0, 0).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-backend-0", "test-ns", "test-pgs", "test-pcsg", 0, 0).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
					// Replica 1: has one terminating clique
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-frontend-1", "test-ns", "test-pgs", "test-pcsg", 0, 1).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
					testutils.NewPCSGPodCliqueBuilder("test-pgs-0-backend-1", "test-ns", "test-pgs", "test-pcsg", 0, 1).
						WithOwnerReference("PodCliqueScalingGroup", "test-pcsg", "").
						WithOptions(testutils.WithPCLQTerminating()).Build(),
				}
				return pcsg, pgs, cliques
			},
			wantAvailable: 1,     // only replica 0 has all non-terminated cliques
			wantScheduled: 1,     // only replica 0 has sufficient non-terminated cliques
			wantBreached:  false, // 1 >= 1 (default minAvailable)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcsg, pgs, cliques := tt.setup()
			allObjects := append([]client.Object{pcsg, pgs}, cliques...)
			fakeClient := testutils.SetupFakeClient(allObjects...)
			reconciler := &Reconciler{client: fakeClient}

			result := reconciler.reconcileStatus(ctx, logger, pcsg)

			require.False(t, result.HasErrors())
			assert.Equal(t, tt.wantAvailable, pcsg.Status.AvailableReplicas)
			assert.Equal(t, tt.wantScheduled, pcsg.Status.ScheduledReplicas)

			if pcsg.Status.ObservedGeneration != nil {
				assertCondition(t, pcsg, tt.wantBreached)
			}
		})
	}
}

func TestReconcileStatus_EdgeCases(t *testing.T) {
	ctx := context.Background()
	logger := testutils.SetupTestLogger()

	tests := []struct {
		name string
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
	}{
		{
			name: "zero replicas",
			pcsg: testutils.NewPCSGBuilder("test-pcsg", "test-ns", "test-pgs", 0).
				WithReplicas(0).
				WithOptions(testutils.WithPCSGObservedGeneration(1)).Build(),
		},
		{
			name: "empty clique names",
			pcsg: testutils.NewPCSGBuilder("test-pcsg", "test-ns", "test-pgs", 0).
				WithReplicas(1).
				WithCliqueNames([]string{}).
				WithOptions(testutils.WithPCSGObservedGeneration(1)).Build(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pgs := testutils.NewPGSBuilder("test-pgs", "test-ns").Build()
			fakeClient := testutils.SetupFakeClient(tt.pcsg, pgs)
			reconciler := &Reconciler{client: fakeClient}

			result := reconciler.reconcileStatus(ctx, logger, tt.pcsg)

			assert.False(t, result.HasErrors())
		})
	}
}
