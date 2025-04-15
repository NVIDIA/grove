package serviceaccount

import (
	"context"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errGetServiceAccount    v1alpha1.ErrorCode = "ERR_GET_SERVICEACCOUNT"
	errSyncServiceAccount   v1alpha1.ErrorCode = "ERR_SYNC_SERVICEACCOUNT"
	errDeleteServiceAccount v1alpha1.ErrorCode = "ERR_DELETE_SERVICEACCOUNT"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of ServiceAccount component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[v1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the ServiceAccount Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgs *v1alpha1.PodGangSet) ([]string, error) {
	saNames := make([]string, 0, 1)
	objectKey := getObjectKey(pgs.ObjectMeta)
	objMeta := &metav1.PartialObjectMetadata{}
	objMeta.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))
	if err := r.client.Get(ctx, objectKey, objMeta); err != nil {
		if errors.IsNotFound(err) {
			return saNames, nil
		}
		return saNames, groveerr.WrapError(err,
			errGetServiceAccount,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error getting ServiceAccount: %v for PodGangSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	if metav1.IsControlledBy(objMeta, &pgs.ObjectMeta) {
		saNames = append(saNames, objMeta.Name)
	}
	return saNames, nil
}

// Sync synchronizes all resources that the ServiceAccount Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	objectKey := getObjectKey(pgs.ObjectMeta)
	sa := emptyServiceAccount(objectKey)

	logger.Info("Running CreateOrUpdate ServiceAccount", "objectKey", objectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, sa, func() error {
		return r.buildResource(pgs, sa)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncServiceAccount,
			component.OperationSync,
			fmt.Sprintf("Error syncing ServiceAccount: %v for PodGangSet: %v", objectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("triggered create or update of ServiceAccount", "objectKey", objectKey, "result", opResult)
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) error {
	objectKey := getObjectKey(pgsObjMeta)
	logger.Info("Triggering delete of ServiceAccount", "objectKey", objectKey)
	if err := r.client.Delete(ctx, emptyServiceAccount(objectKey)); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("ServiceAccount not found, deletion is a no-op", "objectKey", objectKey)
			return nil
		}
		return groveerr.WrapError(err,
			errDeleteServiceAccount,
			component.OperationDelete,
			fmt.Sprintf("Error deleting ServiceAccount: %v for PodGangSet: %v", objectKey, pgsObjMeta),
		)
	}
	logger.Info("deleted ServiceAccount", "objectKey", objectKey)
	return nil
}

func (r _resource) buildResource(pgs *v1alpha1.PodGangSet, sa *corev1.ServiceAccount) error {
	sa.Labels = getLabels(pgs.ObjectMeta)
	if err := controllerutil.SetControllerReference(pgs, sa, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncServiceAccount,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for ServiceAccount: %v", client.ObjectKeyFromObject(sa)),
		)
	}
	// TODO: Check if we need to make sa.AutomountServiceAccountToken configurable. This will enable use to use ServiceAccount projected tokens.
	return nil
}

func getLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	roleLabels := map[string]string{
		v1alpha1.LabelComponentKey: component.NamePodServiceAccount,
		v1alpha1.LabelAppNameKey:   component.GeneratePodServiceAccountName(pgsObjMeta),
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		roleLabels,
	)
}

func getObjectKey(pgsObjMeta metav1.ObjectMeta) client.ObjectKey {
	return client.ObjectKey{
		Name:      component.GeneratePodServiceAccountName(pgsObjMeta),
		Namespace: pgsObjMeta.Namespace,
	}
}

func emptyServiceAccount(objKey client.ObjectKey) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
