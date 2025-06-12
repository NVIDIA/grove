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

package podcliquescalinggroup

import (
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	controllerName = "podcliquescalingroup-controller"
)

// RegisterWithManager registers the PodCliqueScalingGroup Reconciler with the manager.
// This reconciler will only be called when the PodCliqueScalingGroup resource is updated. The resource can either be
// updated by an HPA or an equivalent external component.
func (r *Reconciler) RegisterWithManager(mgr manager.Manager) error {
	return builder.ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.config.ConcurrentSyncs,
		}).
		For(&grovecorev1alpha1.PodCliqueScalingGroup{}).
		WithEventFilter(
			predicate.And(
				predicate.GenerationChangedPredicate{},
				podCliqueScalingGroupUpdatePredicate(),
			)).
		Complete(r)
}

func podCliqueScalingGroupUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return ctrlutils.IsManagedByGrove(createEvent.Object.GetLabels()) &&
				ctrlutils.HasExpectedOwner(grovecorev1alpha1.PodGangSetKind, createEvent.Object.GetOwnerReferences())
		},
		DeleteFunc: func(_ event.DeleteEvent) bool { return false },
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return ctrlutils.IsManagedByGrove(updateEvent.ObjectOld.GetLabels()) &&
				ctrlutils.HasExpectedOwner(grovecorev1alpha1.PodGangSetKind, updateEvent.ObjectOld.GetOwnerReferences())
		},
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
