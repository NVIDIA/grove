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

package kubernetes

import (
	"testing"

	testutils "github.com/ai-dynamo/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

const (
	testPodName      = "test-pod"
	testPodNamespace = "test-ns"
)

func TestIsPodActive(t *testing.T) {
	testCases := []struct {
		description   string
		phase         corev1.PodPhase
		isTerminating bool
		want          bool
	}{
		{
			description: "should return true for pod that is running",
			phase:       corev1.PodRunning,
			want:        true,
		},
		{
			description:   "should return false for termination pod",
			phase:         corev1.PodRunning,
			isTerminating: true,
			want:          false,
		},
		{
			description: "should return false when pod has failed",
			phase:       corev1.PodFailed,
			want:        false,
		},
		{
			description: "should return false for pod that has finished execution with success",
			phase:       corev1.PodSucceeded,
			want:        false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			podBuilder := testutils.NewPodWithBuilderWithDefaultSpec(testPodName, testPodNamespace).WithPhase(tc.phase)
			if tc.isTerminating {
				podBuilder.MarkForTermination()
			}
			pod := podBuilder.Build()
			assert.Equal(t, tc.want, IsPodActive(pod))
		})
	}
}

func TestIsPodScheduled(t *testing.T) {
	testCases := []struct {
		description        string
		scheduledCondition *corev1.PodCondition
		expectedResult     bool
	}{
		{
			description:    "Pod does not have a PodScheduled condition",
			expectedResult: false,
		},
		{
			description: "Pod has a PodScheduled condition with status True",
			scheduledCondition: &corev1.PodCondition{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionTrue,
			},
			expectedResult: true,
		},
		{
			description: "Pod has a PodScheduled condition with status False",
			scheduledCondition: &corev1.PodCondition{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionFalse,
			},
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			podBuilder := testutils.NewPodWithBuilderWithDefaultSpec(testPodName, testPodNamespace)
			if tc.scheduledCondition != nil {
				podBuilder.WithCondition(*tc.scheduledCondition)
			}
			pod := podBuilder.Build()
			result := IsPodScheduled(pod)
			if result != tc.expectedResult {
				t.Errorf("expected %v, got %v", tc.expectedResult, result)
			}
		})
	}
}
