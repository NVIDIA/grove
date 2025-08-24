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

package podgangset_test

import (
	"testing"
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/test/integration/framework"
	"github.com/NVIDIA/grove/operator/test/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPodGangSetCreatesChildResources(t *testing.T) {
	// Setup test environment with PGS controller only
	env, err := framework.NewEnvBuilder(t).
		WithController(framework.ControllerPodGangSet).
		WithNamespace("test-ns").
		Build()
	require.NoError(t, err)
	// Start the environment
	err = env.Start()
	require.NoError(t, err)
	defer env.Shutdown()

	// Create a simple PGS with 2 cliques
	pgs := utils.NewPodGangSetBuilder("test-pgs", "test-ns").
		WithMinimal().
		WithReplicas(1).
		WithPodCliqueParameters("clique-1", 2, 2, nil).
		WithPodCliqueParameters("clique-2", 1, 1, nil).
		WithPodCliqueParameters("clique-3", 1, 1, nil).
		WithPodCliqueScalingGroupConfig(grovecorev1alpha1.PodCliqueScalingGroupConfig{
			Name:         "new",
			CliqueNames:  []string{"clique-3"},
			Replicas:     ptr.To[int32](1),
			MinAvailable: ptr.To[int32](1),
			ScaleConfig:  nil,
		}).Build()

	// Submit PGS to cluster
	err = env.Client.Create(env.Ctx, pgs)
	require.NoError(t, err)

	// Debug: Check if PGS is actually in the cluster and monitor status changes
	time.Sleep(2 * time.Second)
	fetchedPGS := &grovecorev1alpha1.PodGangSet{}
	err = env.Client.Get(env.Ctx, client.ObjectKey{Name: "test-pgs", Namespace: "test-ns"}, fetchedPGS)
	require.NoError(t, err, "Should be able to fetch PGS from cluster")

	// Wait for PCSG creation using Eventually with better polling
	assert.Eventually(t, func() bool {
		pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
		err = env.Client.List(env.Ctx, pcsgList, client.InNamespace("test-ns"))
		if err != nil {
			t.Logf("Error listing PCSGs: %v", err)
			return false
		}
		t.Logf("Found %d PCSGs", len(pcsgList.Items))
		return len(pcsgList.Items) == 1
	}, 15*time.Second, 500*time.Millisecond, "PCSG should be created")

	// Wait for PCLQ creation using Eventually with better polling
	assert.Eventually(t, func() bool {
		pclqList := &grovecorev1alpha1.PodCliqueList{}
		err := env.Client.List(env.Ctx, pclqList, client.InNamespace("test-ns"))
		if err != nil {
			t.Logf("Error listing PCLQs: %v", err)
			return false
		}
		t.Logf("Found %d PCLQs", len(pclqList.Items))
		return len(pclqList.Items) == 3
	}, 20*time.Second, 500*time.Millisecond, "All non-scaling-group PCLQs should be created")

	// Verify final state
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	err = env.Client.List(env.Ctx, pcsgList, client.InNamespace("test-ns"))
	require.NoError(t, err)
	require.Len(t, pcsgList.Items, 1)

	pclqList := &grovecorev1alpha1.PodCliqueList{}
	err = env.Client.List(env.Ctx, pclqList, client.InNamespace("test-ns"))
	require.NoError(t, err)
	require.Len(t, pclqList.Items, 3)

	// Verify ownership and basic properties
	pcsg := pcsgList.Items[0]
	assert.Equal(t, "test-pgs", pcsg.Labels["app.kubernetes.io/part-of"])
	assert.Equal(t, string(pgs.UID), string(pcsg.GetOwnerReferences()[0].UID))

	for _, pclq := range pclqList.Items {
		assert.Equal(t, "test-pgs", pclq.Labels["app.kubernetes.io/part-of"])
		assert.Equal(t, string(pgs.UID), string(pclq.GetOwnerReferences()[0].UID))
	}
}
