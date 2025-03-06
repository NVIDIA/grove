package podclique

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/utils"
)

// Reconciler reconciles PodClique objects.
type Reconciler struct {
	config        configv1alpha1.PodCliqueControllerConfiguration
	client        ctrlclient.Client
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

// NewReconciler creates a new instance of the PodClique Reconciler.
func NewReconciler(mgr ctrl.Manager, controllerCfg configv1alpha1.PodCliqueControllerConfiguration) *Reconciler {
	logger := ctrllogger.Log.WithName(controllerName)
	return &Reconciler{
		config:        controllerCfg,
		client:        mgr.GetClient(),
		eventRecorder: mgr.GetEventRecorderFor(controllerName),
		logger:        logger,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger.Info("PodClique reconciliation started", "resource", req.NamespacedName)

	pclq := &v1alpha1.PodClique{}
	if err := r.client.Get(ctx, req.NamespacedName, pclq); err != nil {
		r.logger.Error(err, "PodClique not found", "name", req.NamespacedName.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer if not being deleted and finalizer is missing
	if pclq.ObjectMeta.DeletionTimestamp.IsZero() {
		if !utils.ContainsString(pclq.ObjectMeta.Finalizers, finalizerName) {
			pclq.ObjectMeta.Finalizers = append(pclq.ObjectMeta.Finalizers, finalizerName)
		}
		return r.reconcile(ctx, pclq)
	} else {
		// Handle deletion if finalizer is present
		if utils.ContainsString(pclq.ObjectMeta.Finalizers, finalizerName) {
			// Remove finalizer and update object
			pclq.ObjectMeta.Finalizers = utils.RemoveString(pclq.ObjectMeta.Finalizers, finalizerName)
		}
		return r.delete(ctx, pclq)
	}
}

func (r *Reconciler) reconcile(ctx context.Context, pclq *v1alpha1.PodClique) (ctrl.Result, error) {
	for replicaID := range pclq.Spec.Replicas {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        fmt.Sprintf("%s-%d", pclq.Name, replicaID),
				Namespace:   pclq.Namespace,
				Labels:      pclq.Labels,
				Annotations: pclq.Annotations,
			},
			Spec: pclq.Spec.PodSpec,
		}
		// Set the owner reference and finalizers
		controllerutil.SetControllerReference(pclq, pod, r.client.Scheme())

		// Check if the PodClique already exists
		found := &corev1.Pod{}
		err := r.client.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				r.logger.Info("Creating Pod", "name", pod.Name, "namespace", pod.Namespace)
				if err := r.client.Create(ctx, pod); err != nil {
					return ctrl.Result{}, err
				}
			} else {
				return ctrl.Result{}, err
			}
		} else {
			// If the PodClique exists, ensure it matches the desired state
			r.logger.Info("Updating Pod", "name", pod.Name, "namespace", pod.Namespace)
			if err := r.client.Update(ctx, found); err != nil {
				return ctrl.Result{}, err
			}
		}

	}

	// TODO: in case an error in the internal loop, do we need to roll back already created objects?
	return ctrl.Result{}, nil
}

func (r *Reconciler) delete(ctx context.Context, pclq *v1alpha1.PodClique) (ctrl.Result, error) {
	// TODO: implement
	r.logger.Info("Deleting PodClique", "name", pclq.Name, "namespace", pclq.Namespace)
	return ctrl.Result{}, fmt.Errorf("not implemented")
}
