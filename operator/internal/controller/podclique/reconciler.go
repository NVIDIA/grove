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

	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	pclqcomponent "github.com/NVIDIA/grove/operator/internal/component/podclique"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler reconciles PodClique objects.
type Reconciler struct {
	config                  configv1alpha1.PodCliqueControllerConfiguration
	client                  ctrlclient.Client
	eventRecorder           record.EventRecorder
	reconcileStatusRecorder ctrlcommon.ReconcileStatusRecorder
	operatorRegistry        component.OperatorRegistry[grovecorev1alpha1.PodClique]
}

// NewReconciler creates a new instance of the PodClique Reconciler.
func NewReconciler(mgr ctrl.Manager, controllerCfg configv1alpha1.PodCliqueControllerConfiguration) *Reconciler {
	eventRecorder := mgr.GetEventRecorderFor(controllerName)
	return &Reconciler{
		config:                  controllerCfg,
		client:                  mgr.GetClient(),
		eventRecorder:           eventRecorder,
		reconcileStatusRecorder: ctrlcommon.NewReconcileStatusRecorder(mgr.GetClient(), mgr.GetEventRecorderFor(controllerName)),
		operatorRegistry:        pclqcomponent.CreateOperatorRegistry(mgr, eventRecorder),
	}
}

// Reconcile reconciles the `PodClique` resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllogger.FromContext(ctx).
		WithName(controllerName).
		WithValues("pclq-name", req.Name, "pclq-namespace", req.Namespace)

	pclq := &grovecorev1alpha1.PodClique{}
	if result := ctrlutils.GetPodClique(ctx, r.client, logger, req.NamespacedName, pclq, true); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	if !pclq.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(pclq, grovecorev1alpha1.FinalizerPodClique) {
			return ctrlcommon.DoNotRequeue().Result()
		}
		dLog := logger.WithValues("operation", "delete")
		return r.triggerDeletionFlow(ctx, dLog, pclq).Result()
	}
	specLog := logger.WithValues("operation", "specReconcile")

	reconcileSpecFlowResult := r.reconcileSpec(ctx, specLog, pclq)
	if statusReconcileResult := r.reconcileStatus(ctx, logger, pclq); ctrlcommon.ShortCircuitReconcileFlow(statusReconcileResult) {
		return statusReconcileResult.Result()
	}

	return reconcileSpecFlowResult.Result()
}
