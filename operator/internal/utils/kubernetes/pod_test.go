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
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			pod := createPod(testCase.phase, testCase.isTerminating)
			assert.Equal(t, testCase.want, IsPodActive(pod))
		})
	}
}

func createPod(phase corev1.PodPhase, isTerminating bool) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPodName,
			Namespace: testPodNamespace,
		},
	}
	pod.Status.Phase = phase
	if isTerminating {
		pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}
	return pod
}
