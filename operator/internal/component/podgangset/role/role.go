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

package role

import (
	"context"
	"fmt"
	"strings"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errCodeGetRole grovecorev1alpha1.ErrorCode = "ERR_GET_ROLE"
	errSyncRole    grovecorev1alpha1.ErrorCode = "ERR_SYNC_ROLE"
	errDeleteRole  grovecorev1alpha1.ErrorCode = "ERR_DELETE_ROLE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of Role component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodCliqueSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the Role Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgsObjMeta metav1.ObjectMeta) ([]string, error) {
	roleNames := make([]string, 0, 1)
	objectKey := getObjectKey(pgsObjMeta)
	partialObjMeta, err := k8sutils.GetExistingPartialObjectMetadata(ctx, r.client, rbacv1.SchemeGroupVersion.WithKind("Role"), objectKey)
	if err != nil {
		if errors.IsNotFound(err) {
			return roleNames, nil
		}
		return nil, groveerr.WrapError(err,
			errCodeGetRole,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error getting Role: %v for PodCliqueSet: %v", objectKey, k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	if metav1.IsControlledBy(partialObjMeta, &pgsObjMeta) {
		roleNames = append(roleNames, partialObjMeta.Name)
	}
	return roleNames, nil
}

// Sync synchronizes all resources that the Role Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodCliqueSet) error {
	existingRoleNames, err := r.GetExistingResourceNames(ctx, logger, pgs.ObjectMeta)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeGetRole,
			component.OperationSync,
			fmt.Sprintf("Error getting existing Role names for PodCliqueSet: %v", client.ObjectKeyFromObject(pgs)),
		)
	}
	if len(existingRoleNames) > 0 {
		logger.Info("Role already exists, skipping creation", "existingRole", existingRoleNames[0])
		return nil
	}
	objectKey := getObjectKey(pgs.ObjectMeta)
	role := emptyRole(objectKey)
	logger.Info("Running CreateOrUpdate Role", "objectKey", objectKey)
	if err := r.buildResource(pgs, role); err != nil {
		return groveerr.WrapError(err,
			errSyncRole,
			component.OperationSync,
			fmt.Sprintf("Error building Role: %v for PodCliqueSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	if err := client.IgnoreAlreadyExists(r.client.Create(ctx, role)); err != nil {
		return groveerr.WrapError(err,
			errSyncRole,
			component.OperationSync,
			fmt.Sprintf("Error syncing Role: %v for PodCliqueSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("Created Role", "objectKey", objectKey)
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) error {
	objectKey := getObjectKey(pgsObjMeta)
	logger.Info("Triggering delete of Role", "objectKey", objectKey)
	if err := r.client.Delete(ctx, emptyRole(objectKey)); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Role not found, deletion is a no-op", "objectKey", objectKey)
			return nil
		}
		return groveerr.WrapError(err,
			errDeleteRole,
			component.OperationDelete,
			fmt.Sprintf("Error deleting Role: %v for PodCliqueSet: %v", objectKey, k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	logger.Info("deleted Role", "objectKey", objectKey)
	return nil
}

func (r _resource) buildResource(pgs *grovecorev1alpha1.PodCliqueSet, role *rbacv1.Role) error {
	role.Labels = getLabels(pgs.ObjectMeta)
	if err := controllerutil.SetControllerReference(pgs, role, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncRole,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for role: %v", client.ObjectKeyFromObject(role)),
		)
	}
	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "pods/status"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
	return nil
}

func getLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	roleLabels := map[string]string{
		apicommon.LabelComponentKey: apicommon.LabelComponentNamePodRole,
		apicommon.LabelAppNameKey:   strings.ReplaceAll(apicommon.GeneratePodRoleName(pgsObjMeta.Name), ":", "-"),
	}
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pgsObjMeta.Name),
		roleLabels,
	)
}

func getObjectKey(pgsObjMeta metav1.ObjectMeta) client.ObjectKey {
	return client.ObjectKey{
		Name:      apicommon.GeneratePodRoleName(pgsObjMeta.Name),
		Namespace: pgsObjMeta.Namespace,
	}
}

func emptyRole(objKey client.ObjectKey) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
