package podclique

import (
	"context"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
)

// Reconciler reconciles PodClique objects.
type Reconciler struct {
	config        configv1alpha1.PodCliqueControllerConfiguration
	client        ctrlclient.Client
	eventRecorder record.EventRecorder
}

// NewReconciler creates a new instance of the PodClique Reconciler.
func NewReconciler(mgr ctrl.Manager, controllerCfg configv1alpha1.PodCliqueControllerConfiguration) *Reconciler {
	return &Reconciler{
		config:        controllerCfg,
		client:        mgr.GetClient(),
		eventRecorder: mgr.GetEventRecorderFor(controllerName),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllogger.FromContext(ctx).
		WithName(controllerName).
		WithValues("pgs-name", req.Name, "pgs-namespace", req.Namespace)

	pclq := &v1alpha1.PodClique{}
	if result := ctrlutils.GetPodClique(ctx, r.client, logger, req.NamespacedName, pclq); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	if result := r.reconcileDelete(ctx, logger, pclq); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	return r.reconcileSpec(ctx, logger, pclq).Result()
}

func (r *Reconciler) reconcileDelete(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique) ctrlcommon.ReconcileStepResult {
	if !pclq.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(pclq, v1alpha1.FinalizerPodGangSet) {
			return ctrlcommon.DoNotRequeue()
		}
		dLog := logger.WithValues("operation", "delete")
		return r.triggerDeletionFlow(ctx, dLog, pclq)
	}
	return ctrlcommon.ContinueReconcile()
}

//func (r *Reconciler) reconcile(ctx context.Context, pclq *v1alpha1.PodClique) (ctrl.Result, error) {
//	for replicaID := range pclq.Spec.Replicas {
//		pod := &corev1.Pod{
//			ObjectMeta: metav1.ObjectMeta{
//				Name:        fmt.Sprintf("%s-%d", pclq.Name, replicaID),
//				Namespace:   pclq.Namespace,
//				Labels:      pclq.Labels,
//				Annotations: pclq.Annotations,
//			},
//			Spec: pclq.Spec.PodSpec,
//		}
//		// Set the owner reference and finalizers
//		controllerutil.SetControllerReference(pclq, pod, r.client.Scheme())
//
//		// Check if the PodClique already exists
//		found := &corev1.Pod{}
//		err := r.client.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
//		if err != nil {
//			if client.IgnoreNotFound(err) == nil {
//				r.logger.Info("Creating Pod", "name", pod.Name, "namespace", pod.Namespace)
//				if err := r.client.Create(ctx, pod); err != nil {
//					return ctrl.Result{}, err
//				}
//			} else {
//				return ctrl.Result{}, err
//			}
//		} else {
//			// If the PodClique exists, ensure it matches the desired state
//			r.logger.Info("Updating Pod", "name", pod.Name, "namespace", pod.Namespace)
//			if err := r.client.Update(ctx, found); err != nil {
//				return ctrl.Result{}, err
//			}
//		}
//
//	}
//
//	// TODO: in case an error in the internal loop, do we need to roll back already created objects?
//	return ctrl.Result{}, nil
//}
