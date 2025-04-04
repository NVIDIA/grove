// /*
// Copyright 2024 The Grove Authors.
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

package main

import (
	"context"
	"flag"
	"os"
	"strings"
	"time"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/client/clientset/versioned"
	"github.com/NVIDIA/grove/operator/client/informers/externalversions"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func main() {
	var namespace, after string
	flag.StringVar(&namespace, "namespace", "default", "namespace")
	flag.StringVar(&after, "after", "", "start after podcliques")

	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()

	if err := mainInternal(namespace, after); err != nil {
		klog.Error(err.Error())
		os.Exit(1)
	}
}

func mainInternal(namespace, after string) error {
	pclqs := make(map[string]struct{})
	for _, name := range strings.Split(after, ",") {
		if len(name) != 0 {
			klog.Infof("waiting for pclq %s", name)
			pclqs[name] = struct{}{}
		}
	}
	if len(pclqs) == 0 {
		return nil
	}
	klog.Infof("waiting for %d pclqs", len(pclqs))

	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return err
	}

	completed := make(chan string)

	ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Minute)
	defer cancel()

	factory := externalversions.NewSharedInformerFactoryWithOptions(clientset, 0, externalversions.WithNamespace(namespace))

	informer := factory.Grove().V1alpha1().PodCliques().Informer()

	// Define the event handler
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pclq := obj.(*v1alpha1.PodClique)
			klog.InfoS("pclq added", "name", pclq.Name, "replicas", pclq.Status.Replicas, "ready", pclq.Status.ReadyReplicas)
			if pclq.Status.Replicas == pclq.Status.ReadyReplicas {
				klog.Info("mark as completed")
				completed <- pclq.Name
			}
		},
		DeleteFunc: func(obj interface{}) {
			pclq := obj.(*v1alpha1.PodClique)
			klog.InfoS("pclq deleted", "name", pclq.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pclqNew := newObj.(*v1alpha1.PodClique)
			pclqOld := oldObj.(*v1alpha1.PodClique)
			if pclqNew.ResourceVersion != pclqOld.ResourceVersion {
				klog.InfoS("pclq updated", "name", pclqNew.Name, "replicas", pclqNew.Status.Replicas, "ready", pclqNew.Status.ReadyReplicas)
				if pclqNew.Status.Replicas == pclqNew.Status.ReadyReplicas {
					klog.Info("mark as completed")
					completed <- pclqNew.Name
				}
			}
		},
	})

	// Start the informer
	stop := make(chan struct{})
	defer close(stop)
	factory.Start(stop)
	klog.Info("started informer")

	// Wait until the cache is synced
	//if !cache.WaitForCacheSync(stop, informer.HasSynced) {
	//	return fmt.Errorf("Timed out waiting for caches to sync")
	//}

	for {
		select {
		case name := <-completed:
			klog.Infof("pclq %s completed", name)
			delete(pclqs, name)
			if len(pclqs) == 0 {
				klog.Info("all pclqs completed")
				return nil
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
