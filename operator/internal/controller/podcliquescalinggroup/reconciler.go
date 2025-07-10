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

	groveconfigv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	pcsgcomponent "github.com/NVIDIA/grove/operator/internal/component/podcliquescalinggroup"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
	return &Reconciler{
		config:                  controllerCfg,
		client:                  mgr.GetClient(),
		reconcileStatusRecorder: ctrlcommon.NewReconcileStatusRecorder(mgr.GetClient(), mgr.GetEventRecorderFor(controllerName)),
		operatorRegistry:        pcsgcomponent.CreateOperatorRegistry(mgr, nil),
	}
}

// Reconcile reconciles a PodCliqueScalingGroup resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllogger.FromContext(ctx).
		WithName(controllerName).
		WithValues("pcsg-name", req.Name, "pcsg-namespace", req.Namespace)

	pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{}
	if result := ctrlutils.GetPodCliqueScalingGroup(ctx, r.client, logger, req.NamespacedName, pcsg); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	// Check if the deletion timestamp has not been set, do not handle if it is
	var deletionOrSpecReconcileFlowResult ctrlcommon.ReconcileStepResult
	if !pcsg.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(pcsg, grovecorev1alpha1.FinalizerPodCliqueScalingGroup) {
			return ctrlcommon.DoNotRequeue().Result()
		}
		dLog := logger.WithValues("operation", "delete")
		deletionOrSpecReconcileFlowResult = r.triggerDeletionFlow(ctx, dLog, pcsg)
	} else {
		specLog := logger.WithValues("operation", "specReconcile")
		deletionOrSpecReconcileFlowResult = r.reconcileSpec(ctx, specLog, pcsg)
	}

	if statusReconcileResult := r.reconcileStatus(ctx, pcsg); ctrlcommon.ShortCircuitReconcileFlow(statusReconcileResult) {
		return statusReconcileResult.Result()
	}

	if ctrlcommon.ShortCircuitReconcileFlow(deletionOrSpecReconcileFlowResult) {
		return deletionOrSpecReconcileFlowResult.Result()
	}

	return ctrlcommon.DoNotRequeue().Result()
}

func (r *Reconciler) reconcileStatus(ctx context.Context, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	pcsg.Status.Replicas = pcsg.Spec.Replicas

	pgsName := k8sutils.GetFirstOwnerName(pcsg.ObjectMeta)
	labels := lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsg.Name,
		},
	)

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to generate label selector", err)
	}
	pcsg.Status.Selector = ptr.To(selector.String())

	if err := r.client.Status().Update(ctx, pcsg); err != nil {
		return ctrlcommon.ReconcileWithErrors("failed to update the status with label selector and replicas", err)
	}

	return ctrlcommon.ContinueReconcile()
}
