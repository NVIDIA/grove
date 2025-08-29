package ci

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/k3s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestWithK3sCluster(t *testing.T) {
	ctx := context.Background()

	// 1. Request a K3s container
	k3sContainer, err := k3s.Run(ctx, "rancher/k3s:v1.28.2-k3s1")
	if err != nil {
		t.Fatalf("could not start k3s container: %s", err)
	}

	// 2. Clean up the container after the test is done
	defer func() {
		if err := k3sContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	// 3. Get the kubeconfig from the running container
	kubeConfigYaml, err := k3sContainer.GetKubeConfig(ctx)
	if err != nil {
		t.Fatalf("could not get kubeconfig: %s", err)
	}

	// 4. Use the kubeconfig to create a Kubernetes client
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		t.Fatalf("could not create rest config: %s", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("could not create clientset: %s", err)
	}

	// 5. Run your tests against the cluster!
	// For example, list the nodes to prove it's working.
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("could not list nodes: %s", err)
	}

	fmt.Printf("✅ Found %d nodes in the cluster\n", len(nodes.Items))

	// Your integration test logic would go here.
	// You can apply manifests, create pods, services, etc.
}
