package rolebinding

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
	errSyncRoleBinding   v1alpha1.ErrorCode = "ERR_SYNC_ROLEBINDING"
	errDeleteRoleBinding v1alpha1.ErrorCode = "ERR_DELETE_ROLEBINDING"
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
	obj := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgs.Name,
			Namespace: pgs.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup:  "",
				Kind:      "ServiceAccount",
				Name:      pgs.Name,
				Namespace: pgs.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     pgs.Name,
		},
	}
	logger.Info("Creating RoleBinding", "name", obj.Name, "namespace", obj.Namespace)

	controllerutil.SetControllerReference(pgs, obj, r.scheme)

	rolebinding := &rbacv1.RoleBinding{}
	err := r.client.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, rolebinding)
	if err == nil {
		logger.Info("RoleBinding already exist", "name", obj.Name, "namespace", obj.Namespace)
		return nil
	}

	if client.IgnoreNotFound(err) == nil {
		err = r.client.Create(ctx, obj)
	}

	return groveerr.WrapError(err,
		errSyncRoleBinding,
		component.OperationSync,
		fmt.Sprintf("Error syncing RoleBinding for PodGangSet: %v", client.ObjectKey{Name: pgs.Name, Namespace: pgs.Namespace}),
	)
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, objMeta metav1.ObjectMeta) error {
	logger.Info("Deleting RoleBinding", "objectKey", client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace})

	obj := &rbacv1.RoleBinding{}
	err := r.client.Get(ctx, client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace}, obj)
	if err == nil {
		err = r.client.Delete(ctx, obj)
	} else if client.IgnoreNotFound(err) == nil {
		logger.Info("RoleBinding not found", "name", obj.Name, "namespace", obj.Namespace)
		return nil
	}

	if err != nil {
		return groveerr.WrapError(err,
			errDeleteRoleBinding,
			component.OperationDelete,
			fmt.Sprintf("Error deleting RoleBinding for PodGangSet: %v", client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace}),
		)
	}
	return nil
}
