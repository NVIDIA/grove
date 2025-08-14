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

package status

import (
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/testutils"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespace    = "default"
	testPGSName      = "test-pgs"
	testReplicaIndex = 0
)

func newTestPodClique(name string, opts ...testutils.PCLQOption) grovecorev1alpha1.PodClique {
	return *testutils.NewPodCliqueBuilder(name, testNamespace, testPGSName, testReplicaIndex).
		WithOptions(opts...).
		Build()
}

func newTestPCSG(name string, opts ...testutils.PCSGOption) grovecorev1alpha1.PodCliqueScalingGroup {
	return *testutils.NewPCSGBuilder(name, testNamespace, testPGSName, testReplicaIndex).
		WithOptions(opts...).
		Build()
}

func TestValidateReplicaAvailable(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name          string
		resources     []grovecorev1alpha1.PodClique
		expectedCount int
		expected      bool
	}{
		{
			name: "all resources available",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("available-1", testutils.WithPCLQAvailable()),
				newTestPodClique("available-2", testutils.WithPCLQAvailable()),
			},
			expectedCount: 2,
			expected:      true,
		},
		{
			name: "some resources unavailable",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("available", testutils.WithPCLQAvailable()),
				newTestPodClique("unavailable", testutils.WithPCLQMinAvailableBreached()),
			},
			expectedCount: 2,
			expected:      false,
		},
		{
			name: "insufficient resource count",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("single-available", testutils.WithPCLQAvailable()),
			},
			expectedCount: 2,
			expected:      false,
		},
		{
			name: "terminating resources ignored",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("available-1", testutils.WithPCLQAvailable()),
				newTestPodClique("terminating",
					testutils.WithPCLQAvailable(), testutils.WithPCLQTerminating()),
				newTestPodClique("available-2", testutils.WithPCLQAvailable()),
			},
			expectedCount: 2,
			expected:      true,
		},
		{
			name: "insufficient after filtering terminating",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("available", testutils.WithPCLQAvailable()),
				newTestPodClique("terminating",
					testutils.WithPCLQAvailable(), testutils.WithPCLQTerminating()),
			},
			expectedCount: 2,
			expected:      false,
		},
		{
			name:          "empty resources",
			resources:     []grovecorev1alpha1.PodClique{},
			expectedCount: 1,
			expected:      false,
		},
		{
			name: "zero expected count",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("any-resource", testutils.WithPCLQAvailable()),
			},
			expectedCount: 0,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateReplicaAvailable[grovecorev1alpha1.PodClique](
				logger,
				tt.resources,
				tt.expectedCount,
				GetPCLQCondition,
				grovecorev1alpha1.PodCliqueKind,
				testReplicaIndex,
			)
			if result != tt.expected {
				t.Errorf("ValidateReplicaAvailable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateReplicaScheduled(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name          string
		resources     []grovecorev1alpha1.PodClique
		expectedCount int
		expected      bool
	}{
		{
			name: "all resources scheduled",
			resources: []grovecorev1alpha1.PodClique{
				*testutils.NewPodCliqueBuilder("pclq-1", "default", "test-pgs", 0).WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
				*testutils.NewPodCliqueBuilder("pclq-2", "default", "test-pgs", 0).WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(),
			},
			expectedCount: 2,
			expected:      true,
		},
		{
			name: "some resources not scheduled",
			resources: []grovecorev1alpha1.PodClique{
				*testutils.NewPodCliqueBuilder("pclq-1", "default", "test-pgs", 0).WithOptions(testutils.WithPCLQScheduledAndAvailable()).Build(), // scheduled
				*testutils.NewPodCliqueBuilder("pclq-2", "default", "test-pgs", 0).WithOptions(testutils.WithPCLQNotScheduled()).Build(),          // not scheduled
			},
			expectedCount: 2,
			expected:      false,
		},
		{
			name: "insufficient resource count",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("single-scheduled", testutils.WithPCLQScheduledAndAvailable()),
			},
			expectedCount: 2,
			expected:      false,
		},
		{
			name: "terminating resources ignored for scheduling",
			resources: []grovecorev1alpha1.PodClique{
				newTestPodClique("scheduled-1", testutils.WithPCLQScheduledAndAvailable()),
				newTestPodClique("terminating-scheduled",
					testutils.WithPCLQScheduledAndAvailable(), testutils.WithPCLQTerminating()),
				newTestPodClique("scheduled-2", testutils.WithPCLQScheduledAndAvailable()),
			},
			expectedCount: 2,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateReplicaScheduled(
				logger,
				tt.resources,
				tt.expectedCount,
				GetPCLQCondition,
				grovecorev1alpha1.PodCliqueKind,
				testReplicaIndex,
			)
			if result != tt.expected {
				t.Errorf("ValidateReplicaScheduled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateReplicaState_WithPCSG(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name          string
		resources     []grovecorev1alpha1.PodCliqueScalingGroup
		expectedCount int
		stateFunc     func([]metav1.Condition) bool
		expected      bool
	}{
		{
			name: "PCSG resources available",
			resources: []grovecorev1alpha1.PodCliqueScalingGroup{
				newTestPCSG("healthy-1", testutils.WithPCSGHealthy()),
				newTestPCSG("healthy-2", testutils.WithPCSGHealthy()),
			},
			expectedCount: 2,
			stateFunc:     IsAvailable,
			expected:      true,
		},
		{
			name: "PCSG resources scheduled",
			resources: []grovecorev1alpha1.PodCliqueScalingGroup{
				newTestPCSG("healthy", testutils.WithPCSGHealthy()),
				newTestPCSG("breached", testutils.WithPCSGMinAvailableBreached()),
			},
			expectedCount: 2,
			stateFunc:     IsScheduled,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateReplicaState(
				logger,
				tt.resources,
				tt.expectedCount,
				GetPCSGCondition,
				tt.stateFunc,
				grovecorev1alpha1.PodCliqueScalingGroupKind,
				testReplicaIndex,
			)
			if result != tt.expected {
				t.Errorf("validateReplicaState() = %v, want %v", result, tt.expected)
			}
		})
	}
}
