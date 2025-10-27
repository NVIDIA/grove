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
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SharedClusterManager manages a shared (singleton) k3d cluster for E2E tests
type SharedClusterManager struct {
	clientset        *kubernetes.Clientset
	restConfig       *rest.Config
	dynamicClient    dynamic.Interface
	cleanup          func()
	logger           *logrus.Logger
	isSetup          bool
	agentNodes       []string
	registryPort     string
	skaffoldYAMLPath string
}

var (
	sharedCluster *SharedClusterManager
	once          sync.Once
)

// SharedCluster returns the singleton shared cluster manager
func SharedCluster(logger *logrus.Logger, skaffoldYAMLPath string) *SharedClusterManager {
	once.Do(func() {
		sharedCluster = &SharedClusterManager{
			logger:           logger,
			skaffoldYAMLPath: skaffoldYAMLPath,
		}
	})
	return sharedCluster
}

// Setup initializes the shared cluster with maximum required resources
func (scm *SharedClusterManager) Setup(ctx context.Context, testImages []string) error {
	if scm.isSetup {
		return nil
	}

	// Configuration for maximum cluster size needed (28 agents + 3 servers)
	customCfg := ClusterConfig{
		Name:             "shared-e2e-test-cluster",
		Servers:          3,
		Agents:           28, // Maximum needed across all tests
		WorkerMemory:     "150m",
		Image:            "rancher/k3s:v1.33.5-k3s1",
		HostPort:         "6560", // Use a different port to avoid conflicts
		LoadBalancerPort: "8090:80",
		EnableRegistry:   true,
		RegistryPort:     "5001",
		AgentNodeLabels: map[string]string{
			"node_role.e2e.grove.nvidia.com": "agent",
		},
		AgentNodeTaints: []NodeTaint{
			{
				Key:    "node_role.e2e.grove.nvidia.com",
				Value:  "agent",
				Effect: "NoSchedule",
			},
		},
	}

	scm.registryPort = customCfg.RegistryPort

	scm.logger.Info("🚀 Setting up shared k3d cluster for all e2e tests...")

	restConfig, cleanup, err := SetupCompleteK3DCluster(ctx, customCfg, scm.skaffoldYAMLPath, scm.logger)
	if err != nil {
		return fmt.Errorf("failed to setup shared k3d cluster: %w", err)
	}

	scm.restConfig = restConfig
	scm.cleanup = cleanup

	// Create clientset from restConfig
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create clientset: %w", err)
	}
	scm.clientset = clientset

	// Create dynamic client from restConfig
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}
	scm.dynamicClient = dynamicClient

	// Setup test images in registry
	if err := setupRegistryTestImages(scm.registryPort, testImages); err != nil {
		cleanup()
		return fmt.Errorf("failed to setup registry test images: %w", err)
	}

	// Get list of agent nodes for cordoning management
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	scm.agentNodes = make([]string, 0)
	for _, node := range nodes.Items {
		if _, isServer := node.Labels["node-role.kubernetes.io/control-plane"]; !isServer {
			scm.agentNodes = append(scm.agentNodes, node.Name)
		}
	}

	scm.logger.Infof("✅ Shared cluster setup complete with %d agent nodes", len(scm.agentNodes))
	scm.isSetup = true
	return nil
}

// PrepareForTest prepares the cluster for a specific test by cordoning the appropriate nodes
func (scm *SharedClusterManager) PrepareForTest(ctx context.Context, requiredAgents int) error {
	if !scm.isSetup {
		return fmt.Errorf("shared cluster not setup")
	}

	// First, uncordon all nodes to reset state
	for _, nodeName := range scm.agentNodes {
		if err := CordonNode(ctx, scm.clientset, nodeName, false); err != nil {
			return fmt.Errorf("failed to uncordon node %s: %w", nodeName, err)
		}
	}

	// Cordon nodes that are not needed for this test
	if requiredAgents < len(scm.agentNodes) {
		nodesToCordon := scm.agentNodes[requiredAgents:]
		for _, nodeName := range nodesToCordon {
			if err := CordonNode(ctx, scm.clientset, nodeName, true); err != nil {
				return fmt.Errorf("failed to cordon node %s: %w", nodeName, err)
			}
		}
	}

	return nil
}

// CleanupWorkloads removes all test workloads from the cluster
func (scm *SharedClusterManager) CleanupWorkloads(ctx context.Context) error {
	if !scm.isSetup {
		return nil
	}

	scm.logger.Debug("🧹 Cleaning up workloads from shared cluster...")

	// Step 1: Delete PodCliqueSets first (should cascade delete other resources)
	if err := scm.deleteAllResources(ctx, "grove.io", "v1alpha1", "podcliquesets"); err != nil {
		scm.logger.Warnf("failed to delete PodCliqueSets: %v", err)
	}

	// Step 2: Poll for all resources and pods to be cleaned up (max 15 seconds)
	if err := scm.waitForAllResourcesAndPodsDeleted(ctx, 15*time.Second); err != nil {
		scm.logger.Warnf("timeout waiting for resources and pods to be deleted: %v", err)
		// List remaining resources and pods for debugging
		scm.listRemainingResources(ctx)
		scm.listRemainingPods(ctx, "default")
	}

	// Step 3: Reset node cordoning state
	if err := scm.resetNodeStates(ctx); err != nil {
		scm.logger.Warnf("failed to reset node states: %v", err)
	}

	return nil
}

// deleteAllResources deletes all resources of a specific type across all namespaces
func (scm *SharedClusterManager) deleteAllResources(ctx context.Context, group, version, resource string) error {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// List all resources across all namespaces
	resourceList, err := scm.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", resource, err)
	}

	// Delete each resource
	for _, item := range resourceList.Items {
		namespace := item.GetNamespace()
		name := item.GetName()

		err := scm.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			scm.logger.Warnf("failed to delete %s %s/%s: %v", resource, namespace, name, err)
		}
	}

	return nil
}

// isSystemPod checks if a pod is a system pod that should be ignored during cleanup
func isSystemPod(pod *v1.Pod) bool {
	// Skip pods managed by DaemonSets or system namespaces
	if pod.Namespace == "kube-system" || pod.Namespace == "grove-system" {
		return true
	}

	// Skip pods with system owner references
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}

	return false
}

// listRemainingPods lists remaining pods for debugging
func (scm *SharedClusterManager) listRemainingPods(ctx context.Context, namespace string) {
	pods, err := scm.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		scm.logger.Warnf("Failed to list remaining pods: %v", err)
		return
	}

	nonSystemPods := []string{}
	for _, pod := range pods.Items {
		if !isSystemPod(&pod) {
			nonSystemPods = append(nonSystemPods, fmt.Sprintf("%s (Phase: %s)", pod.Name, pod.Status.Phase))
		}
	}

	if len(nonSystemPods) > 0 {
		scm.logger.Warnf("Remaining non-system pods: %v", nonSystemPods)
	}
}

// resetNodeStates uncordons all agent nodes to reset cluster state
func (scm *SharedClusterManager) resetNodeStates(ctx context.Context) error {
	for _, nodeName := range scm.agentNodes {
		if err := CordonNode(ctx, scm.clientset, nodeName, false); err != nil {
			scm.logger.Warnf("failed to uncordon node %s: %v", nodeName, err)
			return fmt.Errorf("failed to uncordon node %s: %w", nodeName, err)
		}
	}
	return nil
}

// waitForAllResourcesAndPodsDeleted waits for all Grove resources and pods to be deleted
func (scm *SharedClusterManager) waitForAllResourcesAndPodsDeleted(ctx context.Context, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Define all resource types to check
	resourceTypes := []struct {
		group    string
		version  string
		resource string
		name     string
	}{
		{"grove.io", "v1alpha1", "podcliquesets", "PodCliqueSets"},
		{"grove.io", "v1alpha1", "podcliquescalinggroups", "PodCliqueScalingGroups"},
		{"grove.io", "v1alpha1", "podgangsets", "PodGangSets"},
		{"scheduler.grove.io", "v1alpha1", "podgangs", "PodGangs"},
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for resources and pods to be deleted")
		case <-ticker.C:
			allResourcesDeleted := true
			totalResources := 0

			// Check Grove resources
			for _, rt := range resourceTypes {
				gvr := schema.GroupVersionResource{
					Group:    rt.group,
					Version:  rt.version,
					Resource: rt.resource,
				}

				resourceList, err := scm.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
				if err != nil {
					// If we can't list the resource type, assume it doesn't exist or is being deleted
					continue
				}

				if len(resourceList.Items) > 0 {
					allResourcesDeleted = false
					totalResources += len(resourceList.Items)
				}
			}

			// Check pods
			allPodsDeleted := true
			nonSystemPods := 0
			pods, err := scm.clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
			if err == nil {
				for _, pod := range pods.Items {
					if !isSystemPod(&pod) {
						allPodsDeleted = false
						nonSystemPods++
					}
				}
			}

			if allResourcesDeleted && allPodsDeleted {
				return nil
			}

			if totalResources > 0 || nonSystemPods > 0 {
				scm.logger.Debugf("⏳ Waiting for %d Grove resources and %d pods to be deleted...", totalResources, nonSystemPods)
			}
		}
	}
}

// listRemainingResources lists remaining Grove resources for debugging
func (scm *SharedClusterManager) listRemainingResources(ctx context.Context) {
	resourceTypes := []struct {
		group    string
		version  string
		resource string
		name     string
	}{
		{"grove.io", "v1alpha1", "podcliquesets", "PodCliqueSets"},
		{"grove.io", "v1alpha1", "podcliquescalinggroups", "PodCliqueScalingGroups"},
		{"grove.io", "v1alpha1", "podgangsets", "PodGangSets"},
		{"scheduler.grove.io", "v1alpha1", "podgangs", "PodGangs"},
	}

	for _, rt := range resourceTypes {
		gvr := schema.GroupVersionResource{
			Group:    rt.group,
			Version:  rt.version,
			Resource: rt.resource,
		}

		resourceList, err := scm.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
		if err != nil {
			scm.logger.Warnf("Failed to list %s: %v", rt.name, err)
			continue
		}

		if len(resourceList.Items) > 0 {
			resourceNames := make([]string, 0, len(resourceList.Items))
			for _, item := range resourceList.Items {
				resourceNames = append(resourceNames, fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName()))
			}
			scm.logger.Warnf("Remaining %s: %v", rt.name, resourceNames)
		}
	}
}

// GetClients returns the kubernetes clients for tests to use
func (scm *SharedClusterManager) GetClients() (*kubernetes.Clientset, *rest.Config, dynamic.Interface) {
	return scm.clientset, scm.restConfig, scm.dynamicClient
}

// GetRegistryPort returns the registry port for test image setup
func (scm *SharedClusterManager) GetRegistryPort() string {
	return scm.registryPort
}

// GetAgentNodes returns the list of agent node names
func (scm *SharedClusterManager) GetAgentNodes() []string {
	return scm.agentNodes
}

// IsSetup returns whether the shared cluster is setup
func (scm *SharedClusterManager) IsSetup() bool {
	return scm.isSetup
}

// Teardown cleans up the shared cluster
func (scm *SharedClusterManager) Teardown() {
	if scm.cleanup != nil {
		scm.cleanup()
		scm.isSetup = false
	}
}

// setupRegistryTestImages sets up test images in the registry
func setupRegistryTestImages(registryPort string, images []string) error {
	if len(images) == 0 {
		return nil
	}

	ctx := context.Background()

	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Process each image
	for _, imageName := range images {
		registryImage := fmt.Sprintf("localhost:%s/%s", registryPort, imageName)

		// Step 1: Pull the image
		pullReader, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull %s: %w", imageName, err)
		}

		// Consume the pull output to avoid blocking
		_, err = io.Copy(io.Discard, pullReader)
		pullReader.Close()
		if err != nil {
			return fmt.Errorf("failed to read pull output for %s: %w", imageName, err)
		}

		// Step 2: Tag the image for the local registry
		err = cli.ImageTag(ctx, imageName, registryImage)
		if err != nil {
			return fmt.Errorf("failed to tag image %s as %s: %w", imageName, registryImage, err)
		}

		// Step 3: Push the image to the local registry
		pushReader, err := cli.ImagePush(ctx, registryImage, image.PushOptions{})
		if err != nil {
			return fmt.Errorf("failed to push %s: %w", registryImage, err)
		}

		// Consume the push output to avoid blocking
		_, err = io.Copy(io.Discard, pushReader)
		pushReader.Close()
		if err != nil {
			return fmt.Errorf("failed to read push output for %s: %w", registryImage, err)
		}
	}

	return nil
}
