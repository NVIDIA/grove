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
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
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
	errCodeGetPod    grovecorev1alpha1.ErrorCode = "ERR_GET_POD"
	errCodeSyncPod   grovecorev1alpha1.ErrorCode = "ERR_SYNC_POD"
	errCodeDeletePod grovecorev1alpha1.ErrorCode = "ERR_DELETE_POD"

	errCodeGetPodGang                grovecorev1alpha1.ErrorCode = "ERR_GET_PODGANG"
	errCodeGetPodClique              grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUE"
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

func (r _resource) buildResource(pclq *grovecorev1alpha1.PodClique, podGangName string, pod *corev1.Pod, podIndex int) error {
	// Extract PGS replica index from PodClique name for now (will be replaced with direct parameter)
	pgsName := componentutils.GetPodGangSetName(pclq.ObjectMeta)
	pgsReplicaIndex, err := utils.GetPodGangSetReplicaIndexFromPodCliqueFQN(pgsName, pclq.Name)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeSyncPod,
			component.OperationSync,
			fmt.Sprintf("error extracting PGS replica index for PodClique %v", client.ObjectKeyFromObject(pclq)),
		)
	}

	labels := getLabels(pclq.ObjectMeta, pgsName, podGangName, pgsReplicaIndex)
	pod.ObjectMeta = metav1.ObjectMeta{
		GenerateName: fmt.Sprintf("%s-", pclq.Name),
		Namespace:    pclq.Namespace,
		Labels:       labels,
	}
	if err = controllerutil.SetControllerReference(pclq, pod, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errCodeSyncPod,
			component.OperationSync,
			fmt.Sprintf("error setting controller reference of PodClique: %v on Pod", client.ObjectKeyFromObject(pclq)),
		)
	}
	pod.Spec = *pclq.Spec.PodSpec.DeepCopy()
	pod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: podGangSchedulingGate}}

	addEnvironmentVariables(pod, pclq, pgsName, pgsReplicaIndex, podIndex)
	// Configure hostname and subdomain for service discovery
	configurePodHostname(pod, pclq.Name, podIndex, pgsName, pgsReplicaIndex)

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

func getLabels(pclqObjectMeta metav1.ObjectMeta, pgsName, podGangName string, pgsReplicaIndex int) map[string]string {
	labels := map[string]string{
		grovecorev1alpha1.LabelPodClique:              pclqObjectMeta.Name,
		grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
		grovecorev1alpha1.LabelPodGang:                podGangName,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		pclqObjectMeta.Labels,
		labels)
}

// addEnvironmentVariables adds Grove-specific environment variables to all containers and init-containers.
func addEnvironmentVariables(pod *corev1.Pod, pclq *grovecorev1alpha1.PodClique, pgsName string, pgsReplicaIndex, podIndex int) {
	groveEnvVars := []corev1.EnvVar{
		{
			Name:  grovecorev1alpha1.EnvVarPGSName,
			Value: pgsName,
		},
		{
			Name:  grovecorev1alpha1.EnvVarPGSIndex,
			Value: strconv.Itoa(pgsReplicaIndex),
		},
		{
			Name:  grovecorev1alpha1.EnvVarPCLQName,
			Value: pclq.Name,
		},
		{
			Name: grovecorev1alpha1.EnvVarHeadlessService,
			Value: grovecorev1alpha1.GenerateHeadlessServiceAddress(
				grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplicaIndex},
				pod.Namespace),
		},
		{
			Name:  grovecorev1alpha1.EnvVarPodIndex,
			Value: strconv.Itoa(podIndex),
		},
	}
	componentutils.AddEnvVarsToContainers(pod.Spec.Containers, groveEnvVars)
	componentutils.AddEnvVarsToContainers(pod.Spec.InitContainers, groveEnvVars)
}

// configurePodHostname sets the pod hostname and subdomain for service discovery
func configurePodHostname(pod *corev1.Pod, pclqName string, podIndex int, pgsName string, pgsReplicaIndex int) {
	// Set hostname for service discovery (e.g., "my-pclq-0")
	pod.Spec.Hostname = fmt.Sprintf("%s-%d", pclqName, podIndex)

	// Set subdomain to headless service name (reusing existing logic)
	pod.Spec.Subdomain = grovecorev1alpha1.GenerateHeadlessServiceName(
		grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplicaIndex})
}
