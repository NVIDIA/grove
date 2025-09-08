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
	"context"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	groveconfigv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	pcsgcomponent "github.com/NVIDIA/grove/operator/internal/component/podcliquescalinggroup"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler reconciles PodCliqueScalingGroup objects.
type Reconciler struct {
	config                  groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration
	client                  client.Client
	reconcileStatusRecorder ctrlcommon.ReconcileStatusRecorder
	operatorRegistry        component.OperatorRegistry[grovecorev1alpha1.PodCliqueScalingGroup]
}

// NewReconciler creates a new instance of the PodClique Reconciler.
func NewReconciler(mgr ctrl.Manager, controllerCfg groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration) *Reconciler {
	eventRecorder := mgr.GetEventRecorderFor(controllerName)
	return &Reconciler{
		config:                  controllerCfg,
		client:                  mgr.GetClient(),
		reconcileStatusRecorder: ctrlcommon.NewReconcileStatusRecorder(mgr.GetClient(), eventRecorder),
		operatorRegistry:        pcsgcomponent.CreateOperatorRegistry(mgr, eventRecorder),
	}
}

// Reconcile reconciles a PodCliqueScalingGroup resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllogger.FromContext(ctx).WithName(controllerName)

	pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{}
	if result := ctrlutils.GetPodCliqueScalingGroup(ctx, r.client, logger, req.NamespacedName, pcsg); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	// Check if the deletion timestamp has not been set, do not handle if it is
	var deletionOrSpecReconcileFlowResult ctrlcommon.ReconcileStepResult
	if !pcsg.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(pcsg, constants.FinalizerPodCliqueScalingGroup) {
			return ctrlcommon.DoNotRequeue().Result()
		}
		dLog := logger.WithValues("operation", "delete")
		deletionOrSpecReconcileFlowResult = r.triggerDeletionFlow(ctx, dLog, pcsg)
	} else {
		specLog := logger.WithValues("operation", "specReconcile")
		deletionOrSpecReconcileFlowResult = r.reconcileSpec(ctx, specLog, pcsg)
	}

	if statusReconcileResult := r.reconcileStatus(ctx, logger, client.ObjectKeyFromObject(pcsg)); ctrlcommon.ShortCircuitReconcileFlow(statusReconcileResult) {
		return statusReconcileResult.Result()
	}

	if ctrlcommon.ShortCircuitReconcileFlow(deletionOrSpecReconcileFlowResult) {
		return deletionOrSpecReconcileFlowResult.Result()
	}

	return ctrlcommon.DoNotRequeue().Result()
}
