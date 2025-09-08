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

package podclique

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type syncContext struct {
	ctx                            context.Context
	pgs                            *grovecorev1alpha1.PodGangSet
	pcsg                           *grovecorev1alpha1.PodCliqueScalingGroup
	existingPCLQs                  []grovecorev1alpha1.PodClique
	pcsgIndicesToTerminate         []string
	pcsgIndicesToRequeue           []string
	expectedPCLQFQNsPerPCSGReplica map[int][]string
	expectedPCLQPodTemplateHashMap map[string]string
}

func (r _resource) prepareSyncContext(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) (*syncContext, error) {
	var (
		syncCtx = &syncContext{
			ctx:  ctx,
			pcsg: pcsg,
		}
		err error
	)

	// get the PodGangSet
	syncCtx.pgs, err = componentutils.GetPodGangSet(ctx, r.client, pcsg.ObjectMeta)
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeGetPodGangSet,
			component.OperationSync,
			fmt.Sprintf("failed to get owner PodGangSet for PodCliqueScalingGroup %s", client.ObjectKeyFromObject(pcsg)),
		)
	}

	// compute the expected state and get existing state.
	syncCtx.expectedPCLQFQNsPerPCSGReplica = getExpectedPodCliqueFQNsByPCSGReplica(pcsg)
	syncCtx.existingPCLQs, err = r.getExistingPCLQs(ctx, pcsg)
	if err != nil {
		return nil, err
	}

	// compute the PCSG indices that have their MinAvailableBreached condition set to true. Segregated these into two
	// pcsgIndicesToTerminate will have the indices for which the TerminationDelay has expired.
	// pcsgIndicesToRequeue will have the indices for which the TerminationDelay has not yet expired.
	syncCtx.pcsgIndicesToTerminate, syncCtx.pcsgIndicesToRequeue = getMinAvailableBreachedPCSGIndices(logger, syncCtx.existingPCLQs, syncCtx.pgs.Spec.Template.TerminationDelay.Duration)

	// pre-compute expected PodTemplateHash for each PCLQ
	syncCtx.expectedPCLQPodTemplateHashMap = getExpectedPCLQPodTemplateHashMap(syncCtx.pgs, pcsg)

	return syncCtx, nil
}

func (r _resource) runSyncFlow(logger logr.Logger, sc *syncContext) error {
	// If there are excess PodCliques than expected, delete the ones that are no longer expected but existing.
	// This can happen when PCSG replicas have been scaled-in.
	if err := r.triggerDeletionOfExcessPCSGReplicas(logger, sc); err != nil {
		return err
	}
	// Create or update the expected PodCliques as per the PodCliqueScalingGroup configurations defined in the PodGangSet.
	if err := r.createExpectedPCLQs(logger, sc); err != nil {
		return err
	}

	// Only if the rolling update is not in progress, check for a possibility of gang termination and execute it only if
	// the pcsg.spec.minAvailable is not breached.
	if !componentutils.IsPCSGUpdateInProgress(sc.pcsg) {
		if err := r.processMinAvailableBreachedPCSGReplicas(logger, sc); err != nil {
			if errors.Is(err, errPCCGMinAvailableBreached) {
				logger.Info("Skipping further reconciliation as MinAvailable for the PCSG has been breached. This can potentially trigger PGS replica deletion.")
				return nil
			}
			return err
		}
	} else {
		if err := r.processPendingUpdates(logger, sc); err != nil {
			return err
		}
	}

	// If there are any PCSG replicas which have minAvailableBreached but the terminationDelay has not yet expired, then
	// requeue the event after a fixed delay.
	if len(sc.pcsgIndicesToRequeue) > 0 {
		return groveerr.New(groveerr.ErrCodeRequeueAfter,
			component.OperationSync,
			"Requeuing to re-process PCLQs that have breached MinAvailable but not crossed TerminationDelay",
		)
	}
	return nil
}

func (r _resource) triggerDeletionOfExcessPCSGReplicas(logger logr.Logger, sc *syncContext) error {
	existingPCSGReplicas := getExistingNonTerminatingPCSGReplicas(sc.existingPCLQs)
	// Check if the number of existing PodCliques is greater than expected, if so, we need to delete the extra ones.
	diff := existingPCSGReplicas - int(sc.pcsg.Spec.Replicas)
	if diff > 0 {
		pcsgObjectKey := client.ObjectKeyFromObject(sc.pcsg)
		logger.Info("Found more PodCliques than expected, triggering deletion of excess PodCliques", "expected", int(sc.pcsg.Spec.Replicas), "existing", existingPCSGReplicas, "diff", diff)
		reason := "Delete excess PodCliqueScalingGroup replicas"
		replicaIndicesToDelete := computePCSGReplicasToDelete(existingPCSGReplicas, int(sc.pcsg.Spec.Replicas))
		deletionTasks := r.createDeleteTasks(logger, sc.pgs, pcsgObjectKey.Name, replicaIndicesToDelete, reason)
		if err := r.triggerDeletionOfPodCliques(sc.ctx, logger, pcsgObjectKey, deletionTasks); err != nil {
			return err
		}
		return sc.refreshExistingPCLQs(sc.pcsg)
	}
	return nil
}

func getExistingNonTerminatingPCSGReplicas(existingPCLQs []grovecorev1alpha1.PodClique) int {
	existingIndices := make([]string, 0, len(existingPCLQs))
	for _, pclq := range existingPCLQs {
		if k8sutils.IsResourceTerminating(pclq.ObjectMeta) {
			continue
		}
		pcsgReplicaIndex, ok := pclq.Labels[apicommon.LabelPodCliqueScalingGroupReplicaIndex]
		if !ok {
			continue
		}
		existingIndices = append(existingIndices, pcsgReplicaIndex)
	}
	return len(lo.Uniq(existingIndices))
}

func computePCSGReplicasToDelete(existingReplicas, expectedReplicas int) []string {
	indices := make([]string, 0, existingReplicas-expectedReplicas)
	for i := expectedReplicas; i < existingReplicas; i++ {
		indices = append(indices, strconv.Itoa(i))
	}
	return indices
}

func (r _resource) createExpectedPCLQs(logger logr.Logger, sc *syncContext) error {
	var tasks []utils.Task
	existingPCLQFQNs := lo.Map(sc.existingPCLQs, func(pclq grovecorev1alpha1.PodClique, _ int) string { return pclq.Name })
	for pcsgReplicaIndex, expectedPCLQNames := range sc.expectedPCLQFQNsPerPCSGReplica {
		for _, pclqFQN := range expectedPCLQNames {
			if slices.Contains(existingPCLQFQNs, pclqFQN) {
				continue
			}
			pclqObjectKey := client.ObjectKey{
				Name:      pclqFQN,
				Namespace: sc.pcsg.Namespace,
			}
			createTask := utils.Task{
				Name: fmt.Sprintf("CreatePodClique-%s", pclqObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreate(ctx, logger, sc.pgs, sc.pcsg, pcsgReplicaIndex, pclqObjectKey)
				},
			}
			tasks = append(tasks, createTask)
		}
	}
	if runResult := utils.RunConcurrently(sc.ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errCodeCreatePodCliques,
			component.OperationSync,
			fmt.Sprintf("Error Create of PodCliques for PodCliqueScalingGroup: %v, run summary: %s", client.ObjectKeyFromObject(sc.pcsg), runResult.GetSummary()),
		)
	}
	return nil
}

func (r _resource) processMinAvailableBreachedPCSGReplicas(logger logr.Logger, sc *syncContext) error {
	// If pcsg.spec.minAvailable is breached, then delegate the responsibility to the PodGangSet reconciler which after
	// termination delay terminate the PodGangSet replica. No further processing is required to be done here.
	minAvailableBreachedPCSGReplicas := len(sc.pcsgIndicesToTerminate) + len(sc.pcsgIndicesToRequeue)
	if int(sc.pcsg.Spec.Replicas)-minAvailableBreachedPCSGReplicas < int(*sc.pcsg.Spec.MinAvailable) {
		return errPCCGMinAvailableBreached
	}
	// If pcsg.spec.minAvailable is not breached but if there is one more PCSG replica for which there is at least one PCLQ that has
	// its minAvailable breached for a duration > terminationDelay then gang terminate such PCSG replicas.
	if len(sc.pcsgIndicesToTerminate) > 0 {
		logger.Info("Identified PodCliqueScalingGroup indices for gang termination", "indices", sc.pcsgIndicesToTerminate)
		reason := fmt.Sprintf("Delete PodCliques %v for PodCliqueScalingGroup %v which have breached MinAvailable longer than TerminationDelay: %s", sc.pcsgIndicesToTerminate, client.ObjectKeyFromObject(sc.pcsg), sc.pgs.Spec.Template.TerminationDelay.Duration)
		pclqGangTerminationTasks := r.createDeleteTasks(logger, sc.pgs, sc.pcsg.Name, sc.pcsgIndicesToTerminate, reason)
		if err := r.triggerDeletionOfPodCliques(sc.ctx, logger, client.ObjectKeyFromObject(sc.pcsg), pclqGangTerminationTasks); err != nil {
			return err
		}
		return groveerr.New(groveerr.ErrCodeRequeueAfter,
			component.OperationSync,
			fmt.Sprintf("Requeuing post gang termination of PodCliqueScalingGroup replicas: %v", pclqGangTerminationTasks),
		)
	}
	return nil
}

func getMinAvailableBreachedPCSGIndices(logger logr.Logger, existingPCLQs []grovecorev1alpha1.PodClique, terminationDelay time.Duration) (pcsgIndicesToTerminate []string, pcsgIndicesToRequeue []string) {
	now := time.Now()
	// group existing PCLQs by PCSG replica index. These are PCLQs that belong to one replica of PCSG.
	pcsgReplicaIndexPCLQs := componentutils.GroupPCLQsByPCSGReplicaIndex(existingPCLQs)
	// For each PCSG replica check if minAvailable for any constituent PCLQ has been violated. Those PCSG replicas should be marked for termination.
	for pcsgReplicaIndex, pclqs := range pcsgReplicaIndexPCLQs {
		pclqNames, minWaitFor := componentutils.GetMinAvailableBreachedPCLQInfo(pclqs, terminationDelay, now)
		if len(pclqNames) > 0 {
			logger.Info("minAvailable breached for PCLQs", "pcsgReplicaIndex", pcsgReplicaIndex, "pclqNames", pclqNames, "minWaitFor", minWaitFor)
			if minWaitFor <= 0 {
				pcsgIndicesToTerminate = append(pcsgIndicesToTerminate, pcsgReplicaIndex)
			} else {
				pcsgIndicesToRequeue = append(pcsgIndicesToRequeue, pcsgReplicaIndex)
			}
		}
	}
	return
}

// getExpectedPodCliqueFQNsByPCSGReplica computes expected PCLQ names per expected PCSG replica.
// It returns a map with the key being the PCSG replica index and the value is the expected PCLQ FQNs for that replica. In addition
// it also returns the total number of expected PCLQs.
func getExpectedPodCliqueFQNsByPCSGReplica(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) map[int][]string {
	var (
		expectedPCLQFQNs = make(map[int][]string)
	)
	for pcsgReplicaIndex := range int(pcsg.Spec.Replicas) {
		pclqFQNs := lo.Map(pcsg.Spec.CliqueNames, func(cliqueName string, _ int) string {
			return apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: pcsgReplicaIndex,
			}, cliqueName)
		})
		expectedPCLQFQNs[pcsgReplicaIndex] = pclqFQNs
	}
	return expectedPCLQFQNs
}

func (r _resource) getExistingPCLQs(ctx context.Context, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) ([]grovecorev1alpha1.PodClique, error) {
	existingPCLQs, err := componentutils.GetPCLQsByOwner(ctx, r.client, constants.KindPodCliqueScalingGroup, client.ObjectKeyFromObject(pcsg), getPodCliqueSelectorLabels(pcsg.ObjectMeta))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodCliquesForPCSG,
			component.OperationSync,
			fmt.Sprintf("Unable to fetch existing PodCliques for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	return existingPCLQs, nil
}

func getExpectedPCLQPodTemplateHashMap(pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) map[string]string {
	pclqFQNToHash := make(map[string]string)
	pcsgPCLQNames := pcsg.Spec.CliqueNames
	for _, pcsgCliqueName := range pcsgPCLQNames {
		pclqTemplateSpec, ok := lo.Find(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
			return pclqTemplateSpec.Name == pcsgCliqueName
		})
		if !ok {
			continue
		}
		podTemplateHash := componentutils.ComputePCLQPodTemplateHash(pclqTemplateSpec, pgs.Spec.Template.PriorityClassName)
		for pcsgReplicaIndex := range int(pcsg.Spec.Replicas) {
			cliqueFQN := apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: pcsgReplicaIndex,
			}, pcsgCliqueName)
			pclqFQNToHash[cliqueFQN] = podTemplateHash
		}
	}
	return pclqFQNToHash
}

// refreshExistingPCLQs removes all the excess PCLQs that belong to any PCSG replica > expectedPCSGReplicas.
// After every successful delete operation of PCSG replica(s), this method will be called to ensure that further processing
// operates on a consistent state of existing PCLQs.
// NOTE: We will be adding expectations usage in this component as well. Then all deletions will be captured as expectations and after every
// deletion of PCSG we will re-queued.
func (sc *syncContext) refreshExistingPCLQs(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	revisedExistingPCLQs := make([]grovecorev1alpha1.PodClique, 0, len(sc.existingPCLQs))
	for _, pclq := range sc.existingPCLQs {
		pcsgReplicaIndexStr, ok := pclq.Labels[apicommon.LabelPodCliqueScalingGroupReplicaIndex]
		if !ok {
			continue
		}
		pcsgReplicaIndex, err := strconv.Atoi(pcsgReplicaIndexStr)
		if err != nil {
			return groveerr.WrapError(err,
				errCodeParsePodCliqueScalingGroupReplicaIndex,
				component.OperationSync,
				fmt.Sprintf("invalid pcsg replica index label value found on PodClique: %v", client.ObjectKeyFromObject(&pclq)),
			)
		}
		if pcsgReplicaIndex < int(pcsg.Spec.Replicas) {
			revisedExistingPCLQs = append(revisedExistingPCLQs, pclq)
		}
	}
	sc.existingPCLQs = revisedExistingPCLQs
	return nil
}
