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
	"errors"
	"fmt"
	"strings"

	groveconfigv1alpha1 "github.com/NVIDIA/grove/operator/api/config/v1alpha1"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
)

var undefinedPodCliqueName = errors.New("podclique is not defined in PodGangSet")

// Reconciler reconciles PodCliqueScalingGroup objects.
type Reconciler struct {
	config                  groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration
	client                  client.Client
	reconcileStatusRecorder common.ReconcileStatusRecorder
}

// NewReconciler creates a new instance of the PodClique Reconciler.
func NewReconciler(mgr ctrl.Manager, controllerCfg groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration) *Reconciler {
	return &Reconciler{
		config:                  controllerCfg,
		client:                  mgr.GetClient(),
		reconcileStatusRecorder: common.NewReconcileStatusRecorder(mgr.GetClient(), mgr.GetEventRecorderFor(controllerName)),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrllogger.FromContext(ctx).
		WithName(controllerName).
		WithValues("pcsg-name", req.Name, "pcsg-namespace", req.Namespace)

	pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{}
	if result := ctrlutils.GetPodCliqueScalingGroup(ctx, r.client, logger, req.NamespacedName, pcsg); common.ShortCircuitReconcileFlow(result) {
		return result.Result()
	}

	// Check if the deletion timestamp has not been set, do not handle if it is
	if !pcsg.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcileSpec(ctx, logger, pcsg).Result()
}

func (r *Reconciler) reconcileSpec(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) common.ReconcileStepResult {
	rLog := logger.WithValues("operation", "spec-reconcile")
	reconcileStepFns := []common.ReconcileStepFn[grovecorev1alpha1.PodCliqueScalingGroup]{
		r.recordReconcileStart,
		r.updatePodCliques,
		r.recordReconcileSuccess,
		r.updateObservedGeneration,
	}

	for _, fn := range reconcileStepFns {
		if stepResult := fn(ctx, rLog, pcsg); common.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteReconcile(ctx, logger, pcsg, &stepResult)
		}
	}
	logger.Info("Finished spec reconciliation flow")
	return common.ContinueReconcile()
}

func (r *Reconciler) recordReconcileStart(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) common.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordStart(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile); err != nil {
		logger.Error(err, "failed to record reconcile start operation")
		return common.ReconcileWithErrors("error recoding reconcile start", err)
	}
	return common.ContinueReconcile()
}

func (r *Reconciler) updatePodCliques(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) common.ReconcileStepResult {
	// Get the owner PodGangSet for the PodCliqueScalingGroup
	pgs := &grovecorev1alpha1.PodGangSet{}
	pgsObjectKey := client.ObjectKey{
		Namespace: pcsg.Namespace,
		Name:      k8sutils.GetFirstOwnerName(pcsg.ObjectMeta),
	}
	if result := ctrlutils.GetPodGangSet(ctx, r.client, logger, pgsObjectKey, pgs); common.ShortCircuitReconcileFlow(result) {
		return result
	}

	for _, fullyQualifiedCliqueName := range pcsg.Spec.CliqueNames {
		pclqObjectKey := client.ObjectKey{
			Namespace: pcsg.Namespace,
			Name:      fullyQualifiedCliqueName,
		}
		matchingPCLQTemplateSpec, ok := lo.Find(pgs.Spec.TemplateSpec.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return strings.HasSuffix(fullyQualifiedCliqueName, pclqTemplateSpec.Name)
		})
		if !ok {
			logger.Error(undefinedPodCliqueName, "podclique is not defined in PodGangSet, will skip updating podclique replicas", "pclqObjectKey", pclqObjectKey, "podGangSetObjectKey", pgsObjectKey)
			return common.RecordErrorAndDDoNotRequeue(fmt.Sprintf("podclique %s is not defined in PodGangSet %s", fullyQualifiedCliqueName, pgsObjectKey), undefinedPodCliqueName)
		}
		if result := r.updatePodCliqueReplicas(ctx, logger, pcsg, matchingPCLQTemplateSpec, pclqObjectKey); common.ShortCircuitReconcileFlow(result) {
			return result
		}
	}

	return common.ContinueReconcile()
}

func (r *Reconciler) updatePodCliqueReplicas(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, pclqObjectKey client.ObjectKey) common.ReconcileStepResult {
	pclq := &grovecorev1alpha1.PodClique{}
	if result := ctrlutils.GetPodClique(ctx, r.client, logger, pclqObjectKey, pclq); common.ShortCircuitReconcileFlow(result) {
		return result
	}
	// update the spec
	expectedReplicas := pcsg.Spec.Replicas * pclqTemplateSpec.Spec.Replicas
	if expectedReplicas != pclq.Spec.Replicas {
		patch := client.MergeFrom(pclq.DeepCopy())
		pclq.Spec.Replicas = expectedReplicas
		if err := r.client.Patch(ctx, pclq, patch); err != nil {
			return common.ReconcileWithErrors("error patching PodClique replicas", err)
		}
	}
	return common.ContinueReconcile()
}

func (r *Reconciler) recordReconcileSuccess(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) common.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile, nil); err != nil {
		logger.Error(err, "failed to record reconcile success operation")
		return common.ReconcileWithErrors("error recording reconcile success", err)
	}
	return common.ContinueReconcile()
}

func (r *Reconciler) recordIncompleteReconcile(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, errStepResult *common.ReconcileStepResult) common.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile, errStepResult); err != nil {
		logger.Error(err, "failed to record incomplete reconcile operation")
		// combine all errors
		allErrs := append(errStepResult.GetErrors(), err)
		return common.ReconcileWithErrors("error recording incomplete reconciliation", allErrs...)
	}
	return *errStepResult
}

func (r *Reconciler) updateObservedGeneration(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) common.ReconcileStepResult {
	original := pcsg.DeepCopy()
	pcsg.Status.ObservedGeneration = &pcsg.Generation
	if err := r.client.Status().Patch(ctx, pcsg, client.MergeFrom(original)); err != nil {
		logger.Error(err, "failed to patch status.ObservedGeneration")
		return common.ReconcileWithErrors("error updating observed generation", err)
	}
	logger.Info("patched status.ObservedGeneration", "ObservedGeneration", pcsg.Generation)
	return common.ContinueReconcile()
}
