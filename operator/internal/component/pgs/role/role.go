package role

import (
	"context"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errSyncRole   v1alpha1.ErrorCode = "ERR_SYNC_ROLE"
	errDeleteRole v1alpha1.ErrorCode = "ERR_DELETE_ROLE"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

func New(client client.Client, scheme *runtime.Scheme) component.Operator[v1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	obj := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgs.Name,
			Namespace: pgs.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"grove.io"},
				Resources: []string{"podcliques", "podcliques/status"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
	logger.Info("Creating Role", "name", obj.Name, "namespace", obj.Namespace)

	controllerutil.SetControllerReference(pgs, obj, r.scheme)

	role := &rbacv1.Role{}
	err := r.client.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, role)
	if err == nil {
		logger.Info("Role already exist", "name", obj.Name, "namespace", obj.Namespace)
		return nil
	}

	if client.IgnoreNotFound(err) == nil {
		err = r.client.Create(ctx, obj)
	}

	return groveerr.WrapError(err,
		errSyncRole,
		component.OperationSync,
		fmt.Sprintf("Error syncing Role for PodGangSet: %v", client.ObjectKey{Name: pgs.Name, Namespace: pgs.Namespace}),
	)
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, objMeta metav1.ObjectMeta) error {
	logger.Info("Deleting Role", "objectKey", client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace})

	obj := &rbacv1.Role{}
	err := r.client.Get(ctx, client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace}, obj)
	if err == nil {
		err = r.client.Delete(ctx, obj)
	} else if client.IgnoreNotFound(err) == nil {
		logger.Info("Role not found", "name", obj.Name, "namespace", obj.Namespace)
		return nil
	}

	if err != nil {
		return groveerr.WrapError(err,
			errDeleteRole,
			component.OperationDelete,
			fmt.Sprintf("Error deleting Role for PodGangSet: %v", client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace}),
		)
	}
	return nil
}
