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
	"log"
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
)

func TestWithK3sCluster(t *testing.T) {
	ctx := context.Background()

	// Define a multi-node cluster configuration.
	clusterConfig := v1alpha5.SimpleConfig{
		ObjectMeta: types.ObjectMeta{
			Name: "my-k3d-multinode",
		},
		Servers: 1,
		Agents:  2,
		Image:   "rancher/k3s:v1.28.8-k3s1",
		// This maps the cluster's internal API server port (6443) to port 6550 on your host machine.
		ExposeAPI: v1alpha5.SimpleExposureOpts{
			Host:     "0.0.0.0", // Listen on all host network interfaces
			HostPort: "6550",    // An unused port on your host
		},
		Ports: []v1alpha5.PortWithNodeFilters{
			{
				Port:        "8080:80",
				NodeFilters: []string{"loadbalancer"},
			},
		},
	}

	t.Log("📝 Preparing k3d cluster configuration...")
	cfg, err := config.TransformSimpleToClusterConfig(ctx, runtimes.Docker, clusterConfig, "")
	if err != nil {
		t.Fatalf("Failed to transform config: %v", err)
	}

	log.Printf("🚀 Creating cluster '%s' with 1 server and 2 agents...", cfg.Cluster.Name)
	if err := client.ClusterRun(ctx, runtimes.Docker, cfg); err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}
	t.Log("✅ Cluster created successfully!")

	log.Println("📄 Fetching kubeconfig...")
	cluster, err := client.ClusterGet(ctx, runtimes.Docker, &cfg.Cluster)
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
	kubeConfigYaml := string(kubeconfigBytes)

	// Now this will work because the server address in the kubeconfig is accessible
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeConfigYaml))
	if err != nil {
		t.Fatalf("Could not create rest config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("Could not create clientset: %v", err)
	}

	// 5. Run your tests against the cluster!
	// For example, list the nodes to prove it's working.
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("could not list nodes: %s", err)
	}

	t.Logf("✅ Found %d nodes in the cluster\n", len(nodes.Items))

	// Verify we have exactly 3 nodes
	if len(nodes.Items) != 3 {
		t.Errorf("expected 3 nodes, but found %d", len(nodes.Items))
	}

	t.Log("🗑️ Deleting cluster...")
	if err := client.ClusterDelete(ctx, runtimes.Docker, &cfg.Cluster, k3d.ClusterDeleteOpts{}); err != nil {
		t.Fatalf("Failed to delete cluster: %v", err)
	}
	t.Log("Cluster deleted.")

	// Your integration test logic would go here.
	// You can apply manifests, create pods, services, etc.
}
