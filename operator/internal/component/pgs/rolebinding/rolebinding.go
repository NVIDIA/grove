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

package rolebinding

import (
	"context"
	"fmt"
	"strings"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errGetRoleBinding    v1alpha1.ErrorCode = "ERR_GET_ROLEBINDING"
	errSyncRoleBinding   v1alpha1.ErrorCode = "ERR_SYNC_ROLEBINDING"
	errDeleteRoleBinding v1alpha1.ErrorCode = "ERR_DELETE_ROLEBINDING"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of RoleBinding component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[v1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the RoleBinding Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgs *v1alpha1.PodGangSet) ([]string, error) {
	roleBindingNames := make([]string, 0, 1)
	objectKey := getObjectKey(pgs.ObjectMeta)
	objMeta := &metav1.PartialObjectMetadata{}
	objMeta.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("RoleBinding"))
	if err := r.client.Get(ctx, objectKey, objMeta); err != nil {
		if errors.IsNotFound(err) {
			return roleBindingNames, nil
		}
		return roleBindingNames, groveerr.WrapError(err,
			errGetRoleBinding,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error getting RoleBinding: %v for PodGangSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	if metav1.IsControlledBy(objMeta, &pgs.ObjectMeta) {
		roleBindingNames = append(roleBindingNames, objMeta.Name)
	}
	return roleBindingNames, nil
}

// Sync synchronizes all resources that the RoleBinding Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	objectKey := getObjectKey(pgs.ObjectMeta)
	role := emptyRoleBinding(objectKey)
	logger.Info("Running CreateOrUpdate RoleBinding", "objectKey", objectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, role, func() error {
		return r.buildResource(pgs, role)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncRoleBinding,
			component.OperationSync,
			fmt.Sprintf("Error syncing RoleBinding: %v for PodGangSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("triggered create or update of RoleBinding", "objectKey", objectKey, "result", opResult)
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) error {
	objectKey := getObjectKey(pgsObjMeta)
	logger.Info("Triggering delete of RoleBinding", "objectKey", objectKey)
	if err := r.client.Delete(ctx, emptyRoleBinding(objectKey)); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("RoleBinding not found, deletion is a no-op", "objectKey", objectKey)
			return nil
		}
		return groveerr.WrapError(err,
			errDeleteRoleBinding,
			component.OperationDelete,
			fmt.Sprintf("Error deleting RoleBinding: %v for PodGangSet: %v", objectKey, pgsObjMeta),
		)
	}
	logger.Info("deleted RoleBinding", "objectKey", objectKey)
	return nil
}

func (r _resource) buildResource(pgs *v1alpha1.PodGangSet, roleBinding *rbacv1.RoleBinding) error {
	roleBinding.Labels = getLabels(pgs.ObjectMeta)
	if err := controllerutil.SetControllerReference(pgs, roleBinding, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncRoleBinding,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for RoleBinding: %v", client.ObjectKeyFromObject(roleBinding)),
		)
	}
	roleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.SchemeGroupVersion.Group,
		Kind:     "Role",
		Name:     component.GeneratePodRoleName(pgs.ObjectMeta),
	}
	roleBinding.Subjects = []rbacv1.Subject{
		{
			APIGroup:  corev1.SchemeGroupVersion.Group,
			Kind:      "ServiceAccount",
			Name:      component.GeneratePodServiceAccountName(pgs.ObjectMeta),
			Namespace: pgs.Namespace,
		},
	}
	return nil
}

func getLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	roleLabels := map[string]string{
		v1alpha1.LabelComponentKey: component.NamePodRoleBinding,
		v1alpha1.LabelAppNameKey:   strings.ReplaceAll(component.GeneratePodRoleBindingName(pgsObjMeta), ":", "-"),
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		roleLabels,
	)
}

func getObjectKey(pgsObjMeta metav1.ObjectMeta) client.ObjectKey {
	return client.ObjectKey{
		Name:      component.GeneratePodRoleBindingName(pgsObjMeta),
		Namespace: pgsObjMeta.Namespace,
	}
}

func emptyRoleBinding(objKey client.ObjectKey) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
