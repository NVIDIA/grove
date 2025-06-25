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

package podcliquescalinggroup

import (
	"context"
	"fmt"
	"slices"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
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
	errListPodCliqueScalingGroup   grovecorev1alpha1.ErrorCode = "ERR_GET_POD_CLIQUE_SCALING_GROUPS"
	errSyncPodCliqueScalingGroup   grovecorev1alpha1.ErrorCode = "ERR_SYNC_POD_CLIQUE_SCALING_GROUP"
	errDeletePodCliqueScalingGroup grovecorev1alpha1.ErrorCode = "ERR_DELETE_POD_CLIQUE_SCALING_GROUP"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of PodCliqueScalingGroup component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the PodCliqueScalingGroup Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgs *grovecorev1alpha1.PodGangSet) ([]string, error) {
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(grovecorev1alpha1.SchemeGroupVersion.WithKind("PodCliqueScalingGroup"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(getPodCliqueScalingGroupSelectorLabels(pgs.ObjectMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errListPodCliqueScalingGroup,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliqueScalingGroup for PodGangSet: %v", pgs.Namespace))
	}
	return k8sutils.FilterMapOwnedResourceNames(pgs.ObjectMeta, objMetaList.Items), nil
}

// Sync synchronizes all resources that the PodCliqueScalingGroup Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) error {
	existingPCSGNames, err := r.GetExistingResourceNames(ctx, logger, pgs)
	if err != nil {
		return groveerr.WrapError(err,
			errListPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error listing PodCliqueScalingGroup for PodGangSet: %v", client.ObjectKeyFromObject(pgs)),
		)
	}

	tasks := make([]utils.Task, 0, int(pgs.Spec.Replicas)*len(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs))
	expectedPCSGNames := make([]string, 0, 20)
	for replicaIndex := range pgs.Spec.Replicas {
		for _, pcsgConfig := range pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs {
			pcsgName := grovecorev1alpha1.GeneratePodCliqueScalingGroupName(pgs.Name, replicaIndex, pcsgConfig.Name)
			expectedPCSGNames = append(expectedPCSGNames, pcsgName)
			pcsgObjectKey := client.ObjectKey{
				Name:      pcsgName,
				Namespace: pgs.Namespace,
			}
			exists := slices.Contains(existingPCSGNames, pcsgObjectKey.Name)
			createTask := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdatePodCliqueScalingGroup-%s", pcsgObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdate(ctx, logger, pgs, pcsgObjectKey, int(replicaIndex), pcsgConfig.CliqueNames, exists)
				},
			}
			tasks = append(tasks, createTask)
		}
	}

	excessPCSGNames := lo.Filter(existingPCSGNames, func(existingPCSGName string, _ int) bool {
		return !slices.Contains(expectedPCSGNames, existingPCSGName)
	})
	for _, excessPCSGName := range excessPCSGNames {
		pcsgObjectKey := client.ObjectKey{
			Namespace: pgs.Namespace,
			Name:      excessPCSGName,
		}
		deleteTask := utils.Task{
			Name: fmt.Sprintf("DeletePodCliqueScalingGroup-%s", pcsgObjectKey),
			Fn: func(ctx context.Context) error {
				return r.doDelete(ctx, logger, pcsgObjectKey)
			},
		}
		tasks = append(tasks, deleteTask)
	}

	if runResult := utils.RunConcurrently(ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error creating or updating PodCliqueScalingGroup for PodGangSet: %v, run summary: %s", client.ObjectKeyFromObject(pgs), runResult.GetSummary()),
		)
	}
	logger.Info("Successfully synced PodCliqueScalingGroup for PodGangSet")
	return nil
}

// Delete deletes all resources that the PodCliqueScalingGroup Operator manages.
func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) error {
	logger.Info("Triggering delete of PodCliqueScalingGroups")
	if err := r.client.DeleteAllOf(ctx,
		&grovecorev1alpha1.PodCliqueScalingGroup{},
		client.InNamespace(pgsObjMeta.Namespace),
		client.MatchingLabels(getPodCliqueScalingGroupSelectorLabels(pgsObjMeta))); err != nil {
		return groveerr.WrapError(err,
			errDeletePodCliqueScalingGroup,
			component.OperationDelete,
			fmt.Sprintf("Error deleting PodCliqueScalingGroup for PodGangSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	logger.Info("Deleted PodCliqueScalingGroups")
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pclqScalingGrpObjectKey client.ObjectKey, pgsReplicaIndex int, cliqueNames []string, exists bool) error {
	logger.Info("CreateOrUpdate PodCliqueScalingGroup", "objectKey", pclqScalingGrpObjectKey)
	pclqScalingGrp := emptyPodCliqueScalingGroup(pclqScalingGrpObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pclqScalingGrp, func() error {
		return r.buildResource(pclqScalingGrp, pgs, pgsReplicaIndex, cliqueNames, exists)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error in create/update of PodCliqueScalingGroup: %v for PodGangSet: %v", pclqScalingGrpObjectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("Triggered create or update of PodCliqueScalingGroup", "objectKey", pclqScalingGrpObjectKey, "result", opResult)
	return nil
}

func (r _resource) doDelete(ctx context.Context, logger logr.Logger, pcsgObjectKey client.ObjectKey) error {
	logger.Info("Delete PodCliqueScalingGroup", "objectKey", pcsgObjectKey)
	pcsg := emptyPodCliqueScalingGroup(pcsgObjectKey)
	if err := r.client.Delete(ctx, pcsg); err != nil {
		return groveerr.WrapError(err,
			errDeletePodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error in delete of PodCliqueScalingGroup: %v", pcsg),
		)
	}
	logger.Info("Triggered delete of PodCliqueScalingGroup", "objectKey", pcsgObjectKey)
	return nil
}

func (r _resource) buildResource(pclqScalingGroup *grovecorev1alpha1.PodCliqueScalingGroup, pgs *grovecorev1alpha1.PodGangSet, pgsReplicaIndex int, cliqueNames []string, exists bool) error {
	if err := controllerutil.SetControllerReference(pgs, pclqScalingGroup, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pclqScalingGroup)),
		)
	}
	if !exists {
		cliqueNameFQNs := lo.Map(cliqueNames, func(cliqueName string, _ int) string {
			return grovecorev1alpha1.GeneratePodCliqueName(pgs.Name, pgsReplicaIndex, cliqueName)
		})
		pclqScalingGroup.Spec.Replicas = 1 // default to 1 replica if creating the resource.
		pclqScalingGroup.Spec.CliqueNames = cliqueNameFQNs
	}
	pclqScalingGroup.Labels = getLabels(pgs.Name, client.ObjectKeyFromObject(pclqScalingGroup))
	return nil
}

func getLabels(pgsName string, pclqScalingGroupObjKey client.ObjectKey) map[string]string {
	componentLabels := map[string]string{
		grovecorev1alpha1.LabelAppNameKey:   pclqScalingGroupObjKey.Name,
		grovecorev1alpha1.LabelComponentKey: component.NamePodCliqueScalingGroup,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		componentLabels,
	)
}

func getPodCliqueScalingGroupSelectorLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePodCliqueScalingGroup,
		},
	)
}

func emptyPodCliqueScalingGroup(objKey client.ObjectKey) *grovecorev1alpha1.PodCliqueScalingGroup {
	return &grovecorev1alpha1.PodCliqueScalingGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
