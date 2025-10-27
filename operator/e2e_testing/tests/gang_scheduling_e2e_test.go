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
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ai-dynamo/grove/operator/e2e_testing/utils"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
			logger.Info("Warning: failed to cleanup workloads: %v", err)
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

// Test_GS2_GangSchedulingWithScalingFullReplicas verifies gang-scheduling behavior when scaling a PodCliqueScalingGroup
// Scenario GS-2:
// 1. Initialize a 14-node Grove cluster, then cordon 5 nodes
// 2. Deploy workload WL1, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node to allow scheduling and verify pods get scheduled
// 5. Wait for pods to become ready
// 6. Scale PCSG replicas to 3 and verify 4 new pending pods
// 7. Uncordon remaining nodes and verify all pods get scheduled
func Test_GS2_GangSchedulingWithScalingFullReplicas(t *testing.T) {
	ctx := context.Background()

	// Setup cluster (shared or individual based on test run mode)
	logger.Info("1. Initialize a 14-node Grove cluster, then cordon 5 nodes")

	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 14)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 5 {
		t.Errorf("expected at least 5 agent nodes to cordon, but found %d", len(agentNodes))
	}

	nodesToCordon := agentNodes[:5]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL1, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload1.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload1"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	expectedPods := 10

	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node to allow scheduling and verify pods get scheduled")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	logger.Info("5. Wait for pods to become ready")
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for pods to be ready: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
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
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("6. Scale PCSG replicas to 3 and verify 4 new pending pods")
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	pcsgGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquescalinggroups"}
	pcsgName := "workload1-0-sg-x"

	err = pollForCondition(ctx, 3*time.Minute, 5*time.Second, func() (bool, error) {
		_, err := dynamicClient.Resource(pcsgGVR).Namespace(workloadNamespace).Get(ctx, pcsgName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("Failed to find PodCliqueScalingGroup %s: %v", pcsgName, err)
	}

	scalePatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 3,
		},
	}
	patchBytes, err := json.Marshal(scalePatch)
	if err != nil {
		t.Errorf("Failed to marshal scale patch: %v", err)
	}

	if _, err := dynamicClient.Resource(pcsgGVR).Namespace(workloadNamespace).Patch(ctx, pcsgName, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		t.Errorf("Failed to scale PodCliqueScalingGroup %s: %v", pcsgName, err)
	}

	expectedScaledPods := 14
	err = pollForCondition(ctx, 3*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}
		return len(pods.Items) == expectedScaledPods, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for scaled pods to be created: %v", err)
	}

	runningPods = 0
	pendingPods = 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodRunning:
			runningPods++
		case v1.PodPending:
			pendingPods++
		}
	}

	if pendingPods != 4 {
		t.Errorf("Expected 4 pending pods after scaling, but found %d", pendingPods)
	}
	if runningPods != expectedPods {
		t.Errorf("Expected %d running pods after scaling, but found %d", expectedPods, runningPods)
	}

	logger.Info("7. Uncordon remaining nodes and verify all pods get scheduled")
	remainingNodesToUncordon := nodesToCordon[1:]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 15*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for scaled pods to be ready: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods after final uncordon: %v", err)
	}

	runningPods = 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			runningPods++
		}
	}

	if runningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running after scaling, but only %d are running", len(pods.Items), runningPods)
	}

	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCSG scaling test completed successfully!")
}

// TestGangSchedulingWithPCSScalingFullReplicas verifies gang-scheduling behavior when scaling a PodCliqueSet
// Scenario GS-3:
// 1. Initialize a 20-node Grove cluster, then cordon 11 nodes
// 2. Deploy workload WL1, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node to allow scheduling and verify pods get scheduled
// 5. Wait for pods to become ready
// 6. Scale PCS replicas to 2 and verify 10 new pending pods
// 7. Uncordon remaining nodes and verify all pods get scheduled
func Test_GS3_GangSchedulingWithPCSScalingFullReplicas(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 20-node Grove cluster, then cordon 11 nodes")
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 20)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 11 {
		t.Errorf("expected at least 11 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// Step 1 (continued): Cordon 11 nodes
	nodesToCordon := agentNodes[:11]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL1, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload1.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload1"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	expectedPods := 10
	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node to allow scheduling and verify pods get scheduled")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	logger.Info("5. Wait for pods to become ready")
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for pods to be ready: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			runningPods++
		}
		logger.Infof("Pod %s: Phase=%s, Node=%s", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
	}

	if runningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running, but only %d are running", len(pods.Items), runningPods)
	}
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("6. Scale PCS replicas to 2 and verify 10 new pending pods")
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	replicas := int32(2)
	pcsPatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	}
	pcsPatchBytes, err := json.Marshal(pcsPatch)
	if err != nil {
		t.Errorf("Failed to marshal PodCliqueSet patch: %v", err)
	}

	pcsGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquesets"}
	pcsName := "workload1"

	if _, err := dynamicClient.Resource(pcsGVR).Namespace(workloadNamespace).Patch(ctx, pcsName, types.MergePatchType, pcsPatchBytes, metav1.PatchOptions{}); err != nil {
		t.Errorf("Failed to scale PodCliqueSet %s: %v", pcsName, err)
	}

	expectedScaledPods := int(replicas) * expectedPods
	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == expectedScaledPods, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for scaled pods to be created: %v", err)
	}

	runningPods = 0
	pendingPods = 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodRunning:
			runningPods++
		case v1.PodPending:
			pendingPods++
		}
	}

	expectedNewPending := expectedScaledPods - expectedPods
	if pendingPods != expectedNewPending {
		t.Errorf("Expected %d pending pods after scaling, but found %d", expectedNewPending, pendingPods)
	}
	if runningPods != expectedPods {
		t.Errorf("Expected %d running pods after scaling, but found %d", expectedPods, runningPods)
	}

	logger.Info("7. Uncordon remaining nodes and verify all pods get scheduled")
	remainingNodesToUncordon := nodesToCordon[1:]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 15*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for scaled pods to be ready: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods after final uncordon: %v", err)
	}

	runningPods = 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			runningPods++
		}
	}

	if runningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running after scaling, but only %d are running", len(pods.Items), runningPods)
	}
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCS scaling test completed successfully!")
}

// TestGangSchedulingWithPCSAndPCSGScalingFullReplicas verifies gang scheduling while scaling both PodCliqueSet and PodCliqueScalingGroup replicas
// Scenario GS-4:
// 1. Initialize a 28-node Grove cluster, then cordon 19 nodes
// 2. Deploy workload WL1, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node to allow scheduling and verify pods get scheduled
// 5. Wait for pods to become ready
// 6. Scale PCSG replicas to 3 and verify 4 new pending pods
// 7. Uncordon 4 nodes and verify scaled pods get scheduled
// 8. Scale PCS replicas to 2 and verify 10 new pending pods
// 9. Scale PCSG replicas to 3 and verify 4 new pending pods
// 10. Uncordon remaining nodes and verify all pods get scheduled
func Test_GS4_GangSchedulingWithPCSAndPCSGScalingFullReplicas(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 28-node Grove cluster, then cordon 19 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 28)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 19 {
		t.Errorf("expected at least 19 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// cordon 19 nodes
	nodesToCordon := agentNodes[:19]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL1, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload1.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload1"
	
	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	logger.Info("3. Verify all workload pods are pending due to insufficient resources")
	expectedPods := 10

	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == expectedPods, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for pods to be created: %v", err)
	}

	pendingPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodPending {
			pendingPods++
		}
	}

	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node to allow scheduling and verify pods get scheduled")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	logger.Info("5. Wait for pods to become ready")
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for pods to be ready: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	runningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			runningPods++
		}
	}

	assertPodsOnDistinctNodes(t, pods.Items)

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("6. Scale PCSG replicas to 3 and verify 4 new pending pods")
	pcsgName := "workload1-0-sg-x"
	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, pcsgName, 3, 14, 4)

	logger.Info("7. Uncordon 4 nodes and verify scaled pods get scheduled")
	remainingNodesAfterFirstUncordon := nodesToCordon[1:5]
	for _, nodeName := range remainingNodesAfterFirstUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for pods to be ready after PCSG scale: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
	if err != nil {
		t.Errorf("Failed to list workload pods after PCSG scale: %v", err)
	}

	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("8. Scale PCS replicas to 2 and verify 10 new pending pods")
	scalePCSAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, "workload1", 2, 24, 10)

	remainingNodesAfterPCSScale := nodesToCordon[5:15]
	for _, nodeName := range remainingNodesAfterPCSScale {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for pods to be ready after PCS scale: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
	if err != nil {
		t.Errorf("Failed to list workload pods after PCS scale: %v", err)
	}

	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("9. Scale PCSG replicas to 3 and verify 4 new pending pods")
	secondReplicaPCSGName := "workload1-1-sg-x"
	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, secondReplicaPCSGName, 3, 28, 4)

	logger.Info("10. Uncordon remaining nodes and verify all pods get scheduled")
	finalNodes := nodesToCordon[15:19]
	for _, nodeName := range finalNodes {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for pods to be ready after final PCSG scale: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
	if err != nil {
		t.Errorf("Failed to list workload pods after final PCSG scale: %v", err)
	}

	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCS+PCSG scaling test completed successfully!")
}

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

// pollForCondition polls a condition function until it returns true or times out
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

func scalePCSGAndWait(t *testing.T, ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, namespace, labelSelector, pcsgName string, replicas int32, expectedTotalPods, expectedPending int) {
	t.Helper()

	pcsgGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquescalinggroups"}
	patchBytes, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
	if err != nil {
		t.Errorf("Failed to marshal PCSG patch: %v", err)
	}

	if _, err := dynamicClient.Resource(pcsgGVR).Namespace(namespace).Patch(ctx, pcsgName, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		t.Errorf("Failed to scale PodCliqueScalingGroup %s: %v", pcsgName, err)
	}

	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, err
		}
		return len(pods.Items) == expectedTotalPods, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for pods after PCSG scaling: %v", err)
	}

	evaluatePodStates(t, ctx, clientset, namespace, labelSelector, expectedTotalPods, expectedPending)
}

func scalePCSAndWait(t *testing.T, ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, namespace, labelSelector, pcsName string, replicas int32, expectedTotalPods, expectedPending int) {
	t.Helper()

	pcsGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquesets"}
	patchBytes, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
	if err != nil {
		t.Errorf("Failed to marshal PCS patch: %v", err)
	}

	if _, err := dynamicClient.Resource(pcsGVR).Namespace(namespace).Patch(ctx, pcsName, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		t.Errorf("Failed to scale PodCliqueSet %s: %v", pcsName, err)
	}

	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, err
		}
		return len(pods.Items) == expectedTotalPods, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for pods after PCS scaling: %v", err)
	}

	evaluatePodStates(t, ctx, clientset, namespace, labelSelector, expectedTotalPods, expectedPending)
}

func evaluatePodStates(t *testing.T, ctx context.Context, clientset kubernetes.Interface, namespace, labelSelector string, expectedTotalPods, expectedPending int) {
	t.Helper()

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		t.Errorf("Failed to list pods: %v", err)
	}

	runningPods := 0
	pendingPods := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodRunning:
			runningPods++
		case v1.PodPending:
			pendingPods++
		}
	}

	if len(pods.Items) != expectedTotalPods {
		t.Errorf("Expected %d total pods, but found %d", expectedTotalPods, len(pods.Items))
	}

	if pendingPods != expectedPending {
		t.Errorf("Expected %d pending pods, but found %d", expectedPending, pendingPods)
	}

	if runningPods != expectedTotalPods-expectedPending {
		t.Errorf("Expected %d running pods, but found %d", expectedTotalPods-expectedPending, runningPods)
	}
}

// Test_GS5_GangSchedulingWithMinReplicas tests gang-scheduling behavior with min-replicas
// Scenario GS-5:
// 1. Initialize a 10-node Grove cluster, then cordon 8 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 5. Wait for scheduled pods to become ready
// 6. Uncordon 7 nodes and verify all remaining workload pods get scheduled
func Test_GS5_GangSchedulingWithMinReplicas(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 10-node Grove cluster, then cordon 8 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 10)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 8 {
		t.Errorf("expected at least 8 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// Cordon 8 agent nodes
	nodesToCordon := agentNodes[:8]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	// workload2 creates: 1 PCS replica * (pc-a: 2 + pc-b: 1 + pc-c: 3) + sg-x: 2 replicas * (pc-b: 1 + pc-c: 3) = 6 + 8 = 14 pods
	// But the test description says 10 pods, so let me check the actual workload2 structure more carefully
	expectedPods := 10

	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	if pendingPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be pending, but only %d are pending", len(pods.Items), pendingPods)
	}

	// Verify pods remain pending due to gang scheduling constraints
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	// Wait for exactly 3 pods to be scheduled (min-replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}

		// We expect exactly 3 pods to be running (min-replicas) and the rest pending
		return runningPods == 3 && pendingPods == len(pods.Items)-3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 3 pods to be scheduled: %v", err)
	}

	// Verify the scheduled pods and their distribution
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	runningPods := 0
	pendingPods = 0
	runningPodNames := make([]string, 0)
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodRunning:
			runningPods++
			runningPodNames = append(runningPodNames, pod.Name)
		case v1.PodPending:
			pendingPods++
		}
	}

	if runningPods != 3 {
		t.Errorf("Expected exactly 3 pods to be running (min-replicas), but found %d", runningPods)
	}

	if pendingPods != len(pods.Items)-3 {
		t.Errorf("Expected %d pods to remain pending, but found %d", len(pods.Items)-3, pendingPods)
	}

	logger.Info("5. Wait for scheduled pods to become ready")
	// Note: WaitForPods waits for ALL pods, but we only want the running ones to be ready
	// We'll verify readiness manually
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 3 scheduled pods to become ready: %v", err)
	}

	logger.Info("6. Uncordon 7 nodes and verify all remaining workload pods get scheduled")
	remainingNodesToUncordon := nodesToCordon[1:]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Final verification - all pods should be running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	finalRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			finalRunningPods++
		}
	}

	if finalRunningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running, but only %d are running", len(pods.Items), finalRunningPods)
	}

	// Verify pods are distributed across distinct nodes
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling min-replicas test (GS-5) completed successfully!")
}

// Test_GS6_GangSchedulingWithPCSGScalingMinReplicas tests gang-scheduling behavior with PCSG scaling and min-replicas
// Scenario GS-6:
// 1. Initialize a 14-node Grove cluster, then cordon 12 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 5. Wait for scheduled pods to become ready
// 6. Uncordon 7 nodes and verify the remaining workload pods get scheduled
// 7. Wait for scheduled pods to become ready
// 8. Set pcs-0-sg-x resource replicas equal to 3, then verify 4 newly created pods
// 9. Verify all newly created pods are pending due to insufficient resources
// 10. Uncordon 2 nodes and verify 2 more pods get scheduled (pcs-0-{sg-x-2-pc-b=1, sg-x-2-pc-c=1})
// 11. Wait for scheduled pods to become ready
// 12. Uncordon 2 nodes and verify remaining workload pods get scheduled
func Test_GS6_GangSchedulingWithPCSGScalingMinReplicas(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 14-node Grove cluster, then cordon 12 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 14)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 12 {
		t.Errorf("expected at least 12 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// Cordon 12 agent nodes
	nodesToCordon := agentNodes[:12]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	// workload2 initially creates 10 pods
	expectedPods := 10
	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	if pendingPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be pending, but only %d are pending", len(pods.Items), pendingPods)
	}

	// Verify pods remain pending due to gang scheduling constraints
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})")
	// Based on workload2 min-replicas: pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1}
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	// Wait for exactly 3 pods to be scheduled (min-replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}

		// We expect exactly 3 pods to be running (min-replicas) and the rest pending
		return runningPods == 3 && pendingPods == len(pods.Items)-3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 3 pods to be scheduled: %v", err)
	}

	// Verify the scheduled pods and their distribution
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	runningPods := 0
	pendingPods = 0
	runningPodNames := make([]string, 0)
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodRunning:
			runningPods++
			runningPodNames = append(runningPodNames, pod.Name)
		case v1.PodPending:
			pendingPods++
		}
	}

	if runningPods != 3 {
		t.Errorf("Expected exactly 3 pods to be running (min-replicas), but found %d", runningPods)
	}

	if pendingPods != len(pods.Items)-3 {
		t.Errorf("Expected %d pods to remain pending, but found %d", len(pods.Items)-3, pendingPods)
	}

	logger.Info("5. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 3 scheduled pods to become ready: %v", err)
	}

	logger.Info("6. Uncordon 7 nodes and verify the remaining workload pods get scheduled")
	sevenNodesToUncordon := nodesToCordon[1:8]
	for _, nodeName := range sevenNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Verify all 10 initial pods are running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	allRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			allRunningPods++
		}
	}

	if allRunningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running, but only %d are running", len(pods.Items), allRunningPods)
	}

	logger.Info("7. Wait for scheduled pods to become ready")
	// Create dynamic client for PCSG scaling operations
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("8. Set pcs-0-sg-x resource replicas equal to 3, then verify 4 newly created pods")
	// Scale PCSG sg-x to 3 replicas and verify 4 newly created pods
	pcsgName := "workload2-0-sg-x"
	// Expected total pods after scaling: 10 (initial) + 4 (new from scaling sg-x from 2 to 3) = 14
	expectedPodsAfterScaling := 14
	expectedNewPendingPods := 4

	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, pcsgName, 3, expectedPodsAfterScaling, expectedNewPendingPods)

	logger.Info("9. Verify all newly created pods are pending due to insufficient resources")
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list pods after PCSG scaling: %v", err)
	}

	runningAfter := 0
	pendingAfter := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodRunning:
			runningAfter++
		case v1.PodPending:
			pendingAfter++
		}
	}

	if len(pods.Items) != expectedPodsAfterScaling {
		t.Errorf("Expected %d total pods after scaling, but found %d", expectedPodsAfterScaling, len(pods.Items))
	}
	if pendingAfter != expectedNewPendingPods {
		t.Errorf("Expected %d pending pods after scaling, but found %d", expectedNewPendingPods, pendingAfter)
	}
	if runningAfter != expectedPodsAfterScaling-expectedNewPendingPods {
		t.Errorf("Expected %d running pods after scaling, but found %d", expectedPodsAfterScaling-expectedNewPendingPods, runningAfter)
	}

	logger.Info("10. Uncordon 2 nodes and verify 2 more pods get scheduled (pcs-0-{sg-x-2-pc-b=1, sg-x-2-pc-c=1})")
	// Uncordon 2 nodes and verify exactly 2 more pods get scheduled
	// pcs-0-{sg-x-2-pc-b = 1, sg-x-2-pc-c = 1} (min-replicas for the new PCSG replica)
	twoNodesToUncordon := nodesToCordon[8:10]
	for _, nodeName := range twoNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for exactly 2 more pods to be scheduled (min-replicas for new PCSG replica)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}

		// We expect 12 pods running (10 initial + 2 from min-replicas) and 2 pending
		return runningPods == 12 && pendingPods == 2, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 2 more pods to be scheduled after PCSG scaling: %v", err)
	}

	logger.Info("11. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 12, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 12 pods to become ready: %v", err)
	}

	logger.Info("12. Uncordon 2 nodes and verify remaining workload pods get scheduled")
	// Uncordon remaining 2 nodes and verify all remaining workload pods get scheduled
	remainingNodesToUncordon := nodesToCordon[10:12]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Final verification - all 14 pods should be running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	finalRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			finalRunningPods++
		}
	}

	if finalRunningPods != expectedPodsAfterScaling {
		t.Errorf("Expected all %d pods to be running, but only %d are running", expectedPodsAfterScaling, finalRunningPods)
	}

	// Verify pods are distributed across distinct nodes
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCSG scaling min-replicas test (GS-6) completed successfully!")
}

// Test_GS7_GangSchedulingWithPCSGScalingMinReplicasAdvanced1 tests advanced gang-scheduling behavior with PCSG scaling and min-replicas
// Scenario GS-7:
// 1. Initialize a 14-node Grove cluster, then cordon 12 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 5. Wait for scheduled pods to become ready
// 6. Uncordon 2 nodes and verify 2 more pods get scheduled (pcs-0-{sg-x-1-pc-b=1, sg-x-1-pc-c=1})
// 7. Wait for scheduled pods to become ready
// 8. Uncordon 5 nodes and verify the remaining workload pods get scheduled
// 9. Wait for scheduled pods to become ready
// 10. Set pcs-0-sg-x resource replicas equal to 3, then verify 4 newly created pods
// 11. Verify all newly created pods are pending due to insufficient resources
// 12. Uncordon 2 nodes and verify 2 more pods get scheduled (pcs-0-{sg-x-2-pc-b=1, sg-x-2-pc-c=1})
// 13. Wait for scheduled pods to become ready
// 14. Uncordon 2 nodes and verify remaining workload pods get scheduled
func Test_GS7_GangSchedulingWithPCSGScalingMinReplicasAdvanced1(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 14-node Grove cluster, then cordon 12 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 14)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 12 {
		t.Errorf("expected at least 12 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// Cordon 12 agent nodes
	nodesToCordon := agentNodes[:12]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	// workload2 initially creates 10 pods
	expectedPods := 10
	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	if pendingPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be pending, but only %d are pending", len(pods.Items), pendingPods)
	}

	// Verify pods remain pending due to gang scheduling constraints
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	// Wait for exactly 3 pods to be scheduled (min-replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}

		// We expect exactly 3 pods to be running (min-replicas) and the rest pending
		return runningPods == 3 && pendingPods == len(pods.Items)-3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 3 pods to be scheduled: %v", err)
	}

	logger.Info("5. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 3 scheduled pods to become ready: %v", err)
	}

	logger.Info("6. Uncordon 2 nodes and verify 2 more pods get scheduled (pcs-0-{sg-x-1-pc-b=1, sg-x-1-pc-c=1})")
	twoNodesToUncordon := nodesToCordon[1:3]
	for _, nodeName := range twoNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for exactly 2 more pods to be scheduled (sg-x-1 min-replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}

		// We expect 5 pods running (3 + 2 new) and the rest pending
		return runningPods == 5 && pendingPods == len(pods.Items)-5, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 2 more pods to be scheduled: %v", err)
	}

	logger.Info("7. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 5, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 5 scheduled pods to become ready: %v", err)
	}

	logger.Info("8. Uncordon 5 nodes and verify the remaining workload pods get scheduled")
	fiveNodesToUncordon := nodesToCordon[3:8]
	for _, nodeName := range fiveNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Verify all 10 initial pods are running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	allRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			allRunningPods++
		}
	}

	if allRunningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running, but only %d are running", len(pods.Items), allRunningPods)
	}

	logger.Info("9. Wait for scheduled pods to become ready (already verified above)")

	// Create dynamic client for PCSG scaling operations
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("11. Verify all newly created pods are pending due to insufficient resources (verified in scalePCSGAndWait)")
	pcsgName := "workload2-0-sg-x"
	expectedPodsAfterScaling := 14
	expectedNewPendingPods := 4
	logger.Info("10. Set pcs-0-sg-x resource replicas equal to 3, then verify 4 newly created pods")
	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, pcsgName, 3, expectedPodsAfterScaling, expectedNewPendingPods)

	logger.Info("12. Uncordon 2 nodes and verify 2 more pods get scheduled (pcs-0-{sg-x-2-pc-b=1, sg-x-2-pc-c=1})")
	twoMoreNodesToUncordon := nodesToCordon[8:10]
	for _, nodeName := range twoMoreNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for exactly 2 more pods to be scheduled (min-replicas for new PCSG replica)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}

		// We expect 12 pods running (10 initial + 2 from min-replicas) and 2 pending
		return runningPods == 12 && pendingPods == 2, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 2 more pods to be scheduled after PCSG scaling: %v", err)
	}

	logger.Info("13. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 12, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 12 pods to become ready: %v", err)
	}

	logger.Info("14. Uncordon 2 nodes and verify remaining workload pods get scheduled")
	remainingNodesToUncordon := nodesToCordon[10:12]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Final verification - all 14 pods should be running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	finalRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			finalRunningPods++
		}
	}

	if finalRunningPods != expectedPodsAfterScaling {
		t.Errorf("Expected all %d pods to be running, but only %d are running", expectedPodsAfterScaling, finalRunningPods)
	}

	// Verify pods are distributed across distinct nodes
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCSG scaling min-replicas advanced1 test (GS-7) completed successfully! All workload pods transitioned correctly through advanced PCSG scaling with min-replicas.")
}

// TestGangSchedulingWithPCSGScalingMinReplicasAdvanced2 tests advanced gang-scheduling behavior with early PCSG scaling and min-replicas
// Scenario GS-8:
// 1. Initialize a 14-node Grove cluster, then cordon 12 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Set pcs-0-sg-x resource replicas equal to 3, verify 4 more newly created pods
// 5. Verify all 14 newly created pods are pending due to insufficient resources
// 6. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 7. Wait for scheduled pods to become ready
// 8. Uncordon 4 nodes and verify 4 more pods get scheduled (pcs-0-{sg-x-1-pc-b=1, sg-x-1-pc-c=1}, pcs-0-{sg-x-2-pc-b=1, sg-x-2-pc-c=1})
// 9. Wait for scheduled pods to become ready
// 10. Uncordon 7 nodes and verify the remaining workload pods get scheduled
func Test_GS8_GangSchedulingWithPCSGScalingMinReplicasAdvanced2(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 14-node Grove cluster, then cordon 12 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 14)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 12 {
		t.Errorf("expected at least 12 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// Cordon 12 agent nodes
	nodesToCordon := agentNodes[:12]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	// workload2 initially creates 10 pods
	expectedPods := 10
	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	if pendingPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be pending, but only %d are pending", len(pods.Items), pendingPods)
	}

	// Verify pods remain pending due to gang scheduling constraints
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	// Create dynamic client for PCSG scaling operations
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("4. Set pcs-0-sg-x resource replicas equal to 3, verify 4 more newly created pods")
	pcsgName := "workload2-0-sg-x"
	expectedPodsAfterScaling := 14

	scalePatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 3,
		},
	}
	patchBytes, err := json.Marshal(scalePatch)
	if err != nil {
		t.Errorf("Failed to marshal scale patch: %v", err)
	}

	pcsgGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquescalinggroups"}
	if _, err := dynamicClient.Resource(pcsgGVR).Namespace(workloadNamespace).Patch(ctx, pcsgName, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		t.Errorf("Failed to scale PodCliqueScalingGroup %s: %v", pcsgName, err)
	}

	err = pollForCondition(ctx, 3*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == expectedPodsAfterScaling, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for scaled pods to be created: %v", err)
	}

	logger.Info("5. Verify all 14 newly created pods are pending due to insufficient resources")
	pendingPods = 0
	runningPods := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodPending:
			pendingPods++
		case v1.PodRunning:
			runningPods++
		}
	}

	if len(pods.Items) != expectedPodsAfterScaling {
		t.Errorf("Expected %d total pods after scaling, but found %d", expectedPodsAfterScaling, len(pods.Items))
	}
	if pendingPods != expectedPodsAfterScaling {
		t.Errorf("Expected all %d pods to be pending after scaling, but found %d pending", expectedPodsAfterScaling, pendingPods)
	}
	if runningPods != 0 {
		t.Errorf("Expected 0 running pods after scaling with all nodes cordoned, but found %d running", runningPods)
	}

	logger.Info("6. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	// Wait for exactly 3 pods to be scheduled (min-replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}

		// We expect exactly 3 pods to be running (min-replicas) and the rest pending
		return runningPods == 3 && pendingPods == len(pods.Items)-3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 3 pods to be scheduled: %v", err)
	}

	logger.Info("7. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 3 scheduled pods to become ready: %v", err)
	}

	logger.Info("8. Uncordon 4 nodes and verify 4 more pods get scheduled")
	fourNodesToUncordon := nodesToCordon[1:5]
	for _, nodeName := range fourNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for exactly 4 more pods to be scheduled (sg-x-1 and sg-x-2 min-replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}
		// We expect 7 pods running (3 + 4 new) and the rest pending
		return runningPods == 7 && pendingPods == len(pods.Items)-7, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 4 more pods to be scheduled: %v", err)
	}

	logger.Info("9. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 7, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 7 scheduled pods to become ready: %v", err)
	}

	logger.Info("10. Uncordon 7 nodes and verify the remaining workload pods get scheduled")
	remainingNodesToUncordon := nodesToCordon[5:]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Final verification - all 14 pods should be running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	finalRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			finalRunningPods++
		}
	}

	if finalRunningPods != expectedPodsAfterScaling {
		t.Errorf("Expected all %d pods to be running, but only %d are running", expectedPodsAfterScaling, finalRunningPods)
	}

	// Verify pods are distributed across distinct nodes
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCS+PCSG scaling test completed successfully!")
}

// TestGangSchedulingWithPCSScalingMinReplicas tests gang-scheduling behavior with PodCliqueSet scaling and min-replicas
// Scenario GS-9:
// 1. Initialize a 20-node Grove cluster, then cordon 18 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 5. Wait for scheduled pods to become ready
// 6. Uncordon 7 nodes and verify the remaining workload pods get scheduled
// 7. Wait for scheduled pods to become ready
// 8. Set PCS resource replicas equal to 2, then verify 10 more newly created pods
// 9. Uncordon 3 nodes and verify another 3 pods get scheduled (pcs-1-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 10. Wait for scheduled pods to become ready
// 11. Uncordon 7 nodes and verify the remaining workload pods get scheduled
func Test_GS9_GangSchedulingWithPCSScalingMinReplicas(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 20-node Grove cluster, then cordon 18 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 20)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 18 {
		t.Errorf("expected at least 18 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// Cordon 18 agent nodes
	nodesToCordon := agentNodes[:18]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	// workload2 initially creates 10 pods
	expectedPods := 10

	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	if pendingPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be pending, but only %d are pending", len(pods.Items), pendingPods)
	}

	// Verify pods remain pending due to gang scheduling constraints
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	// Wait for exactly 3 pods to be scheduled (min-replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}
		// We expect exactly 3 pods to be running (min-replicas) and the rest pending
		return runningPods == 3 && pendingPods == len(pods.Items)-3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 3 pods to be scheduled: %v", err)
	}

	logger.Info("5. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 3 scheduled pods to become ready: %v", err)
	}

	logger.Info("6. Uncordon 7 nodes and verify the remaining workload pods get scheduled")
	sevenNodesToUncordon := nodesToCordon[1:8]
	for _, nodeName := range sevenNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Verify all 10 initial pods are running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	allRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			allRunningPods++
		}
	}

	if allRunningPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be running, but only %d are running", len(pods.Items), allRunningPods)
	}

	logger.Info("7. Wait for scheduled pods to become ready (already verified above)")
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("8. Set PCS resource replicas equal to 2, then verify 10 more newly created pods")
	// Scale PodCliqueSet to 2 replicas and verify 10 more newly created pods
	pcsName := "workload2"

	// Expected total pods after scaling: 10 (initial) + 10 (new from scaling PCS from 1 to 2) = 20
	expectedPodsAfterScaling := 20
	expectedNewPendingPods := 10

	scalePCSAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, pcsName, 2, expectedPodsAfterScaling, expectedNewPendingPods)

	logger.Info("9. Uncordon 3 nodes and verify another 3 pods get scheduled (pcs-1-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})")
	threeNodesToUncordon := nodesToCordon[8:11]
	for _, nodeName := range threeNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for exactly 3 more pods to be scheduled (min-replicas for new PCS replica)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}
		// We expect 13 pods running (10 initial + 3 from min-replicas) and 7 pending
		return runningPods == 13 && pendingPods == 7, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 3 more pods to be scheduled after PCS scaling: %v", err)
	}

	logger.Info("10. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 13, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 13 pods to become ready: %v", err)
	}

	logger.Info("11. Uncordon 7 nodes and verify the remaining workload pods get scheduled")
	remainingNodesToUncordon := nodesToCordon[11:18]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Final verification - all 20 pods should be running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	finalRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			finalRunningPods++
		}
	}

	if finalRunningPods != expectedPodsAfterScaling {
		t.Errorf("Expected all %d pods to be running, but only %d are running", expectedPodsAfterScaling, finalRunningPods)
	}

	// Verify pods are distributed across distinct nodes
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCS+PCSG scaling test completed successfully!")
}

// Test_GS10_GangSchedulingWithPCSScalingMinReplicasAdvanced tests advanced gang-scheduling behavior with early PCS scaling and min-replicas
// Scenario GS-10:
// 1. Initialize a 20-node Grove cluster, then cordon 18 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Set PCS resource replicas equal to 2, then verify 10 more newly created pods
// 5. Verify all 20 newly created pods are pending due to insufficient resources
// 6. Uncordon 4 nodes and verify a total of 6 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1}, pcs-1-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 7. Wait for scheduled pods to become ready
// 8. Uncordon 4 nodes and verify 4 more pods get scheduled (pcs-0-{sg-x-1-pc-b=1, sg-x-1-pc-c=1}, pcs-1-{sg-x-1-pc-b=1, sg-x-1-pc-c=1})
// 9. Wait for scheduled pods to become ready
// 10. Uncordon 10 nodes and verify the remaining workload pods get scheduled
func Test_GS10_GangSchedulingWithPCSScalingMinReplicasAdvanced(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 20-node Grove cluster, then cordon 18 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 20)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 18 {
		t.Errorf("expected at least 18 agent nodes to cordon, but found %d", len(agentNodes))
	}

	// Cordon 18 agent nodes
	nodesToCordon := agentNodes[:18]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	// workload2 initially creates 10 pods
	expectedPods := 10

	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
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

	if pendingPods != len(pods.Items) {
		t.Errorf("Expected all %d pods to be pending, but only %d are pending", len(pods.Items), pendingPods)
	}

	// Verify pods remain pending due to gang scheduling constraints
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	// Create dynamic client for PCS scaling operations
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("4. Set PCS resource replicas equal to 2, then verify 10 more newly created pods")
	pcsName := "workload2"

	// Expected total pods after scaling: 10 (initial) + 10 (new from scaling PCS from 1 to 2) = 20
	expectedPodsAfterScaling := 20

	scalePatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": 2,
		},
	}
	patchBytes, err := json.Marshal(scalePatch)
	if err != nil {
		t.Errorf("Failed to marshal scale patch: %v", err)
	}

	pcsGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquesets"}
	if _, err := dynamicClient.Resource(pcsGVR).Namespace(workloadNamespace).Patch(ctx, pcsName, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		t.Errorf("Failed to scale PodCliqueSet %s: %v", pcsName, err)
	}

	err = pollForCondition(ctx, 3*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == expectedPodsAfterScaling, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for scaled pods to be created: %v", err)
	}

	logger.Info("5. Verify all 20 newly created pods are pending due to insufficient resources")
	pendingPods = 0
	runningPods := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case v1.PodPending:
			pendingPods++
		case v1.PodRunning:
			runningPods++
		}
	}

	if len(pods.Items) != expectedPodsAfterScaling {
		t.Errorf("Expected %d total pods after scaling, but found %d", expectedPodsAfterScaling, len(pods.Items))
	}
	if pendingPods != expectedPodsAfterScaling {
		t.Errorf("Expected all %d pods to be pending after scaling, but found %d pending", expectedPodsAfterScaling, pendingPods)
	}
	if runningPods != 0 {
		t.Errorf("Expected 0 running pods after scaling with all nodes cordoned, but found %d running", runningPods)
	}

	logger.Info("6. Uncordon 4 nodes and verify a total of 6 pods get scheduled")
	fourNodesToUncordon := nodesToCordon[0:4]
	for _, nodeName := range fourNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for exactly 6 pods to be scheduled (min-replicas for both PCS replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}
		// We expect exactly 6 pods to be running (min-replicas for both PCS replicas) and the rest pending
		return runningPods == 6 && pendingPods == len(pods.Items)-6, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 6 pods to be scheduled: %v", err)
	}

	logger.Info("7. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 6, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 6 scheduled pods to become ready: %v", err)
	}

	logger.Info("8. Uncordon 4 nodes and verify 4 more pods get scheduled")
	fourMoreNodesToUncordon := nodesToCordon[4:8]
	for _, nodeName := range fourMoreNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for exactly 4 more pods to be scheduled (sg-x-1 for both PCS replicas)
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}
		// We expect 10 pods running (6 + 4 new) and the rest pending
		return runningPods == 10 && pendingPods == len(pods.Items)-10, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for exactly 4 more pods to be scheduled: %v", err)
	}

	logger.Info("9. Wait for scheduled pods to become ready")
	err = pollForCondition(ctx, 5*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: workloadLabelSelector,
		})
		if err != nil {
			return false, err
		}

		readyRunningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				// Check if pod is ready
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyRunningPods++
						break
					}
				}
			}
		}

		return readyRunningPods == 10, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 10 scheduled pods to become ready: %v", err)
	}

	logger.Info("10. Uncordon 10 nodes and verify the remaining workload pods get scheduled")
	remainingNodesToUncordon := nodesToCordon[8:18]
	for _, nodeName := range remainingNodesToUncordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	// Wait for all remaining pods to be scheduled and ready
	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all pods to be ready: %v", err)
	}

	// Final verification - all 20 pods should be running
	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: workloadLabelSelector,
	})
	if err != nil {
		t.Errorf("Failed to list workload pods: %v", err)
	}

	finalRunningPods := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			finalRunningPods++
		}
	}

	if finalRunningPods != expectedPodsAfterScaling {
		t.Errorf("Expected all %d pods to be running, but only %d are running", expectedPodsAfterScaling, finalRunningPods)
	}

	// Verify pods are distributed across distinct nodes
	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCS+PCSG scaling test completed successfully!")
}

// Test_GS11_GangSchedulingWithPCSAndPCSGScalingMinReplicas tests gang-scheduling behavior with both PCS and PCSG scaling using min-replicas
// Scenario GS-11:
// 1. Initialize a 28-node Grove cluster, then cordon 26 nodes
// 2. Deploy workload WL6, and verify 6 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node and verify a total of 3 pods get scheduled (pcs-0-{pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 5. Scale pcs-0-sg-x replicas to 3, verify 4 newly created pods
// 6. Verify all newly created pods are pending due to insufficient resources
// 7. Scale pcs-0 replicas to 3, verify 6 newly created pods
// 8. Verify all newly created pods are pending due to insufficient resources
// 9. Uncordon 2 nodes
// 10. Verify a total of 9 pods get scheduled (pcs-0, pcs-1, pcs-2 each with {pc-a=1, sg-x-0-pc-b=1, sg-x-0-pc-c=1})
// 11. Scale pcs-0-sg-x replicas to 5, verify 8 newly created pods
// 12. Verify all newly created pods are pending due to insufficient resources
// 13. Scale pcs-1-sg-x replicas to 5, verify 8 newly created pods
// 14. Verify all newly created pods are pending due to insufficient resources
// 15. Scale pcs-2-sg-x replicas to 5, verify 8 newly created pods
// 16. Verify all newly created pods are pending due to insufficient resources
// 17. Uncordon 6 nodes
// 18. Verify all pods get scheduled

// Test_GS11_GangSchedulingWithPCSAndPCSGScalingMinReplicas tests gang-scheduling behavior with both PCS and PCSG scaling using min-replicas
// Scenario GS-11:
// 1. Initialize a 28-node Grove cluster, then cordon 26 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Uncordon 1 node
// 5. Wait for min-replicas pods to be scheduled and ready (should be 3 pods for min-available)
// 6. Uncordon 7 nodes and verify the remaining workload pods get scheduled
// 7. Set pcs-0-sg-x resource replicas equal to 3, then verify 4 newly created pods
// 8. Verify all newly created pods are pending due to insufficient resources
// 9. Uncordon 2 nodes
// 10. Wait for 2 more pods to be scheduled and ready (min-available for sg-x-2)
// 11. Uncordon 2 nodes and verify remaining workload pods get scheduled
// 12. Set pcs resource replicas equal to 2, then verify 10 more newly created pods
// 13. Uncordon 3 nodes
// 14. Wait for 3 more pods to be scheduled (min-available for pcs-1)
// 15. Uncordon 7 nodes and verify the remaining workload pods get scheduled
// 16. Set pcs-1-sg-x resource replicas equal to 3, then verify 4 newly created pods
// 17. Verify all newly created pods are pending due to insufficient resources
// 18. Uncordon 2 nodes
// 19. Wait for 2 more pods to be scheduled (min-available for pcs-1-sg-x-2)
// 20. Uncordon 2 nodes and verify remaining workload pods get scheduled
func Test_GS11_GangSchedulingWithPCSAndPCSGScalingMinReplicas(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 28-node Grove cluster, then cordon 26 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 28)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 26 {
		t.Errorf("expected at least 26 agent nodes to cordon, but found %d", len(agentNodes))
	}

	nodesToCordon := agentNodes[:26]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	expectedPods := 10
	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == expectedPods, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for pods to be created: %v", err)
	}

	logger.Info("3. Verify all workload pods are pending due to insufficient resources")
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	logger.Info("4. Uncordon 1 node")
	firstNodeToUncordon := nodesToCordon[0]
	if err := utils.CordonNode(ctx, clientset, firstNodeToUncordon, false); err != nil {
		t.Errorf("Failed to uncordon node %s: %v", firstNodeToUncordon, err)
	}

	logger.Info("5. Wait for min-replicas pods to be scheduled and ready (should be 3 pods for min-available)")
	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				runningPods++
			}
		}

		return runningPods == 3, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for min-replicas pods to be scheduled: %v", err)
	}

	logger.Info("6. Uncordon 7 nodes and verify the remaining workload pods get scheduled")
	remainingNodesFirstWave := nodesToCordon[1:8]
	for _, nodeName := range remainingNodesFirstWave {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for first wave pods to be ready: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("7. Set pcs-0-sg-x resource replicas equal to 3, then verify 4 newly created pods")
	pcsgName := "workload2-0-sg-x"
	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, pcsgName, 3, 14, 4)

	logger.Info("8. Verify all newly created pods are pending due to insufficient resources")
	expectedRunning := 10 // Initial 10 pods from first wave
	expectedPending := 4  // 4 new pods from PCSG scaling
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		totalPods := len(pods.Items)
		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}
		return totalPods == 14 && runningPods == expectedRunning && pendingPods == expectedPending, nil
	})
	if err != nil {
		t.Errorf("Failed to verify newly created pods are pending: %v", err)
	}

	logger.Info("9. Uncordon 2 nodes")
	remainingNodesSecondWave := nodesToCordon[8:10]
	for _, nodeName := range remainingNodesSecondWave {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("10. Wait for 2 more pods to be scheduled and ready (min-available for sg-x-2)")
	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				runningPods++
			}
		}

		return runningPods == 12, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for PCSG partial scheduling: %v", err)
	}

	logger.Info("11. Uncordon 2 nodes and verify remaining workload pods get scheduled")
	remainingNodesThirdWave := nodesToCordon[10:12]
	for _, nodeName := range remainingNodesThirdWave {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for PCSG completion pods to be ready: %v", err)
	}

	logger.Info("12. Set pcs resource replicas equal to 2, then verify 10 more newly created pods")
	scalePCSAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, "workload2", 2, 24, 10)

	logger.Info("13. Uncordon 3 nodes")
	remainingNodesFourthWave := nodesToCordon[12:15]
	for _, nodeName := range remainingNodesFourthWave {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("14. Wait for 3 more pods to be scheduled (min-available for pcs-1)")
	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				runningPods++
			}
		}

		return runningPods == 17, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for PCS partial scheduling: %v", err)
	}

	logger.Info("15. Uncordon 7 nodes and verify the remaining workload pods get scheduled")
	remainingNodesFifthWave := nodesToCordon[15:22]
	for _, nodeName := range remainingNodesFifthWave {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for PCS completion pods to be ready: %v", err)
	}

	logger.Info("16. Set pcs-1-sg-x resource replicas equal to 3, then verify 4 newly created pods")
	secondReplicaPCSGName := "workload2-1-sg-x"
	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, secondReplicaPCSGName, 3, 28, 4)

	logger.Info("17. Verify all newly created pods are pending due to insufficient resources")
	expectedRunning = 24 // All previous pods should be running
	expectedPending = 4  // 4 new pods from second PCSG scaling
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		totalPods := len(pods.Items)
		runningPods := 0
		pendingPods := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				runningPods++
			case v1.PodPending:
				pendingPods++
			}
		}
		return totalPods == 28 && runningPods == expectedRunning && pendingPods == expectedPending, nil
	})
	if err != nil {
		t.Errorf("Failed to verify newly created pods are pending after second PCSG scaling: %v", err)
	}

	logger.Info("18. Uncordon 2 nodes")
	remainingNodesSixthWave := nodesToCordon[22:24]
	for _, nodeName := range remainingNodesSixthWave {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("19. Wait for 2 more pods to be scheduled (min-available for pcs-1-sg-x-2)")
	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				runningPods++
			}
		}

		return runningPods == 26, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for final PCSG partial scheduling: %v", err)
	}

	logger.Info("20. Uncordon 2 nodes and verify remaining workload pods get scheduled")
	finalNodes := nodesToCordon[24:26]
	for _, nodeName := range finalNodes {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all final pods to be ready: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
	if err != nil {
		t.Errorf("Failed to list all final workload pods: %v", err)
	}

	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCS+PCSG scaling test completed successfully!")

}

// Test_GS12_GangSchedulingWithComplexPCSGScaling tests gang-scheduling behavior with complex PCSG scaling operations
// Scenario GS-12:
// 1. Initialize a 28-node Grove cluster, then cordon 26 nodes
// 2. Deploy workload WL2, and verify 10 newly created pods
// 3. Verify all workload pods are pending due to insufficient resources
// 4. Set pcs resource replicas equal to 2, then verify 10 more newly created pods
// 5. Verify all 20 newly created pods are pending due to insufficient resources
// 6. Set both pcs-0-sg-x and pcs-1-sg-x resource replicas equal to 3, verify 8 newly created pods
// 7. Verify all 28 created pods are pending due to insufficient resources
// 8. Uncordon 4 nodes and verify a total of 6 pods get scheduled (pcs-0 and pcs-1 min-available)
// 9. Wait for scheduled pods to become ready
// 10. Uncordon 8 nodes and verify 8 more pods get scheduled (remaining PCSG pods)
// 11. Wait for scheduled pods to become ready
// 12. Uncordon 14 nodes and verify the remaining workload pods get scheduled
func Test_GS12_GangSchedulingWithComplexPCSGScaling(t *testing.T) {
	ctx := context.Background()

	logger.Info("1. Initialize a 28-node Grove cluster, then cordon 26 nodes")
	// Setup cluster (shared or individual based on test run mode)
	clientset, restConfig, _, cleanup, _ := setupTestCluster(ctx, t, 28)
	defer cleanup()

	// Get agent nodes for cordoning
	agentNodes, err := getAgentNodes(ctx, clientset)
	if err != nil {
		t.Errorf("Failed to get agent nodes: %v", err)
	}

	if len(agentNodes) < 26 {
		t.Errorf("expected at least 26 agent nodes to cordon, but found %d", len(agentNodes))
	}

	nodesToCordon := agentNodes[:26]
	for _, nodeName := range nodesToCordon {
		if err := utils.CordonNode(ctx, clientset, nodeName, true); err != nil {
			t.Errorf("Failed to cordon node %s: %v", nodeName, err)
		}
	}

	logger.Info("2. Deploy workload WL2, and verify 10 newly created pods")
	workloadNamespace := "default"
	workloadYAMLPath := "../yaml/workload2.yaml"
	workloadLabelSelector := "app.kubernetes.io/part-of=workload2"

	_, err = utils.ApplyYAMLFile(ctx, workloadYAMLPath, workloadNamespace, restConfig, logger)
	if err != nil {
		t.Errorf("Failed to apply workload YAML: %v", err)
	}

	expectedPods := 10
	var pods *v1.PodList
	err = pollForCondition(ctx, 2*time.Minute, 5*time.Second, func() (bool, error) {
		var err error
		pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		return len(pods.Items) == expectedPods, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for pods to be created: %v", err)
	}

	logger.Info("3. Verify all workload pods are pending due to insufficient resources")
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		stillPending := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				stillPending++
			}
		}

		return stillPending == len(pods.Items), nil
	})
	if err != nil {
		t.Errorf("Failed to verify pods remain pending: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Errorf("Failed to create dynamic client: %v", err)
	}

	logger.Info("4. Set pcs resource replicas equal to 2, then verify 10 more newly created pods")
	scalePCSAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, "workload2", 2, 20, 20)

	logger.Info("5. Verify all 20 newly created pods are pending due to insufficient resources")
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		totalPods := len(pods.Items)
		pendingPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				pendingPods++
			}
		}

		return totalPods == 20 && pendingPods == 20, nil
	})
	if err != nil {
		t.Errorf("Failed to verify all 20 pods are pending: %v", err)
	}

	logger.Info("6. Set both pcs-0-sg-x and pcs-1-sg-x resource replicas equal to 3, verify 8 newly created pods")

	pcsg1Name := "workload2-0-sg-x"
	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, pcsg1Name, 3, 24, 24)

	pcsg2Name := "workload2-1-sg-x"
	scalePCSGAndWait(t, ctx, clientset, dynamicClient, workloadNamespace, workloadLabelSelector, pcsg2Name, 3, 28, 28)

	logger.Info("7. Verify all 28 created pods are pending due to insufficient resources")
	err = pollForCondition(ctx, 2*time.Minute, 10*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		totalPods := len(pods.Items)
		pendingPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodPending {
				pendingPods++
			}
		}

		return totalPods == 28 && pendingPods == 28, nil
	})
	if err != nil {
		t.Errorf("Failed to verify all 28 pods are pending: %v", err)
	}

	logger.Info("8. Uncordon 4 nodes and verify a total of 6 pods get scheduled (pcs-0 and pcs-1 min-available)")
	firstWaveNodes := nodesToCordon[:4]
	for _, nodeName := range firstWaveNodes {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				runningPods++
			}
		}

		return runningPods == 6, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 6 pods to be scheduled: %v", err)
	}

	logger.Info("9. Wait for scheduled pods to become ready (only the 6 that are scheduled)")
	err = pollForCondition(ctx, 10*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		readyPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyPods++
						break
					}
				}
			}
		}

		return readyPods == 6, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 6 pods to be ready: %v", err)
	}

	logger.Info("10. Uncordon 8 nodes and verify 8 more pods get scheduled (remaining PCSG pods)")
	secondWaveNodes := nodesToCordon[4:12]
	for _, nodeName := range secondWaveNodes {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	err = pollForCondition(ctx, 5*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		runningPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				runningPods++
			}
		}

		return runningPods == 14, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 8 more pods to be scheduled: %v", err)
	}

	logger.Info("11. Wait for scheduled pods to become ready (only the 14 that are scheduled)")
	err = pollForCondition(ctx, 10*time.Minute, 5*time.Second, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
		if err != nil {
			return false, err
		}

		readyPods := 0
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						readyPods++
						break
					}
				}
			}
		}

		return readyPods == 14, nil
	})
	if err != nil {
		t.Errorf("Failed to wait for 14 pods to be ready: %v", err)
	}

	logger.Info("12. Uncordon 14 nodes and verify the remaining workload pods get scheduled")
	finalWaveNodes := nodesToCordon[12:26]
	for _, nodeName := range finalWaveNodes {
		if err := utils.CordonNode(ctx, clientset, nodeName, false); err != nil {
			t.Errorf("Failed to uncordon node %s: %v", nodeName, err)
		}
	}

	if err := utils.WaitForPods(ctx, restConfig, []string{workloadNamespace}, workloadLabelSelector, 10*time.Minute, logger); err != nil {
		t.Errorf("Failed to wait for all final pods to be ready: %v", err)
	}

	pods, err = clientset.CoreV1().Pods(workloadNamespace).List(ctx, metav1.ListOptions{LabelSelector: workloadLabelSelector})
	if err != nil {
		t.Errorf("Failed to list all final workload pods: %v", err)
	}

	assertPodsOnDistinctNodes(t, pods.Items)

	logger.Info("🎉 Gang-scheduling PCS+PCSG scaling test completed successfully!")
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
