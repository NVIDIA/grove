//go:build e2e

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

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/NVIDIA/grove/operator/e2e_testing/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_GS1_GangSchedulingWithFullReplicas tests gang-scheduling behavior with insufficient resources
// Scenario GS-1:
// 1. Initialize a 10-node Grove cluster, then cordon 1 node
// 2. Deploy workload WL1, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon the node and verify all pods get scheduled
func Test_GS1_GangSchedulingWithFullReplicas(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 10-node Grove cluster, then cordon 1 node")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 10)
	defer cleanup()

	// Get agent (worker) nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Fatalf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 1 {
		t.Fatalf("Need at least 1 agent node to cordon, but found %d", len(agentNodes))
	}

	agentNodeToCordon := agentNodes[0]
	logger.Debugf("🚫 Cordoning agent node: %s", agentNodeToCordon)
	if err := utils.CordonNode(ctx, clientset, agentNodeToCordon, true); err != nil {
		t.Fatalf("Failed to cordon node %s: %v", agentNodeToCordon, err)
	}

	logger.Info("2. Deploy workload WL1, and verify 10 newly created pods")
	// Deploy workload1.yaml
	workloadNamespace := "default"

	_, err = utils.ApplyYAMLFile(ctx, "../yaml/workload1.yaml", workloadNamespace, restConfig, logger)
	if err != nil {
		t.Fatalf("Failed to apply workload YAML: %v", err)
	}

	// Poll for pod creation and verify they are pending
	expectedPods := 10 // pc-a: 2 replicas, pc-b: 1*2 (scaling group), pc-c: 3*2 (scaling group) = 2+2+6=10

	// Poll until we have the expected number of pods created
	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/part-of=workload1",
		})
		if err != nil {
			return false, err
		}
		return len(pods.Items) == expectedPods, nil
	})
	if err != nil {
		t.Fatalf("Failed to wait for pods to be created: %v", err)
	}

	logger.Info("3. Verify all workload pods are pending due to insufficient resources")
	pendingPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodPending {
			pendingPods++
		}
	}

	// Poll to verify pods remain pending for a reasonable time (gang scheduling should prevent partial scheduling)
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/part-of=workload1",
		})
		if err != nil {
			return false, err
		}

		stillPendingPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPendingPods++
			}
		}

		// We're checking that they remain pending, so we want this condition to be true consistently
		return stillPendingPods == len(pods.Items), nil
	})
	if err != nil {
		t.Fatalf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon the node and verify all pods get scheduled")
	if err := utils.CordonNode(ctx, clientset, agentNodeToCordon, false); err != nil {
		t.Fatalf("Failed to uncordon node %s: %v", agentNodeToCordon, err)
	}

	// Wait for all pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, "", 10*time.Minute, logger); err != nil {
		t.Fatalf("Failed to wait for pods to be ready: %v", err)
	}

	// Verify all pods are now running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/part-of=workload1",
	})
	if err != nil {
		t.Fatalf("Failed to list workload pods: %v", err)
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			runningPods++
		}
	}

	if runningPods != len(pods.Items) {
		t.Fatalf("Expected all %d pods to be running, but only %d are running", len(pods.Items), runningPods)
	}

	// Verify that each pod is scheduled on a unique node, agent nodes have 150m memory
	// and workload pods requests 80m memory, so only 1 should fit per node
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling With Full Replicas test completed successfully!")
}
