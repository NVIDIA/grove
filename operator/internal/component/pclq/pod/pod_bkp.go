package pod

//import (
//	"context"
//	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
//	"github.com/NVIDIA/grove/operator/internal/component"
//	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
//	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
//	"github.com/go-logr/logr"
//	"github.com/samber/lo"
//	corev1 "k8s.io/api/core/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"k8s.io/apimachinery/pkg/runtime"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//)
//
//const (
//	errGetPod    grovecorev1alpha1.ErrorCode = "ERR_GET_POD"
//	errSyncPod   grovecorev1alpha1.ErrorCode = "ERR_SYNC_POD"
//	errDeletePod grovecorev1alpha1.ErrorCode = "ERR_DELETE_POD"
//)
//
//type _resource struct {
//	client client.Client
//	scheme *runtime.Scheme
//}
//
//func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodClique] {
//	return &_resource{
//		client: client,
//		scheme: scheme,
//	}
//}
//
//// GetExistingResourceNames returns the names of all the existing pods for the given PodClique.
//// NOTE: Since we do not currently support Jobs, therefore we do not have to filter the pods that are reached their final state.
//// Pods created for Jobs can reach corev1.PodSucceeded state or corev1.PodFailed state but these are not relevant for us at the moment.
//// In future when these states become relevant then we have to list the pods and filter on their status.Phase.
//func (r _resource) GetExistingResourceNames(ctx context.Context, log logr.Logger, pclq *grovecorev1alpha1.PodClique) ([]string, error) {
//	podNames := make([]string, 0, pclq.Spec.Replicas)
//	objMetaList := &metav1.PartialObjectMetadataList{}
//	objMetaList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
//	if err := r.client.List(ctx,
//		objMetaList,
//		client.InNamespace(pclq.Namespace),
//		client.MatchingLabels(getSelectorLabelsForPods(pclq.ObjectMeta)),
//	); err != nil {
//		return podNames, groveerr.WrapError(err,
//			errGetPod,
//			component.OperationGetExistingResourceNames,
//			"failed to list pods",
//		)
//	}
//	for _, pod := range objMetaList.Items {
//		if metav1.IsControlledBy(&pod, &pclq.ObjectMeta) {
//			podNames = append(podNames, pod.Name)
//		}
//	}
//	return podNames, nil
//}
//
//func (r _resource) Sync(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) error {
//	//TODO Implement me
//	return nil
//}
//
//func (r _resource) Delete(ctx context.Context, logger logr.Logger, pclqObjectMeta metav1.ObjectMeta) error {
//	//TODO Implement me
//	return nil
//}
//
//func getSelectorLabelsForPods(pclqObjectMeta metav1.ObjectMeta) map[string]string {
//	lo.Assign(
//		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(),
//	)
//}
