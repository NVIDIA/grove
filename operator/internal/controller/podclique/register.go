package podclique

import (
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	grovectrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	controllerName = "podclique-controller"
	finalizerName  = "podclique.grove.io"
)

func (r *Reconciler) RegisterWithManager(mgr ctrl.Manager) error {
	return builder.ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: *r.config.ConcurrentSyncs,
		}).
		For(&v1alpha1.PodClique{}).
		WithEventFilter(podCliquePredicate()).
		Owns(&corev1.Pod{}, builder.WithPredicates(podPredicate())).
		Complete(r)
}

// podCliquesPredicate returns a predicate that filters out PodClique resources that are not managed by Grove.
func podCliquePredicate() predicate.Predicate {
	isManagedClique := func(obj client.Object) bool {
		podClique, ok := obj.(*v1alpha1.PodClique)
		if !ok {
			return false
		}
		return grovectrlutils.HasExpectedOwner(v1alpha1.PodGangSetKind, podClique.OwnerReferences) && grovectrlutils.IsManagedByGrove(podClique.GetLabels())
	}
	return predicate.Funcs{
		CreateFunc: func(createEvent event.CreateEvent) bool {
			return isManagedClique(createEvent.Object)
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return isManagedClique(deleteEvent.Object)
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return isManagedClique(updateEvent.ObjectNew)
		},
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
		UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}
