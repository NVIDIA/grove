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
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllogger "sigs.k8s.io/controller-runtime/pkg/log"
)

var errUndefinedPodCliqueName = errors.New("podclique is not defined in PodGangSet")

// Reconciler reconciles PodCliqueScalingGroup objects.
type Reconciler struct {
	config                  groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration
	client                  client.Client
	reconcileStatusRecorder ctrlcommon.ReconcileStatusRecorder
}

// NewReconciler creates a new instance of the PodClique Reconciler.
func NewReconciler(mgr ctrl.Manager, controllerCfg groveconfigv1alpha1.PodCliqueScalingGroupControllerConfiguration) *Reconciler {
	return &Reconciler{
		config:                  controllerCfg,
		client:                  mgr.GetClient(),
		reconcileStatusRecorder: ctrlcommon.NewReconcileStatusRecorder(mgr.GetClient(), mgr.GetEventRecorderFor(controllerName)),
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
	if !pcsg.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	if reconcileSpecResult := r.reconcileSpec(ctx, logger, pcsg); ctrlcommon.ShortCircuitReconcileFlow(reconcileSpecResult) {
		return reconcileSpecResult.Result()
	}

	return r.reconcileStatus(ctx, pcsg).Result()
}

func (r *Reconciler) reconcileSpec(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	rLog := logger.WithValues("operation", "spec-reconcile")
	reconcileStepFns := []ctrlcommon.ReconcileStepFn[grovecorev1alpha1.PodCliqueScalingGroup]{
		r.recordReconcileStart,
		r.updatePodCliques,
		r.recordReconcileSuccess,
		r.updateObservedGeneration,
	}

	for _, fn := range reconcileStepFns {
		if stepResult := fn(ctx, rLog, pcsg); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteReconcile(ctx, logger, pcsg, &stepResult)
		}
	}
	logger.Info("Finished spec reconciliation flow")
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) reconcileStatus(ctx context.Context, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	pcsg.Status.Replicas = pcsg.Spec.Replicas

	pgsName := k8sutils.GetFirstOwnerName(pcsg.ObjectMeta)
	labels := lo.Assign(k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName), map[string]string{
		grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsg.Name,
	})

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

func (r *Reconciler) recordReconcileStart(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordStart(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile); err != nil {
		logger.Error(err, "failed to record reconcile start operation")
		return ctrlcommon.ReconcileWithErrors("error recoding reconcile start", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) updatePodCliques(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	// Get the owner PodGangSet for the PodCliqueScalingGroup
	pgs := &grovecorev1alpha1.PodGangSet{}
	pgsObjectKey := client.ObjectKey{
		Namespace: pcsg.Namespace,
		Name:      k8sutils.GetFirstOwnerName(pcsg.ObjectMeta),
	}
	if result := ctrlutils.GetPodGangSet(ctx, r.client, logger, pgsObjectKey, pgs); ctrlcommon.ShortCircuitReconcileFlow(result) {
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
			logger.Error(errUndefinedPodCliqueName, "podclique is not defined in PodGangSet, will skip updating podclique replicas", "pclqObjectKey", pclqObjectKey, "podGangSetObjectKey", pgsObjectKey)
			return ctrlcommon.RecordErrorAndDoNotRequeue(fmt.Sprintf("podclique %s is not defined in PodGangSet %s", fullyQualifiedCliqueName, pgsObjectKey), errUndefinedPodCliqueName)
		}
		if result := r.updatePodCliqueReplicas(ctx, logger, pcsg, matchingPCLQTemplateSpec, pclqObjectKey); ctrlcommon.ShortCircuitReconcileFlow(result) {
			return result
		}
	}

	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) updatePodCliqueReplicas(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, pclqObjectKey client.ObjectKey) ctrlcommon.ReconcileStepResult {
	pclq := &grovecorev1alpha1.PodClique{}
	if result := ctrlutils.GetPodClique(ctx, r.client, logger, pclqObjectKey, pclq); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result
	}
	// update the spec
	expectedReplicas := pcsg.Spec.Replicas * pclqTemplateSpec.Spec.Replicas
	if expectedReplicas != pclq.Spec.Replicas {
		patch := client.MergeFrom(pclq.DeepCopy())
		pclq.Spec.Replicas = expectedReplicas
		if err := r.client.Patch(ctx, pclq, patch); err != nil {
			return ctrlcommon.ReconcileWithErrors("error patching PodClique replicas", err)
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileSuccess(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile, nil); err != nil {
		logger.Error(err, "failed to record reconcile success operation")
		return ctrlcommon.ReconcileWithErrors("error recording reconcile success", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordIncompleteReconcile(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, errStepResult *ctrlcommon.ReconcileStepResult) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pcsg, grovecorev1alpha1.LastOperationTypeReconcile, errStepResult); err != nil {
		logger.Error(err, "failed to record incomplete reconcile operation")
		// combine all errors
		allErrs := append(errStepResult.GetErrors(), err)
		return ctrlcommon.ReconcileWithErrors("error recording incomplete reconciliation", allErrs...)
	}
	return *errStepResult
}

func (r *Reconciler) updateObservedGeneration(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	original := pcsg.DeepCopy()
	pcsg.Status.ObservedGeneration = &pcsg.Generation
	if err := r.client.Status().Patch(ctx, pcsg, client.MergeFrom(original)); err != nil {
		logger.Error(err, "failed to patch status.ObservedGeneration")
		return ctrlcommon.ReconcileWithErrors("error updating observed generation", err)
	}
	logger.Info("patched status.ObservedGeneration", "ObservedGeneration", pcsg.Generation)
	return ctrlcommon.ContinueReconcile()
}
