package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/utils"

	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errSyncPGService v1alpha1.ErrorCode = "ERR_SYNC_PGSERVICE"
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
	// Do not create headless service if service spec is not defined.
	if pgs.Spec.Template.ServiceSpec == nil {
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

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet, pgServiceObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate PodGangService", "pgServiceObjectKey", pgServiceObjectKey)
	pgService := emptyPGService(pgServiceObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pgService, func() error {
		return r.buildResource(logger, pgService, pgs)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPGService,
			component.OperationSync,
			fmt.Sprintf("Error syncing Service: %v for PodGang: %v", pgServiceObjectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("triggered create or update of PodGangService", "pgServiceObjectKey", pgServiceObjectKey, "result", opResult)
	return nil
}

func (r _resource) buildResource(logger logr.Logger, svc *corev1.Service, pgs *v1alpha1.PodGangSet) error {
	// Set Service Spec
	// ------------------------------------
	svc.Spec = corev1.ServiceSpec{
		Selector: map[string]string{
			v1alpha1.LabelPartOfKey: pgs.Name,
		},
		ClusterIP:                "None",
		PublishNotReadyAddresses: pgs.Spec.Template.ServiceSpec.PublishNotReadyAddresses,
	}

	if err := controllerutil.SetControllerReference(pgs, svc, r.scheme); err != nil {
		return err
	}

	return nil
}

func getObjectKeys(pgs *v1alpha1.PodGangSet) []client.ObjectKey {
	pgServiceNames := getPGServiceNames(pgs)
	serviceObjKeys := make([]client.ObjectKey, 0, pgs.Spec.Replicas)
	for _, pgServiceName := range pgServiceNames {
		serviceObjKeys = append(serviceObjKeys, client.ObjectKey{
			Name:      pgServiceName,
			Namespace: pgs.Namespace,
		})
	}
	return serviceObjKeys
}

func getPGServiceNames(pgs *v1alpha1.PodGangSet) []string {
	pgServiceNames := make([]string, 0, pgs.Spec.Replicas)
	for replicaID := range pgs.Spec.Replicas {
		pgServiceNames = append(pgServiceNames, createPGName(pgs.Name, replicaID))
	}
	return pgServiceNames
}

func createPGName(pgName string, replicaID int32) string {
	return fmt.Sprintf("%s-%d", pgName, replicaID)
}

func emptyPGService(objKey client.ObjectKey) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}

func (_ _resource) Delete(ctx context.Context, logger logr.Logger, objMeta metav1.ObjectMeta) error {
	//TODO implement me
	return nil
}
