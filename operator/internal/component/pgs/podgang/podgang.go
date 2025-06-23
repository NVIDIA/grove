package podgang

import (
	"context"
	"errors"
	"fmt"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	groveschedulerv1alpha1 "github.com/NVIDIA/grove/scheduler/api/core/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	errCodeListPodGangs            grovecorev1alpha1.ErrorCode = "ERR_LIST_PODGANGS"
	errCodeDeletePodGangs          grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODGANGS"
	errCodeDeleteExcessPodGang     grovecorev1alpha1.ErrorCode = "ERR_DELETE_EXCESS_PODGANG"
	errCodeSyncPodGangs            grovecorev1alpha1.ErrorCode = "ERR_SYNC_PODGANGS"
	errCodeGetPodClique            grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUE_FOR_PODGANG"
	errCodeListPods                grovecorev1alpha1.ErrorCode = "ERR_LIST_PODS_FOR_PODGANGSET"
	errCodeListPodCliques          grovecorev1alpha1.ErrorCode = "ERR_LIST_PODCLIQUES_FOR_PODGANGSET"
	errCodePatchPodLabel           grovecorev1alpha1.ErrorCode = "ERR_PATCH_POD_LABELS_FOR_PODGANG"
	errCodeComputeExistingPodGangs grovecorev1alpha1.ErrorCode = "ERR_COMPUTE_EXISTING_PODGANG"
	errCodeSetControllerReference  grovecorev1alpha1.ErrorCode = "ERR_SET_CONTROLLER_REFERENCE"
	errCodeCreatePodGang           grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODGANG"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
	// eventRecorder record.EventRecorder
}

// func New(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) component.Operator[grovecorev1alpha1.PodGangSet] {
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
		// eventRecorder: eventRecorder,
	}
}

func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) ([]string, error) {
	logger.Info("Looking for existing PodGang resources created per replica of PodGangSet")
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(groveschedulerv1alpha1.SchemeGroupVersion.WithKind("PodGang"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(getPodGangSelectorLabels(pgs.ObjectMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errCodeListPodGangs,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing PodGang for PodGangSet: %v", client.ObjectKeyFromObject(pgs)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pgs.ObjectMeta, objMetaList.Items), nil
}

func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) error {
	sc, err := r.prepareSyncFlow(ctx, logger, pgs)
	if err != nil {
		return err
	}
	result := r.runSyncFlow(sc)
	if result.hasErrors() {
		return result.getAggregatedError()
	}
	if result.hasPodGangsPendingCreation() {
		return groveerr.New(groveerr.ErrCodeRequeueAfter, component.OperationSync, fmt.Sprintf("PodGangs pending creation: %v", result.podsGangsPendingCreation))
	}
	return errors.Join(result.errs...)
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering deletion of PodGangs")
	if err := r.client.DeleteAllOf(ctx,
		&groveschedulerv1alpha1.PodGang{},
		client.InNamespace(pgsObjectMeta.Namespace),
		client.MatchingLabels(getPodGangSelectorLabels(pgsObjectMeta))); err != nil {
		return groveerr.WrapError(err,
			errCodeDeletePodGangs,
			component.OperationDelete,
			fmt.Sprintf("Failed to delete PodGangs for PodGangSet: %v", k8sutils.GetObjectKeyFromObjectMeta(pgsObjectMeta)),
		)
	}
	logger.Info("Deleted PodGangs")
	return nil
}

func buildResource(podGang *groveschedulerv1alpha1.PodGang, pgs *grovecorev1alpha1.PodGangSet) error {
	return nil
}

func getPodGangSelectorLabels(pgsObjMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePodGang,
		})
}

func emptyPodGang(objKey client.ObjectKey) *groveschedulerv1alpha1.PodGang {
	return &groveschedulerv1alpha1.PodGang{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: objKey.Namespace,
			Name:      objKey.Name,
		},
	}
}

func getLabels(pgsName string) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NamePodGang,
		})
}
