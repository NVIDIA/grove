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

package podcliqueset

import (
	"context"
	"sync"

	"github.com/NVIDIA/grove/operator/api/common/constants"
	configv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	"github.com/NVIDIA/grove/operator/internal/controller/common/component"
	pcscomponent "github.com/NVIDIA/grove/operator/internal/controller/podcliqueset/components"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler reconciles PodCliqueSet resources.
type Reconciler struct {
	config                        configv1alpha1.PodCliqueSetControllerConfiguration
	client                        ctrlclient.Client
	reconcileStatusRecorder       ctrlcommon.ReconcileErrorRecorder
	operatorRegistry              component.OperatorRegistry[grovecorev1alpha1.PodCliqueSet]
	pcsGenerationHashExpectations sync.Map
}

// NewReconciler creates a new reconciler for PodCliqueSet.
func NewReconciler(mgr ctrl.Manager, controllerCfg configv1alpha1.PodCliqueSetControllerConfiguration) *Reconciler {
	eventRecorder := mgr.GetEventRecorderFor(controllerName)
	client := mgr.GetClient()
	return &Reconciler{
		config:                        controllerCfg,
		client:                        client,
		reconcileStatusRecorder:       ctrlcommon.NewReconcileErrorRecorder(client),
		operatorRegistry:              pcscomponent.CreateOperatorRegistry(mgr, eventRecorder),
		pcsGenerationHashExpectations: sync.Map{},
	}
}

// Reconcile reconciles a PodCliqueSet resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllogger.FromContext(ctx).WithName(controllerName)

	pcs := &grovecorev1alpha1.PodCliqueSet{}
	if result := ctrlutils.GetPodCliqueSet(ctx, r.client, logger, req.NamespacedName, pcs); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	if result := r.reconcileDelete(ctx, logger, pcs); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	reconcileSpecFlowResult := r.reconcileSpec(ctx, logger, pcs)
	if statusReconcileResult := r.reconcileStatus(ctx, logger, pcs); ctrlcommon.ShortCircuitReconcileFlow(statusReconcileResult) {
		return statusReconcileResult.Result()
	}

	return reconcileSpecFlowResult.Result()
}

func (r *Reconciler) reconcileDelete(ctx context.Context, logger logr.Logger, pcs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	if !pcs.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(pcs, constants.FinalizerPodCliqueSet) {
			return ctrlcommon.DoNotRequeue()
		}
		dLog := logger.WithValues("operation", "delete")
		return r.triggerDeletionFlow(ctx, dLog, pcs)
	}
	return ctrlcommon.ContinueReconcile()
}
