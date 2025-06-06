package podcliquescalinggroup

import (
	"context"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errListPodCliqueScalingGroup   v1alpha1.ErrorCode = "ERR_GET_POD_CLIQUE_SCALING_GROUPS"
	errSyncPodCliqueScalingGroup   v1alpha1.ErrorCode = "ERR_SYNC_POD_CLIQUE_SCALING_GROUP"
	errDeletePodCliqueScalingGroup v1alpha1.ErrorCode = "ERR_DELETE_POD_CLIQUE_SCALING_GROUP"
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

func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pgs *v1alpha1.PodGangSet) ([]string, error) {
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(v1alpha1.SchemeGroupVersion.WithKind("PodCliqueScalingGroup"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(getPodCliqueScalingGroupSelectorLabels(pgs.ObjectMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errListPodCliqueScalingGroup,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodCliqueScalingGroup for PodGangSet: %v", pgs.Namespace))
	}
	return k8sutils.FilterMapOwnedResourceNames(pgs.ObjectMeta, objMetaList.Items), nil
}

func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet) error {
	tasks := make([]utils.Task, 0, int(pgs.Spec.Replicas)*len(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs))
	for replicaIndex := range pgs.Spec.Replicas {
		for _, pclqScalingGrpConfig := range pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs {
			pclqScalingGrpConfigObjKey := client.ObjectKey{
				Name:      v1alpha1.GeneratePodCliqueScalingGroupName(pgs.Name, replicaIndex, pclqScalingGrpConfig.Name),
				Namespace: pgs.Namespace,
			}
			createTask := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdatePodCliqueScalingGroup-%s", pclqScalingGrpConfigObjKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdate(ctx, logger, pgs, pclqScalingGrpConfigObjKey)
				},
			}
			tasks = append(tasks, createTask)
		}
	}
	if runResult := utils.RunConcurrently(ctx, logger, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error creating or updating PodCliqueScalingGroup for PodGangSet: %v, run summary: %s", client.ObjectKeyFromObject(pgs), runResult.GetSummary()),
		)
	}
	logger.Info("Successfully synced PodCliqueScalingGroup for PodGangSet")
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) error {
	logger.Info("Triggering delete of PodCliqueScalingGroups")
	if err := r.client.DeleteAllOf(ctx,
		&v1alpha1.PodCliqueScalingGroup{},
		client.InNamespace(pgsObjMeta.Namespace),
		client.MatchingLabels(getPodCliqueScalingGroupSelectorLabels(pgsObjMeta))); err != nil {
		return groveerr.WrapError(err,
			errDeletePodCliqueScalingGroup,
			component.OperationDelete,
			fmt.Sprintf("Error deleting PodCliqueScalingGroup for PodGangSet: %v", pgsObjMeta.Name),
		)
	}
	logger.Info("Deleted PodCliqueScalingGroups")
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pgs *v1alpha1.PodGangSet, pclqScalingGrpObjectKey client.ObjectKey) error {
	logger.Info("CreateOrUpdate PodCliqueScalingGroup", "objectKey", pclqScalingGrpObjectKey)
	pclqScalingGrp := emptyPodCliqueScalingGroup(pclqScalingGrpObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pclqScalingGrp, func() error {
		return r.buildResource(pclqScalingGrp, pgs)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error in create/update of PodCliqueScalingGroup: %v for PodGangSet: %v", pclqScalingGrpObjectKey, client.ObjectKeyFromObject(pgs)),
		)
	}
	logger.Info("Triggered create or update of PodCliqueScalingGroup", "objectKey", pclqScalingGrpObjectKey, "result", opResult)
	return nil
}

func (r _resource) buildResource(pclqScalingGroup *v1alpha1.PodCliqueScalingGroup, pgs *v1alpha1.PodGangSet) error {
	if err := controllerutil.SetControllerReference(pgs, pclqScalingGroup, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncPodCliqueScalingGroup,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for PodCliqueScalingGroup: %v", client.ObjectKeyFromObject(pclqScalingGroup)),
		)
	}
	// NOTE: If the resource is created for the first time then there is no need to explicitly set the replicas as it will
	// be defaulted to 1 as the defaults are captured as part of the generated OpenAPIv3 specification for the resource.
	// If there is an update event for PodGangSet then replicas
	pclqScalingGroup.Labels = getLabels(pgs.Name, client.ObjectKeyFromObject(pclqScalingGroup))
	return nil
}

func getLabels(pgsName string, pclqScalingGroupObjKey client.ObjectKey) map[string]string {
	componentLabels := map[string]string{
		v1alpha1.LabelAppNameKey:   pclqScalingGroupObjKey.Name,
		v1alpha1.LabelComponentKey: component.NamePodCliqueScalingGroup,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		componentLabels,
	)
}

func getPodCliqueScalingGroupSelectorLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		map[string]string{
			v1alpha1.LabelComponentKey: component.NamePodCliqueScalingGroup,
		},
	)
}

func emptyPodCliqueScalingGroup(objKey client.ObjectKey) *v1alpha1.PodCliqueScalingGroup {
	return &v1alpha1.PodCliqueScalingGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
