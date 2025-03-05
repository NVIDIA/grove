package podclique

import (
	"context"
	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
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
	return ctrl.Result{}, nil
}
