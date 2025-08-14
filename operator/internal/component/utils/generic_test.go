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
	"github.com/NVIDIA/grove/operator/internal/testutils"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGroupByLabel(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name:     "groups PodCliques by label correctly",
			testFunc: testPodCliqueGroupByLabel,
		},
		{
			name:     "groups PodCliqueScalingGroups by label correctly",
			testFunc: testPCSGGroupByLabel,
		},
		{
			name:     "handles empty input",
			testFunc: testEmptyInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

func testPodCliqueGroupByLabel(t *testing.T) {
	pclqs := []grovecorev1alpha1.PodClique{
		*testutils.NewPodCliqueBuilder("pclq1", "test-ns", "test-pgs", 0).Build(),
		*testutils.NewPodCliqueBuilder("pclq2", "test-ns", "test-pgs", 1).Build(),
		*testutils.NewPodCliqueBuilder("pclq3", "test-ns", "test-pgs", 0).Build(),
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pclq4",
				Namespace: "test-ns",
				Labels:    map[string]string{"other-label": "value"},
			},
			Spec: grovecorev1alpha1.PodCliqueSpec{Replicas: 1},
		},
	}

	result := groupByLabel(pclqs, grovecorev1alpha1.LabelPodGangSetReplicaIndex)

	assert.Len(t, result, 2, "should group into 2 groups")
	assert.Len(t, result["0"], 2, "replica index 0 should have 2 items")
	assert.Len(t, result["1"], 1, "replica index 1 should have 1 item")
	assert.NotContains(t, result, "2", "should not include item without the label")

	assert.Equal(t, "pclq1", result["0"][0].Name)
	assert.Equal(t, "pclq3", result["0"][1].Name)
	assert.Equal(t, "pclq2", result["1"][0].Name)
}

func testPCSGGroupByLabel(t *testing.T) {
	pcsgs := []grovecorev1alpha1.PodCliqueScalingGroup{
		*testutils.NewPCSGBuilder("pcsg1", "test-ns", "test-pgs", 0).Build(),
		*testutils.NewPCSGBuilder("pcsg2", "test-ns", "test-pgs", 1).Build(),
		*testutils.NewPCSGBuilder("pcsg3", "test-ns", "test-pgs", 0).Build(),
	}

	result := groupByLabel(pcsgs, grovecorev1alpha1.LabelPodGangSetReplicaIndex)

	assert.Len(t, result, 2, "should group into 2 groups")
	assert.Len(t, result["0"], 2, "replica index 0 should have 2 items")
	assert.Len(t, result["1"], 1, "replica index 1 should have 1 item")

	assert.Equal(t, "pcsg1", result["0"][0].Name)
	assert.Equal(t, "pcsg3", result["0"][1].Name)
	assert.Equal(t, "pcsg2", result["1"][0].Name)
}

func testEmptyInput(t *testing.T) {
	var emptyPclqs []grovecorev1alpha1.PodClique
	var emptyPcsgs []grovecorev1alpha1.PodCliqueScalingGroup

	resultPclq := groupByLabel(emptyPclqs, grovecorev1alpha1.LabelPodGangSetReplicaIndex)
	resultPcsg := groupByLabel(emptyPcsgs, grovecorev1alpha1.LabelPodGangSetReplicaIndex)

	assert.Empty(t, resultPclq, "empty PodClique slice should return empty map")
	assert.Empty(t, resultPcsg, "empty PCSG slice should return empty map")
}
