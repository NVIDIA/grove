package podgangset

import (
	"context"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler reconciles PodGangSet resources.
type Reconciler struct {
	config        configv1alpha1.PodGangSetControllerConfiguration
	client        ctrlclient.Client
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

// NewReconciler creates a new reconciler for PodGangSet.
func NewReconciler(mgr ctrl.Manager, controllerCfg configv1alpha1.PodGangSetControllerConfiguration) *Reconciler {
	logger := ctrllogger.Log.WithName(controllerName)
	return &Reconciler{
		config:        controllerCfg,
		client:        mgr.GetClient(),
		eventRecorder: mgr.GetEventRecorderFor(controllerName),
		logger:        logger,
	}
}

// Reconcile reconciles a PodGangSet resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	panic("implement me")
}
