/*
Copyright 2024 The Grove Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	podgangsetv1alpha1 "github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
	versioned "github.com/NVIDIA/grove/operator/client/clientset/versioned"
	internalinterfaces "github.com/NVIDIA/grove/operator/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/NVIDIA/grove/operator/client/listers/podgangset/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// PodGangInformer provides access to a shared informer and lister for
// PodGangs.
type PodGangInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.PodGangLister
}

type podGangInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewPodGangInformer constructs a new informer for PodGang type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewPodGangInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredPodGangInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredPodGangInformer constructs a new informer for PodGang type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredPodGangInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1alpha1().PodGangs(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1alpha1().PodGangs(namespace).Watch(context.TODO(), options)
			},
		},
		&podgangsetv1alpha1.PodGang{},
		resyncPeriod,
		indexers,
	)
}

func (f *podGangInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredPodGangInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *podGangInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&podgangsetv1alpha1.PodGang{}, f.defaultInformer)
}

func (f *podGangInformer) Lister() v1alpha1.PodGangLister {
	return v1alpha1.NewPodGangLister(f.Informer().GetIndexer())
}
