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

package podgangsetreplica

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	groveevents "github.com/NVIDIA/grove/operator/internal/component/events"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// deletionWork captures the PodGangSet replica deletion work.
type deletionWork struct {
	// deletionTasks are a slice of PodGangSet replica deletion tasks. These are the replicas where there is at least
	// one child resource whose MinAvailableBreached condition is set to true and TerminationDelay has expired.
	deletionTasks []utils.Task
	// pgsIndicesToTerminate are the PodGangSet replica indices for which one or more constituent standalone PCLQ or PCSG
	// have MinAvailableBreached condition set to true for a duration greater than TerminationDelay.
	pgsIndicesToTerminate []int
	// minAvailableBreachedConstituents is map of PGS replica index to PCLQ FQNs which have MinAvailableBreached condition
	// set to true but for these PCLQs TerminationDelay has not expired yet. If there is at least one such PCLQ then
	// a requeue should be done and the reconciler should re-check if the TerminationDelay for these PCLQs eventually expires
	// at which point the corresponding PGS replica should be deleted.
	minAvailableBreachedConstituents map[int][]string
}

func (d deletionWork) shouldRequeue() bool {
	return len(d.minAvailableBreachedConstituents) > 0
}

func (d deletionWork) hasPendingPGSReplicaDeletion() bool {
	return len(d.deletionTasks) > 0
}

func (r _resource) getPGSReplicaDeletionWork(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) (*deletionWork, error) {
	var (
		now              = time.Now()
		pgsObjectKey     = client.ObjectKeyFromObject(pgs)
		terminationDelay = pgs.Spec.Template.TerminationDelay.Duration
		deletionTasks    = make([]utils.Task, 0, pgs.Spec.Replicas)
		work             = &deletionWork{
			minAvailableBreachedConstituents: make(map[int][]string),
		}
	)

	for pgsReplicaIndex := range int(pgs.Spec.Replicas) {
		breachedPCSGNames, minPCSGWaitFor, err := r.getMinAvailableBreachedPCSGs(ctx, pgsObjectKey, pgsReplicaIndex, terminationDelay, now)
		if err != nil {
			return nil, err
		}
		breachedPCLQNames, minPCLQWaitFor, skipPGSReplicaIndex, err := r.getMinAvailableBreachedPCLQsNotInPCSG(ctx, pgs, pgsReplicaIndex, now)
		if err != nil {
			return nil, err
		}
		if skipPGSReplicaIndex {
			continue
		}
		if (len(breachedPCSGNames) > 0 && minPCSGWaitFor <= 0) ||
			(len(breachedPCLQNames) > 0 && minPCLQWaitFor <= 0) {
			// terminate all PodCliques for this PGS replica index
			reason := fmt.Sprintf("Delete all PodCliques for PodGangSet %v with replicaIndex :%d due to MinAvailable breached longer than TerminationDelay: %s", pgsObjectKey, pgsReplicaIndex, terminationDelay)
			pclqGangTerminationTask := r.createPGSReplicaDeleteTask(logger, pgs, pgsReplicaIndex, reason)
			deletionTasks = append(deletionTasks, pclqGangTerminationTask)
			work.pgsIndicesToTerminate = append(work.pgsIndicesToTerminate, pgsReplicaIndex)
		} else if len(breachedPCSGNames) > 0 || len(breachedPCLQNames) > 0 {
			work.minAvailableBreachedConstituents[pgsReplicaIndex] = append(breachedPCLQNames, breachedPCLQNames...)
		}
	}
	work.deletionTasks = deletionTasks
	return work, nil
}

func (r _resource) getMinAvailableBreachedPCSGs(ctx context.Context, pgsObjKey client.ObjectKey, pgsReplicaIndex int, terminationDelay time.Duration, since time.Time) ([]string, time.Duration, error) {
	pcsgList := &grovecorev1alpha1.PodCliqueScalingGroupList{}
	if err := r.client.List(ctx,
		pcsgList,
		client.InNamespace(pgsObjKey.Namespace),
		client.MatchingLabels(lo.Assign(
			apicommon.GetDefaultLabelsForPodGangSetManagedResources(pgsObjKey.Name),
			map[string]string{
				apicommon.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
			},
		)),
	); err != nil {
		return nil, 0, err
	}
	breachedPCSGNames, minWaitFor := getMinAvailableBreachedPCSGInfo(pcsgList.Items, terminationDelay, since)
	return breachedPCSGNames, minWaitFor, nil
}

func (r _resource) getMinAvailableBreachedPCLQsNotInPCSG(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex int, since time.Time) (breachedPCLQNames []string, minWaitFor time.Duration, skipPGSReplica bool, err error) {
	pclqFQNsNotInPCSG := make([]string, 0, len(pgs.Spec.Template.Cliques))
	for _, pclqTemplateSpec := range pgs.Spec.Template.Cliques {
		if !isPCLQInPCSG(pclqTemplateSpec.Name, pgs.Spec.Template.PodCliqueScalingGroupConfigs) {
			pclqFQNsNotInPCSG = append(pclqFQNsNotInPCSG, apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex}, pclqTemplateSpec.Name))
		}
	}
	var (
		pclqs            []grovecorev1alpha1.PodClique
		notFoundPCLQFQNs []string
	)
	pclqs, notFoundPCLQFQNs, err = r.getExistingPCLQsByNames(ctx, pgs.Namespace, pclqFQNsNotInPCSG)
	if err != nil {
		return
	}
	if len(notFoundPCLQFQNs) > 0 {
		skipPGSReplica = true
		return
	}
	breachedPCLQNames, minWaitFor = componentutils.GetMinAvailableBreachedPCLQInfo(pclqs, pgs.Spec.Template.TerminationDelay.Duration, since)
	return
}

// getExistingPCLQsByNames fetches PodClique objects. It returns the PCLQ objects that it found and a slice of PCLQ FQNs for which no PCLQ object exists. If there is an error it just returns the error.
func (r _resource) getExistingPCLQsByNames(ctx context.Context, namespace string, pclqFQNs []string) (pclqs []grovecorev1alpha1.PodClique, notFoundPCLQFQNs []string, err error) {
	for _, pclqFQN := range pclqFQNs {
		pclq := grovecorev1alpha1.PodClique{}
		if err = r.client.Get(ctx, client.ObjectKey{Name: pclqFQN, Namespace: namespace}, &pclq); err != nil {
			if apierrors.IsNotFound(err) {
				notFoundPCLQFQNs = append(notFoundPCLQFQNs, pclqFQN)
				continue
			}
			return nil, nil, err
		}
		pclqs = append(pclqs, pclq)
	}
	return pclqs, notFoundPCLQFQNs, nil
}

// getMinAvailableBreachedPCSGInfo filters PodCliqueScalingGroups that have grovecorev1alpha1.ConditionTypeMinAvailableBreached set to true.
// It returns the names of all such PodCliqueScalingGroups and minimum of all the waitDurations.
func getMinAvailableBreachedPCSGInfo(pcsgs []grovecorev1alpha1.PodCliqueScalingGroup, terminationDelay time.Duration, since time.Time) ([]string, time.Duration) {
	pcsgCandidateNames := make([]string, 0, len(pcsgs))
	waitForDurations := make([]time.Duration, 0, len(pcsgs))
	for _, pcsg := range pcsgs {
		cond := meta.FindStatusCondition(pcsg.Status.Conditions, constants.ConditionTypeMinAvailableBreached)
		if cond == nil {
			continue
		}
		if cond.Status == metav1.ConditionTrue {
			pcsgCandidateNames = append(pcsgCandidateNames, pcsg.Name)
			waitFor := terminationDelay - since.Sub(cond.LastTransitionTime.Time)
			waitForDurations = append(waitForDurations, waitFor)
		}
	}
	if len(waitForDurations) == 0 {
		return pcsgCandidateNames, 0
	}
	slices.Sort(waitForDurations)
	return pcsgCandidateNames, waitForDurations[0]
}

// createPGSReplicaDeleteTask creates a Task to delete all the PodCliques that are part of a PGS replica.
func (r _resource) createPGSReplicaDeleteTask(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex int, reason string) utils.Task {
	return utils.Task{
		Name: fmt.Sprintf("DeletePGSReplicaPodCliques-%d", pgsReplicaIndex),
		Fn: func(ctx context.Context) error {
			if err := r.client.DeleteAllOf(ctx,
				&grovecorev1alpha1.PodClique{},
				client.InNamespace(pgs.Namespace),
				client.MatchingLabels(
					lo.Assign(
						apicommon.GetDefaultLabelsForPodGangSetManagedResources(pgs.Name),
						map[string]string{
							apicommon.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
						},
					))); err != nil {
				logger.Error(err, "failed to delete PodCliques for PGS Replica index", "pgsReplicaIndex", pgsReplicaIndex, "reason", reason)
				r.eventRecorder.Eventf(pgs, corev1.EventTypeWarning, groveevents.ReasonPodGangSetReplicaDeleteFailed, "Error deleting PodGangSet replica %d: %v", pgsReplicaIndex, err)
				return err
			}
			logger.Info("Deleted PGS replica PodCliques", "pgsReplicaIndex", pgsReplicaIndex, "reason", reason)
			r.eventRecorder.Eventf(pgs, corev1.EventTypeNormal, groveevents.ReasonPodGangSetReplicaDeleteSuccessful, "PodGangSet replica %d deleted", pgsReplicaIndex)
			return nil
		},
	}
}

func isPCLQInPCSG(pclqName string, pcsgConfigs []grovecorev1alpha1.PodCliqueScalingGroupConfig) bool {
	return lo.Reduce(pcsgConfigs, func(agg bool, pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig, _ int) bool {
		return agg || slices.Contains(pcsgConfig.CliqueNames, pclqName)
	}, false)
}
