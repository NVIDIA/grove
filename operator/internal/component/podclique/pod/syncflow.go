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

	pgsName := componentutils.GetPodGangSetName(pclq.ObjectMeta)

	associatedPodGangName, err := r.getAssociatedPodGangName(pclq.ObjectMeta)
	if err != nil {
		return nil, err
	}
	sc.associatedPodGangName = associatedPodGangName

	// we treat NotFound as expected during startup flow
	// SPECIAL CASE: Associated PodGang NotFound during startup flow
	// =================================================================
	//
	// WHY THIS IS NORMAL AND EXPECTED:
	//
	// 1. NORMAL STARTUP SEQUENCE:
	//    - PodGangSet controller creates PodCliques first
	//    - PodClique controller starts syncing (creating pods)
	//    - PodGangSet controller creates PodGang and associates existing pods
	//    - This means PodClique MUST be able to sync before its PodGang exists
	//
	// 2. DIFFERENT FROM DEPENDENCY CHECKS:
	//    - Base PodGang readiness: Scaled pods depend on base being ready (should requeue)
	//    - PodClique readiness: Base depends on constituent PodCliques (should requeue)
	//    - Associated PodGang lookup: Just checking sync state, not a dependency (can proceed)
	//
	// 3. PURPOSE OF THIS CHECK:
	//    - Determines which pods in this PodClique are already "updated in PodGang"
	//    - During startup: no PodGang exists → no pods are updated yet → empty list
	//    - This allows sync to proceed with creating pods from scratch
	//
	// 4. BREAKING THIS LOGIC CAUSES DEADLOCK:
	//    - PodClique waits for PodGang → PodGangSet can't create PodGang → infinite requeue
	//
	existingPodGang, err := r.getPodGang(ctx, associatedPodGangName, pclq.Namespace)
	if err != nil {
		// Actual API error (different from NotFound) - should requeue
		return nil, err
	}
	sc.podNamesUpdatedInPCLQPodGangs = r.getPodNamesUpdatedInAssociatedPodGang(existingPodGang, pclq.Name)
	existingPCLQPods, err := componentutils.GetPCLQPods(ctx, r.client, pgsName, pclq)
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

// getPodGang attempts to get a PodGang and returns a nil podGang if it is not found.
// This is specifically for cases where NotFound is a normal, expected condition
// rather than an error that should be wrapped and propagated.
//
// Returns: (podGang, error)
// - If error!=nil: actual API error occurred
// - If error==nil: podGang contains the resource
// - If error==nil and podGang==nil: podGang not found
func (r _resource) getPodGang(ctx context.Context, podGangName, namespace string) (*groveschedulerv1alpha1.PodGang, error) {
	podGang := &groveschedulerv1alpha1.PodGang{}
	podGangObjectKey := client.ObjectKey{Namespace: namespace, Name: podGangName}
	if err := r.client.Get(ctx, podGangObjectKey, podGang); err != nil {
		if apierrors.IsNotFound(err) {
			// NotFound is expected in some scenarios - not an error
			return nil, nil
		}
		// Wrap actual API errors (network, permissions, etc.)
		return nil, groveerr.WrapError(err,
			errCodeGetPodGang,
			component.OperationSync,
			fmt.Sprintf("failed to get PodGang: %v", podGangObjectKey),
		)
	}
	return podGang, nil
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
		numScheduleGatedPods, err := r.createPods(sc.ctx, logger, sc.pclq, sc.associatedPodGangName, diff, sc.existingPCLQPods)
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
	for i, p := range sc.existingPCLQPods {
		if hasPodGangSchedulingGate(p) {
			if !slices.Contains(sc.podNamesUpdatedInPCLQPodGangs, p.Name) {
				logger.Info("Pod has scheduling gate but it has not yet been updated in PodGang", "podObjectKey", client.ObjectKeyFromObject(p))
				skippedScheduleGatedPods = append(skippedScheduleGatedPods, p.Name)
				continue
			}
			// Apply PodGang scheduling gate logic (base vs scaled PodGang handling)
			shouldSkip, skipReason, err := r.shouldSkipPodSchedulingGateRemoval(sc.ctx, logger, p)
			if err != nil {
				// API error - return early to trigger requeue
				logger.Error(err, "Error in scheduling gate logic - will requeue", "podObjectKey", client.ObjectKeyFromObject(p))
				return nil, groveerr.WrapError(err,
					errCodeRemovePodSchedulingGate,
					component.OperationSync,
					fmt.Sprintf("failed to check PodGang readiness for pod %v", client.ObjectKeyFromObject(p)),
				)
			}
			if shouldSkip {
				logger.Info("Skipping scheduling gate removal", "podObjectKey", client.ObjectKeyFromObject(p), "reason", skipReason)
				skippedScheduleGatedPods = append(skippedScheduleGatedPods, p.Name)
				continue
			}

			// Capture pod to avoid loop variable issues in concurrent execution
			podToUpdate := p
			podObjectKey := client.ObjectKeyFromObject(podToUpdate)
			task := utils.Task{
				Name: fmt.Sprintf("RemoveSchedulingGate-%s-%d", podToUpdate.Name, i),
				Fn: func(ctx context.Context) error {
					podClone := podToUpdate.DeepCopy()
					podToUpdate.Spec.SchedulingGates = nil
					if err := client.IgnoreNotFound(r.client.Patch(ctx, podToUpdate, client.MergeFrom(podClone))); err != nil {
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
// This version properly distinguishes between "not ready" (legitimate state) and "error" (needs requeue).
//
// BASE PODGANG READINESS DEFINITION:
// A base PodGang is considered "ready" when ALL of its constituent PodCliques have achieved
// their minimum required number of ready pods (PodClique.Status.ReadyReplicas >= PodGroup.MinReplicas).
//
// WHY THIS MATTERS:
// Base PodGang readiness is the gate that controls when scaled PodGang pods
// can have their scheduling gates removed. This ensures that:
//
//  1. CORE FUNCTIONALITY FIRST: The minimum viable cluster (first minAvailable replicas)
//     is fully operational before any additional scale-out pods are scheduled
//
//  2. RESOURCE EFFICIENCY: Prevents wasteful scheduling of extra pods when the core
//     workload itself is not yet healthy/ready
//
//  3. DEPENDENCY ORDERING: Many distributed systems require a stable core before
//     adding workers/replicas (e.g., databases, queue systems, consensus algorithms)
//
// EXAMPLE SCENARIO (minAvailable=3, replicas=5):
//   - Base PodGang "simple1-0" contains replicas 0,1,2 (core cluster)
//   - Scaled PodGangs "simple1-0-sga-0", "simple1-0-sga-1" wait for base readiness
//   - Only when ALL PodCliques in base PodGang are ready do the scale-out pods get scheduled
//
// Returns (isReady bool, error)
// - If error is non-nil, caller should requeue rather than assume "not ready"
// - If error is nil, isReady indicates actual readiness status
func (r _resource) isBasePodGangReady(ctx context.Context, logger logr.Logger, namespace, basePodGangName string) (bool, error) {
	// Get the base PodGang - treat all errors (including NotFound) as requeue-able
	basePodGang, err := r.getPodGang(ctx, basePodGangName, namespace)
	if err != nil {
		// All errors should trigger requeue for reliable retry
		// This ensures we actively check for base PodGang creation rather than waiting passively
		return false, err
	}
	if basePodGang == nil {
		// NotFound is not expected in this context - create proper error for requeue
		notFoundErr := fmt.Errorf("base PodGang not found")
		return false, groveerr.WrapError(notFoundErr,
			errCodeGetPodGang,
			component.OperationSync,
			fmt.Sprintf("base PodGang %v not found", client.ObjectKey{Namespace: namespace, Name: basePodGangName}),
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

// shouldSkipPodSchedulingGateRemoval implements the core PodGang scheduling gate logic.
//
// PodGang Scheduling Gate Logic: Base vs Scaled PodGangs
// ============================================================
//
// CONCEPT OVERVIEW:
// In scaling groups with minAvailable > 1, pods are divided into two categories:
//
// 1. BASE PODGANG PODS (replicas 0 to minAvailable-1):
//   - Grouped into a single "base" PodGang (e.g., "simple1-0")
//   - Scheduled together as a gang - all-or-nothing scheduling
//   - Gates removed immediately when pods are assigned to PodGang
//   - Forms the minimum viable cluster that other replicas depend on
//
// 2. SCALED PODGANG PODS (replicas minAvailable and above):
//   - Each gets its own "scaled" PodGang (e.g., "simple1-0-sga-0", "simple1-0-sga-1")
//   - Gates are only removed AFTER the base PodGang is ready and running
//   - Provides scale-out capacity once core functionality is established
//   - Has grove.io/base-podgang label pointing to their base PodGang
//
// EXAMPLE: With minAvailable=3, replicas=5:
//   - Replicas 0,1,2 → Base PodGang "simple1-0" (scheduled together)
//   - Replica 3     → Scaled PodGang "simple1-0-sga-0" (waits for base)
//   - Replica 4     → Scaled PodGang "simple1-0-sga-1" (waits for base)
//
// Returns (shouldSkip, skipReason, error)
// If error is non-nil, the caller should requeue rather than skip the pod
func (r _resource) shouldSkipPodSchedulingGateRemoval(ctx context.Context, logger logr.Logger, pod *corev1.Pod) (bool, string, error) {
	scaledPodGangName, hasPodGangLabel := pod.GetLabels()[grovecorev1alpha1.LabelPodGang]
	if !hasPodGangLabel {
		// Pod not assigned to any PodGang yet
		return false, "", nil
	}

	basePodGangName, hasBasePodGangLabel := pod.GetLabels()[grovecorev1alpha1.LabelBasePodGang]
	if !hasBasePodGangLabel || basePodGangName == scaledPodGangName {
		// BASE PODGANG POD: No base-podgang label or base-podgang equals scaled PodGang
		// These pods form the core gang and get their gates removed immediately once assigned to PodGang
		// They represent the minimum viable cluster (first minAvailable replicas) that must start together
		logger.Info("Proceeding with gate removal for base PodGang pod",
			"podObjectKey", client.ObjectKeyFromObject(pod),
			"podGangName", scaledPodGangName)
		return false, "", nil
	}

	// SCALED PODGANG POD: This pod belongs to a scaled replica that must wait for base PodGang
	ready, err := r.isBasePodGangReady(ctx, logger, pod.Namespace, basePodGangName)
	if err != nil {
		// API error - should requeue rather than skip indefinitely
		logger.Error(err, "Error checking base PodGang readiness - will requeue",
			"podObjectKey", client.ObjectKeyFromObject(pod),
			"basePodGangName", basePodGangName)
		return false, "", err
	}

	if !ready {
		logger.Info("Scaled PodGang pod has scheduling gate but base PodGang is not ready yet",
			"podObjectKey", client.ObjectKeyFromObject(pod),
			"scaledPodGangName", scaledPodGangName,
			"basePodGangName", basePodGangName)
		return true, "base PodGang not ready", nil
	}

	logger.Info("Base PodGang is ready, removing scheduling gate from scaled PodGang pod",
		"podObjectKey", client.ObjectKeyFromObject(pod),
		"scaledPodGangName", scaledPodGangName,
		"basePodGangName", basePodGangName)
	return false, "", nil
}

func hasPodGangSchedulingGate(pod *corev1.Pod) bool {
	return slices.ContainsFunc(pod.Spec.SchedulingGates, func(schedulingGate corev1.PodSchedulingGate) bool {
		return podGangSchedulingGate == schedulingGate.Name
	})
}

func (r _resource) createPods(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique, podGangName string, numPods int, existingPods []*corev1.Pod) (int, error) {
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
				if err := r.buildResource(pclq, podGangName, pod, podIndex); err != nil {
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
