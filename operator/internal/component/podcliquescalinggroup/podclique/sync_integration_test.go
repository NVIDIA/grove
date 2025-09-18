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
	"testing"
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	testutils "github.com/NVIDIA/grove/operator/test/utils"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testSyncNamespace = "test-sync"
)

// TestRunSyncFlow verifies that the main sync orchestrator correctly handles
// different scenarios including scaling, rolling updates, and error conditions.
func TestRunSyncFlow(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Sync context with current state
		syncContext *syncContext
		// Whether an error is expected
		expectError bool
		// Expected error message substring (if expectError is true)
		expectedErrorSubstring string
	}{
		{
			// Successful sync with no changes needed
			name: "successful sync no changes needed",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
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
						Namespace: testSyncNamespace,
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     1,
						MinAvailable: ptr.To(int32(1)),
						CliqueNames:  []string{"worker"},
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createTestPodCliqueForSync("test-pcsg-0-worker", "0", "test-pcsg", false),
				},
				expectedPCLQFQNsPerPCSGReplica: map[int][]string{
					0: {"test-pcsg-0-worker"},
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "hash-123",
				},
			},
			expectError: false,
		},
		{
			// Scale up scenario - create new replicas
			name: "scale up create new replicas",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
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
						Namespace: testSyncNamespace,
						Labels: map[string]string{
							apicommon.LabelPartOfKey:                "test-pcs",
							apicommon.LabelPodCliqueSetReplicaIndex: "0",
						},
					},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     2, // Scale up from 1 to 2
						MinAvailable: ptr.To(int32(1)),
						CliqueNames:  []string{"worker"},
					},
					Status: grovecorev1alpha1.PodCliqueScalingGroupStatus{
						RollingUpdateProgress: &grovecorev1alpha1.PodCliqueScalingGroupRollingUpdateProgress{},
					},
				},
				existingPCLQs: []grovecorev1alpha1.PodClique{
					createTestPodCliqueForSync("test-pcsg-0-worker", "0", "test-pcsg", false),
				},
				expectedPCLQFQNsPerPCSGReplica: map[int][]string{
					0: {"test-pcsg-0-worker"},
					1: {"test-pcsg-1-worker"},
				},
				expectedPCLQPodTemplateHashMap: map[string]string{
					"test-pcsg-0-worker": "hash-123",
					"test-pcsg-1-worker": "hash-123",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()

			// Create fake client with the PCSG and PCS
			objects := []client.Object{tt.syncContext.pcs, tt.syncContext.pcsg}
			for i := range tt.syncContext.existingPCLQs {
				objects = append(objects, &tt.syncContext.existingPCLQs[i])
			}

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.runSyncFlow(logger, tt.syncContext)

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

// TestTriggerDeletionOfExcessPCSGReplicas verifies that the function correctly
// identifies and deletes excess replicas during scale-down operations.
func TestTriggerDeletionOfExcessPCSGReplicas(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Sync context with current state
		syncContext *syncContext
		// Whether an error is expected
		expectError bool
	}{
		{
			// No excess replicas to delete
			name: "no excess replicas to delete",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 2,
					},
				},
				// No specific field needed - function will compute this internally
			},
			expectError: false,
		},
		{
			// Delete excess replicas during scale down
			name: "delete excess replicas during scale down",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas: 1, // Scale down from 3 to 1
					},
				},
				// No specific field needed - function will compute this internally
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()

			// Create fake client with the PCSG and PCS
			objects := []client.Object{tt.syncContext.pcs, tt.syncContext.pcsg}
			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.triggerDeletionOfExcessPCSGReplicas(logger, tt.syncContext)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
			}
		})
	}
}

// TestProcessMinAvailableBreachedPCSGReplicas verifies that the function correctly
// handles replicas that have breached MinAvailable constraints.
func TestProcessMinAvailableBreachedPCSGReplicas(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Sync context with current state
		syncContext *syncContext
		// Whether an error is expected
		expectError bool
	}{
		{
			// No replicas breached MinAvailable
			name: "no replicas breached minAvailable",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     2,
						MinAvailable: ptr.To(int32(1)),
					},
				},
				pcsgIndicesToTerminate: []string{}, // No replicas to terminate
				pcsgIndicesToRequeue:   []string{}, // No replicas to requeue
			},
			expectError: false,
		},
		{
			// Replicas ready for termination
			name: "replicas ready for termination",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueSetSpec{
						Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
							TerminationDelay: &metav1.Duration{Duration: 5 * time.Minute},
						},
					},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     4,
						MinAvailable: ptr.To(int32(2)),
					},
				},
				pcsgIndicesToTerminate: []string{"2", "3"}, // Replicas ready for termination
				pcsgIndicesToRequeue:   []string{},         // No replicas to requeue
			},
			expectError: true, // Should return requeue error after gang termination
		},
		{
			// Replicas need requeue (still within termination delay)
			name: "replicas need requeue within termination delay",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueSetSpec{
						Template: grovecorev1alpha1.PodCliqueSetTemplateSpec{
							TerminationDelay: &metav1.Duration{Duration: 5 * time.Minute},
						},
					},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     4,
						MinAvailable: ptr.To(int32(2)),
					},
				},
				pcsgIndicesToTerminate: []string{},         // No replicas ready for termination
				pcsgIndicesToRequeue:   []string{"1", "2"}, // Replicas need requeue
			},
			expectError: false, // MinAvailable not breached, no termination needed
		},
		{
			// MinAvailable breached - should return error
			name: "minAvailable breached should return error",
			syncContext: &syncContext{
				ctx: context.Background(),
				pcs: &grovecorev1alpha1.PodCliqueSet{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
				},
				pcsg: &grovecorev1alpha1.PodCliqueScalingGroup{
					ObjectMeta: metav1.ObjectMeta{Name: "test-pcsg", Namespace: testSyncNamespace},
					Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
						Replicas:     4,
						MinAvailable: ptr.To(int32(3)), // High MinAvailable
					},
				},
				pcsgIndicesToTerminate: []string{},         // No replicas ready for termination
				pcsgIndicesToRequeue:   []string{"1", "2"}, // 2 replicas need requeue
			},
			expectError: true, // 4 - (0 + 2) = 2 < 3 (MinAvailable breached)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()

			// Create fake client with the PCSG and PCS
			objects := []client.Object{tt.syncContext.pcs, tt.syncContext.pcsg}
			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			err := operator.processMinAvailableBreachedPCSGReplicas(logger, tt.syncContext)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
			} else {
				assert.NoError(t, err, "should not return an error")
			}
		})
	}
}

// TestPrepareSyncContext verifies that the function correctly initializes
// the sync context with all required state for the sync operation.
func TestPrepareSyncContext(t *testing.T) {
	tests := []struct {
		// Test case description
		name string
		// Input PCS and PCSG
		pcs  *grovecorev1alpha1.PodCliqueSet
		pcsg *grovecorev1alpha1.PodCliqueScalingGroup
		// Existing PodCliques in the cluster
		existingPCLQs []grovecorev1alpha1.PodClique
		// Whether an error is expected
		expectError bool
	}{
		{
			// Successful context preparation
			name: "successful context preparation",
			pcs: &grovecorev1alpha1.PodCliqueSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pcs", Namespace: testSyncNamespace},
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
					Namespace: testSyncNamespace,
					Labels: map[string]string{
						apicommon.LabelPartOfKey:                "test-pcs",
						apicommon.LabelPodCliqueSetReplicaIndex: "0",
					},
				},
				Spec: grovecorev1alpha1.PodCliqueScalingGroupSpec{
					Replicas:     1,
					MinAvailable: ptr.To(int32(1)),
					CliqueNames:  []string{"worker"},
				},
			},
			existingPCLQs: []grovecorev1alpha1.PodClique{
				createTestPodCliqueForSync("test-pcsg-0-worker", "0", "test-pcsg", false),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			logger := logr.Discard()

			// Create fake client with the PCSG and PCS
			objects := []client.Object{tt.pcs, tt.pcsg}
			for i := range tt.existingPCLQs {
				objects = append(objects, &tt.existingPCLQs[i])
			}

			client := testutils.SetupFakeClient(objects...)
			scheme := runtime.NewScheme()
			require.NoError(t, grovecorev1alpha1.AddToScheme(scheme))
			eventRecorder := record.NewFakeRecorder(100)

			operator := New(client, scheme, eventRecorder).(*_resource)

			// Execute the function under test
			syncCtx, err := operator.prepareSyncContext(context.Background(), logger, tt.pcsg)

			// Verify results
			if tt.expectError {
				assert.Error(t, err, "expected an error")
				assert.Nil(t, syncCtx, "sync context should be nil on error")
			} else {
				assert.NoError(t, err, "should not return an error")
				assert.NotNil(t, syncCtx, "sync context should not be nil")
				assert.Equal(t, tt.pcs, syncCtx.pcs, "PCS should match")
				assert.Equal(t, tt.pcsg, syncCtx.pcsg, "PCSG should match")
				assert.NotNil(t, syncCtx.expectedPCLQFQNsPerPCSGReplica, "expected FQNs should be set")
				assert.NotNil(t, syncCtx.expectedPCLQPodTemplateHashMap, "expected hash map should be set")
			}
		})
	}
}

// Helper function to create test PodCliques for sync tests
func createTestPodCliqueForSync(name, replicaIndex, pcsgName string, terminating bool) grovecorev1alpha1.PodClique {
	pclq := grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testSyncNamespace,
			Labels: map[string]string{
				apicommon.LabelPodCliqueScalingGroupReplicaIndex: replicaIndex,
				apicommon.LabelManagedByKey:                      apicommon.LabelManagedByValue,
				apicommon.LabelPartOfKey:                         "test-pcs",
				apicommon.LabelComponentKey:                      apicommon.LabelComponentNamePodCliqueScalingGroupPodClique,
				apicommon.LabelPodCliqueScalingGroup:             pcsgName,
				apicommon.LabelPodTemplateHash:                   "hash-123", // Add pod template hash
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: grovecorev1alpha1.SchemeGroupVersion.String(),
					Kind:       "PodCliqueScalingGroup",
					Name:       pcsgName,
					UID:        types.UID("test-pcsg-uid"),
					Controller: ptr.To(true),
				},
			},
		},
		Spec: grovecorev1alpha1.PodCliqueSpec{
			MinAvailable: ptr.To(int32(1)),
		},
		Status: grovecorev1alpha1.PodCliqueStatus{
			ReadyReplicas: 1,
		},
	}
	if terminating {
		now := metav1.Now()
		pclq.DeletionTimestamp = &now
		pclq.Finalizers = []string{"test-finalizer"}
	}
	return pclq
}
