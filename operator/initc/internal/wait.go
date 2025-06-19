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
	"fmt"
	"slices"
	"sync"
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	groveclientset "github.com/NVIDIA/grove/operator/client/clientset/versioned"
	groveinformers "github.com/NVIDIA/grove/operator/client/informers/externalversions"
	"github.com/go-logr/logr"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// CliqueState contains the last known (readiness) state of all parent PodCliques.
type CliqueState struct {
	log         logr.Logger
	mutex       *sync.Mutex
	namespace   string
	cliqueFQNs  []string
	parentReady map[string]bool
	readyCh     chan bool
}

// NewCliqueState creates and initializes all parent PodCliques with an unready state.
func NewCliqueState(cliqueFQNs []string, cliqueNamespace string, log logr.Logger) CliqueState {
	state := CliqueState{
		log:         log,
		mutex:       &sync.Mutex{},
		namespace:   cliqueNamespace,
		cliqueFQNs:  cliqueFQNs,
		parentReady: make(map[string]bool),
		readyCh:     make(chan bool, len(cliqueFQNs)),
	}
	for _, cliqueFQN := range cliqueFQNs {
		state.parentReady[cliqueFQN] = false
	}
	return state
}

func (c *CliqueState) WaitForReady(ctx context.Context) error {
	c.log.Info("The clique names are:", "cliques", c.cliqueFQNs)
	c.log.Info("The clique namespace is:", "cliquesNamespace", c.namespace)

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to fetch the in cluster config with error %w", err)
	}
	clientSet, err := groveclientset.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientSet with the fetched restConfig with error %w", err)
	}

	// the readyCh channel is written to in two cases:
	// 1. When an unready clique becomes ready: true
	// 2. When a ready clique becomes unready: false
	var wg sync.WaitGroup
	wg.Add(len(c.cliqueFQNs))
	go func() {
		for ready := range c.readyCh {
			if !ready {
				wg.Add(1) // a ready PodClique has become unready
			} else {
				wg.Done() // an unready PodClique has become ready
			}
		}
	}()

	eventHandlerContext, cancel := context.WithCancel(ctx)

	defer close(c.readyCh) // close the channel the informers write to after the context they use is cancelled
	defer cancel()         // cancel the context used by the informers if the wait is successful, or an err occurs

	if err = c.startPodCliqueEventHandler(eventHandlerContext, clientSet); err != nil {
		return fmt.Errorf("Unable to start PodClique event handler with error %w", err)
	}

	wg.Wait()
	return nil
}

func (c *CliqueState) startPodCliqueEventHandler(ctx context.Context, clientSet *groveclientset.Clientset) error {
	factory := groveinformers.NewSharedInformerFactoryWithOptions(clientSet, time.Second, groveinformers.WithNamespace(c.namespace))
	typedInformer := factory.Grove().V1alpha1().PodCliques().Informer()
	_, err := typedInformer.AddEventHandlerWithOptions(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			pclq, ok := obj.(*grovecorev1alpha1.PodClique)
			if !ok {
				return
			}
			if !slices.Contains(c.cliqueFQNs, pclq.Name) {
				// pclq is not a parent
				return
			}
			c.verifyParentReadiness(pclq)
		},
		UpdateFunc: func(_, obj any) {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			pclq, ok := obj.(*grovecorev1alpha1.PodClique)
			if !ok {
				return
			}
			if !slices.Contains(c.cliqueFQNs, pclq.Name) {
				// pclq is not a parent
				return
			}
			c.verifyParentReadiness(pclq)
		},
		DeleteFunc: func(_ any) {
			// the delete function should not be entered, since PodCliques are immutable in a PodGangSet.
			c.log.Error(
				fmt.Errorf("PodCliques were deleted"),
				"PodCliques which are immutable in a PodGangSet were deleted while the initcontainer was waiting for startup",
			)
			return
		},
	}, cache.HandlerOptions{Logger: &c.log})
	if err != nil {
		return fmt.Errorf("failed to add the event handler to the informer")
	}

	synced := factory.WaitForCacheSync(ctx.Done())
	for v, ok := range synced {
		if !ok {
			c.log.Error(fmt.Errorf("failed to sync informer cache"), "Caches failed to sync", "type", v)
		}
	}

	factory.Start(ctx.Done())
	return nil
}

func (c *CliqueState) verifyParentReadiness(pclq *grovecorev1alpha1.PodClique) {
	c.log.Info("Parent PodClique:",
		"PodClique.Name", pclq.Name,
		"PodClique.Spec.Replicas", pclq.Spec.Replicas,
		"PodClique.Status.ReadyReplicas", pclq.Status.ReadyReplicas,
	)
	// check if the PodClique was already ready
	if c.parentReady[pclq.Name] {
		if pclq.Spec.Replicas != pclq.Status.ReadyReplicas {
			c.parentReady[pclq.Name] = false
			c.readyCh <- false
			c.log.Info("The parent PodClique that was previously ready is not ready any more", "PodClique", pclq.Name)
		}
		c.log.Info("The parent PodClique was already ready", "PodClique", pclq.Name)
		return
	}
	// The PodClique was not ready previously, check if it is ready now
	if pclq.Spec.Replicas != pclq.Status.ReadyReplicas {
		c.log.Info("The parent PodClique is not ready", "PodClique", pclq.Name)
	} else {
		c.parentReady[pclq.Name] = true
		c.readyCh <- true
		c.log.Info("The parent PodClique has become ready", "PodClique", pclq.Name)
	}
}
