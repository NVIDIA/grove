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

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveevents "github.com/NVIDIA/grove/operator/internal/component/events"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errListPodClique               grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUE"
	errSyncPodClique               grovecorev1alpha1.ErrorCode = "ERR_SYNC_PODCLIQUE"
	errDeletePodClique             grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE"
	errCodeListPodCliques          grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUES"
	errCodeCreateOrUpdatePodClique grovecorev1alpha1.ErrorCode = "ERR_CREATE_OR_UPDATE_PODCLIQUE"
)

type _resource struct {
	client        client.Client
	scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
}

// New creates an instance of PodClique component operator.
func New(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) component.Operator[grovecorev1alpha1.PodGangSet] {
	return &_resource{
		client:        client,
		scheme:        scheme,
		eventRecorder: eventRecorder,
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
	existingPCLQFQNs, err := r.GetExistingResourceNames(ctx, logger, pgs.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodClique,
			component.OperationSync,
			fmt.Sprintf("Unable to fetch existing PodClique names for PodGangSet: %v", client.ObjectKeyFromObject(pgs)),
		)
	}

	if err := r.triggerDeletionOfExcessPCLQs(ctx, logger, pgs, existingPCLQFQNs); err != nil {
		return err
	}
	if err := r.createOrUpdatePCLQs(ctx, logger, pgs, existingPCLQFQNs); err != nil {
		return err
	}

	return nil
}

func (r _resource) triggerDeletionOfExcessPCLQs(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, existingPCLQFQNs []string) error {
	expectedPCLQFQNs := componentutils.GetPodCliqueFQNsForPGSNotInPCSG(pgs)
	// Check if the number of existing PodCliques is greater than expected, if so, we need to delete the extra ones.
	diff := len(existingPCLQFQNs) - len(expectedPCLQFQNs)
	if diff > 0 {
		logger.Info("Found more PodCliques than expected", "expected", expectedPCLQFQNs, "existing", existingPCLQFQNs)
		logger.Info("Triggering deletion of extra PodCliques", "count", diff)
		// collect the names of the extra PodCliques to delete
		deletionCandidateNames, err := getPodCliqueNamesToDelete(pgs.Name, int(pgs.Spec.Replicas), existingPCLQFQNs)
		if err != nil {
			return err
		}
		deletePCLQTasks := r.createDeleteTasks(logger, pgs, deletionCandidateNames)
		return r.triggerDeletionOfPodCliques(ctx, logger, client.ObjectKeyFromObject(pgs), deletePCLQTasks)
	}
	return nil
}

func (r _resource) createOrUpdatePCLQs(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, existingPCLQFQNs []string) error {
	expectedPCLQNames, _ := componentutils.GetExpectedPCLQNamesGroupByOwner(pgs)
	tasks := make([]utils.Task, 0, len(expectedPCLQNames))

	for pgsReplica := range pgs.Spec.Replicas {
		for _, expectedPCLQName := range expectedPCLQNames {
			pclqObjectKey := client.ObjectKey{
				Name:      apicommon.GeneratePodCliqueName(apicommon.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, expectedPCLQName),
				Namespace: pgs.Namespace,
			}
			pclqExists := slices.Contains(existingPCLQFQNs, pclqObjectKey.Name)
			createOrUpdateTask := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdatePodClique-%s", pclqObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdate(ctx, logger, pgs, pgsReplica, pclqObjectKey, pclqExists)
				},
			}
			tasks = append(tasks, createOrUpdateTask)
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

func (r _resource) createDeleteTasks(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, targetPCLQNames []string) []utils.Task {
	deletionTasks := make([]utils.Task, 0, len(targetPCLQNames))
	for _, pclqName := range targetPCLQNames {
		pclqObjectKey := client.ObjectKey{
			Name:      pclqName,
			Namespace: pgs.Namespace,
		}
		pclq := emptyPodClique(pclqObjectKey)
		task := utils.Task{
			Name: "DeleteExcessPodClique-" + pclqName,
			Fn: func(ctx context.Context) error {
				if err := client.IgnoreNotFound(r.client.Delete(ctx, pclq)); err != nil {
					logger.Error(err, "failed to delete excess PodClique", "objectKey", pclqObjectKey)
					r.eventRecorder.Eventf(pgs, corev1.EventTypeWarning, groveevents.ReasonPodCliqueDeleteFailed, "Error deleting PodClique %v: %v", pclqObjectKey, err)
					return err
				}
				logger.Info("Deleted PodClique", "pclqObjectKey", pclqObjectKey)
				r.eventRecorder.Eventf(pgs, corev1.EventTypeNormal, groveevents.ReasonPodCliqueDeleteSuccessful, "Deleted PodClique: %s", pclqName)
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

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int32, pclqObjectKey client.ObjectKey, pclqExists bool) error {
	logger.Info("Running CreateOrUpdate PodClique", "pclqObjectKey", pclqObjectKey)
	pclq := emptyPodClique(pclqObjectKey)
	pgsObjKey := client.ObjectKeyFromObject(pgs)

	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pclq, func() error {
		return r.buildResource(logger, pclq, pgs, int(pgsReplica), pclqExists)
	})
	if err != nil {
		r.eventRecorder.Eventf(pgs, corev1.EventTypeWarning, groveevents.ReasonPodCliqueCreateOrUpdateFailed, "PodClique %v creation or updation failed: %v", pclqObjectKey, err)
		return groveerr.WrapError(err,
			errCodeCreateOrUpdatePodClique,
			component.OperationSync,
			fmt.Sprintf("Error creating or updating PodClique: %v for PodGangSet: %v", pclqObjectKey, pgsObjKey),
		)
	}

	r.eventRecorder.Eventf(pgs, corev1.EventTypeNormal, groveevents.ReasonPodCliqueCreateOrUpdateSuccessful, "PodClique %v created or updated successfully", pclqObjectKey)
	logger.Info("triggered create or update of PodClique for PodGangSet", "pgs", pgsObjKey, "pclqObjectKey", pclqObjectKey, "result", opResult)
	return nil
}

func (r _resource) buildResource(logger logr.Logger, pclq *grovecorev1alpha1.PodClique, pgs *grovecorev1alpha1.PodGangSet, pgsReplica int, pclqExists bool) error {
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
	pclq.Labels = getLabels(pgs, pgsReplica, pclqObjectKey, pclqTemplateSpec, apicommon.GeneratePodGangNameForPodCliqueOwnedByPodGangSet(pgs, pgsReplica))
	pclq.Annotations = pclqTemplateSpec.Annotations
	// set PodCliqueSpec
	// ------------------------------------
	if pclqExists {
		// If an HPA is mutating the number of replicas, then it should not be overwritten by the template spec replicas.
		currentPCLQReplicas := pclq.Spec.Replicas
		pclq.Spec = pclqTemplateSpec.Spec
		pclq.Spec.Replicas = currentPCLQReplicas
	} else {
		pclq.Spec = pclqTemplateSpec.Spec
	}
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
		return getExplicitStartupDependencies(pgs, pgsReplicaIndex, pclq), nil
	default:
		return nil, nil
	}
}

func getInOrderStartupDependencies(pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex, foundAtIndex int) []string {
	if foundAtIndex == 0 {
		return nil
	}
	previousCliqueName := pgs.Spec.Template.Cliques[foundAtIndex-1].Name
	return componentutils.GenerateDependencyNamesForBasePodGang(pgs, pgsReplicaIndex, previousCliqueName)
}

func getExplicitStartupDependencies(pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex int, pclq *grovecorev1alpha1.PodClique) []string {
	dependencies := make([]string, 0, len(pclq.Spec.StartsAfter))
	for _, dependency := range pclq.Spec.StartsAfter {
		dependencies = append(dependencies, componentutils.GenerateDependencyNamesForBasePodGang(pgs, pgsReplicaIndex, dependency)...)
	}
	return dependencies
}

func getPodCliqueSelectorLabels(pgsObjectMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodGangSetManagedResources(pgsObjectMeta.Name),
		map[string]string{
			apicommon.LabelComponentKey: apicommon.LabelComponentNamePodGangSetPodClique,
		},
	)
}

func getLabels(pgs *grovecorev1alpha1.PodGangSet, pgsReplica int, pclqObjectKey client.ObjectKey, pclqTemplateSpec *grovecorev1alpha1.PodCliqueTemplateSpec, podGangName string) map[string]string {
	pclqComponentLabels := map[string]string{
		apicommon.LabelAppNameKey:             pclqObjectKey.Name,
		apicommon.LabelComponentKey:           apicommon.LabelComponentNamePodGangSetPodClique,
		apicommon.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplica),
		apicommon.LabelPodGang:                podGangName,
		apicommon.LabelPodTemplateHash:        componentutils.ComputePCLQPodTemplateHash(pclqTemplateSpec, pgs.Spec.Template.PriorityClassName),
	}
	return lo.Assign(
		pclqTemplateSpec.Labels,
		apicommon.GetDefaultLabelsForPodGangSetManagedResources(pgs.Name),
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
