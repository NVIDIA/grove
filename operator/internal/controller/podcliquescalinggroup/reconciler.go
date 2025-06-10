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
	"fmt"
	"strings"
	"time"

	groveconfigv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/samber/lo"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconciler reconciles PodCliqueScalingGroup objects.
type Reconciler struct {
	config        groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration
	client        ctrlclient.Client
	eventRecorder record.EventRecorder
}

// NewReconciler creates a new instance of the PodClique Reconciler.
func NewReconciler(mgr ctrl.Manager, controllerCfg groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration) *Reconciler {
	return &Reconciler{
		config:        controllerCfg,
		client:        mgr.GetClient(),
		eventRecorder: mgr.GetEventRecorderFor(controllerName),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllogger.FromContext(ctx).
		WithName(controllerName).
		WithValues("pcsg-name", req.Name, "pcsg-namespace", req.Namespace)

	pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{}
	if result := ctrlutils.GetPodCliqueScalingGroup(ctx, r.client, logger, req.NamespacedName, pcsg); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	pgs := &grovecorev1alpha1.PodGangSet{}
	pgsName := k8sutils.GetOwnerName(pcsg.ObjectMeta)
	if pgsName == "" {
		logger.Info("Skipping reconcile for this PodCliqueScalingGroup as PodGangSet is not the owner")
		return ctrl.Result{}, nil
	}

	pgsObjectKey := client.ObjectKey{
		Namespace: pcsg.Namespace,
		Name:      pgsName,
	}
	if result := ctrlutils.GetPodGangSet(ctx, r.client, logger, pgsObjectKey, pgs); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	// Check if the deletion timestamp has not been set, do not handle if it is
	if !pcsg.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcileUpdate(ctx, logger, pgs, pcsg).Result()
}

// List existing pclqs that belong to the scaling group
func (r *Reconciler) reconcileUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	pclq := &grovecorev1alpha1.PodClique{}
	for _, fullyQualifiedCliqueName := range pcsg.Spec.CliqueNames {
		namespacedName := types.NamespacedName{
			Namespace: pcsg.Namespace,
			Name:      fullyQualifiedCliqueName,
		}

		if result := ctrlutils.GetPodClique(ctx, r.client, logger, namespacedName, pclq); ctrlcommon.ShortCircuitReconcileFlow(result) {
			return result
		}

		matchPclqTemplateSpec, ok := lo.Find(pgs.Spec.TemplateSpec.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return strings.HasSuffix(fullyQualifiedCliqueName, pclqTemplateSpec.Name)
		})
		if !ok {
			logger.Error(fmt.Errorf("error finding the PodClique in the PodGangSet"), "Unable to find the PodClique specified in the PodCliqueScalingGroup in the PodGangSet")
			return ctrlcommon.DoNotRequeue()
		}

		// update the spec
		if pcsg.Spec.Replicas*matchPclqTemplateSpec.Spec.Replicas != pclq.Spec.Replicas {
			patch := ctrlclient.MergeFrom(pclq.DeepCopy())
			pclq.Spec.Replicas = pcsg.Spec.Replicas * matchPclqTemplateSpec.Spec.Replicas
			if err := r.client.Patch(ctx, pclq, patch); err != nil {
				return ctrlcommon.ReconcileAfter(time.Duration(10*time.Second), "Unable to patch the resource")
			}
		}
		// update the status
	}
	return ctrlcommon.ReconcileStepResult{}
}
