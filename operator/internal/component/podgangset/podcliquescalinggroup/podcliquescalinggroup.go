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
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveevents "github.com/NVIDIA/grove/operator/internal/component/events"
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
	errListPodCliqueScalingGroup       grovecorev1alpha1.ErrorCode = "ERR_GET_POD_CLIQUE_SCALING_GROUPS"
	errSyncPodCliqueScalingGroup       grovecorev1alpha1.ErrorCode = "ERR_SYNC_POD_CLIQUE_SCALING_GROUP"
	errDeletePodCliqueScalingGroup     grovecorev1alpha1.ErrorCode = "ERR_DELETE_POD_CLIQUE_SCALING_GROUP"
	errCodeCreatePodCliqueScalingGroup grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODCLIQUESCALINGGROUP"
)

type _resource struct {
	client        client.Client
	scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
}

// New creates an instance of PodCliqueScalingGroup component operator.
func New(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) component.Operator[grovecorev1alpha1.PodCliqueSet] {
	return &_resource{
		client:        client,
		scheme:        scheme,
		eventRecorder: eventRecorder,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the PodCliqueScalingGroup Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgsObjMeta metav1.ObjectMeta) ([]string, error) {
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(grovecorev1alpha1.SchemeGroupVersion.WithKind("PodCliqueScalingGroup"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgsObjMeta.Namespace),
		client.MatchingLabels(getPodCliqueScalingGroupSelectorLabels(pgsObjMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errListPodCliqueScalingGroup,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliqueScalingGroup for PodCliqueSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pgsObjMeta, objMetaList.Items), nil
}

// Sync synchronizes all resources that the PodCliqueScalingGroup Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) error {
	existingPCSGNames, err := r.GetExistingResourceNames(ctx, logger, pgs.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errListPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error listing PodCliqueScalingGroup for PodCliqueSet: %v", client.ObjectKeyFromObject(pgs)),
		)
	}

	tasks := make([]utils.Task, 0, int(pgs.Spec.Replicas)*len(pgs.Spec.Template.PodCliqueScalingGroupConfigs))
	expectedPCSGNames := make([]string, 0, 20)
	for pgsReplica := range pgs.Spec.Replicas {
		for _, pcsgConfig := range pgs.Spec.Template.PodCliqueScalingGroupConfigs {
			pcsgName := apicommon.GeneratePodCliqueScalingGroupName(apicommon.ResourceNameReplica{Name: pgs.Name, Replica: int(pgsReplica)}, pcsgConfig.Name)
			expectedPCSGNames = append(expectedPCSGNames, pcsgName)
			pcsgObjectKey := client.ObjectKey{
				Name:      pcsgName,
				Namespace: pgs.Namespace,
			}
			pcsgExists := slices.Contains(existingPCSGNames, pcsgName)
			createOrUpdateTask := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdatePodCliqueScalingGroup-%s", pcsgObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdate(ctx, logger, pgs, int(pgsReplica), pcsgObjectKey, pcsgConfig, pcsgExists)
				},
			}
			tasks = append(tasks, createOrUpdateTask)
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
				return r.doDelete(ctx, logger, pgs, pcsgObjectKey)
			},
		}
		tasks = append(tasks, deleteTask)
	}

	if runResult := utils.RunConcurrently(ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error creating or updating PodCliqueScalingGroup for PodCliqueSet: %v, run summary: %s", client.ObjectKeyFromObject(pgs), runResult.GetSummary()),
		)
	}
	logger.Info("Successfully synced PodCliqueScalingGroup for PodCliqueSet")
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
			fmt.Sprintf("Error deleting PodCliqueScalingGroup for PodCliqueSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	logger.Info("Deleted PodCliqueScalingGroups")
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet, pgsReplica int, pcsgObjectKey client.ObjectKey, pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig, pcsgExists bool) error {
	logger.Info("CreateOrUpdate PodCliqueScalingGroup", "objectKey", pcsgObjectKey)
	pcsg := emptyPodCliqueScalingGroup(pcsgObjectKey)

	if _, err := controllerutil.CreateOrPatch(ctx, r.client, pcsg, func() error {
		if err := r.buildResource(pcsg, pgs, pgsReplica, pcsgConfig, pcsgExists); err != nil {
			return groveerr.WrapError(err,
				errCodeCreatePodCliqueScalingGroup,
				component.OperationSync,
				fmt.Sprintf("Error creating or updating PodCliqueScalingGroup: %v for PodCliqueSet: %v", pcsgObjectKey, client.ObjectKeyFromObject(pgs)),
			)
		}
		return nil
	}); err != nil {
		r.eventRecorder.Eventf(pgs, corev1.EventTypeWarning, groveevents.ReasonPodCliqueScalingGroupCreateOrUpdateFailed, "Error creating or updating PodCliqueScalingGroup %v: %v", pcsgObjectKey, err)
		return err
	}

	r.eventRecorder.Eventf(pgs, corev1.EventTypeNormal, groveevents.ReasonPodCliqueScalingGroupCreateSuccessful, "Created PodCliqueScalingGroup %v", pcsgObjectKey)
	logger.Info("Created or updated PodCliqueScalingGroup", "objectKey", pcsgObjectKey)
	return nil
}

func (r _resource) doDelete(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet, pcsgObjectKey client.ObjectKey) error {
	logger.Info("Delete PodCliqueScalingGroup", "objectKey", pcsgObjectKey)
	pcsg := emptyPodCliqueScalingGroup(pcsgObjectKey)
	if err := r.client.Delete(ctx, pcsg); err != nil {
		r.eventRecorder.Eventf(pgs, corev1.EventTypeWarning, groveevents.ReasonPodCliqueScalingGroupDeleteFailed, "Error deleting PodCliqueScalingGroup %v: %v", pcsgObjectKey, err)
		return groveerr.WrapError(err,
			errDeletePodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error in delete of PodCliqueScalingGroup: %v", pcsg),
		)
	}
	r.eventRecorder.Eventf(pgs, corev1.EventTypeNormal, groveevents.ReasonPodCliqueScalingGroupDeleteSuccessful, "Deleted PodCliqueScalingGroup %v", pcsgObjectKey)
	logger.Info("Triggered delete of PodCliqueScalingGroup", "objectKey", pcsgObjectKey)
	return nil
}

func (r _resource) buildResource(pcsg *grovecorev1alpha1.PodCliqueScalingGroup, pgs *grovecorev1alpha1.PodCliqueSet, pgsReplica int, pcsgConfig grovecorev1alpha1.PodCliqueScalingGroupConfig, pcsgExists bool) error {
	if err := controllerutil.SetControllerReference(pgs, pcsg, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pcsg)),
		)
	}
	if !pcsgExists {
		pcsg.Spec.Replicas = *pcsgConfig.Replicas
	}
	pcsg.Spec.MinAvailable = pcsgConfig.MinAvailable
	pcsg.Spec.CliqueNames = pcsgConfig.CliqueNames
	pcsg.Labels = getLabels(pgs, pgsReplica, client.ObjectKeyFromObject(pcsg))
	return nil
}

func getLabels(pgs *grovecorev1alpha1.PodCliqueSet, pgsReplica int, pclqScalingGroupObjKey client.ObjectKey) map[string]string {
	componentLabels := map[string]string{
		apicommon.LabelAppNameKey:               pclqScalingGroupObjKey.Name,
		apicommon.LabelComponentKey:             apicommon.LabelComponentNamePodCliqueScalingGroup,
		apicommon.LabelPodCliqueSetReplicaIndex: strconv.Itoa(pgsReplica),
	}
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pgs.Name),
		componentLabels,
	)
}

func getPodCliqueScalingGroupSelectorLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pgsObjMeta.Name),
		map[string]string{
			apicommon.LabelComponentKey: apicommon.LabelComponentNamePodCliqueScalingGroup,
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
