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

package pod

import (
	"context"
	"fmt"
	"strconv"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// constants for error codes
const (
	errCodeGetPod                    grovecorev1alpha1.ErrorCode = "ERR_GET_POD"
	errCodeSyncPod                   grovecorev1alpha1.ErrorCode = "ERR_SYNC_POD"
	errCodeDeletePod                 grovecorev1alpha1.ErrorCode = "ERR_DELETE_POD"
	errCodeGetPodGang                grovecorev1alpha1.ErrorCode = "ERR_GET_PODGANG"
	errCodeListPod                   grovecorev1alpha1.ErrorCode = "ERR_LIST_POD"
	errCodeRemovePodSchedulingGate   grovecorev1alpha1.ErrorCode = "ERR_REMOVE_POD_SCHEDULING_GATE"
	errCodeCreatePods                grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODS"
	errCodeMissingPodGangLabelOnPCLQ grovecorev1alpha1.ErrorCode = "ERR_MISSING_PODGANG_LABEL_ON_PODCLIQUE"
)

// constants used for pod events
const (
	reasonPodCreationSuccessful = "PodCreationSuccessful"
	reasonPodCreationFailed     = "PodCreationFailed"
	reasonPodDeletionSuccessful = "PodDeletionSuccessful"
	reasonPodDeletionFailed     = "PodDeletionFailed"
)

const (
	podGangSchedulingGate = "grove.io/podgang-pending-creation"
)

type _resource struct {
	client        client.Client
	scheme        *runtime.Scheme
	eventRecorder record.EventRecorder
}

// New creates an instance of Pod component operator.
func New(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) component.Operator[grovecorev1alpha1.PodClique] {
	return &_resource{
		client:        client,
		scheme:        scheme,
		eventRecorder: eventRecorder,
	}
}

// GetExistingResourceNames returns the names of all the existing pods for the given PodClique.
// NOTE: Since we do not currently support Jobs, therefore we do not have to filter the pods that are reached their final state.
// Pods created for Jobs can reach corev1.PodSucceeded state or corev1.PodFailed state but these are not relevant for us at the moment.
// In future when these states become relevant then we have to list the pods and filter on their status.Phase.
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pclqObjMeta metav1.ObjectMeta) ([]string, error) {
	var podNames []string
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pclqObjMeta.Namespace),
		client.MatchingLabels(getSelectorLabelsForPods(pclqObjMeta)),
	); err != nil {
		return podNames, groveerr.WrapError(err,
			errCodeGetPod,
			component.OperationGetExistingResourceNames,
			"failed to list pods",
		)
	}
	for _, pod := range objMetaList.Items {
		if metav1.IsControlledBy(&pod, &pclqObjMeta) {
			podNames = append(podNames, pod.Name)
		}
	}
	return podNames, nil
}

func (r _resource) Sync(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) error {
	sc, err := r.prepareSyncFlow(ctx, logger, pclq)
	if err != nil {
		return err
	}
	result := r.runSyncFlow(sc, logger)
	if result.hasErrors() {
		return result.getAggregatedError()
	}
	if result.hasPendingScheduleGatedPods() {
		return groveerr.New(groveerr.ErrCodeRequeueAfter,
			component.OperationSync,
			"some pods are still schedule gated. requeuing request to retry removal of scheduling gates",
		)
	}
	return nil
}

func (r _resource) buildResource(pclq *grovecorev1alpha1.PodClique, podGangName string, pod *corev1.Pod) error {
	labels, err := getLabels(pclq.ObjectMeta, podGangName)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeSyncPod,
			component.OperationSync,
			fmt.Sprintf("error building Pod resource for PodClique %v", client.ObjectKeyFromObject(pclq)),
		)
	}
	pod.ObjectMeta = metav1.ObjectMeta{
		GenerateName: fmt.Sprintf("%s-", pclq.Name),
		Namespace:    pclq.Namespace,
		Labels:       labels,
	}
	if err := controllerutil.SetControllerReference(pclq, pod, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errCodeSyncPod,
			component.OperationSync,
			fmt.Sprintf("error setting controller reference of PodClique: %v on Pod", client.ObjectKeyFromObject(pclq)),
		)
	}
	pod.Spec = *pclq.Spec.PodSpec.DeepCopy()
	pod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: podGangSchedulingGate}}

	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pclqObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering delete of all pods for the PodClique")
	if err := r.client.DeleteAllOf(ctx,
		&corev1.Pod{},
		client.InNamespace(pclqObjectMeta.Namespace),
		client.MatchingLabels(getSelectorLabelsForPods(pclqObjectMeta))); err != nil {
		return groveerr.WrapError(err,
			errCodeDeletePod,
			component.OperationDelete,
			fmt.Sprintf("failed to delete all pods for PodClique %v", k8sutils.GetObjectKeyFromObjectMeta(pclqObjectMeta)),
		)
	}
	logger.Info("Successfully deleted all pods for the PodClique")
	return nil
}

func getSelectorLabelsForPods(pclqObjectMeta metav1.ObjectMeta) map[string]string {
	pgsName := k8sutils.GetFirstOwnerName(pclqObjectMeta)
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelPodClique: pclqObjectMeta.Name,
		},
	)
}

func getLabels(pclqObjectMeta metav1.ObjectMeta, podGangName string) (map[string]string, error) {
	pgsName := k8sutils.GetFirstOwnerName(pclqObjectMeta)
	pgsReplicaIndex, err := utils.GetPodGangSetReplicaIndexFromPodCliqueFQN(pgsName, pclqObjectMeta.Name)
	if err != nil {
		return nil, err
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		pclqObjectMeta.Labels,
		map[string]string{
			grovecorev1alpha1.LabelPodClique:              pclqObjectMeta.Name,
			grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
			grovecorev1alpha1.LabelPodGang:                podGangName,
		}), nil
}
