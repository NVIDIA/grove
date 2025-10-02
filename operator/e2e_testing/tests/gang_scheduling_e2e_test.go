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

// Package tests contains end-to-end tests for the Grove operator.
//
// These tests are disabled by default due to the 'e2e' build tag above.
// To run these tests, use:
//
//	go test -tags=e2e ./e2e_testing/tests/...
//
// Without the -tags=e2e flag, these tests will be skipped entirely.
package tests

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ai-dynamo/grove/operator/e2e_testing/utils"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

var (
	// isRunningFullSuite tracks whether we're running the full test suite via TestMain
	isRunningFullSuite bool

	// logger for the tests
	logger *logrus.Logger

	// testImages are the Docker images to push to the test registry
	testImages = []string{"nginx:alpine-slim"}
)

func init() {
	// Initialize klog flags and set them to suppress stderr output.
	// This prevents warning messages like "restartPolicy will be ignored" from appearing in test output.
	// Comment this out if you want to see the warnings, but they all seem harmless and noisy.
	klog.InitFlags(nil)
	if err := flag.Set("logtostderr", "false"); err != nil {
		panic("Failed to set logtostderr flag")
	}

	if err := flag.Set("alsologtostderr", "false"); err != nil {
		panic("Failed to set alsologtostderr flag")
	}

	// increase logger verbosity for debugging
	logger = utils.NewTestLogger(logrus.InfoLevel)
}

// TestMain manages the lifecycle of the shared cluster for all tests
func TestMain(m *testing.M) {
	ctx := context.Background()

	// Mark that we're running the full test suite
	isRunningFullSuite = true

	// Setup shared cluster once for all tests
	sharedCluster := utils.SharedCluster(logger, "../../skaffold.yaml")
	if err := sharedCluster.Setup(ctx, testImages); err != nil {
		logger.Errorf("failed to setup shared cluster: %s", err)
		os.Exit(1)
	}

	// Run all tests
	code := m.Run()

	// Teardown shared cluster
	sharedCluster.Teardown()

	os.Exit(code)
}

// setupTestCluster initializes a shared Kubernetes cluster for testing.
// It creates the cluster if needed, ensures the required number of agent nodes are available,
// and returns K8s clients along with a cleanup function and registry port.
// The cleanup function removes workloads and optionally tears down the cluster for individual test runs.
func setupTestCluster(ctx context.Context, t *testing.T, requiredAgents int) (*kubernetes.Clientset, *rest.Config, dynamic.Interface, func(), string) {
	// Always use shared cluster approach
	sharedCluster := utils.SharedCluster(logger, "../../skaffold.yaml")

	// Setup shared cluster if not already done
	if !sharedCluster.IsSetup() {
		if err := sharedCluster.Setup(ctx, testImages); err != nil {
			t.Errorf("Failed to setup shared cluster: %v", err)
		}
	}

	if err := sharedCluster.PrepareForTest(ctx, requiredAgents); err != nil {
		t.Errorf("Failed to prepare shared cluster for test: %v", err)
	}

	clientset, restConfig, dynamicClient := sharedCluster.GetClients()

	// Cleanup function cleans workloads and handles teardown for individual tests
	cleanup := func() {
		if err := sharedCluster.CleanupWorkloads(ctx); err != nil {
			t.Logf("Warning: failed to cleanup workloads: %v", err)
		}

		// If running individual test (not full suite), teardown the cluster completely
		if !isRunningFullSuite {
			sharedCluster.Teardown()
		}
	}

	return clientset, restConfig, dynamicClient, cleanup, sharedCluster.GetRegistryPort()
}

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
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 1 {
		t.Errorf("Need at least 1 agent node to cordon, but found %d", len(agentNodes))
	}

	agentNodeToCordon := agentNodes[0]
	logger.Debugf("🚫 Cordoning agent node: %s", agentNodeToCordon)
	if err := utils.CordonNode(ctx, clientset, agentNodeToCordon, true); err != nil {
		t.Errorf("Failed to cordon node %s: %v", agentNodeToCordon, err)
	}

	logger.Info("2. Deploy workload WL1, and verify 10 newly created pods")
	// Deploy workload1.yaml
	workloadNamespace := "default"

	_, err = utils.ApplyYAMLFile(ctx, "../yaml/workload1.yaml", workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
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
		t.Errorf("Failed to wait for pods to be created: %v", err)
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
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon the node and verify all pods get scheduled")
	if err := utils.CordonNode(ctx, clientset, agentNodeToCordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", agentNodeToCordon, err)
	}

	// Wait for all pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, "", 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for pods to be ready: %v", err)
	}

	// Verify all pods are now running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/part-of=workload1",
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			runningPods++
		}
	}

	if runningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running, but only %d are running", len(pods.Items), runningPods)
	}

	// Verify that each pod is scheduled on a unique node, agent nodes have 150m memory
	// and workload pods requests 80m memory, so only 1 should fit per node
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling With Full Replicas test completed successfully!")
}

// assertPodsOnDistinctNodes verifies that each pod in the provided list is scheduled on a unique node.
// It fails the test if any pod lacks a node assignment or if multiple pods share the same node.
func assertPodsOnDistinctNodes(t *testing.T, pods []v1.Pod) {
	t.Helper()

	assignedNodes := make(map[string]string, len(pods))
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			t.Errorf("Pod %s is running but has no assigned node", pod.Name)
		}
		if existingPod, exists := assignedNodes[nodeName]; exists {
			t.Errorf("Pods %s and %s are scheduled on the same node %s; expected unique nodes", existingPod, pod.Name, nodeName)
		}
		assignedNodes[nodeName] = pod.Name
	}
}

// pollForCondition repeatedly evaluates a condition function at the specified interval
// until it returns true or the timeout is reached. Returns an error if the condition fails,
// returns an error, or the timeout expires.
func pollForCondition(ctx context.Context, timeout, interval time.Duration, condition func() (bool, error)) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check immediately first
	if satisfied, err := condition(); err != nil {
		return err
	} else if satisfied {
		return nil
	}

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("condition not met within timeout of %v", timeout)
		case <-ticker.C:
			if satisfied, err := condition(); err != nil {
				return err
			} else if satisfied {
				return nil
			}
		}
	}
}

// getAgentNodes retrieves the names of all agent (worker) nodes in the cluster,
// excluding control plane nodes. Returns an error if the node list cannot be retrieved.
func getAgentNodes(ctx context.Context, clientset kubernetes.Interface) ([]string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	agentNodes := make([]string, 0)
	for _, node := range nodes.Items {
		if _, isServer := node.Labels["node-role.kubernetes.io/control-plane"]; !isServer {
			agentNodes = append(agentNodes, node.Name)
		}
	}

	return agentNodes, nil
}
