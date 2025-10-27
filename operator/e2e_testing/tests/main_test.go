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

const (
	// defaultPollTimeout is the timeout for most polling conditions
	defaultPollTimeout = 30 * time.Second
	// defaultPollInterval is the interval for most polling conditions
	defaultPollInterval = 5 * time.Second
)

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

// assertPodsOnDistinctNodes asserts that the pods are scheduled on distinct nodes and fails the test if not.
func assertPodsOnDistinctNodes(t *testing.T, pods []v1.Pod) {
	t.Helper()

	assignedNodes := make(map[string]string, len(pods))
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			t.Fatalf("Pod %s is running but has no assigned node", pod.Name)
		}
		if existingPod, exists := assignedNodes[nodeName]; exists {
			t.Fatalf("Pods %s and %s are scheduled on the same node %s; expected unique nodes", existingPod, pod.Name, nodeName)
		}
		assignedNodes[nodeName] = pod.Name
	}
}

// verifyAllPodsArePending verifies that all pods matching the label selector are in pending state.
// Returns an error if verification fails or timeout occurs.
func verifyAllPodsArePending(ctx context.Context, clientset kubernetes.Interface, namespace, labelSelector string, timeout, interval time.Duration) error {
	return pollForCondition(ctx, timeout, interval, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		// Check if all pods are pending
		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodPending {
				logger.Debugf("Pod %s is not pending: %s", pod.Name, pod.Status.Phase)
				return false, nil
			}
		}

		return true, nil
	})
}

// verifyPodsArePendingWithUnschedulableEvents verifies that pods are pending with Unschedulable events from kai-scheduler.
// If allPodsMustBePending is true, verifies ALL pods are pending; otherwise only checks pending pods for Unschedulable events.
// Returns an error if verification fails, or nil if successful after finding Unschedulable events for all (pending) pods.
func verifyPodsArePendingWithUnschedulableEvents(ctx context.Context, clientset kubernetes.Interface, namespace, labelSelector string, allPodsMustBePending bool, timeout, interval time.Duration) error {
	// First verify all pods are pending if required
	if allPodsMustBePending {
		if err := verifyAllPodsArePending(ctx, clientset, namespace, labelSelector, timeout, interval); err != nil {
			return fmt.Errorf("not all pods are pending: %w", err)
		}
	}

	// Now verify that all pending pods have Unschedulable events
	return pollForCondition(ctx, timeout, interval, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return false, err
		}

		// Track pods with Unschedulable events
		podsWithUnschedulableEvent := 0
		pendingCount := 0

		for _, pod := range pods.Items {
			// Check if pod is pending
			if pod.Status.Phase == v1.PodPending {
				pendingCount++

				// Check for Unschedulable event from kai-scheduler
				events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
					FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", pod.Name),
				})
				if err != nil {
					return false, err
				}

				// Find the most recent event
				var mostRecentEvent *v1.Event
				for i := range events.Items {
					event := &events.Items[i]
					if mostRecentEvent == nil || event.LastTimestamp.After(mostRecentEvent.LastTimestamp.Time) {
						mostRecentEvent = event
					}
				}

				// Check if the most recent event is Warning/Unschedulable from kai-scheduler or Warning/PodGrouperWarning from pod-grouper
				if mostRecentEvent != nil &&
					mostRecentEvent.Type == v1.EventTypeWarning &&
					((mostRecentEvent.Reason == "Unschedulable" && mostRecentEvent.Source.Component == "kai-scheduler") ||
						(mostRecentEvent.Reason == "PodGrouperWarning" && mostRecentEvent.Source.Component == "pod-grouper")) {
					logger.Debugf("Pod %s has Unschedulable event: %s", pod.Name, mostRecentEvent.Message)
					podsWithUnschedulableEvent++
				} else if mostRecentEvent != nil {
					logger.Debugf("Pod %s most recent event is not Unschedulable: type=%s, reason=%s, component=%s",
						pod.Name, mostRecentEvent.Type, mostRecentEvent.Reason, mostRecentEvent.Source.Component)
				}
			}
		}

		// Return true only when all pending pods have the Unschedulable event
		if podsWithUnschedulableEvent == pendingCount {
			return true, nil
		}

		logger.Debugf("Waiting for all pending pods to have Unschedulable events: %d/%d", podsWithUnschedulableEvent, pendingCount)
		return false, nil
	})
}

// waitForPodConditions polls until the expected pod state is reached or timeout occurs.
// Returns the current state (total, running, pending) for logging purposes.
func waitForPodConditions(ctx context.Context, clientset kubernetes.Interface, namespace, labelSelector string, expectedTotalPods, expectedPending int, timeout, interval time.Duration) (int, int, int, error) {
	var lastTotal, lastRunning, lastPending int

	err := pollForCondition(ctx, timeout, interval, func() (bool, error) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return false, err
		}

		lastTotal = len(pods.Items)
		lastRunning = 0
		lastPending = 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case v1.PodRunning:
				lastRunning++
			case v1.PodPending:
				lastPending++
			}
		}

		// Check if conditions are met
		return lastTotal == expectedTotalPods && lastPending == expectedPending, nil
	})

	return lastTotal, lastRunning, lastPending, err
}

// scalePCSGAndWait scales a PCSG and waits for the expected pod conditions to be reached.
func scalePCSGAndWait(t *testing.T, ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, namespace, labelSelector, pcsgName string, replicas int32, expectedTotalPods, expectedPending int) {
	t.Helper()

	pcsgGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquescalinggroups"}
	patchBytes, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
	if err != nil {
		t.Fatalf("Failed to marshal PCSG patch: %v", err)
	}

	if _, err := dynamicClient.Resource(pcsgGVR).Namespace(namespace).Patch(ctx, pcsgName, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		t.Fatalf("Failed to scale PodCliqueScalingGroup %s: %v", pcsgName, err)
	}

	totalPods, runningPods, pendingPods, err := waitForPodConditions(ctx, clientset, namespace, labelSelector, expectedTotalPods, expectedPending, 5*time.Minute, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to wait for expected pod conditions after PCSG scaling: %v. Final state: total=%d, running=%d, pending=%d (expected: total=%d, pending=%d)",
			err, totalPods, runningPods, pendingPods, expectedTotalPods, expectedPending)
	}
}

// scalePCSAndWait scales a PCS and waits for the expected pod conditions to be reached.
func scalePCSAndWait(t *testing.T, ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, namespace, labelSelector, pcsName string, replicas int32, expectedTotalPods, expectedPending int) {
	t.Helper()

	pcsGVR := schema.GroupVersionResource{Group: "grove.io", Version: "v1alpha1", Resource: "podcliquesets"}
	patchBytes, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
	})
	if err != nil {
		t.Fatalf("Failed to marshal PCS patch: %v", err)
	}

	if _, err := dynamicClient.Resource(pcsGVR).Namespace(namespace).Patch(ctx, pcsName, types.MergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		t.Fatalf("Failed to scale PodCliqueSet %s: %v", pcsName, err)
	}

	totalPods, runningPods, pendingPods, err := waitForPodConditions(ctx, clientset, namespace, labelSelector, expectedTotalPods, expectedPending, 1*time.Minute, 1*time.Second)
	if err != nil {
		t.Fatalf("Failed to wait for expected pod conditions after PCS scaling: %v. Final state: total=%d, running=%d, pending=%d (expected: total=%d, pending=%d)",
			err, totalPods, runningPods, pendingPods, expectedTotalPods, expectedPending)
	}
}
