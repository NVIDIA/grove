package podclique

import (
	"context"
	"fmt"
	"time"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
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

	pc := &v1alpha1.PodClique{}
	if err := r.client.Get(ctx, req.NamespacedName, pc); err != nil {
		if errors.IsNotFound(err) {
			r.logger.V(1).Info("Object not found, stop reconciling")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "Failed to get PodClique; requeuing")
		// TODO: do we need to requeue?
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	if !pc.DeletionTimestamp.IsZero() {
		return r.delete(ctx, pc)
	}

	return r.reconcile(ctx, pc)
}

func (r *Reconciler) reconcile(ctx context.Context, pc *v1alpha1.PodClique) (ctrl.Result, error) {
	// TODO: implement
	return ctrl.Result{}, fmt.Errorf("not implemented")
}

func (r *Reconciler) delete(ctx context.Context, pc *v1alpha1.PodClique) (ctrl.Result, error) {
	// TODO: implement
	return ctrl.Result{}, fmt.Errorf("not implemented")
}
