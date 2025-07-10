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
	"slices"
	"strconv"
	"strings"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errListPodClique             grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE"
	errSyncPodClique             grovecorev1alpha1.ErrorCode = "ERR_SYNC_PODCLIQUE"
	errDeletePodClique           grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE"
	errGetPodGangSet             grovecorev1alpha1.ErrorCode = "ERR_GET_PODGANGSET"
	errMissingPGSReplicaIndex    grovecorev1alpha1.ErrorCode = "ERR_MISSING_PODGANGSET_REPLICA_INDEX"
	errReplicaIndexIntConversion grovecorev1alpha1.ErrorCode = "ERR_PODGANGSET_REPLICA_INDEX_CONVERSION"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of PodClique component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodCliqueScalingGroup] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the PodClique Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pcsgObjectMeta metav1.ObjectMeta) ([]string, error) {
	logger.Info("Looking for existing PodCliques managed by PodCliqueScalingGroup")
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(grovecorev1alpha1.SchemeGroupVersion.WithKind("PodClique"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pcsgObjectMeta.Namespace),
		client.MatchingLabels(getPodCliqueSelectorLabels(pcsgObjectMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errListPodClique,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliques for PodGangSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pcsgObjectMeta, objMetaList.Items), nil
}

// Sync synchronizes all resources that the PodClique Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup) error {
	expectedPCLQs := int(pcsg.Spec.Replicas) * len(pcsg.Spec.CliqueNames)
	existingPCLQNames, err := r.GetExistingResourceNames(ctx, logger, pcsg.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Unable to fetch existing PodClique names for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	// Check if the number of existing PodCliques is greater than expected, if so, we need to delete the extra ones.
	diff := len(existingPCLQNames) - expectedPCLQs
	if diff > 0 {
		logger.Info("Found more PodCliques than expected", "expected", expectedPCLQs, "existing", len(existingPCLQNames))
		logger.Info("Triggering deletion of extra PodCliques", "count", diff)
		// collect the names of the extra PodCliques to delete
		deletionCandidateNames, err := getPodCliqueNamesToDelete(pcsg.Name, int(pcsg.Spec.Replicas), existingPCLQNames)
		if err != nil {
			return err
		}
		if err := r.triggerDeletionOfExcessPodCliques(ctx, logger, pcsg, deletionCandidateNames); err != nil {
			return err
		}
	}

	pgs, err := componentutils.GetOwnerPodGangSet(ctx, r.client, pcsg.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errGetPodGangSet,
			component.OperationSync,
			fmt.Sprintf("failed to get owner PodGangSet for PodCliqueScalingGroup %s", client.ObjectKeyFromObject(pcsg)),
		)
	}
	// Update or create PodCliques
	tasks := make([]utils.Task, 0, expectedPCLQs)
	for pcsgReplicaIndex := range pcsg.Spec.Replicas {
		pclqFQNs := lo.FilterMap(pgs.Spec.Template.Cliques, func(pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, _ int) (string, bool) {
			pclqFQN := grovecorev1alpha1.GeneratePodCliqueName(grovecorev1alpha1.ResourceNameReplica{
				Name:    pcsg.Name,
				Replica: int(pcsgReplicaIndex),
			}, pclqTemplateSpec.Name)
			return pclqFQN, slices.Contains(pcsg.Spec.CliqueNames, pclqTemplateSpec.Name)
		})

		for _, pclqFQN := range pclqFQNs {
			pclqObjectKey := client.ObjectKey{
				Name:      pclqFQN,
				Namespace: pcsg.Namespace,
			}
			exists := slices.Contains(existingPCLQNames, pclqObjectKey.Name)
			createOrUpdateTask := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdatePodClique-%s", pclqObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdate(ctx, logger, pgs, pcsg, int(pcsgReplicaIndex), pclqObjectKey, exists)
				},
			}
			tasks = append(tasks, createOrUpdateTask)
		}
	}
	if runResult := utils.RunConcurrently(ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error CreateOrUpdate of PodCliques for PodCliqueScalingGroup: %v, run summary: %s", client.ObjectKeyFromObject(pcsg), runResult.GetSummary()),
		)
	}
	return nil
}

func (r _resource) triggerDeletionOfExcessPodCliques(ctx context.Context, logger logr.Logger, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, deletionCandidateNames []string) error {
	deletionTasks := make([]utils.Task, 0, len(deletionCandidateNames))
	for _, pclqName := range deletionCandidateNames {
		pclqObjectKey := client.ObjectKey{
			Name:      pclqName,
			Namespace: pcsg.Namespace,
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
	if runResult := utils.RunConcurrently(ctx, logger, deletionTasks); runResult.HasErrors() {
		logger.Error(runResult.GetAggregatedError(), "Error deleting excess PodCliques", "run summary", runResult.GetSummary())
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error deleting excess PodCliques for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	logger.Info("Deleted excess PodCliques", "pclqNames", deletionCandidateNames)
	return nil
}

func getPodCliqueNamesToDelete(pgsName string, pcsgReplicas int, existingPCLQNames []string) ([]string, error) {
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
		if extractedPGSReplica >= pcsgReplicas {
			// If the extracted replica index is greater than or equal to the number of replicas in the PodGangSet,
			// then this PodClique is an extra one that should be deleted.
			pclqsToDelete = append(pclqsToDelete, pclqName)
		}
	}
	return pclqsToDelete, nil
}

// Delete deletes all resources that the PodClique Operator manages.
func (r _resource) Delete(ctx context.Context, logger logr.Logger, pcsgObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering deletion of PodCliques managed by PodCliqueScalingGroup")
	existingPCLQNames, err := r.GetExistingResourceNames(ctx, logger, pcsgObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errListPodClique,
			component.OperationDelete,
			fmt.Sprintf("Unable to fetch existing PodClique names for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
		)
	}
	deleteTasks := make([]utils.Task, 0, len(existingPCLQNames))
	for _, pclqName := range existingPCLQNames {
		pclqObjectKey := client.ObjectKey{Name: pclqName, Namespace: pcsgObjectMeta.Namespace}
		task := utils.Task{
			Name: "DeletePodClique-" + pclqName,
			Fn: func(ctx context.Context) error {
				if err := client.IgnoreNotFound(r.client.Delete(ctx, emptyPodClique(pclqObjectKey))); err != nil {
					return groveerr.WrapError(err,
						errDeletePodClique,
						component.OperationDelete,
						fmt.Sprintf("Failed to delete PodClique: %v for PodCliqueScalingGroup: %v", pclqObjectKey, k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
					)
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
			fmt.Sprintf("Error deleting PodCliques for PodCliqueScalingGroup: %v", k8sutils.GetObjectKeyFromObjectMeta(pcsgObjectMeta)),
		)
	}

	logger.Info("Deleted PodCliques belonging to PodCliqueScalingGroup")
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclqObjectKey client.ObjectKey, exists bool) error {
	logger.Info("Running CreateOrUpdate PodClique", "pclqObjectKey", pclqObjectKey)
	pclq := emptyPodClique(pclqObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pclq, func() error {
		return r.buildResource(logger, pgs, pcsg, pcsgReplicaIndex, pclq, exists)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error syncing PodClique: %v for PodCliqueScalingGroup: %v", pclqObjectKey, client.ObjectKeyFromObject(pcsg)),
		)
	}
	logger.Info("triggered create or update of PodClique", "pclqObjectKey", pclqObjectKey, "result", opResult)
	return nil
}

func (r _resource) buildResource(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pcsgReplicaIndex int, pclq *grovecorev1alpha1.PodClique, exists bool) error {
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
	if err = controllerutil.SetControllerReference(pcsg, pclq, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for PodClique: %v", client.ObjectKeyFromObject(pclq)),
		)
	}
	pgsReplicaIndex, ok := pcsg.GetLabels()[grovecorev1alpha1.LabelPodGangSetReplicaIndex]
	if !ok {
		return groveerr.New(errMissingPGSReplicaIndex, component.OperationSync, fmt.Sprintf("failed to get the PodGangSet replica ind value from the labels for PodCliqueScalingGroup %s", client.ObjectKeyFromObject(pcsg)))
	}
	pgsReplica, err := strconv.Atoi(pgsReplicaIndex)
	if err != nil {
		return groveerr.WrapError(err,
			errReplicaIndexIntConversion,
			component.OperationSync,
			"failed to convert replica index value from string to integer",
		)
	}
	pclq.Labels = getLabels(pgs.Name, pgsReplica, pcsg.Name, pcsgReplicaIndex, pclqObjectKey, pclqTemplateSpec)
	pclq.Annotations = pclqTemplateSpec.Annotations
	// set PodCliqueSpec
	// ------------------------------------
	if exists {
		// If an HPA is mutating the number of replicas, then it should not be overwritten by the template spec replicas.
		currentPCLQReplicas := pclq.Spec.Replicas
		pclq.Spec = pclqTemplateSpec.Spec
		pclq.Spec.Replicas = currentPCLQReplicas
	} else {
		pclq.Spec = pclqTemplateSpec.Spec
	}
	dependentPclqNames, err := identifyFullyQualifiedStartupDependencyNames(pgs, pclq, pgsReplica, foundAtIndex)
	if err != nil {
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

func getPodCliqueSelectorLabels(pcsgObjectMeta metav1.ObjectMeta) map[string]string {
	pgsName := componentutils.GetPodGangSetName(pcsgObjectMeta)
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(*pgsName),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey:          component.NamePCSGPodClique,
			grovecorev1alpha1.LabelPodCliqueScalingGroup: pcsgObjectMeta.Name,
		},
	)
}

func getLabels(pgsName string, pgsReplicaIndex int, pcsgName string, pcsgReplicaIndex int, pclqObjectKey client.ObjectKey, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec) map[string]string {
	podGangName := componentutils.CreatePodGangNameForPCSG(pgsName, pgsReplicaIndex, pcsgName, pcsgReplicaIndex)
	pclqComponentLabels := map[string]string{
		grovecorev1alpha1.LabelAppNameKey:             pclqObjectKey.Name,
		grovecorev1alpha1.LabelComponentKey:           component.NamePCSGPodClique,
		grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
		grovecorev1alpha1.LabelPodCliqueScalingGroup:  pcsgName,
		grovecorev1alpha1.LabelPodGangName:            podGangName,
	}
	return lo.Assign(
		pclqTemplateSpec.Labels,
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
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
