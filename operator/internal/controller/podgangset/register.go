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

package podgangset

import (
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	grovectrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
)

const (
	controllerName = "podgangset-controller"
	finalizerName  = "podgangset.grove.io"
)

// RegisterWithManager registers the PodGangSet Reconciler with the manager.
func (r *Reconciler) RegisterWithManager(mgr manager.Manager) error {
	r.logger.Info("Registering reconciler", "controllerName", controllerName)
	return builder.ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.config.ConcurrentSyncs,
		}).
		For(&v1alpha1.PodGangSet{}).
		Owns(&v1alpha1.PodClique{}, builder.WithPredicates(podCliquePredicate())).
		Complete(r)
}

// podCliquesPredicate returns a predicate that filters out PodClique resources that are not managed by Grove.
func podCliquePredicate() predicate.Predicate {
	isManagedClique := func(obj client.Object) bool {
		podClique, ok := obj.(*v1alpha1.PodClique)
		if !ok {
			return false
		}
		return grovectrlutils.IsManagedByGrove(podClique.GetLabels())
	}
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return isManagedClique(deleteEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			// Only allow update event if the PodClique is managed by Grove and the spec has not changed.
			return isManagedClique(updateEvent.ObjectNew)
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
