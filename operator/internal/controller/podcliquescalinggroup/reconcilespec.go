package podcliquescalinggroup

import (
	"context"
	"fmt"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlutils "github.com/NVIDIA/grove/operator/internal/controller/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

func (r *Reconciler) reconcileSpec(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	rLog := logger.WithValues("operation", "spec-reconcile")
	reconcileStepFns := []ctrlcommon.ReconcileStepFn[grovecorev1alpha1.PodCliqueScalingGroup]{
		r.ensureFinalizer,
		r.recordReconcileStart,
		r.syncPodCliqueScalingGroupResources,
		// r.updatePodCliques,
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

func (r *Reconciler) ensureFinalizer(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	if !controllerutil.ContainsFinalizer(pcsg, grovecorev1alpha1.FinalizerPodGangSet) {
		logger.Info("Adding finalizer", "finalizerName", grovecorev1alpha1.FinalizerPodCliqueScalingGroup)
		if err := ctrlutils.AddAndPatchFinalizer(ctx, r.client, pcsg, grovecorev1alpha1.FinalizerPodCliqueScalingGroup); err != nil {
			return ctrlcommon.ReconcileWithErrors("error adding finalizer", fmt.Errorf("failed to add finalizer: %s to PodGangSet: %v: %w", grovecorev1alpha1.FinalizerPodCliqueScalingGroup, client.ObjectKeyFromObject(pcsg), err))
		}
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

func (r *Reconciler) syncPodCliqueScalingGroupResources(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ctrlcommon.ReconcileStepResult {
	for _, kind := range getOrderedKindsForSync() {
		operator, err := r.operatorRegistry.GetOperator(kind)
		if err != nil {
			return ctrlcommon.ReconcileWithErrors(fmt.Sprintf("error getting operator for kind: %s", kind), err)
		}
		logger.Info("Syncing PodCliqueScalingGroup resource", "kind", kind)
		if err = operator.Sync(ctx, logger, pcsg); err != nil {
			if ctrlutils.ShouldRequeueAfter(err) {
				logger.Info("retrying sync due to component", "kind", kind, "syncRetryInterval", ctrlcommon.ComponentSyncRetryInterval)
				return ctrlcommon.ReconcileAfter(ctrlcommon.ComponentSyncRetryInterval, fmt.Sprintf("requeueing sync due to component %s after %s", kind, ctrlcommon.ComponentSyncRetryInterval))
			}
			logger.Error(err, "failed to sync PodGangSet resources", "kind", kind)
			return ctrlcommon.ReconcileWithErrors("error syncing managed resources", fmt.Errorf("failed to sync %s: %w", kind, err))
		}
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
		matchingPCLQTemplateSpec, ok := lo.Find(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
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
	if result := ctrlutils.GetPodClique(ctx, r.client, logger, pclqObjectKey, pclq, false); ctrlcommon.ShortCircuitReconcileFlow(result) {
		return result
	}
	// update the spec
	expectedReplicas := pcsg.Spec.Replicas * pclqTemplateSpec.Spec.Replicas
	if expectedReplicas != pclq.Spec.Replicas {
		logger.Info("Updating PCLQ replicas due to change in PCSG replicas", "pcsgReplicas", pcsg.Spec.Replicas, "expectedReplicasForPCLQ", expectedReplicas)
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

func getOrderedKindsForSync() []component.Kind {
	return []component.Kind{
		component.KindPodClique,
	}
}
