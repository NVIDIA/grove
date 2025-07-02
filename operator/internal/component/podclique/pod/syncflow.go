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

package pod

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/index"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r _resource) prepareSyncFlow(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) (*syncContext, error) {
	sc := &syncContext{
		ctx:  ctx,
		pclq: pclq,
	}

	associatedPodGangSet, err := componentutils.GetOwnerPodGangSet(ctx, r.client, pclq.ObjectMeta)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeGetPodGangSet,
			component.OperationSync,
			fmt.Sprintf("failed to get owner PodGangSet of PodClique: %v", client.ObjectKeyFromObject(pclq)),
		)
	}
	sc.pgs = associatedPodGangSet

	associatedPodGangName, err := r.getAssociatedPodGangName(pclq.ObjectMeta)
	if err != nil {
		return nil, err
	}
	sc.associatedPodGangName = associatedPodGangName

	existingPodGang, err := componentutils.GetPodGang(ctx, r.client, sc.associatedPodGangName, pclq.Namespace)
	if err = lo.Ternary(apierrors.IsNotFound(err), nil, err); err != nil {
		return nil, err
	}
	sc.podNamesUpdatedInPCLQPodGangs = r.getPodNamesUpdatedInAssociatedPodGang(existingPodGang, pclq.Name)

	existingPCLQPods, err := componentutils.GetPCLQPods(ctx, r.client, sc.pgs.Name, pclq)
	if err != nil {
		logger.Error(err, "Failed to list pods that belong to PodClique", "pclqObjectKey", client.ObjectKeyFromObject(pclq))
		return nil, groveerr.WrapError(err,
			errCodeListPod,
			component.OperationSync,
			fmt.Sprintf("failed to list pods that belong to the PodClique %v", client.ObjectKeyFromObject(pclq)),
		)
	}
	sc.existingPCLQPods = existingPCLQPods

	return sc, nil
}

func (r _resource) getAssociatedPodGangName(pclqObjectMeta metav1.ObjectMeta) (string, error) {
	podGangName, ok := pclqObjectMeta.GetLabels()[grovecorev1alpha1.LabelPodGang]
	if !ok {
		return "", groveerr.New(errCodeMissingPodGangLabelOnPCLQ,
			component.OperationSync,
			fmt.Sprintf("PodClique: %v is missing required label: %s", k8sutils.GetObjectKeyFromObjectMeta(pclqObjectMeta), grovecorev1alpha1.LabelPodGang),
		)
	}
	return podGangName, nil
}

func (r _resource) getPodNamesUpdatedInAssociatedPodGang(existingPodGang *groveschedulerv1alpha1.PodGang, pclqFQN string) []string {
	if existingPodGang == nil {
		return nil
	}
	podGroup, ok := lo.Find(existingPodGang.Spec.PodGroups, func(podGroup groveschedulerv1alpha1.PodGroup) bool {
		return podGroup.Name == pclqFQN
	})
	if !ok {
		return nil
	}
	return lo.Map(podGroup.PodReferences, func(nsName groveschedulerv1alpha1.NamespacedName, _ int) string {
		return nsName.Name
	})
}

func (r _resource) runSyncFlow(sc *syncContext, logger logr.Logger) syncFlowResult {
	result := syncFlowResult{}
	diff := len(sc.existingPCLQPods) - int(sc.pclq.Spec.Replicas)
	if diff < 0 {
		logger.Info("found fewer pods than desired", "pclq", client.ObjectKeyFromObject(sc.pclq), "specReplicas", sc.pclq.Spec.Replicas, "delta", diff)
		diff *= -1
		numScheduleGatedPods, err := r.createPods(sc.ctx, logger, sc.pgs, sc.pclq, sc.associatedPodGangName, diff, sc.existingPCLQPods)
		logger.Info("created unassigned and scheduled gated pods", "numberOfCreatedPods", numScheduleGatedPods)
		if err != nil {
			logger.Error(err, "failed to create pods", "pclqObjectKey", client.ObjectKeyFromObject(sc.pclq))
			result.recordError(err)
		}
	} else if diff > 0 {
		if err := r.deleteExcessPods(sc, logger, diff); err != nil {
			result.recordError(err)
		}
	}

	skippedScheduleGatedPods, err := r.checkAndRemovePodSchedulingGates(sc, logger)
	if err != nil {
		result.recordError(err)
	}
	result.recordPendingScheduleGatedPods(skippedScheduleGatedPods)
	return result
}

func (r _resource) deleteExcessPods(sc *syncContext, logger logr.Logger, diff int) error {
	candidatePodsToDelete := selectExcessPodsToDelete(sc, logger)
	numPodsToSelectForDeletion := min(diff, len(candidatePodsToDelete))
	selectedPodsToDelete := candidatePodsToDelete[:numPodsToSelectForDeletion]

	deleteTasks := make([]utils.Task, 0, len(selectedPodsToDelete))
	for i, podToDelete := range selectedPodsToDelete {
		podObjectKey := client.ObjectKeyFromObject(podToDelete)
		deleteTask := utils.Task{
			Name: fmt.Sprintf("DeletePod-%s-%d", podToDelete.Name, i),
			Fn: func(ctx context.Context) error {
				if err := client.IgnoreNotFound(r.client.Delete(ctx, podToDelete)); err != nil {
					r.eventRecorder.Eventf(sc.pclq, corev1.EventTypeWarning, reasonPodDeletionFailed, "Error deleting pod: %v", err)
					return groveerr.WrapError(err,
						errCodeDeletePod,
						component.OperationSync,
						fmt.Sprintf("failed to delete Pod: %v for PodClique %v", podObjectKey, client.ObjectKeyFromObject(sc.pclq)),
					)
				}
				logger.Info("Deleted Pod", "podObjectKey", podObjectKey)
				r.eventRecorder.Eventf(sc.pclq, corev1.EventTypeNormal, reasonPodDeletionSuccessful, "Deleted Pod: %s", podToDelete.Name)
				return nil
			},
		}
		deleteTasks = append(deleteTasks, deleteTask)
	}

	if runResult := utils.RunConcurrentlyWithSlowStart(sc.ctx, logger, 1, deleteTasks); runResult.HasErrors() {
		err := runResult.GetAggregatedError()
		pclqObjectKey := client.ObjectKeyFromObject(sc.pclq)
		logger.Error(err, "failed to delete pods for PCLQ", "pclqObjectKey", pclqObjectKey, "runSummary", runResult.GetSummary())
		return groveerr.WrapError(err,
			errCodeDeletePod,
			component.OperationSync,
			fmt.Sprintf("failed to delete Pods for PodClique %v", pclqObjectKey),
		)
	}
	logger.Info("Deleted excess pods", "diff", diff, "noOfPodsDeleted", numPodsToSelectForDeletion)
	return nil
}

func selectExcessPodsToDelete(sc *syncContext, logger logr.Logger) []*corev1.Pod {
	var candidatePodsToDelete []*corev1.Pod
	if diff := len(sc.existingPCLQPods) - int(sc.pclq.Spec.Replicas); diff > 0 {
		logger.Info("found excess pods for PodClique", "pclqObjectKey", client.ObjectKeyFromObject(sc.pclq), "numExcessPods", diff)
		sort.Sort(DeletionSorter(sc.existingPCLQPods))
		candidatePodsToDelete = append(candidatePodsToDelete, sc.existingPCLQPods[:diff]...)
	}
	return candidatePodsToDelete
}

func (r _resource) checkAndRemovePodSchedulingGates(sc *syncContext, logger logr.Logger) ([]string, error) {
	tasks := make([]utils.Task, 0, len(sc.existingPCLQPods))
	skippedScheduleGatedPods := make([]string, 0, len(sc.existingPCLQPods))

	// Pre-compute base PodGang readiness once for all pods in this PodClique
	// All pods in the same PodClique have the same base PodGang
	basePodGangReady, basePodGangName, err := r.checkBasePodGangReadinessForPodClique(sc.ctx, logger, sc.pclq)
	if err != nil {
		logger.Error(err, "Error checking base PodGang readiness for PodClique - will requeue")
		return nil, groveerr.WrapError(err,
			errCodeRemovePodSchedulingGate,
			component.OperationSync,
			"failed to check base PodGang readiness for PodClique",
		)
	}

	for i, p := range sc.existingPCLQPods {
		if hasPodGangSchedulingGate(p) {
			podObjectKey := client.ObjectKeyFromObject(p)
			if !slices.Contains(sc.podNamesUpdatedInPCLQPodGangs, p.Name) {
				logger.Info("Pod has scheduling gate but it has not yet been updated in PodGang", "podObjectKey", podObjectKey)
				skippedScheduleGatedPods = append(skippedScheduleGatedPods, p.Name)
				continue
			}
			shouldSkip := r.shouldSkipPodSchedulingGateRemoval(logger, p, basePodGangReady, basePodGangName)
			if shouldSkip {
				skippedScheduleGatedPods = append(skippedScheduleGatedPods, p.Name)
				continue
			}
			task := utils.Task{
				Name: fmt.Sprintf("RemoveSchedulingGate-%s-%d", p.Name, i),
				Fn: func(ctx context.Context) error {
					podClone := p.DeepCopy()
					p.Spec.SchedulingGates = nil
					if err := client.IgnoreNotFound(r.client.Patch(ctx, p, client.MergeFrom(podClone))); err != nil {
						return err
					}
					logger.Info("Removed scheduling gate from pod", "podObjectKey", podObjectKey)
					return nil
				},
			}
			tasks = append(tasks, task)
		}
	}

	if len(tasks) > 0 {
		pclqObjectKey := client.ObjectKeyFromObject(sc.pclq)
		if runResult := utils.RunConcurrentlyWithSlowStart(sc.ctx, logger, 1, tasks); runResult.HasErrors() {
			err := runResult.GetAggregatedError()
			logger.Error(err, "failed to remove scheduling gates from pods for PCLQ", "pclqObjectKey", pclqObjectKey, "runSummary", runResult.GetSummary())
			return skippedScheduleGatedPods, groveerr.WrapError(err,
				errCodeRemovePodSchedulingGate,
				component.OperationSync,
				fmt.Sprintf("failed to remove scheduling gates from Pods for PodClique %v", pclqObjectKey),
			)
		}
	}

	return skippedScheduleGatedPods, nil
}

// isBasePodGangReady checks if the base PodGang (identified by name) is ready, returning errors for API failures.
// A base PodGang is considered "ready" when ALL of its constituent PodCliques have achieved
// their minimum required number of ready pods (PodClique.Status.ReadyReplicas >= PodGroup.MinReplicas).
func (r _resource) isBasePodGangReady(ctx context.Context, logger logr.Logger, namespace, basePodGangName string) (bool, error) {
	// Get the base PodGang - treat all errors (including NotFound) as requeue-able
	basePodGang, err := componentutils.GetPodGang(ctx, r.client, basePodGangName, namespace)
	if err != nil {
		return false, groveerr.WrapError(err,
			errCodeGetPodGang,
			component.OperationSync,
			fmt.Sprintf("failed to get base PodGang %v", client.ObjectKey{Namespace: namespace, Name: basePodGangName}),
		)
	}

	// Check if all PodGroups in the base PodGang have sufficient ready replicas
	// Each PodGroup represents a PodClique within the base PodGang and must meet its MinReplicas requirement
	for _, podGroup := range basePodGang.Spec.PodGroups {
		pclqName := podGroup.Name

		// Get the PodClique
		pclq := &grovecorev1alpha1.PodClique{}
		pclqKey := client.ObjectKey{Name: pclqName, Namespace: namespace}
		if err := r.client.Get(ctx, pclqKey, pclq); err != nil {
			// All errors (including NotFound) should trigger requeue for reliable retry
			// This ensures PodClique exists before we evaluate base PodGang readiness
			return false, groveerr.WrapError(err,
				errCodeGetPodClique,
				component.OperationSync,
				fmt.Sprintf("failed to get PodClique %s in namespace %s for base PodGang readiness check", pclqName, namespace),
			)
		}

		// CRITICAL READINESS CHECK: Compare actual ready pods vs required minimum
		// If ANY PodClique in the base PodGang fails this check, the entire base is considered not ready
		if pclq.Status.ReadyReplicas < podGroup.MinReplicas {
			logger.Info("Base PodGang not ready: PodClique has insufficient ready replicas",
				"basePodGangName", basePodGangName,
				"pclqName", pclqName,
				"readyReplicas", pclq.Status.ReadyReplicas,
				"minReplicas", podGroup.MinReplicas)
			return false, nil // Not ready, but no error - legitimate state
		}
	}

	logger.Info("Base PodGang is ready - all PodCliques meet MinAvailable requirements", "basePodGangName", basePodGangName)
	return true, nil
}

// checkBasePodGangReadinessForPodClique determines if there's a base PodGang that needs to be checked
// for readiness, and if so, performs that check once for the entire PodClique.
func (r _resource) checkBasePodGangReadinessForPodClique(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) (bool, string, error) {
	// Check if this PodClique has a base PodGang dependency
	basePodGangName, hasBasePodGangLabel := pclq.GetLabels()[grovecorev1alpha1.LabelBasePodGang]
	if !hasBasePodGangLabel {
		// This PodClique is a base PodGang itself - no dependency
		return true, "", nil
	}

	ready, err := r.isBasePodGangReady(ctx, logger, pclq.Namespace, basePodGangName)
	if err != nil {
		return false, basePodGangName, err
	}

	return ready, basePodGangName, nil
}

// shouldSkipPodSchedulingGateRemoval implements the core PodGang scheduling gate logic.
// It returns true if the pod scheduling gate removal should be skipped, false otherwise.
func (r _resource) shouldSkipPodSchedulingGateRemoval(logger logr.Logger, pod *corev1.Pod, basePodGangReady bool, basePodGangName string) bool {
	if basePodGangName == "" {
		// BASE PODGANG POD: This PodClique has no base PodGang dependency
		// These pods form the core gang and get their gates removed immediately once assigned to PodGang
		// They represent the minimum viable cluster (first minAvailable replicas) that must start together
		logger.Info("Proceeding with gate removal for base PodGang pod",
			"podObjectKey", client.ObjectKeyFromObject(pod))
		return false
	}
	// SCALED PODGANG POD: This PodClique depends on a base PodGang
	if basePodGangReady {
		logger.Info("Base PodGang is ready, proceeding with gate removal for scaled PodGang pod",
			"podObjectKey", client.ObjectKeyFromObject(pod),
			"basePodGangName", basePodGangName)
		return false
	}
	logger.Info("Scaled PodGang pod has scheduling gate but base PodGang is not ready yet, skipping scheduling gate removal",
		"podObjectKey", client.ObjectKeyFromObject(pod),
		"basePodGangName", basePodGangName)
	return true
}

func hasPodGangSchedulingGate(pod *corev1.Pod) bool {
	return slices.ContainsFunc(pod.Spec.SchedulingGates, func(schedulingGate corev1.PodSchedulingGate) bool {
		return podGangSchedulingGate == schedulingGate.Name
	})
}

func (r _resource) createPods(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique, podGangName string, numPods int, existingPods []*corev1.Pod) (int, error) {
	// Pre-calculate all needed indices to avoid race conditions
	availableIndices, err := index.GetAvailableIndices(existingPods, numPods)
	if err != nil {
		return 0, groveerr.WrapError(err,
			errCodeSyncPod,
			component.OperationSync,
			fmt.Sprintf("error getting available indices for Pods in PodClique %v", client.ObjectKeyFromObject(pclq)),
		)
	}

	createTasks := make([]utils.Task, 0, numPods)

	for i := range numPods {
		podIndex := availableIndices[i] // Capture the specific index for this pod
		createTask := utils.Task{
			Name: fmt.Sprintf("CreatePod-%s-%d", pclq.Name, i),
			Fn: func(ctx context.Context) error {
				pod := &corev1.Pod{}
				if err := r.buildResource(pgs, pclq, podGangName, pod, podIndex); err != nil {
					return groveerr.WrapError(err,
						errCodeSyncPod,
						component.OperationSync,
						fmt.Sprintf("failed to build Pod resource for PodClique %v", client.ObjectKeyFromObject(pclq)),
					)
				}
				if err := r.client.Create(ctx, pod); err != nil {
					r.eventRecorder.Eventf(pclq, corev1.EventTypeWarning, reasonPodCreationFailed, "Error creating pod: %v", err)
					return groveerr.WrapError(err,
						errCodeSyncPod,
						component.OperationSync,
						fmt.Sprintf("failed to create Pod: %v for PodClique %v", client.ObjectKeyFromObject(pod), client.ObjectKeyFromObject(pclq)),
					)
				}
				logger.Info("Created pod for PodClique", "pclqName", pclq.Name, "podName", pod.Name)
				r.eventRecorder.Eventf(pclq, corev1.EventTypeNormal, reasonPodCreationSuccessful, "Created Pod: %s", pod.Name)
				return nil
			},
		}
		createTasks = append(createTasks, createTask)
	}
	runResult := utils.RunConcurrentlyWithSlowStart(ctx, logger, 1, createTasks)
	if runResult.HasErrors() {
		err := runResult.GetAggregatedError()
		pclqObjectKey := client.ObjectKeyFromObject(pclq)
		logger.Error(err, "failed to create pods for PCLQ", "pclqObjectKey", pclqObjectKey, "runSummary", runResult.GetSummary())
		return 0, groveerr.WrapError(err,
			errCodeCreatePods,
			component.OperationSync,
			fmt.Sprintf("failed to create Pods for PodClique %v", pclqObjectKey),
		)
	}
	return len(runResult.SuccessfulTasks), nil
}

// Convenience types and methods on these types that are used during sync flow run.
// ------------------------------------------------------------------------------------------------

// syncContext holds the relevant state required during the sync flow run.
type syncContext struct {
	ctx                           context.Context
	pgs                           *grovecorev1alpha1.PodGangSet
	pclq                          *grovecorev1alpha1.PodClique
	associatedPodGangName         string
	existingPCLQPods              []*corev1.Pod
	podNamesUpdatedInPCLQPodGangs []string
}

// syncFlowResult captures the result of a sync flow run.
type syncFlowResult struct {
	// scheduleGatedPods are the pods that were created but are still schedule gated.
	scheduleGatedPods []string
	// errs are the list of errors during the sync flow run.
	errs []error
}

func (sfr *syncFlowResult) getAggregatedError() error {
	return errors.Join(sfr.errs...)
}

func (sfr *syncFlowResult) hasPendingScheduleGatedPods() bool {
	return len(sfr.scheduleGatedPods) > 0
}

func (sfr *syncFlowResult) recordError(err error) {
	sfr.errs = append(sfr.errs, err)
}

func (sfr *syncFlowResult) recordPendingScheduleGatedPods(podNames []string) {
	sfr.scheduleGatedPods = append(sfr.scheduleGatedPods, podNames...)
}

func (sfr *syncFlowResult) hasErrors() bool {
	return len(sfr.errs) > 0
}
