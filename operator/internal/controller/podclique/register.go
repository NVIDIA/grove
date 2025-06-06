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

package podclique

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	grovectrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	controllerName = "podclique-controller"
	finalizerName  = "podclique.grove.io"
)

// RegisterWithManager registers the PodClique controller with the given controller manager.
func (r *Reconciler) RegisterWithManager(mgr ctrl.Manager) error {
	return builder.ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.config.ConcurrentSyncs,
		}).
		For(&v1alpha1.PodClique{}, builder.WithPredicates(managedPodCliquePredicate(), predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Pod{}, builder.WithPredicates(podPredicate())).
		Complete(r)
}

func managedPodCliquePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return grovectrlutils.IsManagedPodClique(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return grovectrlutils.IsManagedPodClique(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return grovectrlutils.IsManagedPodClique(e.ObjectOld) },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// podPredicate returns a predicate that filters out pods that are not managed by Grove.
func podPredicate() predicate.Predicate {
	isManagedPod := func(pod *corev1.Pod) bool {
		return grovectrlutils.HasExpectedOwner(v1alpha1.PodCliqueKind, pod.OwnerReferences) && grovectrlutils.IsManagedByGrove(pod.GetLabels())
	}
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			deletedPod, ok := deleteEvent.Object.(*corev1.Pod)
			if !ok {
				return false
			}
			return isManagedPod(deletedPod)
		},
		/*
			Allow update events where the pod status has changed.
		*/
		UpdateFunc:  func(_ event.UpdateEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
