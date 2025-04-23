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
	"errors"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
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
	errSyncPodGangService   v1alpha1.ErrorCode = "ERR_SYNC_PODGANG_SERVICE"
	errDeletePodGangService v1alpha1.ErrorCode = "ERR_DELETE_PODGANG_SERVICE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of Service component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[v1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the Service Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) ([]string, error) {
	logger.Info("Looking for existing PodGangSet Headless Services", "objectKey", client.ObjectKeyFromObject(pgs))
	existingServiceNames := make([]string, 0, int(pgs.Spec.Replicas))
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Service"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(getSelectorLabelsForAllHeadlessServices(pgs.Name)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errSyncPodGangService,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodGangSet Headless Services: %v", client.ObjectKeyFromObject(pgs)),
		)
	}
	for _, serviceObjMeta := range objMetaList.Items {
		if metav1.IsControlledBy(&serviceObjMeta, &pgs.ObjectMeta) {
			existingServiceNames = append(existingServiceNames, serviceObjMeta.Name)
		}
	}
	return existingServiceNames, nil
}

// Sync synchronizes all resources that the Service Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	// Do not create headless service if service spec is not defined.
	if pgs.Spec.TemplateSpec.HeadlessServiceConfig == nil {
		return nil
	}
	objectKeys := getObjectKeys(pgs)
	tasks := make([]utils.Task, 0, len(objectKeys))
	for _, objectKey := range objectKeys {
		createOrUpdateTask := utils.Task{
			Name: fmt.Sprintf("CreateOrUpdatePodGangService-%s", objectKey),
			Fn: func(ctx context.Context) error {
				return r.doCreateOrUpdate(ctx, logger, pgs, objectKey)
			},
		}
		tasks = append(tasks, createOrUpdateTask)
	}
	if errs := utils.RunConcurrently(ctx, tasks); len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgObjMeta metav1.ObjectMeta) error {
	logger.Info("Deleting PodGangSet Headless Services")
	if err := r.client.DeleteAllOf(ctx,
		&corev1.Service{},
		client.InNamespace(pgObjMeta.Namespace),
		client.MatchingLabels(getSelectorLabelsForAllHeadlessServices(pgObjMeta.Name))); err != nil {
		return groveerr.WrapError(err,
			errDeletePodGangService,
			component.OperationDelete,
			fmt.Sprintf("Failed to delete PodGang Headless Services for PodGangSet: %v", client.ObjectKey{Name: pgObjMeta.Name, Namespace: pgObjMeta.Namespace}),
		)
	}
	logger.Info("Deleted PodGangSet Headless Services", "name", pgObjMeta.Name)
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet, pgServiceObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate PodGang Headless Service", "objectKey", pgServiceObjectKey)
	pgService := emptyPGService(pgServiceObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pgService, func() error {
		return r.buildResource(pgService, pgs)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodGangService,
			component.OperationSync,
			fmt.Sprintf("Error syncing Headless Service: %v for PodGang: %v", pgServiceObjectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("triggered create or update of PodGang Headless Service", "pgServiceObjectKey", pgServiceObjectKey, "result", opResult)
	return nil
}

func (r _resource) buildResource(svc *corev1.Service, pgs *v1alpha1.PodGangSet) error {
	svc.Labels = getLabels(pgs.Name, client.ObjectKeyFromObject(svc))
	svc.Spec = corev1.ServiceSpec{
		Selector:                 getLabelSelectorForPodsInAGang(pgs.Name, svc.Name),
		ClusterIP:                "None",
		PublishNotReadyAddresses: pgs.Spec.TemplateSpec.HeadlessServiceConfig.PublishNotReadyAddresses,
	}

	if err := controllerutil.SetControllerReference(pgs, svc, r.scheme); err != nil {
		return err
	}

	return nil
}

func getLabels(pgsName string, svcObjectKey client.ObjectKey) map[string]string {
	svcLabels := map[string]string{
		v1alpha1.LabelAppNameKey:     svcObjectKey.Name,
		v1alpha1.LabelComponentKey:   component.NamePodGangHeadlessService,
		v1alpha1.LabelPodGangNameKey: svcObjectKey.Name,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		svcLabels,
	)
}

func getLabelSelectorForPodsInAGang(pgsName, podGangName string) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			v1alpha1.LabelPodGangNameKey: podGangName,
		},
	)
}

func getSelectorLabelsForAllHeadlessServices(pgsName string) map[string]string {
	svcMatchingLabels := map[string]string{
		v1alpha1.LabelComponentKey: component.NamePodGangHeadlessService,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		svcMatchingLabels,
	)
}

func getObjectKeys(pgs *v1alpha1.PodGangSet) []client.ObjectKey {
	pgServiceNames := getPodGangServiceNames(pgs)
	serviceObjKeys := make([]client.ObjectKey, 0, pgs.Spec.Replicas)
	for _, pgServiceName := range pgServiceNames {
		serviceObjKeys = append(serviceObjKeys, client.ObjectKey{
			Name:      pgServiceName,
			Namespace: pgs.Namespace,
		})
	}
	return serviceObjKeys
}

func getPodGangServiceNames(pgs *v1alpha1.PodGangSet) []string {
	pgServiceNames := make([]string, 0, pgs.Spec.Replicas)
	for replicaIndex := range pgs.Spec.Replicas {
		pgServiceNames = append(pgServiceNames, v1alpha1.GeneratePodGangName(pgs.Name, replicaIndex))
	}
	return pgServiceNames
}

func emptyPGService(objKey client.ObjectKey) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
