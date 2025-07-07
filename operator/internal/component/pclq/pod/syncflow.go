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
	"strconv"
	"strings"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r _resource) prepareSyncFlow(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) (*syncContext, error) {
	sc := &syncContext{
		ctx:  ctx,
		pclq: pclq,
	}
	pgs, err := r.getOwnerPodGangSet(ctx, pclq.ObjectMeta)
	if err != nil {
		return nil, err
	}
	sc.pgs = pgs

	expectedPodGangNames, err := r.getExpectedPodGangNamesAssociatedWithPCLQ(ctx, pgs, pclq.ObjectMeta)
	if err != nil {
		return nil, err
	}
	sc.expectedPodGangNames = expectedPodGangNames

	existingPodGangsForPCLQ, err := r.getExistingPodGangsAssociatedWithPCLQ(ctx, pgs.ObjectMeta, pclq.ObjectMeta, expectedPodGangNames)
	if err != nil {
		return nil, err
	}
	sc.existingPodGangsForPCLQ = existingPodGangsForPCLQ
	sc.podNamesUpdatedInPCLQPodGangs = getPodNamesUpdatedInAssociatedPodGangs(existingPodGangsForPCLQ)

	existingPCLQPods, err := componentutils.GetPCLQPods(ctx, r.client, pgs.Name, pclq)
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

// getOwnerPodGangSet gets the owner PodGangSet object for the PodClique.
func (r _resource) getOwnerPodGangSet(ctx context.Context, pclqObjectMeta metav1.ObjectMeta) (*grovecorev1alpha1.PodGangSet, error) {
	pgsName := k8sutils.GetFirstOwnerName(pclqObjectMeta)
	pgs := &grovecorev1alpha1.PodGangSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgsName,
			Namespace: pclqObjectMeta.Namespace,
		},
	}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(pgs), pgs); err != nil {
		return nil, groveerr.WrapError(err,
			errCodeGetPodGangSet,
			component.OperationSync,
			fmt.Sprintf("failed to get owner PodGangSet %s ", pgsName),
		)
	}
	return pgs, nil
}

func (r _resource) getExpectedPodGangNamesAssociatedWithPCLQ(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, pclqObjectMeta metav1.ObjectMeta) ([]string, error) {
	var expectedPodGangNames []string
	pgsReplica, err := getPGSReplicaIndexForPCLQ(pclqObjectMeta)
	if err != nil {
		return nil, err
	}
	pgsReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplica}, nil)
	expectedPodGangNames = append(expectedPodGangNames, pgsReplicaPodGangName)

	// check if the PCLQ is associated to a PCSG
	pcsgFQN, ok := pclqObjectMeta.GetLabels()[grovecorev1alpha1.LabelPodCliqueScalingGroup]
	if !ok {
		return expectedPodGangNames, nil
	}

	pcsgReplicas, err := r.getPCSGReplicasAssociatedWithPCLQ(ctx, pcsgFQN, pclqObjectMeta.Namespace)
	if err != nil {
		return nil, err
	}

	if pcsgReplicas > 1 {
		matchingPCSGConfig := componentutils.FindMatchingPCSGConfig(pgs, pcsgFQN)
		for replica := 1; replica < int(pcsgReplicas); replica++ {
			pcsgReplicaPodGangName := grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplica}, &grovecorev1alpha1.ResourceNameReplica{Name: matchingPCSGConfig.Name, Replica: replica - 1})
			expectedPodGangNames = append(expectedPodGangNames, pcsgReplicaPodGangName)
		}
	}
	return expectedPodGangNames, nil
}

func getPGSReplicaIndexForPCLQ(pclqObjectMeta metav1.ObjectMeta) (int, error) {
	pgsReplicaLabelValue, ok := pclqObjectMeta.GetLabels()[grovecorev1alpha1.LabelPodGangSetReplicaIndex]
	if !ok {
		return 0, groveerr.New(
			errCodeMissingPodGangSetReplicaIndexLabel,
			component.OperationSync,
			fmt.Sprintf("PodClique %v is missing a required label :%s. This should ideally not happen.", k8sutils.GetObjectKeyFromObjectMeta(pclqObjectMeta), grovecorev1alpha1.LabelPodGangSetReplicaIndex))
	}
	pgsReplica, err := strconv.Atoi(pgsReplicaLabelValue)
	if err != nil {
		return 0, groveerr.WrapError(err,
			errCodeInvalidPodGangSetReplicaLabelValue,
			component.OperationSync,
			fmt.Sprintf("failed to convert label value %v to int for PodClique %v", pgsReplicaLabelValue, k8sutils.GetObjectKeyFromObjectMeta(pclqObjectMeta)),
		)
	}
	return pgsReplica, nil
}

func (r _resource) getPCSGReplicasAssociatedWithPCLQ(ctx context.Context, pcsgName, namespace string) (int32, error) {
	// Get the PCSG resource and check its current spec.replicas.
	pcsg := &grovecorev1alpha1.PodCliqueScalingGroup{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: pcsgName, Namespace: namespace}, pcsg); err != nil {
		return 0, groveerr.WrapError(err,
			errCodeGetPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("failed to get PodCliqueScalingGroup %s associated to PodClique", client.ObjectKey{Namespace: namespace, Name: pcsgName}),
		)
	}
	return pcsg.Spec.Replicas, nil
}

func (r _resource) getExistingPodGangsAssociatedWithPCLQ(ctx context.Context, pgsObjectMeta metav1.ObjectMeta, pclqObjectMeta metav1.ObjectMeta, expectedPodGangNames []string) ([]*groveschedulerv1alpha1.PodGang, error) {
	podGangList := &groveschedulerv1alpha1.PodGangList{}
	if err := r.client.List(ctx,
		podGangList,
		client.InNamespace(pclqObjectMeta.Namespace),
		client.MatchingLabels(componentutils.GetPodGangSelectorLabels(pgsObjectMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodGang,
			component.OperationSync,
			"failed to list Podgangs",
		)
	}
	podGangsAssociatedWithPCLQ := lo.FilterMap(podGangList.Items, func(pg groveschedulerv1alpha1.PodGang, _ int) (*groveschedulerv1alpha1.PodGang, bool) {
		return &pg, slices.Contains(expectedPodGangNames, pg.Name)
	})
	return podGangsAssociatedWithPCLQ, nil
}

func getPodNamesUpdatedInAssociatedPodGangs(existingPodGangsForPCLQ []*groveschedulerv1alpha1.PodGang) []string {
	updatedPodNames := sets.New[string]()
	for _, pg := range existingPodGangsForPCLQ {
		pgPodNames := lo.FlatMap(pg.Spec.PodGroups, func(podGroup groveschedulerv1alpha1.PodGroup, _ int) []string {
			return lo.Map(podGroup.PodReferences, func(nsName groveschedulerv1alpha1.NamespacedName, _ int) string {
				return nsName.Name
			})
		})
		updatedPodNames.Insert(pgPodNames...)
	}
	return updatedPodNames.UnsortedList()
}

func (r _resource) runSyncFlow(sc *syncContext, logger logr.Logger) syncFlowResult {
	result := syncFlowResult{}
	diff := len(sc.existingPCLQPods) - int(sc.pclq.Spec.Replicas)
	if diff < 0 {
		diff *= -1
		numScheduleGatedPods, err := r.createPods(sc.ctx, logger, sc.pclq, diff)
		logger.Info("created unassigned and scheduled gated pods", "numberOfCreatedPods", numScheduleGatedPods)
		if err != nil {
			logger.Error(err, "failed to create pods", "pclqObjectKey", client.ObjectKeyFromObject(sc.pclq))
			result.recordError(err)
		}
	} else {
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
	candidatePodsToDelete := collectCandidatePodsToDelete(sc, logger)
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

func collectCandidatePodsToDelete(sc *syncContext, logger logr.Logger) []*corev1.Pod {
	var candidatePodsToDelete []*corev1.Pod
	// First collect the pods whose associated PodGang is no longer existing and is also not expected.
	candidatePodsToDelete = append(candidatePodsToDelete, findLeftOverPodsAssociatedToNotExpectedPodGangs(sc, logger)...)
	candidatePodsToDelete = append(candidatePodsToDelete, selectExcessPodsFromExpectedPodGangs(sc, logger)...)

	return candidatePodsToDelete
}

// findLeftOverPodsAssociatedToNotExpectedPodGangs searches for pods which are associated to a PodGang that is not expected.
// If a Pod is associated to a PodGang that is no longer expected, this can be due to scale-in's that have reduced the replicas
// for a PCSG which has resulted in removal of PodGangs. If for some reason the PodGang is removed but the associated Pods are not
// then this function will collect such pods. This is achieved by checking if the existing Pod associated to a PodGang
// does not exist, and it is also not in the list of expected PodGangs.
func findLeftOverPodsAssociatedToNotExpectedPodGangs(sc *syncContext, logger logr.Logger) []*corev1.Pod {
	existingPodGangNames := lo.Map(sc.existingPodGangsForPCLQ, func(pg *groveschedulerv1alpha1.PodGang, _ int) string {
		return pg.Name
	})
	nonExistingPodGangNames := make([]string, 0, len(sc.expectedPodGangNames))
	candidatePodsToDelete := lo.Filter(sc.existingPCLQPods, func(p *corev1.Pod, _ int) bool {
		podGangLabelValue, ok := p.GetLabels()[grovecorev1alpha1.LabelPodGangName]
		shouldDeletePod := ok && !slices.Contains(existingPodGangNames, podGangLabelValue) && !slices.Contains(sc.expectedPodGangNames, podGangLabelValue)
		if shouldDeletePod {
			nonExistingPodGangNames = append(nonExistingPodGangNames, podGangLabelValue)
		}
		return shouldDeletePod
	})

	if len(candidatePodsToDelete) > 0 {
		candidatePodNames := lo.Map(candidatePodsToDelete, func(pod *corev1.Pod, _ int) string { return pod.Name })
		logger.Info("found pods that are associated to PodGangs that do not exist and are no longer expected", "podGangNames", nonExistingPodGangNames, "podNames", candidatePodNames)
	} else {
		logger.V(4).Info("no pod found that is associated to PodGangs that do not exist and are no longer expected")
	}
	return candidatePodsToDelete
}

func selectExcessPodsFromExpectedPodGangs(sc *syncContext, logger logr.Logger) []*corev1.Pod {
	var candidatePodsToDelete []*corev1.Pod
	expectedPodGangReplicas := computeExpectedPodGangReplicas(sc.pgs, sc.pclq)
	for podGangName, pods := range sc.getExistingPodsByExpectedPodGangName() {
		if len(pods) > expectedPodGangReplicas {
			numExcessPods := len(pods) - expectedPodGangReplicas
			logger.Info("found excess pods for PodGang", "podGangName", podGangName, "numExcessPods", len(pods), "expectedPodGangReplicasFromThisPCLQ", expectedPodGangReplicas)
			sort.Sort(DeletionSorter(pods))
			candidatePodsToDelete = append(candidatePodsToDelete, pods[:numExcessPods]...)
		}
	}
	return candidatePodsToDelete
}

func computeExpectedPodGangReplicas(pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique) int {
	if metav1.HasLabel(pclq.ObjectMeta, grovecorev1alpha1.LabelPodCliqueScalingGroup) {
		cliqueTemplateSpec, _ := lo.Find(pgs.Spec.Template.Cliques, func(cliqueTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return strings.Contains(pclq.Name, cliqueTemplateSpec.Name)
		})
		return int(cliqueTemplateSpec.Spec.Replicas)
	}
	return int(pclq.Spec.Replicas)
}

func (r _resource) checkAndRemovePodSchedulingGates(sc *syncContext, logger logr.Logger) ([]string, error) {
	tasks := make([]utils.Task, 0, len(sc.existingPCLQPods))
	skippedScheduleGatedPods := make([]string, 0, len(sc.existingPCLQPods))
	for _, p := range sc.existingPCLQPods {
		if hasPodGangSchedulingGate(p) {
			if !slices.Contains(sc.podNamesUpdatedInPCLQPodGangs, p.Name) {
				logger.Info("Pod has scheduling gate but it has not yet been updated in PodGang", "podObjectKey", client.ObjectKeyFromObject(p))
				skippedScheduleGatedPods = append(skippedScheduleGatedPods, p.Name)
				continue
			}
			task := utils.Task{
				Name: fmt.Sprintf("RemoveSchedulingGate-%s", p.Name),
				Fn: func(ctx context.Context) error {
					podClone := p.DeepCopy()
					p.Spec.SchedulingGates = nil
					if err := client.IgnoreNotFound(r.client.Patch(ctx, p, client.MergeFrom(podClone))); err != nil {
						return err
					}
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

func hasPodGangSchedulingGate(pod *corev1.Pod) bool {
	return slices.ContainsFunc(pod.Spec.SchedulingGates, func(schedulingGate corev1.PodSchedulingGate) bool {
		return podGangSchedulingGate == schedulingGate.Name
	})
}

func (r _resource) createPods(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique, numPods int) (int, error) {
	createTasks := make([]utils.Task, 0, numPods)
	for i := range numPods {
		createTask := utils.Task{
			Name: fmt.Sprintf("CreatePod-%s-%d", pclq.Name, i),
			Fn: func(ctx context.Context) error {
				pod := &corev1.Pod{}
				if err := r.buildResource(pclq, pod); err != nil {
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
	existingPodGangsForPCLQ       []*groveschedulerv1alpha1.PodGang
	expectedPodGangNames          []string
	existingPCLQPods              []*corev1.Pod
	podNamesUpdatedInPCLQPodGangs []string
}

func (sc *syncContext) getExistingPodsByExpectedPodGangName() map[string][]*corev1.Pod {
	podGangNameToPods := make(map[string][]*corev1.Pod)
	for _, existingPod := range sc.existingPCLQPods {
		podGangLabelValue, ok := existingPod.GetLabels()[grovecorev1alpha1.LabelPodGangName]
		if !ok && !slices.Contains(sc.expectedPodGangNames, podGangLabelValue) {
			continue
		}
		podGangNameToPods[podGangLabelValue] = append(podGangNameToPods[podGangLabelValue], existingPod)
	}
	return podGangNameToPods
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
