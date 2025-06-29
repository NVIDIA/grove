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
	"context"
	"strings"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	grovectrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	controllerName = "podclique-controller"
)

// RegisterWithManager registers the PodClique controller with the given controller manager.
func (r *Reconciler) RegisterWithManager(mgr ctrl.Manager) error {
	return builder.ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.config.ConcurrentSyncs,
		}).
		For(&v1alpha1.PodClique{},
			builder.WithPredicates(
				predicate.And(
					predicate.GenerationChangedPredicate{},
					managedPodCliquePredicate(),
				),
			),
		).
		Owns(&corev1.Pod{}, builder.WithPredicates(podPredicate())).
		Watches(
			&groveschedulerv1alpha1.PodGang{},
			handler.EnqueueRequestsFromMapFunc(mapPodGangToPCLQs()),
			builder.WithPredicates(podGangPredicate()),
		).
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
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			deletedPod, ok := deleteEvent.Object.(*corev1.Pod)
			if !ok {
				return false
			}
			return isManagedPod(deletedPod)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return isManagedPod(updateEvent.ObjectOld) && !hasPodSpecChanged(updateEvent) && hasPodStatusChanged(updateEvent)
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func hasPodSpecChanged(updateEvent event.UpdateEvent) bool {
	return updateEvent.ObjectOld.GetGeneration() != updateEvent.ObjectNew.GetGeneration()
}

func hasPodStatusChanged(updateEvent event.UpdateEvent) bool {
	oldPod, oldOk := updateEvent.ObjectOld.(*corev1.Pod)
	newPod, newOk := updateEvent.ObjectNew.(*corev1.Pod)
	if !oldOk || !newOk {
		return false
	}
	return hasReadyConditionChanged(oldPod.Status.Conditions, newPod.Status.Conditions)
}

func hasReadyConditionChanged(oldPodConditions, newPodConditions []corev1.PodCondition) bool {
	getReadyCondition := func(podConditions []corev1.PodCondition) (corev1.PodCondition, bool) {
		return lo.Find(podConditions, func(condition corev1.PodCondition) bool {
			return condition.Type == corev1.PodReady
		})
	}
	oldPodReadyCondition, oldOk := getReadyCondition(oldPodConditions)
	newPodReadyCondition, newOk := getReadyCondition(newPodConditions)
	oldPodReady := oldOk && oldPodReadyCondition.Status == corev1.ConditionTrue
	newPodReady := newOk && newPodReadyCondition.Status == corev1.ConditionTrue
	return !oldPodReady && newPodReady
}

// mapPodGangToPCLQs maps a PodGang to one or more reconcile.Request(s) for its constituent PodClique's.
func mapPodGangToPCLQs() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		podGang, ok := obj.(*groveschedulerv1alpha1.PodGang)
		if !ok {
			return nil
		}
		requests := make([]reconcile.Request, 0, len(podGang.Spec.PodGroups))
		for _, podGroup := range podGang.Spec.PodGroups {
			if len(podGroup.PodReferences) == 0 {
				continue
			}
			podRefName := podGroup.PodReferences[0].Name
			pclqFQN := extractPCLQNameFromPodName(podRefName)
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: pclqFQN, Namespace: podGang.Namespace},
			})
		}
		return requests
	}
}

func extractPCLQNameFromPodName(podName string) string {
	endIndex := strings.LastIndex(podName, "-")
	return podName[:endIndex]
}

func podGangPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		UpdateFunc:  func(_ event.UpdateEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func isManagedPod(obj client.Object) bool {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false
	}
	return grovectrlutils.HasExpectedOwner(v1alpha1.PodCliqueKind, pod.OwnerReferences) && grovectrlutils.IsManagedByGrove(pod.GetLabels())
}
