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
	"fmt"
	"strconv"
	"strings"
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errListPodClique                 grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE"
	errSyncPodClique                 grovecorev1alpha1.ErrorCode = "ERR_SYNC_PODCLIQUE"
	errDeletePodClique               grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE"
	errCodeListPodCliqueScalingGroup grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUESCALINGGROUP"
	errCodeListPodCliques            grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUES"
	errCodeCreatePodClique           grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODCLIQUE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of PodClique component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the PodClique Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) ([]string, error) {
	logger.Info("Looking for existing PodCliques")
	pclqPartialObjMetaList, err := k8sutils.ListExistingPartialObjectMetadata(ctx,
		r.client,
		grovecorev1alpha1.SchemeGroupVersion.WithKind("PodClique"),
		pgsObjMeta,
		getPodCliqueSelectorLabels(pgsObjMeta))
	if err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodCliques,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliques for PodGangSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pgsObjMeta, pclqPartialObjMetaList), nil
}

// Sync synchronizes all resources that the PodClique Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) error {
	expectedPCLQNames, _ := componentutils.GetExpectedPCLQNamesGroupByOwner(pgs)

	if err := r.triggerDeletionOfExcessPCLQs(ctx, logger, pgs, expectedPCLQNames); err != nil {
		return err
	}

	terminationDelay := pgs.Spec.Template.TerminationDelay.Duration
	podGangsRequiringRequeue, err := r.checkMinAvailableBreachAndDeletePGSReplicaPodGang(ctx, logger, pgs, terminationDelay)
	if err != nil {
		return err
	}
	if err = r.createExpectedPCLQs(ctx, logger, pgs, expectedPCLQNames); err != nil {
		return err
	}

	if len(podGangsRequiringRequeue) > 0 {
		logger.Info("Found PCLQs with MinAvailable breached but which have not crossed TerminationDelay", "podGangs", podGangsRequiringRequeue, "terminationDelay", terminationDelay)
		return groveerr.New(groveerr.ErrCodeContinueReconcileAndRequeue,
			component.OperationSync,
			"Requeuing to re-process PCLQs that have breached MinAvailable but not crossed TerminationDelay",
		)
	}

	return nil
}

func (r _resource) triggerDeletionOfExcessPCLQs(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, expectedPCLQNames []string) error {
	existingPCLQNames, err := r.GetExistingResourceNames(ctx, logger, pgs.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Unable to fetch existing PodClique names for PodGangSet: %v", client.ObjectKeyFromObject(pgs)),
		)
	}
	// Check if the number of existing PodCliques is greater than expected, if so, we need to delete the extra ones.
	diff := len(existingPCLQNames) - len(expectedPCLQNames)
	if diff > 0 {
		logger.Info("Found more PodCliques than expected", "expected", expectedPCLQNames, "existing", existingPCLQNames)
		logger.Info("Triggering deletion of extra PodCliques", "count", diff)
		// collect the names of the extra PodCliques to delete
		deletionCandidateNames, err := getPodCliqueNamesToDelete(pgs.Name, int(pgs.Spec.Replicas), existingPCLQNames)
		if err != nil {
			return err
		}
		deletePCLQTasks := r.createDeleteTasks(logger, pgs.Namespace, deletionCandidateNames)
		return r.triggerDeletionOfPodCliques(ctx, logger, client.ObjectKeyFromObject(pgs), deletePCLQTasks)
	}
	return nil
}

func (r _resource) createExpectedPCLQs(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, expectedPCLQNames []string) error {
	tasks := make([]utils.Task, 0, len(expectedPCLQNames))

	for pgsReplica := range pgs.Spec.Replicas {
		for _, expectedPCLQName := range expectedPCLQNames {
			pclqObjectKey := client.ObjectKey{
				Name:      grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, expectedPCLQName),
				Namespace: pgs.Namespace,
			}
			createTask := utils.Task{
				Name: fmt.Sprintf("CreatePodClique-%s", pclqObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreate(ctx, logger, pgs, pgsReplica, pclqObjectKey)
				},
			}
			tasks = append(tasks, createTask)
		}
	}
	if runResult := utils.RunConcurrently(ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error Create of PodCliques for PodGangSet: %v, run summary: %s", client.ObjectKeyFromObject(pgs), runResult.GetSummary()),
		)
	}
	return nil
}

func (r _resource) getMinAvailableBreachedPCSGsForPGSReplica(ctx context.Context, pgsObjKey client.ObjectKey, pgsReplicaIndex int, terminationDelay time.Duration, since time.Time) ([]string, time.Duration, error) {
	pcsgs, err := componentutils.GetPCSGsForPGSReplicaIndex(ctx, r.client, pgsObjKey, pgsReplicaIndex)
	if err != nil {
		return nil, 0, groveerr.WrapError(err,
			errCodeListPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("failed to list PodCliqueScalingGroups for PodGangSet: %v,  replica Index: %d", pgsObjKey, pgsReplicaIndex))
	}

	breachedPCSGNames, minWaitFor := componentutils.GetMinAvailableBreachedPCSGInfo(pcsgs, terminationDelay, since)
	return breachedPCSGNames, minWaitFor, nil
}

func (r _resource) getMinAvailableBreachedPCLQsNotInPCSGForPGSReplica(ctx context.Context, pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex int, since time.Time) (breachedPCLQNames []string, minWaitFor time.Duration, skipPGSReplica bool, err error) {
	pclqFQNsNotInPCSG := componentutils.GetPodCliqueFQNsForPGSReplicaNotInPCSG(pgs, pgsReplicaIndex)
	pclqs, notFoundPCLQFQNs, err := componentutils.GetPCLQsByNames(ctx, r.client, pgs.Namespace, pclqFQNsNotInPCSG)
	if err != nil {
		err = groveerr.WrapError(err,
			errCodeListPodCliques,
			component.OperationSync,
			fmt.Sprintf("failed to list PodCliques: %v for PodGangSet: %v, replica Index: %d", pclqFQNsNotInPCSG, client.ObjectKeyFromObject(pgs), pgsReplicaIndex),
		)
		return
	}
	if len(notFoundPCLQFQNs) > 0 {
		skipPGSReplica = true
		return
	}
	breachedPCLQNames, minWaitFor = componentutils.GetMinAvailableBreachedPCLQInfo(pclqs, pgs.Spec.Template.TerminationDelay.Duration, since)
	return
}

func (r _resource) checkMinAvailableBreachAndDeletePGSReplicaPodGang(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, terminationDelay time.Duration) ([]string, error) {
	now := time.Now()
	pgsObjectKey := client.ObjectKeyFromObject(pgs)
	pgsReplicaIndexRequiringRequeue := make([]string, 0, pgs.Spec.Replicas)
	deletionTasks := make([]utils.Task, 0, pgs.Spec.Replicas)
	for pgsReplicaIndex := range int(pgs.Spec.Replicas) {
		breachedPCSGNames, minPCSGWaitFor, err := r.getMinAvailableBreachedPCSGsForPGSReplica(ctx, pgsObjectKey, pgsReplicaIndex, terminationDelay, now)
		if err != nil {
			return nil, err
		}
		breachedPCLQNames, minPCLQWaitFor, skipPGSReplicaIndex, err := r.getMinAvailableBreachedPCLQsNotInPCSGForPGSReplica(ctx, pgs, pgsReplicaIndex, now)
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
			pclqGangTerminationTask := r.createPGSReplicaDeleteTask(logger, pgsObjectKey, pgsReplicaIndex, reason)
			deletionTasks = append(deletionTasks, pclqGangTerminationTask)
		} else if len(breachedPCSGNames) > 0 || len(breachedPCLQNames) > 0 {
			pgsReplicaIndexRequiringRequeue = append(pgsReplicaIndexRequiringRequeue, strconv.Itoa(pgsReplicaIndex))
		}
	}

	return pgsReplicaIndexRequiringRequeue, r.triggerDeletionOfPodCliques(ctx, logger, pgsObjectKey, deletionTasks)
}

func (r _resource) createPGSReplicaDeleteTask(logger logr.Logger, pgsObjKey client.ObjectKey, pgsReplicaIndex int, reason string) utils.Task {
	return utils.Task{
		Name: fmt.Sprintf("DeletePGSReplicaPodCliques-%d", pgsReplicaIndex),
		Fn: func(ctx context.Context) error {
			if err := r.client.DeleteAllOf(ctx,
				&grovecorev1alpha1.PodClique{},
				client.InNamespace(pgsObjKey.Namespace),
				client.MatchingLabels(
					lo.Assign(
						k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjKey.Name),
						map[string]string{
							grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
						},
					))); err != nil {
				logger.Error(err, "failed to delete PodCliques for PGS Replica index", "pgsReplicaIndex", pgsReplicaIndex, "reason", reason)
				return err
			}
			return nil
		},
	}
}

func (r _resource) triggerDeletionOfPodCliques(ctx context.Context, logger logr.Logger, pgsObjKey client.ObjectKey, deletionTasks []utils.Task) error {
	if len(deletionTasks) == 0 {
		return nil
	}
	if runResult := utils.RunConcurrently(ctx, logger, deletionTasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errDeletePodClique,
			component.OperationSync,
			fmt.Sprintf("Error deleting PodCliques for PodGangSet: %v", pgsObjKey.Name),
		)
	}
	logger.Info("Deleted PodCliques of PodGangSet", "pgsObjectKey", pgsObjKey)
	return nil
}

func (r _resource) createDeleteTasks(logger logr.Logger, namespace string, targetPCLQNames []string) []utils.Task {
	deletionTasks := make([]utils.Task, 0, len(targetPCLQNames))
	for _, pclqName := range targetPCLQNames {
		pclqObjectKey := client.ObjectKey{
			Name:      pclqName,
			Namespace: namespace,
		}
		pclq := emptyPodClique(pclqObjectKey)
		task := utils.Task{
			Name: "DeleteExcessPodClique-" + pclqName,
			Fn: func(ctx context.Context) error {
				if err := client.IgnoreNotFound(r.client.Delete(ctx, pclq)); err != nil {
					logger.Error(err, "failed to delete excess PodClique", "objectKey", pclqObjectKey)
					return err
				}
				return nil
			},
		}
		deletionTasks = append(deletionTasks, task)
	}
	return deletionTasks
}

func getPodCliqueNamesToDelete(pgsName string, pgsReplicas int, existingPCLQNames []string) ([]string, error) {
	pclqsToDelete := make([]string, 0, len(existingPCLQNames))
	for _, pclqName := range existingPCLQNames {
		extractedPGSReplica, err := utils.GetPodGangSetReplicaIndexFromPodCliqueFQN(pgsName, pclqName)
		if err != nil {
			return nil, groveerr.WrapError(err,
				errSyncPodClique,
				component.OperationSync,
				fmt.Sprintf("Failed to extract PodGangSet replica index from PodClique name: %s", pclqName),
			)
		}
		if extractedPGSReplica >= pgsReplicas {
			// If the extracted replica index is greater than or equal to the number of replicas in the PodGangSet,
			// then this PodClique is an extra one that should be deleted.
			pclqsToDelete = append(pclqsToDelete, pclqName)
		}
	}
	return pclqsToDelete, nil
}

// Delete deletes all resources that the PodClique Operator manages.
func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering deletion of PodCliques")
	existingPCLQNames, err := r.GetExistingResourceNames(ctx, logger, pgsObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errListPodClique,
			component.OperationDelete,
			fmt.Sprintf("Unable to fetch existing PodClique names for PodGangSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjectMeta)),
		)
	}
	deleteTasks := make([]utils.Task, 0, len(existingPCLQNames))
	for _, pclqName := range existingPCLQNames {
		pclqObjectKey := client.ObjectKey{Name: pclqName, Namespace: pgsObjectMeta.Namespace}
		task := utils.Task{
			Name: "DeletePodClique-" + pclqName,
			Fn: func(ctx context.Context) error {
				if err := client.IgnoreNotFound(r.client.Delete(ctx, emptyPodClique(pclqObjectKey))); err != nil {
					return fmt.Errorf("failed to delete PodClique: %v for PodGangSet: %v with error: %w", pclqObjectKey, k8sutils.GetObjectKeyFromObjectMeta(pgsObjectMeta), err)
				}
				return nil
			},
		}
		deleteTasks = append(deleteTasks, task)
	}
	if runResult := utils.RunConcurrently(ctx, logger, deleteTasks); runResult.HasErrors() {
		logger.Error(runResult.GetAggregatedError(), "Error deleting PodCliques", "run summary", runResult.GetSummary())
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errDeletePodClique,
			component.OperationDelete,
			fmt.Sprintf("Error deleting PodCliques for PodGangSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjectMeta)),
		)
	}

	logger.Info("Deleted PodCliques")
	return nil
}

func (r _resource) doCreate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int32, pclqObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate PodClique", "pclqObjectKey", pclqObjectKey)
	pclq := emptyPodClique(pclqObjectKey)
	pgsObjKey := client.ObjectKeyFromObject(pgs)
	if err := r.buildResource(logger, pclq, pgs, int(pgsReplica)); err != nil {
		return err
	}
	if err := r.client.Create(ctx, pclq); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.Info("PodClique creation failed for PodGangSet as it already exists", "pgs", pgsObjKey, "pclq", client.ObjectKeyFromObject(pclq))
			return nil
		}
		return groveerr.WrapError(err,
			errCodeCreatePodClique,
			component.OperationSync,
			fmt.Sprintf("Error creating PodClique: %v for PodGangSet: %v", pclqObjectKey, pgsObjKey),
		)
	}
	logger.Info("triggered create of PodClique for PodGangSet", "pgs", pgsObjKey, "pclqObjectKey", pclqObjectKey)
	return nil
}

func (r _resource) buildResource(logger logr.Logger, pclq *grovecorev1alpha1.PodClique, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int) error {
	var err error
	pclqObjectKey, pgsObjectKey := client.ObjectKeyFromObject(pclq), client.ObjectKeyFromObject(pgs)
	pclqTemplateSpec, foundAtIndex, ok := lo.FindIndexOf(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) bool {
		return strings.HasSuffix(pclq.Name, pclqTemplateSpec.Name)
	})
	if !ok {
		logger.Info("PodClique template spec not found in PodGangSet", "podCliqueObjectKey", pclqObjectKey, "podGangSetObjectKey", pgsObjectKey)
		return groveerr.New(errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("PodCliqueTemplateSpec for PodClique: %v not found in PodGangSet: %v", pclqObjectKey, pgsObjectKey),
		)
	}
	// Set PodClique.ObjectMeta
	// ------------------------------------
	if err = controllerutil.SetControllerReference(pgs, pclq, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for PodClique: %v", client.ObjectKeyFromObject(pclq)),
		)
	}
	pclq.Labels = getLabels(pgs, pgsReplica, pclqObjectKey, pclqTemplateSpec, grovecorev1alpha1.GeneratePodGangNameForPodCliqueOwnedByPodGangSet(pgs, pgsReplica))
	pclq.Annotations = pclqTemplateSpec.Annotations
	// set PodCliqueSpec
	// ------------------------------------
	pclq.Spec = pclqTemplateSpec.Spec
	var dependentPclqNames []string
	if dependentPclqNames, err = identifyFullyQualifiedStartupDependencyNames(pgs, pclq, pgsReplica, foundAtIndex); err != nil {
		return err
	}
	pclq.Spec.StartsAfter = dependentPclqNames
	return nil
}

func identifyFullyQualifiedStartupDependencyNames(pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique, pgsReplicaIndex, foundAtIndex int) ([]string, error) {
	cliqueStartupType := pgs.Spec.Template.StartupType
	if cliqueStartupType == nil {
		// Ideally this should never happen as the defaulting webhook should set it v1alpha1.CliqueStartupTypeInOrder as the default value.
		// If it is still nil, then by not returning an error we break the API contract. It is a bug that should be fixed.
		return nil, groveerr.New(errSyncPodClique, component.OperationSync, fmt.Sprintf("PodClique: %v has nil StartupType", client.ObjectKeyFromObject(pclq)))
	}
	switch *cliqueStartupType {
	case grovecorev1alpha1.CliqueStartupTypeInOrder:
		return getInOrderStartupDependencies(pgs, pgsReplicaIndex, foundAtIndex), nil
	case grovecorev1alpha1.CliqueStartupTypeExplicit:
		return getExplicitStartupDependencies(pgs.Name, pgsReplicaIndex, pclq), nil
	default:
		return nil, nil
	}
}

func getInOrderStartupDependencies(pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex, foundAtIndex int) []string {
	if foundAtIndex == 0 {
		return []string{}
	}
	previousClique := pgs.Spec.Template.Cliques[foundAtIndex-1]
	// get the name of the previous PodCliqueTemplateSpec
	previousPCLQName := grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgs.Name, Replica: pgsReplicaIndex}, previousClique.Name)
	return []string{previousPCLQName}
}

func getExplicitStartupDependencies(pgsName string, pgsReplicaIndex int, pclq *grovecorev1alpha1.PodClique) []string {
	dependencies := make([]string, 0, len(pclq.Spec.StartsAfter))
	for _, dependency := range pclq.Spec.StartsAfter {
		dependencies = append(dependencies, grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplicaIndex}, dependency))
	}
	return dependencies
}

func getPodCliqueSelectorLabels(pgsObjectMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjectMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePGSPodClique,
		},
	)
}

func getLabels(pgs *grovecorev1alpha1.PodGangSet, pgsReplica int, pclqObjectKey client.ObjectKey, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, podGangName string) map[string]string {
	pclqComponentLabels := map[string]string{
		grovecorev1alpha1.LabelAppNameKey:             pclqObjectKey.Name,
		grovecorev1alpha1.LabelComponentKey:           component.NamePGSPodClique,
		grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplica),
		grovecorev1alpha1.LabelPodGang:                podGangName,
	}
	return lo.Assign(
		pclqTemplateSpec.Labels,
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgs.Name),
		pclqComponentLabels,
	)
}

func emptyPodClique(objKey client.ObjectKey) *grovecorev1alpha1.PodClique {
	return &grovecorev1alpha1.PodClique{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
