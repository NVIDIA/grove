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

// Constants for error codes.
const (
	errCodeClientCreation               grovecorev1alpha1.ErrorCode = "ERR_CLIENT_CREATION"
	errCodeLabelSelectorCreationForPods grovecorev1alpha1.ErrorCode = "ERR_LABEL_SELECTOR_CREATION_FOR_PODS"
	errCodeRegisterEventHandler         grovecorev1alpha1.ErrorCode = "ERR_REGISTER_EVENT_HANDLER"
)

const (
	operationWaitForParentPodClique = "WaitForParentPodClique"
)

// PodCliqueState contains the last known (readiness) state of all parent PodCliques.
type PodCliqueState struct {
	log                logr.Logger
	mutex              *sync.Mutex
	namespace          string
	podgang            string
	podCliqueInfo      map[string]int
	currentlyReadyPods map[string]sets.Set[string]
	allReadyCh         chan struct{}
}

// NewPodCliqueState creates and initializes all parent PodCliques with an unready state.
func NewPodCliqueState(podCliqueInfo map[string]int, log logr.Logger) (*PodCliqueState, error) {
	podNamespaceFilePath := filepath.Join(common.VolumeMountPathPodInfo, common.PodNamespaceFileName)
	podNamespace, err := os.ReadFile(podNamespaceFilePath)
	if err != nil {
		log.Error(err, "Failed to read the pod namespace from the file", "filepath", podNamespaceFilePath)
		return nil, err
	}

	podGangNameFilePath := filepath.Join(common.VolumeMountPathPodInfo, common.PodGangNameFileName)
	podGangName, err := os.ReadFile(podGangNameFilePath)
	if err != nil {
		log.Error(err, "Failed to read the PodGang name from the file", "filepath", podGangNameFilePath)
		return nil, err
	}

	currentlyReadyPods := make(map[string]sets.Set[string])
	// Initialize the keys to indicate these are the parent PodCliques.
	for parentPodCliqueName := range podCliqueInfo {
		currentlyReadyPods[parentPodCliqueName] = sets.New[string]()
	}

	state := &PodCliqueState{
		log:                log,
		mutex:              &sync.Mutex{},
		namespace:          string(podNamespace),
		podgang:            string(podGangName),
		podCliqueInfo:      podCliqueInfo,
		currentlyReadyPods: currentlyReadyPods,
		allReadyCh:         make(chan struct{}, len(podCliqueInfo)),
	}

	return state, nil
}

// WaitForReady waits for all upstream start-up dependencies to be ready.
func (c *PodCliqueState) WaitForReady(ctx context.Context) error {
	defer close(c.allReadyCh) // Close the channel the informers write to *after* the context they use is cancelled.

	for podCliqueName, minAvailable := range c.podCliqueInfo {
		c.log.Info("Parent PodClique being waited on", "podCliqueName", podCliqueName, "minAvailable", minAvailable)
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return groveerr.WrapError(
			err,
			errCodeClientCreation,
			operationWaitForParentPodClique,
			"failed to fetch the in cluster config",
		)
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return groveerr.WrapError(
			err,
			errCodeClientCreation,
			operationWaitForParentPodClique,
			"failed to create clientSet with the fetched restConfig",
		)
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: getLabelSelectorForPods(c.podgang),
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
		))
	if err := c.registerEventHandler(factory); err != nil {
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

func (c *PodCliqueState) registerEventHandler(factory informers.SharedInformerFactory) error {
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
	}, cache.HandlerOptions{Logger: &c.log})
	return err
}

func (c *PodCliqueState) refreshReadyPodsOfPodClique(pod *corev1.Pod, deletionEvent bool) {
	podCliqueName, ok := lo.Find(lo.Keys(c.podCliqueInfo), func(podCliqueFQN string) bool {
		return strings.HasPrefix(pod.Name, podCliqueFQN)
	})
	if !ok {
		return // If not found, the Pod is not related to any parent PodClique.
	}

	if deletionEvent {
		c.currentlyReadyPods[podCliqueName].Delete(pod.Name)
		return
	}

	readyCondition, ok := lo.Find(pod.Status.Conditions, func(podCondition corev1.PodCondition) bool {
		return podCondition.Type == corev1.PodReady
	})
	podReady := ok && readyCondition.Status == corev1.ConditionTrue

	if podReady {
		c.currentlyReadyPods[podCliqueName].Insert(pod.Name)
	} else {
		c.currentlyReadyPods[podCliqueName].Delete(pod.Name)
	}
}

func (c *PodCliqueState) checkAllParentsReady() bool {
	for cliqueName, readyPods := range c.currentlyReadyPods {
		if len(readyPods) < c.podCliqueInfo[cliqueName] {
			return false // If any single parent is not ready, wait.
		}
	}
	return true
}

func getLabelSelectorForPods(podGangName string) map[string]string {
	return lo.Assign(
		map[string]string{
			grovecorev1alpha1.LabelPodGang: podGangName,
		},
	)
}
