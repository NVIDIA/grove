package serviceaccount

import (
	"context"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errSyncServiceAccount   v1alpha1.ErrorCode = "ERR_SYNC_SERVICEACCOUNT"
	errDeleteServiceAccount v1alpha1.ErrorCode = "ERR_DELETE_SERVICEACCOUNT"
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
	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgs.Name,
			Namespace: pgs.Namespace,
		},
	}
	logger.Info("Creating ServiceAccount", "name", obj.Name, "namespace", obj.Namespace)

	controllerutil.SetControllerReference(pgs, obj, r.scheme)

	sa := &corev1.ServiceAccount{}
	err := r.client.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, sa)
	if err == nil {
		logger.Info("ServiceAccount already exist", "name", obj.Name, "namespace", obj.Namespace)
		return nil
	}

	if client.IgnoreNotFound(err) == nil {
		err = r.client.Create(ctx, obj)
	}

	return groveerr.WrapError(err,
		errSyncServiceAccount,
		component.OperationSync,
		fmt.Sprintf("Error syncing ServiceAccount for PodGangSet: %v", client.ObjectKey{Name: pgs.Name, Namespace: pgs.Namespace}),
	)
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, objMeta metav1.ObjectMeta) error {
	logger.Info("Deleting ServiceAccount", "objectKey", client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace})

	obj := &corev1.ServiceAccount{}
	err := r.client.Get(ctx, client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace}, obj)
	if err == nil {
		err = r.client.Delete(ctx, obj)
	} else if client.IgnoreNotFound(err) == nil {
		logger.Info("ServiceAccount not found", "name", obj.Name, "namespace", obj.Namespace)
		return nil
	}

	if err != nil {
		return groveerr.WrapError(err,
			errDeleteServiceAccount,
			component.OperationDelete,
			fmt.Sprintf("Error deleting ServiceAccount for PodGangSet: %v", client.ObjectKey{Name: objMeta.Name, Namespace: objMeta.Namespace}),
		)
	}
	return nil
}
