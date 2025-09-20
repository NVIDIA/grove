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
	"github.com/NVIDIA/grove/operator/internal/constants"
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

// Constants for error codes.
const (
	errCodeClientCreation               grovecorev1alpha1.ErrorCode = "ERR_CLIENT_CREATION"
	errCodeLabelSelectorCreationForPods grovecorev1alpha1.ErrorCode = "ERR_LABEL_SELECTOR_CREATION_FOR_PODS"
	errCodeRegisterEventHandler         grovecorev1alpha1.ErrorCode = "ERR_REGISTER_EVENT_HANDLER"
)

const (
	operationWaitForParentPodClique = "WaitForParentPodClique"
)

// ParentPodCliqueDependencies contains the last known (readiness) state of all parent PodCliques.
type ParentPodCliqueDependencies struct {
	mutex                 sync.Mutex
	namespace             string
	podGang               string
	pclqFQNToMinAvailable map[string]int
	currentPCLQReadyPods  map[string]sets.Set[string]
	allReadyCh            chan struct{}
}

// NewPodCliqueState creates and initializes all parent PodCliques with an unready state.
func NewPodCliqueState(podCliqueDependencies map[string]int, log logr.Logger) (*ParentPodCliqueDependencies, error) {
	podNamespaceFilePath := filepath.Join(constants.VolumeMountPathPodInfo, constants.PodNamespaceFileName)
	podNamespace, err := os.ReadFile(podNamespaceFilePath)
	if err != nil {
		log.Error(err, "Failed to read the pod namespace from the file", "filepath", podNamespaceFilePath)
		return nil, err
	}

	podGangNameFilePath := filepath.Join(constants.VolumeMountPathPodInfo, constants.PodGangNameFileName)
	podGangName, err := os.ReadFile(podGangNameFilePath)
	if err != nil {
		log.Error(err, "Failed to read the PodGang name from the file", "filepath", podGangNameFilePath)
		return nil, err
	}

	currentlyReadyPods := make(map[string]sets.Set[string])
	// Initialize the keys to indicate these are the parent PodCliques.
	for parentPodCliqueName := range podCliqueDependencies {
		currentlyReadyPods[parentPodCliqueName] = sets.New[string]()
	}

	state := &ParentPodCliqueDependencies{
		namespace:             string(podNamespace),
		podGang:               string(podGangName),
		pclqFQNToMinAvailable: podCliqueDependencies,
		currentPCLQReadyPods:  currentlyReadyPods,
		allReadyCh:            make(chan struct{}, len(podCliqueDependencies)),
	}

	return state, nil
}

// WaitForReady waits for all upstream start-up dependencies to be ready.
func (c *ParentPodCliqueDependencies) WaitForReady(ctx context.Context, log logr.Logger) error {
	defer close(c.allReadyCh) // Close the channel the informers write to *after* the context they use is cancelled.

	log.Info("Parent PodClique(s) being waited on", "pclqFQNToMinAvailable", c.pclqFQNToMinAvailable)

	client, err := createClient()
	if err != nil {
		return err
	}

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

	eventHandlerContext, cancel := context.WithCancel(ctx)
	defer cancel() // Cancel the context used by the informers if the wait is successful, or an err occurs.

	factory := informers.NewSharedInformerFactoryWithOptions(
		client,
		time.Second,
		informers.WithNamespace(c.namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = selector.String()
		},
		),
	)
	if err := c.registerEventHandler(factory, log); err != nil {
		return groveerr.WrapError(
			err,
			errCodeRegisterEventHandler,
			operationWaitForParentPodClique,
			"failed to register the Pod event handler",
		)
	}

	factory.WaitForCacheSync(eventHandlerContext.Done())
	factory.Start(eventHandlerContext.Done())

	select {
	case <-c.allReadyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func createClient() (*kubernetes.Clientset, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, groveerr.WrapError(
			err,
			errCodeClientCreation,
			operationWaitForParentPodClique,
			"failed to fetch the in cluster config",
		)
	}
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

func (c *ParentPodCliqueDependencies) registerEventHandler(factory informers.SharedInformerFactory, log logr.Logger) error {
	typedInformer := factory.Core().V1().Pods().Informer()
	_, err := typedInformer.AddEventHandlerWithOptions(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}

			c.refreshReadyPodsOfPodClique(pod, false)
			if c.checkAllParentsReady() {
				c.allReadyCh <- struct{}{}
			}
		},
		UpdateFunc: func(_, newObj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			pod, ok := newObj.(*corev1.Pod)
			if !ok {
				return
			}

			c.refreshReadyPodsOfPodClique(pod, false)
			if c.checkAllParentsReady() {
				c.allReadyCh <- struct{}{}
			}
		},
		DeleteFunc: func(obj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}

			c.refreshReadyPodsOfPodClique(pod, true)
			if c.checkAllParentsReady() {
				c.allReadyCh <- struct{}{}
			}
		},
	}, cache.HandlerOptions{Logger: &log})
	return err
}

func (c *ParentPodCliqueDependencies) refreshReadyPodsOfPodClique(pod *corev1.Pod, deletionEvent bool) {
	podCliqueName, ok := lo.Find(lo.Keys(c.pclqFQNToMinAvailable), func(podCliqueFQN string) bool {
		return strings.HasPrefix(pod.Name, podCliqueFQN)
	})
	if !ok {
		return // If not found, the Pod is not related to any parent PodClique.
	}

	if deletionEvent {
		c.currentPCLQReadyPods[podCliqueName].Delete(pod.Name)
		return
	}

	readyCondition, ok := lo.Find(pod.Status.Conditions, func(podCondition corev1.PodCondition) bool {
		return podCondition.Type == corev1.PodReady
	})
	podReady := ok && readyCondition.Status == corev1.ConditionTrue

	if podReady {
		c.currentPCLQReadyPods[podCliqueName].Insert(pod.Name)
	} else {
		c.currentPCLQReadyPods[podCliqueName].Delete(pod.Name)
	}
}

func (c *ParentPodCliqueDependencies) checkAllParentsReady() bool {
	for cliqueName, readyPods := range c.currentPCLQReadyPods {
		if len(readyPods) < c.pclqFQNToMinAvailable[cliqueName] {
			return false // If any single parent is not ready, wait.
		}
	}
	return true
}

func getLabelSelectorForPods(podGangName string) map[string]string {
	return map[string]string{
		apicommon.LabelPodGang: podGangName,
	}
}
