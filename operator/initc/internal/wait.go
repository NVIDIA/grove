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

package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/common"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// Error codes for client and event handling operations.
const (
	errCodeClientCreation               grovecorev1alpha1.ErrorCode = "ERR_CLIENT_CREATION"
	errCodeLabelSelectorCreationForPods grovecorev1alpha1.ErrorCode = "ERR_LABEL_SELECTOR_CREATION_FOR_PODS"
	errCodeRegisterEventHandler         grovecorev1alpha1.ErrorCode = "ERR_REGISTER_EVENT_HANDLER"
)

// Operation names for error tracking and logging.
const (
	operationWaitForParentPodClique = "WaitForParentPodClique"
)

// ParentPodCliqueDependencies tracks the readiness state of all parent PodCliques.
// It maintains the current state of ready pods for each PodClique and provides
// synchronization mechanisms to wait for all dependencies to be satisfied.
type ParentPodCliqueDependencies struct {
	// mutex protects concurrent access to the state fields
	mutex sync.Mutex
	// namespace is the Kubernetes namespace where the PodCliques are located
	namespace string
	// podGang is the name of the PodGang that groups related PodCliques
	podGang string
	// pclqFQNToMinAvailable maps PodClique fully qualified names to their minimum required ready replicas
	pclqFQNToMinAvailable map[string]int
	// currentPCLQReadyPods tracks the currently ready pods for each PodClique by name
	currentPCLQReadyPods map[string]sets.Set[string]
	// allReadyCh signals when all parent PodCliques have reached their minimum ready state
	allReadyCh chan struct{}
}

// NewPodCliqueState creates and initializes a new ParentPodCliqueDependencies instance.
// It reads the pod namespace and PodGang name from mounted files and sets up the state
// tracking for all parent PodCliques in an initially unready state.
func NewPodCliqueState(podCliqueDependencies map[string]int, log logr.Logger) (*ParentPodCliqueDependencies, error) {
	namespacePath := filepath.Join(common.VolumeMountPathPodInfo, common.PodNamespaceFileName)
	podGangPath := filepath.Join(common.VolumeMountPathPodInfo, common.PodGangNameFileName)
	return NewPodCliqueStateWithFilePaths(podCliqueDependencies, namespacePath, podGangPath, log)
}

// NewPodCliqueStateWithFilePaths creates and initializes a new ParentPodCliqueDependencies instance
// using the provided file paths. This enables testing by allowing file paths to be controlled
// while using the standard file reading operations.
func NewPodCliqueStateWithFilePaths(podCliqueDependencies map[string]int, namespacePath, podGangPath string, log logr.Logger) (*ParentPodCliqueDependencies, error) {
	// Read the pod namespace from the specified file
	podNamespace, err := os.ReadFile(namespacePath)
	if err != nil {
		log.Error(err, "Failed to read the pod namespace from the file", "filepath", namespacePath)
		return nil, err
	}

	// Read the PodGang name from the specified file
	podGangName, err := os.ReadFile(podGangPath)
	if err != nil {
		log.Error(err, "Failed to read the PodGang name from the file", "filepath", podGangPath)
		return nil, err
	}

	// Initialize the ready pods tracking map for each parent PodClique
	currentlyReadyPods := make(map[string]sets.Set[string])
	for parentPodCliqueName := range podCliqueDependencies {
		currentlyReadyPods[parentPodCliqueName] = sets.New[string]()
	}

	// Create and return the initialized state
	state := &ParentPodCliqueDependencies{
		namespace:             string(podNamespace),
		podGang:               string(podGangName),
		pclqFQNToMinAvailable: podCliqueDependencies,
		currentPCLQReadyPods:  currentlyReadyPods,
		allReadyCh:            make(chan struct{}, len(podCliqueDependencies)),
	}

	return state, nil
}

// WaitForReady waits for all parent PodCliques to reach their minimum ready replica count.
// It sets up Kubernetes informers to watch pod events and blocks until all dependencies are satisfied
// or the provided context is cancelled.
func (c *ParentPodCliqueDependencies) WaitForReady(ctx context.Context, log logr.Logger) error {
	// Create Kubernetes client for accessing the API
	client, err := createClient()
	if err != nil {
		return err
	}
	return c.WaitForReadyWithClient(ctx, client, log)
}

// WaitForReadyWithClient waits for all parent PodCliques to reach their minimum ready replica count
// using the provided Kubernetes client. This enables testing by allowing client injection.
func (c *ParentPodCliqueDependencies) WaitForReadyWithClient(ctx context.Context, client kubernetes.Interface, log logr.Logger) error {
	// Ensure the channel is closed when we exit, regardless of success or failure
	defer close(c.allReadyCh)

	log.Info("Parent PodClique(s) being waited on", "pclqFQNToMinAvailable", c.pclqFQNToMinAvailable)

	// Check if we're already ready (e.g., empty dependencies)
	if c.checkAllParentsReady() {
		return nil
	}

	// Create label selector to filter pods belonging to this PodGang
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: getLabelSelectorForPods(c.podGang),
	})
	if err != nil {
		return groveerr.WrapError(
			err,
			errCodeLabelSelectorCreationForPods,
			operationWaitForParentPodClique,
			"failed to convert labels required for the PodGang to selector",
		)
	}

	// Create a cancellable context for the informers
	eventHandlerContext, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up the shared informer factory with namespace and label filtering
	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		time.Second,
		informers.WithNamespace(c.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector.String()
		},
		),
	)

	// Register event handlers for pod lifecycle events
	if err := c.registerEventHandler(factory, log); err != nil {
		return groveerr.WrapError(
			err,
			errCodeRegisterEventHandler,
			operationWaitForParentPodClique,
			"failed to register the Pod event handler",
		)
	}

	// Wait for informer cache to sync and start the informers
	factory.WaitForCacheSync(eventHandlerContext.Done())
	factory.Start(eventHandlerContext.Done())

	// Block until either all dependencies are ready or context is cancelled
	select {
	case <-c.allReadyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// createClient creates and returns a Kubernetes clientset using the in-cluster configuration.
// This function is designed to be called from within a pod running in a Kubernetes cluster.
func createClient() (*kubernetes.Clientset, error) {
	// Get the in-cluster REST configuration
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, groveerr.WrapError(
			err,
			errCodeClientCreation,
			operationWaitForParentPodClique,
			"failed to fetch the in cluster config",
		)
	}

	// Create the Kubernetes clientset from the REST configuration
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, groveerr.WrapError(
			err,
			errCodeClientCreation,
			operationWaitForParentPodClique,
			"failed to create clientSet with the fetched restConfig",
		)
	}
	return client, nil
}

// registerEventHandler registers pod lifecycle event handlers with the shared informer factory.
// It handles pod addition, updates, and deletion events to track readiness state changes.
func (c *ParentPodCliqueDependencies) registerEventHandler(factory informers.SharedInformerFactory, log logr.Logger) error {
	// Get the pod informer from the factory
	typedInformer := factory.Core().V1().Pods().Informer()

	// Register event handlers for pod lifecycle events
	_, err := typedInformer.AddEventHandlerWithOptions(cache.ResourceEventHandlerFuncs{
		// Handle new pod creation
		AddFunc: func(obj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}

			// Update readiness state and signal if all dependencies are met
			c.refreshReadyPodsOfPodClique(pod, false)
			if c.checkAllParentsReady() {
				c.allReadyCh <- struct{}{}
			}
		},
		// Handle pod updates (typically readiness changes)
		UpdateFunc: func(_, newObj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			pod, ok := newObj.(*corev1.Pod)
			if !ok {
				return
			}

			// Update readiness state and signal if all dependencies are met
			c.refreshReadyPodsOfPodClique(pod, false)
			if c.checkAllParentsReady() {
				c.allReadyCh <- struct{}{}
			}
		},
		// Handle pod deletion
		DeleteFunc: func(obj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			// Handle tombstone objects from the cache
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}

			// Remove pod from ready set and signal if all dependencies are still met
			c.refreshReadyPodsOfPodClique(pod, true)
			if c.checkAllParentsReady() {
				c.allReadyCh <- struct{}{}
			}
		},
	}, cache.HandlerOptions{Logger: &log})
	return err
}

// refreshReadyPodsOfPodClique updates the ready pod tracking for a given pod.
// It determines which PodClique the pod belongs to and updates the readiness state accordingly.
func (c *ParentPodCliqueDependencies) refreshReadyPodsOfPodClique(pod *corev1.Pod, deletionEvent bool) {
	// Find which PodClique this pod belongs to by matching name prefixes
	podCliqueName, ok := lo.Find(lo.Keys(c.pclqFQNToMinAvailable), func(podCliqueFQN string) bool {
		return strings.HasPrefix(pod.Name, podCliqueFQN)
	})
	if !ok {
		// Pod doesn't belong to any parent PodClique we're tracking
		return
	}

	// Handle pod deletion events
	if deletionEvent {
		c.currentPCLQReadyPods[podCliqueName].Delete(pod.Name)
		return
	}

	// Check if the pod is ready by examining its conditions
	readyCondition, ok := lo.Find(pod.Status.Conditions, func(podCondition corev1.PodCondition) bool {
		return podCondition.Type == corev1.PodReady
	})
	podReady := ok && readyCondition.Status == corev1.ConditionTrue

	// Update the ready pod set based on the pod's readiness state
	if podReady {
		c.currentPCLQReadyPods[podCliqueName].Insert(pod.Name)
	} else {
		c.currentPCLQReadyPods[podCliqueName].Delete(pod.Name)
	}
}

// checkAllParentsReady determines if all parent PodCliques have reached their minimum ready replica count.
// Returns true only when every PodClique has at least its required number of ready pods.
func (c *ParentPodCliqueDependencies) checkAllParentsReady() bool {
	// Check each PodClique's ready pod count against its minimum requirement
	for cliqueName, readyPods := range c.currentPCLQReadyPods {
		if len(readyPods) < c.pclqFQNToMinAvailable[cliqueName] {
			// If any PodClique doesn't have enough ready pods, we're not ready
			return false
		}
	}
	// All PodCliques have met their minimum ready replica requirements
	return true
}

// getLabelSelectorForPods creates a label selector map to filter pods belonging to a specific PodGang.
// This is used to set up informer filtering to only watch relevant pods.
func getLabelSelectorForPods(podGangName string) map[string]string {
	return map[string]string{
		apicommon.LabelPodGang: podGangName,
	}
}
