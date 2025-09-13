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

	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	grovectrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

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
		For(&grovecorev1alpha1.PodClique{},
			builder.WithPredicates(
				predicate.And(
					predicate.GenerationChangedPredicate{},
					managedPodCliquePredicate(),
				),
			),
		).
		Owns(&corev1.Pod{}, builder.WithPredicates(podPredicate())).
		Watches(
			&grovecorev1alpha1.PodCliqueSet{},
			handler.EnqueueRequestsFromMapFunc(mapPodGangSetToPCLQs()),
			builder.WithPredicates(podGangSetPredicate()),
		).
		Watches(
			&grovecorev1alpha1.PodCliqueScalingGroup{},
			handler.EnqueueRequestsFromMapFunc(mapPodCliqueScalingGroupToPCLQs()),
			builder.WithPredicates(podCliqueScalingGroupPredicate()),
		).
		Watches(
			&groveschedulerv1alpha1.PodGang{},
			handler.EnqueueRequestsFromMapFunc(mapPodGangToPCLQs()),
			builder.WithPredicates(podGangPredicate()),
		).
		Complete(r)
}

func managedPodCliquePredicate() predicate.Predicate {
	expectedOwnerKinds := []string{constants.KindPodCliqueScalingGroup, constants.KindPodCliqueSet}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return grovectrlutils.IsManagedPodClique(e.Object, expectedOwnerKinds...)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return grovectrlutils.IsManagedPodClique(e.Object, expectedOwnerKinds...)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return grovectrlutils.IsManagedPodClique(e.ObjectOld, expectedOwnerKinds...)
		},
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
	return hasReadyConditionChanged(oldPod.Status.Conditions, newPod.Status.Conditions) ||
		hasLastTerminationStateChanged(oldPod.Status.InitContainerStatuses, newPod.Status.InitContainerStatuses) ||
		hasLastTerminationStateChanged(oldPod.Status.ContainerStatuses, newPod.Status.ContainerStatuses) ||
		hasStartedAndReadyChangedForAnyContainer(oldPod.Status.ContainerStatuses, newPod.Status.ContainerStatuses)
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
	return oldPodReady != newPodReady
}

func hasLastTerminationStateChanged(oldContainerStatuses []corev1.ContainerStatus, newContainerStatuses []corev1.ContainerStatus) bool {
	oldErroneousContainerStatus := k8sutils.GetContainerStatusIfTerminatedErroneously(oldContainerStatuses)
	newErroneousContainerStatus := k8sutils.GetContainerStatusIfTerminatedErroneously(newContainerStatuses)
	return utils.OnlyOneIsNil(oldErroneousContainerStatus, newErroneousContainerStatus)
}

func hasStartedAndReadyChangedForAnyContainer(oldContainerStatuses []corev1.ContainerStatus, newContainerStatuses []corev1.ContainerStatus) bool {
	for _, oldContainerStatus := range oldContainerStatuses {
		matchingNewContainerStatus, ok := lo.Find(newContainerStatuses, func(containerStatus corev1.ContainerStatus) bool {
			return oldContainerStatus.Name == containerStatus.Name
		})
		if !ok {
			return true
		}
		if matchingNewContainerStatus.Ready != oldContainerStatus.Ready ||
			matchingNewContainerStatus.Started != oldContainerStatus.Started {
			return true
		}
	}
	return false
}

// mapPodGangSetToPCLQs maps a PodCliqueSet to one or more reconcile.Request(s) to its constituent standalone Podcliques.
// These events are needed to keep the PodClique.Status.CurrentPodCliqueSetGenerationHash in sync with the PodCliqueSet.
func mapPodGangSetToPCLQs() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		pgs, ok := obj.(*grovecorev1alpha1.PodCliqueSet)
		if !ok {
			return nil
		}
		return lo.Map(componentutils.GetPodCliqueFQNsForPGSNotInPCSG(pgs), func(pclqFQN string, _ int) reconcile.Request {
			return reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pgs.Namespace,
				Name:      pclqFQN,
			}}
		})
	}
}

func podGangSetPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		DeleteFunc: func(_ event.DeleteEvent) bool { return false },
		UpdateFunc: func(event event.UpdateEvent) bool {
			oldPGS, okOld := event.ObjectOld.(*grovecorev1alpha1.PodCliqueSet)
			newPGS, okNew := event.ObjectNew.(*grovecorev1alpha1.PodCliqueSet)
			if !okOld || !okNew {
				return false
			}
			return oldPGS.Status.CurrentGenerationHash != newPGS.Status.CurrentGenerationHash
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// mapPodCliqueScalingGroupToPCLQs maps a PodCliqueScalingGroup to one or more reconcile.Request(s) to its constituent PodCliques.
// These events are needed to keep the PodClique.Status.CurrentPodCliqueSetGenerationHash in sync with the PodCliqueSet.
func mapPodCliqueScalingGroupToPCLQs() handler.MapFunc {
	return func(_ context.Context, obj client.Object) []reconcile.Request {
		pcsg, ok := obj.(*grovecorev1alpha1.PodCliqueScalingGroup)
		if !ok {
			return nil
		}
		return lo.Map(componentutils.GetPodCliqueFQNsForPCSG(pcsg), func(pclqFQN string, _ int) reconcile.Request {
			return reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pcsg.Namespace,
				Name:      pclqFQN,
			}}
		})
	}
}

func podCliqueScalingGroupPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool { return false },
		DeleteFunc: func(_ event.DeleteEvent) bool { return false },
		UpdateFunc: func(event event.UpdateEvent) bool {
			oldPCSG, okOld := event.ObjectOld.(*grovecorev1alpha1.PodCliqueScalingGroup)
			newPCSG, okNew := event.ObjectNew.(*grovecorev1alpha1.PodCliqueScalingGroup)
			if !okOld || !okNew {
				return false
			}
			return oldPCSG.Status.CurrentPodCliqueSetGenerationHash != nil && newPCSG.Status.RollingUpdateProgress != nil &&
				*oldPCSG.Status.CurrentPodCliqueSetGenerationHash != newPCSG.Status.RollingUpdateProgress.PodCliqueSetGenerationHash
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
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
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		UpdateFunc:  func(_ event.UpdateEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

func isManagedPod(obj client.Object) bool {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false
	}
	return grovectrlutils.HasExpectedOwner(constants.KindPodClique, pod.OwnerReferences) && grovectrlutils.IsManagedByGrove(pod.GetLabels())
}
