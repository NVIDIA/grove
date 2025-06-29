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
	"context"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	grovectrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	controllerName = "podgangset-controller"
)

// RegisterWithManager registers the PodGangSet Reconciler with the manager.
func (r *Reconciler) RegisterWithManager(mgr manager.Manager) error {
	return builder.ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.config.ConcurrentSyncs,
		}).
		For(&grovecorev1alpha1.PodGangSet{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&grovecorev1alpha1.PodClique{},
			handler.EnqueueRequestsFromMapFunc(mapPCLQToPodGangSet()),
			builder.WithPredicates(podCliquePredicate()),
		).
		Complete(r)
}

func mapPCLQToPodGangSet() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		pclq, ok := obj.(*grovecorev1alpha1.PodClique)
		if !ok {
			return nil
		}
		pgsName := k8sutils.GetFirstOwnerName(pclq.ObjectMeta)
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: pgsName, Namespace: pclq.Namespace}}}
	}
}

// podCliquesPredicate returns a predicate that filters out PodClique resources that are not managed by Grove.
func podCliquePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return grovectrlutils.IsManagedPodClique(deleteEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return grovectrlutils.IsManagedPodClique(updateEvent.ObjectOld) &&
				(hasSpecChanged(updateEvent) || hasStatusChanged(updateEvent))
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func hasSpecChanged(updateEvent event.UpdateEvent) bool {
	return updateEvent.ObjectOld.GetGeneration() != updateEvent.ObjectNew.GetGeneration()
}

func hasStatusChanged(updateEvent event.UpdateEvent) bool {
	oldPCLQ, okOld := updateEvent.ObjectOld.(*grovecorev1alpha1.PodClique)
	newPCLQ, okNew := updateEvent.ObjectNew.(*grovecorev1alpha1.PodClique)
	if !okOld || !okNew {
		return false
	}
	return oldPCLQ.Status.Replicas != newPCLQ.Status.Replicas ||
		oldPCLQ.Status.ReadyReplicas != newPCLQ.Status.ReadyReplicas ||
		oldPCLQ.Status.UpdatedReplicas != newPCLQ.Status.UpdatedReplicas ||
		oldPCLQ.Status.ScheduleGatedReplicas != newPCLQ.Status.ScheduleGatedReplicas
}
