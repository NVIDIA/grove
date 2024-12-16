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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
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
	r.logger.Info("PodGangSet reconciliation started", "resource", req.NamespacedName)

	pgs := &v1alpha1.PodGangSet{}
	if err := r.client.Get(ctx, req.NamespacedName, pgs); err != nil {
		if errors.IsNotFound(err) {
			r.logger.V(1).Info("Object not found, stop reconciling")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "Failed to get PodGangSet; requeuing")
		// TODO: do we need to requeue?
		return ctrl.Result{RequeueAfter: 10 * time.Second}, err
	}

	if !pgs.DeletionTimestamp.IsZero() {
		return r.delete(ctx, pgs)
	}

	return r.reconcile(ctx, pgs)
}

func (r *Reconciler) reconcile(ctx context.Context, pgs *v1alpha1.PodGangSet) (ctrl.Result, error) {
	// TODO: implement
	return ctrl.Result{}, fmt.Errorf("not implemented")
}

func (r *Reconciler) delete(ctx context.Context, pgs *v1alpha1.PodGangSet) (ctrl.Result, error) {
	// TODO: implement
	return ctrl.Result{}, fmt.Errorf("not implemented")
}
