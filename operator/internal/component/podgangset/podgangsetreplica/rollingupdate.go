package podgangsetreplica

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r _resource) orchestrateRollingUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pgsIndicesToTerminate, minAvailableBreachedPGSReplicaIndices []int) error {
	updateWork, err := r.computePendingUpdateWork(ctx, pgs, pgsIndicesToTerminate)
	if err != nil {
		return err
	}

	var lastUpdatedPGSReplicaIndex *int32
	if pgs.Status.RollingUpdateProgress.CurrentlyUpdating != nil {
		lastUpdatedPGSReplicaIndex = &pgs.Status.RollingUpdateProgress.CurrentlyUpdating.ReplicaIndex
		if !updateWork.currentlyUpdatingReplicaInfo.updateProgress.done {
			if err = r.updatePGSWithReplicaUpdateProgress(ctx, logger, pgs, updateWork.currentlyUpdatingReplicaInfo.updateProgress); err != nil {
				return err
			}
			return groveerr.New(
				groveerr.ErrCodeContinueReconcileAndRequeue,
				component.OperationSync,
				fmt.Sprintf("rolling update of PodGangSet replica index %d is not completed", updateWork.currentlyUpdatingReplicaInfo.replicaIndex),
			)
		}
	}

	// pick the next replica index to update.
	nextReplicaToUpdate := updateWork.getNextReplicaToUpdate(pgs, minAvailableBreachedPGSReplicaIndices)
	if err = r.updatePGSWithNextSelectedReplica(ctx, logger, pgs, lastUpdatedPGSReplicaIndex, nextReplicaToUpdate); err != nil {
		return err
	}

	if nextReplicaToUpdate != nil {
		return groveerr.New(
			groveerr.ErrCodeContinueReconcileAndRequeue,
			component.OperationSync,
			fmt.Sprintf("commencing rolling update of PodGangSet replica index %d", nextReplicaToUpdate),
		)
	}
	return nil
}

func (r _resource) computePendingUpdateWork(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, pgsIndicesToTerminate []int) (*pendingUpdateWork, error) {
	replicaInfos, err := r.getPGSReplicaInfos(ctx, pgs, pgsIndicesToTerminate)
	if err != nil {
		return nil, err
	}
	// iterate through each replica
	pendingWork := &pendingUpdateWork{}
	for _, replicaInfo := range replicaInfos {
		updateProgress := replicaInfo.getUpdateProgress(pgs)
		replicaInfo.updateProgress = updateProgress

		if pgs.Status.RollingUpdateProgress.CurrentlyUpdating != nil &&
			pgs.Status.RollingUpdateProgress.CurrentlyUpdating.ReplicaIndex == int32(replicaInfo.replicaIndex) {
			pendingWork.currentlyUpdatingReplicaInfo = &replicaInfo
			continue
		}

		if !updateProgress.done {
			pendingWork.pendingUpdateReplicaInfos = append(pendingWork.pendingUpdateReplicaInfos, replicaInfo)
		}
	}
	return pendingWork, nil
}

func (r _resource) getPGSReplicaInfos(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, pgsIndicesToTerminate []int) ([]pgsReplicaInfo, error) {
	pgsObjectKey := client.ObjectKeyFromObject(pgs)
	pclqsByPGSIndex, err := componentutils.GetPCLQsByOwnerReplicaIndex(ctx, r.client, constants.KindPodGangSet, client.ObjectKeyFromObject(pgs), apicommon.GetDefaultLabelsForPodGangSetManagedResources(pgs.Name))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPCLQs,
			component.OperationSync,
			fmt.Sprintf("could not list PCLQs for PGS: %v", pgsObjectKey),
		)
	}
	pcsgsByPGSIndex, err := componentutils.GetPCSGsByPGSReplicaIndex(ctx, r.client, client.ObjectKeyFromObject(pgs))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPCSGs,
			component.OperationSync,
			fmt.Sprintf("could not list PCSGs for PGS: %v", pgsObjectKey),
		)
	}
	replicaInfos := make([]pgsReplicaInfo, 0, pgs.Spec.Replicas)
	for pgsReplicaIndex := range int(pgs.Spec.Replicas) {
		if slices.Contains(pgsIndicesToTerminate, pgsReplicaIndex) {
			continue
		}
		pgsReplicaIndexStr := strconv.Itoa(pgsReplicaIndex)
		replicaInfos = append(replicaInfos, pgsReplicaInfo{
			replicaIndex: pgsReplicaIndex,
			pclqs:        pclqsByPGSIndex[pgsReplicaIndexStr],
			pcsgs:        pcsgsByPGSIndex[pgsReplicaIndexStr],
		})
	}
	return replicaInfos, nil
}

func (r _resource) updatePGSWithReplicaUpdateProgress(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, currentReplicaUpdateProgress replicaUpdateProgress) error {
	pgs.Status.RollingUpdateProgress.CurrentlyUpdating.UpdatedPodCliques = currentReplicaUpdateProgress.updatedPCLQFQNs
	pgs.Status.RollingUpdateProgress.CurrentlyUpdating.UpdatedPodCliqueScalingGroups = currentReplicaUpdateProgress.updatedPCSGFQNs
	logger.Info("Updating PodGangSet status with newly updated PodCliques and PodClique")
	if err := r.updateRollingUpdateProgressStatus(ctx, logger, pgs); err != nil {
		logger.Error(err, "failed to update rolling update progress", "replicaIndex", pgs.Status.RollingUpdateProgress.CurrentlyUpdating.ReplicaIndex)
		return err
	}
	return nil
}

func (r _resource) updatePGSWithNextSelectedReplica(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, previouslyUpdatedPGSReplica *int32, nextPGSReplicaToUpdate *int) error {
	if nextPGSReplicaToUpdate == nil {
		logger.Info("Rolling update has completed")
		pgs.Status.RollingUpdateProgress.UpdateEndedAt = ptr.To(metav1.Now())
		pgs.Status.RollingUpdateProgress.CurrentlyUpdating = nil
	} else {
		logger.Info("Initiating rolling update for next replica index", "nextReplicaIndex", *nextPGSReplicaToUpdate)
		pgs.Status.RollingUpdateProgress.CurrentlyUpdating = &grovecorev1alpha1.PodGangSetReplicaRollingUpdateProgress{
			ReplicaIndex:    int32(*nextPGSReplicaToUpdate),
			UpdateStartedAt: metav1.Now(),
		}
	}
	// update the PodGangSet.Status
	if previouslyUpdatedPGSReplica != nil {
		pgs.Status.UpdatedReplicas++
	}
	return r.updateRollingUpdateProgressStatus(ctx, logger, pgs)
}

func (r _resource) updateRollingUpdateProgressStatus(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) error {
	if err := r.client.Status().Update(ctx, pgs); err != nil {
		return groveerr.WrapError(
			err,
			errCodeUpdatePGSStatus,
			component.OperationSync,
			"could not update rolling update progress",
		)
	}
	logger.Info("Updated the PodGangSet status with rolling update progress")
	return nil
}

func orderPGSReplicaInfoForPGS(pgs *grovecorev1alpha1.PodGangSet, minAvailableBreachedPGSReplicaIndices []int) func(a, b pgsReplicaInfo) int {
	return func(a, b pgsReplicaInfo) int {
		scheduledPodsInA, scheduledPodsInB := a.getNumScheduledPods(pgs), b.getNumScheduledPods(pgs)
		// 1. Pick the PGS Replica that has no scheduled pods.
		if scheduledPodsInA == 0 && scheduledPodsInB != 0 {
			return -1
		} else if scheduledPodsInA != 0 && scheduledPodsInB == 0 {
			return 1
		}

		// 2. Pick the replicas which have the minAvailableBreached condition set to true, but the terminationDelay has not expired yet.
		// The replicas with minAvailableBreached with terminationDelay expired are deleted before the rolling update is started.
		minAvailableBreachedForA := slices.Contains(minAvailableBreachedPGSReplicaIndices, a.replicaIndex)
		minAvailableBreachedForB := slices.Contains(minAvailableBreachedPGSReplicaIndices, b.replicaIndex)
		if minAvailableBreachedForA && !minAvailableBreachedForB {
			return -1
		} else if !minAvailableBreachedForA && minAvailableBreachedForB {
			return 1
		}

		// 3. If all replicas are healthy, then pick the replicas in reverse ordinal value.
		if a.replicaIndex < b.replicaIndex {
			return 1
		} else {
			return -1
		}
	}
}

type pendingUpdateWork struct {
	pendingUpdateReplicaInfos    []pgsReplicaInfo
	currentlyUpdatingReplicaInfo *pgsReplicaInfo
}

type pgsReplicaInfo struct {
	replicaIndex   int
	pclqs          []grovecorev1alpha1.PodClique
	pcsgs          []grovecorev1alpha1.PodCliqueScalingGroup
	updateProgress replicaUpdateProgress
}

type replicaUpdateProgress struct {
	done            bool
	updatedPCLQFQNs []string
	updatedPCSGFQNs []string
}

func (w *pendingUpdateWork) getNextReplicaToUpdate(pgs *grovecorev1alpha1.PodGangSet, minAvailableBreachedPGSReplicaIndices []int) *int {
	slices.SortFunc(w.pendingUpdateReplicaInfos, orderPGSReplicaInfoForPGS(pgs, minAvailableBreachedPGSReplicaIndices))
	if len(w.pendingUpdateReplicaInfos) > 0 {
		return &w.pendingUpdateReplicaInfos[0].replicaIndex
	}
	return nil
}

func (pri *pgsReplicaInfo) getUpdateProgress(pgs *grovecorev1alpha1.PodGangSet) replicaUpdateProgress {
	progress := replicaUpdateProgress{}
	updateComplete := true
	for _, pclq := range pri.pclqs {
		if isPCLQUpdateComplete(&pclq, *pgs.Status.GenerationHash) {
			progress.updatedPCLQFQNs = append(progress.updatedPCLQFQNs, pclq.Name)
		} else {
			updateComplete = false
		}
	}
	for _, pcsg := range pri.pcsgs {
		if isPCSGUpdateComplete(&pcsg, *pgs.Status.GenerationHash) {
			progress.updatedPCSGFQNs = append(progress.updatedPCSGFQNs, pcsg.Name)
		} else {
			updateComplete = false
		}
	}
	progress.done = updateComplete
	return progress
}

// getNumScheduledPods returns a normalized value, which is a sum of number of pending pods
// in individual PodCliques, and the number of pending pods of PodCliqueScalingGroup PodCliques.
func (pri *pgsReplicaInfo) getNumScheduledPods(pgs *grovecorev1alpha1.PodGangSet) int {
	noScheduled := 0
	for _, pclq := range pri.pclqs {
		noScheduled += int(pclq.Status.ScheduledReplicas)
	}

	for _, pcsg := range pri.pcsgs {
		for _, cliqueName := range pcsg.Spec.CliqueNames {
			pclqTemplateSpec, _ := lo.Find(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
				return pclqTemplateSpec.Name == cliqueName
			})
			noScheduled += int(pcsg.Status.ScheduledReplicas * *pclqTemplateSpec.Spec.MinAvailable)
		}
	}
	return noScheduled
}

func isPCLQUpdateComplete(pclq *grovecorev1alpha1.PodClique, pgsGenerationHash string) bool {
	if pclq.Status.RollingUpdateProgress == nil ||
		pclq.Status.RollingUpdateProgress.UpdateEndedAt == nil ||
		pclq.Status.RollingUpdateProgress.PodGangSetGenerationHash != pgsGenerationHash ||
		len(pclq.Status.RollingUpdateProgress.UpdatedPods) != int(pclq.Spec.Replicas) {
		// TODO: pclq.status.availableReplicas < pclq.spec.minAvailable should also be included in this check
		return false
	}
	return true
}

func isPCSGUpdateComplete(pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pgsGenerationHash string) bool {
	if pcsg.Status.RollingUpdateProgress == nil ||
		pcsg.Status.RollingUpdateProgress.UpdateEndedAt == nil ||
		pcsg.Status.RollingUpdateProgress.PodGangSetGenerationHash != pgsGenerationHash ||
		pcsg.Status.UpdatedReplicas != pcsg.Spec.Replicas {
		// TODO: pcsg.status.availableReplicas < pcsg.spec.minAvailable should also be included in this check
		return false
	}
	return true
}

func isRollingUpdateInProgress(pgs *grovecorev1alpha1.PodGangSet) bool {
	return pgs.Status.RollingUpdateProgress != nil && pgs.Status.RollingUpdateProgress.UpdateEndedAt == nil
}
