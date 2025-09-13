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

package podgangset

import (
	"context"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *Reconciler) reconcileSpec(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	rLog := logger.WithValues("operation", "spec-reconcile")
	reconcileStepFns := []ctrlcommon.ReconcileStepFn[grovecorev1alpha1.PodCliqueSet]{
		r.ensureFinalizer,
		r.recordReconcileStart,
		r.processGenerationHashChange,
		r.syncPodGangSetResources,
		r.recordReconcileSuccess,
		r.updateObservedGeneration,
	}

	for _, fn := range reconcileStepFns {
		if stepResult := fn(ctx, rLog, pgs); ctrlcommon.ShortCircuitReconcileFlow(stepResult) {
			return r.recordIncompleteReconcile(ctx, logger, pgs, &stepResult)
		}
	}
	logger.Info("Finished spec reconciliation flow", "PodCliqueSet", client.ObjectKeyFromObject(pgs))
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) ensureFinalizer(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pgs, constants.FinalizerPodCliqueSet) {
		logger.Info("Adding finalizer", "finalizerName", constants.FinalizerPodCliqueSet)
		if err := ctrlutils.AddAndPatchFinalizer(ctx, r.client, pgs, constants.FinalizerPodCliqueSet); err != nil {
			return ctrlcommon.ReconcileWithErrors("error adding finalizer", fmt.Errorf("failed to add finalizer: %s to PodCliqueSet: %v: %w", constants.FinalizerPodCliqueSet, client.ObjectKeyFromObject(pgs), err))
		}
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileStart(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordStart(ctx, pgs, grovecorev1alpha1.LastOperationTypeReconcile); err != nil {
		logger.Error(err, "failed to record reconcile start operation")
		return ctrlcommon.ReconcileWithErrors("error recoding reconcile start", err)
	}
	return ctrlcommon.ContinueReconcile()
}

// processGenerationHashChange computes the generation hash given a PodCliqueSet resource and if the generation has
// changed from the previously persisted pgs.status.generationHash then it resets the pgs.status.rollingUpdateProgress
func (r *Reconciler) processGenerationHashChange(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	pgsObjectKey := client.ObjectKeyFromObject(pgs)
	pgsObjectName := cache.NamespacedNameAsObjectName(pgsObjectKey).String()

	// if the generationHash is not reflected correctly yet, requeue. Allow the informer cache to catch-up.
	if !r.isGenerationHashExpectationSatisfied(pgsObjectName, pgs.Status.CurrentGenerationHash) {
		return ctrlcommon.ReconcileAfter(ctrlcommon.ComponentSyncRetryInterval, fmt.Sprintf("CurrentGenerationHash is not up-to-date for PodCliqueSet: %v", pgsObjectKey))
	} else {
		r.pgsGenerationHashExpectations.Delete(pgsObjectName)
	}

	newGenerationHash := computeGenerationHash(pgs)
	if pgs.Status.CurrentGenerationHash == nil {
		// update the generation hash and continue reconciliation. No rolling update is required.
		if err := r.setGenerationHash(ctx, pgs, pgsObjectName, newGenerationHash); err != nil {
			logger.Error(err, "failed to set generation hash on PGS", "newGenerationHash", newGenerationHash)
			return ctrlcommon.ReconcileWithErrors("error updating generation hash", err)
		}
		return ctrlcommon.ContinueReconcile()
	}

	if newGenerationHash != *pgs.Status.CurrentGenerationHash {
		// trigger rolling update by setting or overriding pgs.Status.RollingUpdateProgress.
		if err := r.initRollingUpdateProgress(ctx, pgs, pgsObjectName, newGenerationHash); err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("could not triggering rolling update for PGS: %v", pgsObjectKey), err)
		}
	}

	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) isGenerationHashExpectationSatisfied(pgsObjectName string, pgsGenerationHash *string) bool {
	expectedGenerationHash, ok := r.pgsGenerationHashExpectations.Load(pgsObjectName)
	return !ok || (pgsGenerationHash != nil && expectedGenerationHash.(string) == *pgsGenerationHash)
}

func computeGenerationHash(pgs *grovecorev1alpha1.PodCliqueSet) string {
	podTemplateSpecs := lo.Map(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, _ int) *corev1.PodTemplateSpec {
		podTemplateSpec := &corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      pclqTemplateSpec.Labels,
				Annotations: pclqTemplateSpec.Annotations,
			},
			Spec: pclqTemplateSpec.Spec.PodSpec,
		}
		podTemplateSpec.Spec.PriorityClassName = pgs.Spec.Template.PriorityClassName
		return podTemplateSpec
	})
	return k8sutils.ComputeHash(podTemplateSpecs...)
}

func (r *Reconciler) setGenerationHash(ctx context.Context, pgs *grovecorev1alpha1.PodCliqueSet, pgsObjectName, generationHash string) error {
	pgs.Status.CurrentGenerationHash = &generationHash
	if err := r.client.Status().Update(ctx, pgs); err != nil {
		return fmt.Errorf("could not update CurrentGenerationHash for PodCliqueSet: %v: %w", client.ObjectKeyFromObject(pgs), err)
	}
	r.pgsGenerationHashExpectations.Store(pgsObjectName, generationHash)
	return nil
}

func (r *Reconciler) initRollingUpdateProgress(ctx context.Context, pgs *grovecorev1alpha1.PodCliqueSet, pgsObjectName, newGenerationHash string) error {
	pgs.Status.RollingUpdateProgress = &grovecorev1alpha1.PodCliqueSetRollingUpdateProgress{
		UpdateStartedAt: metav1.Now(),
	}
	pgs.Status.UpdatedReplicas = 0
	pgs.Status.CurrentGenerationHash = &newGenerationHash
	if err := r.client.Status().Update(ctx, pgs); err != nil {
		return fmt.Errorf("could not set RollingUpdateProgress for PodCliqueSet: %v: %w", client.ObjectKeyFromObject(pgs), err)
	}
	r.pgsGenerationHashExpectations.Store(pgsObjectName, newGenerationHash)
	return nil
}

func (r *Reconciler) syncPodGangSetResources(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	continueReconcileAndRequeueKinds := make([]component.Kind, 0)
	for _, kind := range getOrderedKindsForSync() {
		operator, err := r.operatorRegistry.GetOperator(kind)
		if err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("error getting operator for kind: %s", kind), err)
		}
		logger.Info("Syncing PodCliqueSet resource", "kind", kind)
		if err = operator.Sync(ctx, logger, pgs); err != nil {
			if ctrlutils.ShouldContinueReconcileAndRequeue(err) {
				logger.Info("component has registered a request to requeue post completion of all component syncs", "kind", kind, "message", err.Error())
				continueReconcileAndRequeueKinds = append(continueReconcileAndRequeueKinds, kind)
				continue
			}
			if shouldRequeue := ctrlutils.ShouldRequeueAfter(err); shouldRequeue {
				logger.Info("retrying sync due to component", "kind", kind, "syncRetryInterval", ctrlcommon.ComponentSyncRetryInterval, "message", err.Error())
				return ctrlcommon.ReconcileAfter(ctrlcommon.ComponentSyncRetryInterval, err.Error())
			}
			logger.Error(err, "failed to sync PodCliqueSet resources", "kind", kind)
			return ctrlcommon.ReconcileWithErrors("error syncing managed resources", fmt.Errorf("failed to sync %s: %w", kind, err))
		}
	}
	if len(continueReconcileAndRequeueKinds) > 0 {
		return ctrlcommon.ReconcileAfter(ctrlcommon.ComponentSyncRetryInterval, fmt.Sprintf("requeueing sync due to component(s) %v after %s", continueReconcileAndRequeueKinds, ctrlcommon.ComponentSyncRetryInterval))
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordReconcileSuccess(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pgs, grovecorev1alpha1.LastOperationTypeReconcile, nil); err != nil {
		logger.Error(err, "failed to record reconcile success operation")
		return ctrlcommon.ReconcileWithErrors("error recording reconcile success", err)
	}
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) updateObservedGeneration(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) ctrlcommon.ReconcileStepResult {
	original := pgs.DeepCopy()
	pgs.Status.ObservedGeneration = &pgs.Generation
	if err := r.client.Status().Patch(ctx, pgs, client.MergeFrom(original)); err != nil {
		logger.Error(err, "failed to patch status.ObservedGeneration")
		return ctrlcommon.ReconcileWithErrors("error updating observed generation", err)
	}
	logger.Info("patched status.ObservedGeneration", "ObservedGeneration", pgs.Generation)
	return ctrlcommon.ContinueReconcile()
}

func (r *Reconciler) recordIncompleteReconcile(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet, errResult *ctrlcommon.ReconcileStepResult) ctrlcommon.ReconcileStepResult {
	if err := r.reconcileStatusRecorder.RecordCompletion(ctx, pgs, grovecorev1alpha1.LastOperationTypeReconcile, errResult); err != nil {
		logger.Error(err, "failed to record incomplete reconcile operation")
		// combine all errors
		allErrs := append(errResult.GetErrors(), err)
		return ctrlcommon.ReconcileWithErrors("error recording incomplete reconciliation", allErrs...)
	}
	return *errResult
}

func getOrderedKindsForSync() []component.Kind {
	return []component.Kind{
		component.KindServiceAccount,
		component.KindRole,
		component.KindRoleBinding,
		component.KindServiceAccountTokenSecret,
		component.KindHeadlessService,
		component.KindHorizontalPodAutoscaler,
		component.KindPodGangSetReplica,
		component.KindPodClique,
		component.KindPodCliqueScalingGroup,
		component.KindPodGang,
	}
}
