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

package utils

import (
	"testing"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeterminePodGangNameForPodClique_OwnerReference(t *testing.T) {
	// Create a PodGangSet for testing
	pgs := &grovecorev1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simple1",
		},
		Spec: grovecorev1alpha1.PodGangSetSpec{
			Template: grovecorev1alpha1.PodGangSetTemplateSpec{
				PodCliqueScalingGroupConfigs: []grovecorev1alpha1.PodCliqueScalingGroupConfig{
					{
						Name:         "sga",
						MinAvailable: intPtr(3),
						Replicas:     intPtr(5),
					},
				},
			},
		},
	}

	tests := []struct {
		name                string
		pclq                *grovecorev1alpha1.PodClique
		pgsReplica          int
		pcsgReplicaIndex    int
		expectedPodGangName string
	}{
		{
			name: "standalone PodClique owned by PGS",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple1-0-pcb",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "PodGangSet",
							Name: "simple1",
						},
					},
				},
			},
			pgsReplica:          0,
			pcsgReplicaIndex:    0,
			expectedPodGangName: "simple1-0",
		},
		{
			name: "scaling group PodClique within minAvailable - owned by PCSG",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple1-0-sga-1-pcb",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "PodCliqueScalingGroup",
							Name: "simple1-0-sga",
						},
					},
				},
			},
			pgsReplica:          0,
			pcsgReplicaIndex:    1, // Within minAvailable (< 3)
			expectedPodGangName: "simple1-0",
		},
		{
			name: "scaling group PodClique beyond minAvailable - owned by PCSG",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple1-0-sga-3-pcb",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "PodCliqueScalingGroup",
							Name: "simple1-0-sga",
						},
					},
				},
			},
			pgsReplica:          0,
			pcsgReplicaIndex:    3,                 // Beyond minAvailable (>= 3)
			expectedPodGangName: "simple1-0-sga-0", // First scaled PodGang (3-3=0)
		},
		{
			name: "scaling group PodClique further beyond minAvailable - owned by PCSG",
			pclq: &grovecorev1alpha1.PodClique{
				ObjectMeta: metav1.ObjectMeta{
					Name: "simple1-0-sga-4-pcb",
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "PodCliqueScalingGroup",
							Name: "simple1-0-sga",
						},
					},
				},
			},
			pgsReplica:          0,
			pcsgReplicaIndex:    4,                 // Beyond minAvailable (>= 3)
			expectedPodGangName: "simple1-0-sga-1", // Second scaled PodGang (4-3=1)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeterminePodGangNameForPodClique(tt.pclq, pgs, tt.pgsReplica, tt.pcsgReplicaIndex)
			if result != tt.expectedPodGangName {
				t.Errorf("DeterminePodGangNameForPodClique() = %q, expected %q", result, tt.expectedPodGangName)
			}
		})
	}
}

// Helper function to create int pointer
func intPtr(i int32) *int32 {
	return &i
}

func TestCreatePodGangNameForPCSGFromFQN(t *testing.T) {
	tests := []struct {
		name             string
		pcsgFQN          string
		pcsgReplicaIndex int
		expected         string
	}{
		{
			name:             "scaled PodGang name from FQN",
			pcsgFQN:          "simple1-0-sga",
			pcsgReplicaIndex: 1,
			expected:         "simple1-0-sga-1",
		},
		{
			name:             "scaled PodGang name from FQN with different replica",
			pcsgFQN:          "simple1-0-sga",
			pcsgReplicaIndex: 2,
			expected:         "simple1-0-sga-2",
		},
		{
			name:             "complex scaling group name",
			pcsgFQN:          "test-2-complex-sg",
			pcsgReplicaIndex: 0,
			expected:         "test-2-complex-sg-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreatePodGangNameForPCSGFromFQN(tt.pcsgFQN, tt.pcsgReplicaIndex)
			if result != tt.expected {
				t.Errorf("CreatePodGangNameForPCSGFromFQN() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
