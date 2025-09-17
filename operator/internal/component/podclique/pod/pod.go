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

// Package pod provides pod lifecycle management for PodClique resources.
// It handles pod creation, deletion, synchronization, and scheduling gate management
// to ensure pods within a PodClique meet the desired state and startup ordering requirements.
package pod

import (
	"context"
	"fmt"
	"strconv"

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	"github.com/NVIDIA/grove/operator/api/common/constants"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	componentutils "github.com/NVIDIA/grove/operator/internal/component/utils"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/expect"
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

// Error codes for pod operations
const (
	errCodeGetPod                              grovecorev1alpha1.ErrorCode = "ERR_GET_POD"
	errCodeDeletePod                           grovecorev1alpha1.ErrorCode = "ERR_DELETE_POD"
	errCodeGetAvailablePodHostNameIndices      grovecorev1alpha1.ErrorCode = "ERR_GET_AVAILABLE_POD_HOSTNAME_INDICES"
	errCodeGetPodGang                          grovecorev1alpha1.ErrorCode = "ERR_GET_PODGANG"
	errCodeGetPodCliqueSet                     grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUESET"
	errCodeGetPodClique                        grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUE"
	errCodeListPod                             grovecorev1alpha1.ErrorCode = "ERR_LIST_POD"
	errCodeRemovePodSchedulingGate             grovecorev1alpha1.ErrorCode = "ERR_REMOVE_POD_SCHEDULING_GATE"
	errCodeCreatePod                           grovecorev1alpha1.ErrorCode = "ERR_CREATE_POD"
	errCodeMissingPodGangLabelOnPCLQ           grovecorev1alpha1.ErrorCode = "ERR_MISSING_PODGANG_LABEL_ON_PODCLIQUE"
	errCodeInitContainerImageEnvVarMissing     grovecorev1alpha1.ErrorCode = "ERR_INITCONTAINER_ENVIRONMENT_VARIABLE_MISSING"
	errCodeCreatePodCliqueExpectationsStoreKey grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODCLIQUE_EXPECTATIONS_STORE_KEY"
	errCodeDeletePodCliqueExpectations         grovecorev1alpha1.ErrorCode = "ERR_DELETE_PODCLIQUE_EXPECTATIONS_STORE_KEY"
	errCodeGetPodCliqueSetReplicaIndex         grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUESET_REPLICA_INDEX"
	errCodeSetControllerReference              grovecorev1alpha1.ErrorCode = "ERR_SET_CONTROLLER_REFERENCE"
	errCodeBuildPodResource                    grovecorev1alpha1.ErrorCode = "ERR_BUILD_POD_RESOURCE"
	errCodeMissingPodCliqueTemplate            grovecorev1alpha1.ErrorCode = "ERR_MISSING_PODCLIQUE_TEMPLATE"
	errCodeGetPodCliqueTemplate                grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUE_TEMPLATE"
	errCodeUpdatePodCliqueStatus               grovecorev1alpha1.ErrorCode = "ERR_UPDATE_PODCLIQUE_STATUS"
)

const (
	// podGangSchedulingGate is the scheduling gate applied to pods awaiting PodGang readiness
	podGangSchedulingGate = "grove.io/podgang-pending-creation"
)

// _resource implements the component operator for pod management within PodCliques.
// It provides the necessary dependencies for creating, deleting, and synchronizing pods
// according to PodClique specifications and PodGang scheduling requirements.
type _resource struct {
	client            client.Client             // Kubernetes client for API operations
	scheme            *runtime.Scheme           // Runtime scheme for object conversions
	eventRecorder     record.EventRecorder      // Event recorder for pod lifecycle events
	expectationsStore *expect.ExpectationsStore // Store for tracking create/delete expectations
}

// New creates an instance of Pod component operator.
// It initializes the operator with the necessary dependencies for managing pod lifecycle
// operations within PodCliques, including creation, deletion, and synchronization.
func New(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder, expectationsStore *expect.ExpectationsStore) component.Operator[grovecorev1alpha1.PodClique] {
	return &_resource{
		client:            client,
		scheme:            scheme,
		eventRecorder:     eventRecorder,
		expectationsStore: expectationsStore,
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

// Sync reconciles the current state of pods with the desired state specified in the PodClique.
// It orchestrates pod creation, deletion, updates, and scheduling gate management to ensure
// the PodClique maintains the correct number of pods in the appropriate states.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique) error {
	// Prepare synchronization context with all necessary state and dependencies
	sc, err := r.prepareSyncFlow(ctx, logger, pclq)
	if err != nil {
		return err
	}

	// Execute the main synchronization logic
	result := r.runSyncFlow(logger, sc)
	if result.hasErrors() {
		return result.getAggregatedError()
	}

	// Check if pods are still waiting for scheduling gates to be removed
	if result.hasPendingScheduleGatedPods() {
		return groveerr.New(groveerr.ErrCodeRequeueAfter,
			component.OperationSync,
			"some pods are still schedule gated. requeuing request to retry removal of scheduling gates",
		)
	}
	return nil
}

// buildResource constructs a pod resource from the PodClique specification.
// It sets up all necessary metadata, labels, environment variables, and configuration
// required for the pod to function properly within the PodClique ecosystem.
func (r _resource) buildResource(pcs *grovecorev1alpha1.PodCliqueSet, pclq *grovecorev1alpha1.PodClique, podGangName string, pod *corev1.Pod, podIndex int) error {
	// Extract PCS replica index from PodClique name for now (will be replaced with direct parameter)
	pcsName := componentutils.GetPodCliqueSetName(pclq.ObjectMeta)
	pcsReplicaIndex, err := utils.GetPodCliqueSetReplicaIndexFromPodCliqueFQN(pcsName, pclq.Name)
	if err != nil {
		return groveerr.WrapError(err,
			errCodeGetPodCliqueSetReplicaIndex,
			component.OperationSync,
			fmt.Sprintf("error extracting PCS replica index for PodClique %v", client.ObjectKeyFromObject(pclq)),
		)
	}

	// Set up pod metadata with appropriate labels and annotations
	labels := getLabels(pclq.ObjectMeta, pcsName, podGangName, pcsReplicaIndex)
	pod.ObjectMeta = metav1.ObjectMeta{
		GenerateName: fmt.Sprintf("%s-", pclq.Name),
		Namespace:    pclq.Namespace,
		Labels:       labels,
		Annotations:  pclq.Annotations,
	}

	// Set controller reference to ensure proper ownership chain
	if err = controllerutil.SetControllerReference(pclq, pod, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errCodeSetControllerReference,
			component.OperationSync,
			fmt.Sprintf("error setting controller reference of PodClique: %v on Pod", client.ObjectKeyFromObject(pclq)),
		)
	}

	// Copy pod specification and add scheduling gate for PodGang coordination
	pod.Spec = *pclq.Spec.PodSpec.DeepCopy()
	pod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: podGangSchedulingGate}}

	// Add GROVE specific Pod environment variables
	addEnvironmentVariables(pod, pclq, pcsName, pcsReplicaIndex, podIndex)

	// Configure hostname and subdomain for service discovery
	configurePodHostname(pcsName, pcsReplicaIndex, pclq.Name, pod, podIndex)

	// If there is a need to enforce a Startup-Order then configure the init container and add it to the Pod Spec.
	if len(pclq.Spec.StartsAfter) != 0 {
		return configurePodInitContainer(pcs, pclq, pod)
	}
	return nil
}

// Delete removes all pods associated with the given PodClique.
// It performs a bulk deletion of pods matching the PodClique's labels and cleans up
// the expectations store to ensure consistent state tracking.
func (r _resource) Delete(ctx context.Context, logger logr.Logger, pclqObjectMeta metav1.ObjectMeta) error {
	logger.Info("Triggering delete of all pods for the PodClique")

	// Delete all pods matching the PodClique's selector labels
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

	// Clean up expectations store for this PodClique
	pclqExpStoreKey, err := getPodCliqueExpectationsStoreKey(logger, component.OperationDelete, pclqObjectMeta)
	if err != nil {
		return err
	}
	if err = r.expectationsStore.DeleteExpectations(logger, pclqExpStoreKey); err != nil {
		return groveerr.WrapError(err,
			errCodeDeletePodCliqueExpectations,
			component.OperationDelete,
			fmt.Sprintf("failed to delete expectations store for PodClique %v", pclqObjectMeta.Name))
	}

	logger.Info("Successfully deleted all pods for the PodClique")
	return nil
}

// getSelectorLabelsForPods returns the labels used to select pods belonging to a PodClique.
// These labels are used for listing and bulk operations on pods managed by the PodClique.
func getSelectorLabelsForPods(pclqObjectMeta metav1.ObjectMeta) map[string]string {
	pcsName := k8sutils.GetFirstOwnerName(pclqObjectMeta)
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsName),
		map[string]string{
			apicommon.LabelPodClique: pclqObjectMeta.Name,
		},
	)
}

// getLabels constructs the complete set of labels for a pod within a PodClique.
// It combines default Grove labels, PodClique-specific labels, and any custom labels
// from the PodClique template.
func getLabels(pclqObjectMeta metav1.ObjectMeta, pcsName, podGangName string, pcsReplicaIndex int) map[string]string {
	labels := map[string]string{
		apicommon.LabelPodClique:                pclqObjectMeta.Name,
		apicommon.LabelPodCliqueSetReplicaIndex: strconv.Itoa(pcsReplicaIndex),
		apicommon.LabelPodGang:                  podGangName,
	}
	return lo.Assign(
		apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsName),
		pclqObjectMeta.Labels,
		labels,
	)
}

// addEnvironmentVariables adds Grove-specific environment variables to all containers and init-containers.
// These variables provide runtime context about the pod's position within the PodClique hierarchy
// and enable service discovery and coordination between pods.
func addEnvironmentVariables(pod *corev1.Pod, pclq *grovecorev1alpha1.PodClique, pcsName string, pcsReplicaIndex, podIndex int) {
	// Construct Grove environment variables with pod context
	groveEnvVars := []corev1.EnvVar{
		{
			Name:  constants.EnvVarPodCliqueSetName,
			Value: pcsName,
		},
		{
			Name:  constants.EnvVarPodCliqueSetIndex,
			Value: strconv.Itoa(pcsReplicaIndex),
		},
		{
			Name:  constants.EnvVarPodCliqueName,
			Value: pclq.Name,
		},
		{
			Name: constants.EnvVarHeadlessService,
			Value: apicommon.GenerateHeadlessServiceAddress(
				apicommon.ResourceNameReplica{Name: pcsName, Replica: pcsReplicaIndex},
				pod.Namespace),
		},
		{
			Name:  constants.EnvVarPodIndex,
			Value: strconv.Itoa(podIndex),
		},
	}

	// Add environment variables to all containers and init containers
	componentutils.AddEnvVarsToContainers(pod.Spec.Containers, groveEnvVars)
	componentutils.AddEnvVarsToContainers(pod.Spec.InitContainers, groveEnvVars)
}

// configurePodHostname sets the pod hostname and subdomain for service discovery.
// This enables pods to be addressable via DNS within the PodClique's headless service,
// facilitating inter-pod communication and coordination.
func configurePodHostname(pcsName string, pcsReplicaIndex int, pclqName string, pod *corev1.Pod, podIndex int) {
	// Set hostname for service discovery (e.g., "my-pclq-0")
	pod.Spec.Hostname = fmt.Sprintf("%s-%d", pclqName, podIndex)

	// Set subdomain to headless service name to enable DNS resolution
	pod.Spec.Subdomain = apicommon.GenerateHeadlessServiceName(
		apicommon.ResourceNameReplica{Name: pcsName, Replica: pcsReplicaIndex})
}
