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

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
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
	errGetRole    v1alpha1.ErrorCode = "ERR_GET_ROLE"
	errSyncRole   v1alpha1.ErrorCode = "ERR_SYNC_ROLE"
	errDeleteRole v1alpha1.ErrorCode = "ERR_DELETE_ROLE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of Role component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[v1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the Role Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgs *v1alpha1.PodGangSet) ([]string, error) {
	roleNames := make([]string, 0, 1)
	objectKey := getObjectKey(pgs.ObjectMeta)
	objMeta := &metav1.PartialObjectMetadata{}
	objMeta.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("Role"))
	if err := r.client.Get(ctx, objectKey, objMeta); err != nil {
		if errors.IsNotFound(err) {
			return roleNames, nil
		}
		return roleNames, groveerr.WrapError(err,
			errGetRole,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error getting Role: %v for PodGangSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	if metav1.IsControlledBy(objMeta, &pgs.ObjectMeta) {
		roleNames = append(roleNames, objMeta.Name)
	}
	return roleNames, nil
}

// Sync synchronizes all resources that the Role Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	objectKey := getObjectKey(pgs.ObjectMeta)
	role := emptyRole(objectKey)
	logger.Info("Running CreateOrUpdate Role", "objectKey", objectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, role, func() error {
		return r.buildResource(pgs, role)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncRole,
			component.OperationSync,
			fmt.Sprintf("Error syncing Role: %v for PodGangSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("triggered create or update of Role", "objectKey", objectKey, "result", opResult)
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
			fmt.Sprintf("Error deleting Role: %v for PodGangSet: %v", objectKey, k8sutils.GetObjectKeyFromObjectMeta(pgsObjMeta)),
		)
	}
	logger.Info("deleted Role", "objectKey", objectKey)
	return nil
}

func (r _resource) buildResource(pgs *v1alpha1.PodGangSet, role *rbacv1.Role) error {
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
			APIGroups:     []string{v1alpha1.GroupName},
			Resources:     []string{"podcliques", "podcliques/status"},
			Verbs:         []string{"get", "list", "watch"},
			ResourceNames: getAllPodCliqueNames(pgs),
		},
	}
	return nil
}

func getAllPodCliqueNames(pgs *v1alpha1.PodGangSet) []string {
	cliqueNames := make([]string, 0, int(pgs.Spec.Replicas)*len(pgs.Spec.TemplateSpec.Cliques))
	for replicaIndex := range pgs.Spec.Replicas {
		for _, pclqTemplateSpec := range pgs.Spec.TemplateSpec.Cliques {
			cliqueNames = append(cliqueNames, v1alpha1.GeneratePodCliqueName(pgs.Name, int(replicaIndex), pclqTemplateSpec.Name))
		}
	}
	return cliqueNames
}

func getLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	roleLabels := map[string]string{
		v1alpha1.LabelComponentKey: component.NamePodRole,
		v1alpha1.LabelAppNameKey:   strings.ReplaceAll(v1alpha1.GeneratePodRoleName(pgsObjMeta), ":", "-"),
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		roleLabels,
	)
}

func getObjectKey(pgsObjMeta metav1.ObjectMeta) client.ObjectKey {
	return client.ObjectKey{
		Name:      v1alpha1.GeneratePodRoleName(pgsObjMeta),
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
