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

package ci

import (
	"context"
	"testing"

	"github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/config"
	"github.com/k3d-io/k3d/v5/pkg/config/types"
	"github.com/k3d-io/k3d/v5/pkg/config/v1alpha5"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3d "github.com/k3d-io/k3d/v5/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
)

// ClusterConfig holds configuration for creating a k3d cluster
type ClusterConfig struct {
	Name             string
	Servers          int
	Agents           int
	Image            string
	HostPort         string
	LoadBalancerPort string
}

// DefaultClusterConfig returns a sensible default cluster configuration
func DefaultClusterConfig() ClusterConfig {
	return ClusterConfig{
		Name:             "test-k3d-cluster",
		Servers:          1,
		Agents:           2,
		Image:            "rancher/k3s:v1.28.8-k3s1",
		HostPort:         "6550",
		LoadBalancerPort: "8080:80",
	}
}

// setupCluster creates a k3d cluster and returns a kubernetes clientset
func setupCluster(ctx context.Context, t *testing.T, cfg ClusterConfig) (*kubernetes.Clientset, *v1alpha5.ClusterConfig, func()) {
	t.Logf("📝 Preparing k3d cluster configuration for '%s'...", cfg.Name)

	// Create cluster configuration
	clusterConfig := v1alpha5.SimpleConfig{
		ObjectMeta: types.ObjectMeta{
			Name: cfg.Name,
		},
		Servers: cfg.Servers,
		Agents:  cfg.Agents,
		Image:   cfg.Image,
		ExposeAPI: v1alpha5.SimpleExposureOpts{
			Host:     "0.0.0.0",
			HostPort: cfg.HostPort,
		},
		Ports: []v1alpha5.PortWithNodeFilters{
			{
				Port:        cfg.LoadBalancerPort,
				NodeFilters: []string{"loadbalancer"},
			},
		},
	}

	// Transform configuration
	k3dConfig, err := config.TransformSimpleToClusterConfig(ctx, runtimes.Docker, clusterConfig, "")
	if err != nil {
		t.Fatalf("Failed to transform config: %v", err)
	}

	// Create cluster
	t.Logf("🚀 Creating cluster '%s' with %d server(s) and %d agent(s)...",
		k3dConfig.Cluster.Name, cfg.Servers, cfg.Agents)

	if err := client.ClusterRun(ctx, runtimes.Docker, k3dConfig); err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	t.Log("✅ Cluster created successfully!")

	// Get kubeconfig
	t.Log("📄 Fetching kubeconfig...")
	cluster, err := client.ClusterGet(ctx, runtimes.Docker, &k3dConfig.Cluster)
	if err != nil {
		t.Fatalf("Could not get cluster: %v", err)
	}

	kubeconfig, err := client.KubeconfigGet(ctx, runtimes.Docker, cluster)
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	kubeconfigBytes, err := clientcmd.Write(*kubeconfig)
	if err != nil {
		t.Fatalf("Failed to serialize kubeconfig: %v", err)
	}

	// Create kubernetes clientset
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigBytes)
	if err != nil {
		t.Fatalf("Could not create rest config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("Could not create clientset: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		t.Log("🗑️ Deleting cluster...")
		if err := client.ClusterDelete(ctx, runtimes.Docker, &k3dConfig.Cluster, k3d.ClusterDeleteOpts{}); err != nil {
			t.Logf("Failed to delete cluster: %v", err)
		} else {
			t.Log("✅ Cluster deleted successfully")
		}
	}

	return clientset, k3dConfig, cleanup
}

// KindClusterConfig holds configuration for creating a kind cluster
type KindClusterConfig struct {
	Name          string
	ControlPlanes int
	Workers       int
	Image         string
}

// DefaultKindClusterConfig returns a sensible default kind cluster configuration
func DefaultKindClusterConfig() KindClusterConfig {
	return KindClusterConfig{
		Name:          "test-kind-cluster",
		ControlPlanes: 1,
		Workers:       2,
		Image:         "", // Empty means use kind's default
	}
}

// setupKindCluster creates a kind cluster and returns a kubernetes clientset
func setupKindCluster(ctx context.Context, t *testing.T, cfg KindClusterConfig) (*kubernetes.Clientset, func()) {
	t.Logf("📝 Preparing kind cluster configuration for '%s'...", cfg.Name)

	provider := cluster.NewProvider()

	// Create cluster configuration
	kindConfig := &v1alpha4.Cluster{
		Nodes: []v1alpha4.Node{},
	}

	// Add control plane nodes
	for i := 0; i < cfg.ControlPlanes; i++ {
		node := v1alpha4.Node{
			Role: v1alpha4.ControlPlaneRole,
		}
		if cfg.Image != "" {
			node.Image = cfg.Image
		}
		kindConfig.Nodes = append(kindConfig.Nodes, node)
	}

	// Add worker nodes
	for i := 0; i < cfg.Workers; i++ {
		node := v1alpha4.Node{
			Role: v1alpha4.WorkerRole,
		}
		if cfg.Image != "" {
			node.Image = cfg.Image
		}
		kindConfig.Nodes = append(kindConfig.Nodes, node)
	}

	// Create cluster
	t.Logf("🚀 Creating kind cluster '%s' with %d control-plane(s) and %d worker(s)...",
		cfg.Name, cfg.ControlPlanes, cfg.Workers)

	if err := provider.Create(
		cfg.Name,
		cluster.CreateWithV1Alpha4Config(kindConfig),
	); err != nil {
		t.Fatalf("Failed to create kind cluster: %v", err)
	}
	t.Log("✅ Kind cluster created successfully!")

	// Get kubeconfig
	t.Log("📄 Fetching kubeconfig...")
	kubeConfigYaml, err := provider.KubeConfig(cfg.Name, false)
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	// Create kubernetes clientset
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfigYaml))
	if err != nil {
		t.Fatalf("Could not create rest config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatalf("Could not create clientset: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		t.Log("🗑️ Deleting kind cluster...")
		if err := provider.Delete(cfg.Name, ""); err != nil {
			t.Logf("Failed to delete kind cluster: %v", err)
		} else {
			t.Log("✅ Kind cluster deleted successfully")
		}
	}

	return clientset, cleanup
}

func TestWith3dCluster(t *testing.T) {
	ctx := context.Background()

	// Custom configuration
	customCfg := ClusterConfig{
		Name:             "custom-test-cluster",
		Servers:          2,
		Agents:           3,
		Image:            "rancher/k3s:v1.28.8-k3s1",
		HostPort:         "6551",
		LoadBalancerPort: "8081:80",
	}

	// Setup cluster with custom config
	clientset, _, cleanup := setupCluster(ctx, t, customCfg)
	defer cleanup()

	// Test with custom cluster
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("could not list nodes: %s", err)
	}

	expectedNodes := customCfg.Servers + customCfg.Agents
	t.Logf("✅ Found %d nodes in the custom cluster", len(nodes.Items))

	if len(nodes.Items) != expectedNodes {
		t.Errorf("expected %d nodes, but found %d", expectedNodes, len(nodes.Items))
	}
}

// Example of how to use kind with custom configuration
func TestWithKindCluster(t *testing.T) {
	ctx := context.Background()

	// Custom configuration
	customCfg := KindClusterConfig{
		Name:          "custom-kind-cluster",
		ControlPlanes: 2,
		Workers:       3,
		Image:         "kindest/node:v1.28.0", // Optional: specify custom image
	}

	// Setup cluster with custom config
	clientset, cleanup := setupKindCluster(ctx, t, customCfg)
	defer cleanup()

	// Test with custom cluster
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("could not list nodes: %s", err)
	}

	expectedNodes := customCfg.ControlPlanes + customCfg.Workers
	t.Logf("✅ Found %d nodes in the custom kind cluster", len(nodes.Items))

	if len(nodes.Items) != expectedNodes {
		t.Errorf("expected %d nodes, but found %d", expectedNodes, len(nodes.Items))
	}
}
