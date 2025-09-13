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

package service

import (
	"context"
	"fmt"
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errSyncPodGangSetService   grovecorev1alpha1.ErrorCode = "ERR_SYNC_PODGANGSET_SERVICE"
	errDeletePodGangSetService grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODGANGSET_SERVICE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of Service component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodCliqueSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the Service Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) ([]string, error) {
	logger.Info("Looking for existing PodCliqueSet Headless Services", "objectKey", k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta))
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgsObjMeta.Namespace),
		client.MatchingLabels(getSelectorLabelsForAllHeadlessServices(pgsObjMeta.Name)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errSyncPodGangSetService,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing Headless Services for PodCliqueSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pgsObjMeta, objMetaList.Items), nil
}

// Sync synchronizes all resources that the Service Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) error {
	replicaIndexToObjectKeys := getObjectKeys(pgs)
	tasks := make([]utils.Task, 0, len(replicaIndexToObjectKeys))
	for replicaIndex, objectKey := range replicaIndexToObjectKeys {
		createOrUpdateTask := utils.Task{
			Name: fmt.Sprintf("CreateOrUpdatePodGangSetService-%s", objectKey),
			Fn: func(ctx context.Context) error {
				return r.doCreateOrUpdate(ctx, logger, pgs, replicaIndex, objectKey)
			},
		}
		tasks = append(tasks, createOrUpdateTask)
	}
	if runResult := utils.RunConcurrently(ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodGangSetService,
			component.OperationSync,
			fmt.Sprintf("Error creating or updating PodCliqueSet Headless Services for PodCliqueSet: %v, run summary: %s", client.ObjectKeyFromObject(pgs), runResult.GetSummary()),
		)
	}
	logger.Info("Successfully synced Headless Services")
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgObjMeta metav1.ObjectMeta) error {
	logger.Info("Deleting Headless Services")
	if err := r.client.DeleteAllOf(ctx,
		&corev1.Service{},
		client.InNamespace(pgObjMeta.Namespace),
		client.MatchingLabels(getSelectorLabelsForAllHeadlessServices(pgObjMeta.Name))); err != nil {
		return groveerr.WrapError(err,
			errDeletePodGangSetService,
			component.OperationDelete,
			fmt.Sprintf("Failed to delete Headless Services for PodCliqueSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgObjMeta)),
		)
	}
	logger.Info("Deleted Headless Services")
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet, pgsReplicaIndex int, pgServiceObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate PodCliqueSet Headless Service", "pgsReplicaIndex", pgsReplicaIndex, "objectKey", pgServiceObjectKey)
	pgService := emptyPGService(pgServiceObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pgService, func() error {
		return r.buildResource(pgService, pgs, pgsReplicaIndex)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodGangSetService,
			component.OperationSync,
			fmt.Sprintf("Error syncing Headless Service: %v for PodCliqueSet: %v", pgServiceObjectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("Triggered create or update of PodGang Headless Service", "pgServiceObjectKey", pgServiceObjectKey, "result", opResult)
	return nil
}

func (r _resource) buildResource(svc *corev1.Service, pgs *grovecorev1alpha1.PodCliqueSet, pgsReplicaIndex int) error {
	svc.Labels = getLabels(pgs.Name, client.ObjectKeyFromObject(svc), pgsReplicaIndex)
	var publishNotReadyAddresses bool
	if pgs.Spec.Template.HeadlessServiceConfig != nil {
		publishNotReadyAddresses = pgs.Spec.Template.HeadlessServiceConfig.PublishNotReadyAddresses
	}
	svc.Spec = corev1.ServiceSpec{
		Selector:                 getLabelSelectorForPodsInAPodGangSetReplica(pgs.Name, pgsReplicaIndex),
		ClusterIP:                "None",
		PublishNotReadyAddresses: publishNotReadyAddresses,
	}

	if err := controllerutil.SetControllerReference(pgs, svc, r.scheme); err != nil {
		return err
	}

	return nil
}

func getLabels(pgsName string, svcObjectKey client.ObjectKey, pgsReplicaIndex int) map[string]string {
	svcLabels := map[string]string{
		apicommon.LabelAppNameKey:               svcObjectKey.Name,
		apicommon.LabelComponentKey:             apicommon.LabelComponentNamePodCliqueSetReplicaHeadlessService,
		apicommon.LabelPodCliqueSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
	}
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pgsName),
		svcLabels,
	)
}

func getLabelSelectorForPodsInAPodGangSetReplica(pgsName string, pgsReplicaIndex int) map[string]string {
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pgsName),
		map[string]string{
			apicommon.LabelPodCliqueSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
		},
	)
}

func getSelectorLabelsForAllHeadlessServices(pgsName string) map[string]string {
	svcMatchingLabels := map[string]string{
		apicommon.LabelComponentKey: apicommon.LabelComponentNamePodCliqueSetReplicaHeadlessService,
	}
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pgsName),
		svcMatchingLabels,
	)
}

func getObjectKeys(pgs *grovecorev1alpha1.PodCliqueSet) []client.ObjectKey {
	objectKeys := make([]client.ObjectKey, 0, pgs.Spec.Replicas)
	for replicaIndex := range pgs.Spec.Replicas {
		serviceName := apicommon.GenerateHeadlessServiceName(apicommon.ResourceNameReplica{Name: pgs.Name, Replica: int(replicaIndex)})
		objectKeys = append(objectKeys, client.ObjectKey{
			Name:      serviceName,
			Namespace: pgs.Namespace,
		})
	}
	return objectKeys
}

func emptyPGService(objKey client.ObjectKey) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
