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
	errCodeGetPod                             grovecorev1alpha1.ErrorCode = "ERR_GET_POD"
	errCodeSyncPod                            grovecorev1alpha1.ErrorCode = "ERR_SYNC_POD"
	errCodeDeletePod                          grovecorev1alpha1.ErrorCode = "ERR_DELETE_POD"
	errCodeCreatePodGangNameFromPCLQ          grovecorev1alpha1.ErrorCode = "ERR_CREATE_POD_GANG_NAME"
	errCodeGetPodGangMetadata                 grovecorev1alpha1.ErrorCode = "ERR_GET_PODGANG_METADATA"
	errCodeGetPodGangSet                      grovecorev1alpha1.ErrorCode = "ERR_GET_PODGANGSET"
	errCodeListPodGang                        grovecorev1alpha1.ErrorCode = "ERR_LIST_PODGANG"
	errCodeExtractPGSReplicaFromPCLQFQN       grovecorev1alpha1.ErrorCode = "ERR_EXTRACT_PODGANSET_REPLICA_FROM_PODCLIQUE_FQN"
	errCodeListPod                            grovecorev1alpha1.ErrorCode = "ERR_LIST_POD"
	errCodeGetPodCliqueScalingGroup           grovecorev1alpha1.ErrorCode = "ERR_GET_PODCLIQUESCALINGGROUP"
	errCodeMissingPodGangSetReplicaIndexLabel grovecorev1alpha1.ErrorCode = "ERR_MISSING_PODGANGSET_REPLICA_INDEX_LABEL"
	errCodeInvalidPodGangSetReplicaLabelValue grovecorev1alpha1.ErrorCode = "ERR_INVALID_PODGANGSET_REPLICA_LABEL_VALUE"
	errCodeRemovePodSchedulingGate            grovecorev1alpha1.ErrorCode = "ERR_REMOVE_POD_SCHEDULING_GATE"
	errCodeCreatePods                         grovecorev1alpha1.ErrorCode = "ERR_CREATE_PODS"
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

const (
	// TODO: @renormalize check if this is duplicate
	noPodGangAssigned = "__no_pod_gang_assigned__"
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
func (r _resource) GetExistingResourceNames(ctx context.Context, _ logr.Logger, pclq *grovecorev1alpha1.PodClique) ([]string, error) {
	podNames := make([]string, 0, pclq.Spec.Replicas)
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pclq.Namespace),
		client.MatchingLabels(getSelectorLabelsForPods(pclq.ObjectMeta)),
	); err != nil {
		return podNames, groveerr.WrapError(err,
			errCodeGetPod,
			component.OperationGetExistingResourceNames,
			"failed to list pods",
		)
	}
	for _, pod := range objMetaList.Items {
		if metav1.IsControlledBy(&pod, &pclq.ObjectMeta) {
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

	if err := r.runSyncFlow(sc, logger); err != nil {
		return err
	}

	// pgs, err := r.getOwnerPodGangSet(ctx, &pclq.ObjectMeta)
	// if err != nil {
	// 	return err
	// }

	// pclqPods, err := componentutils.GetPCLQPods(ctx, r.client, pgs.Name, pclq)
	// if err != nil {
	// 	logger.Error(err, "Failed to list pods that belong to PodClique %v", client.ObjectKeyFromObject(pclq))
	// 	return groveerr.WrapError(err,
	// 		errCodeListPod,
	// 		component.OperationSync,
	// 		fmt.Sprintf("failed to list pods that belong to the PodClique %v", client.ObjectKeyFromObject(pclq)),
	// 	)
	// }

	// existingPodGangNames, err := r.getExistingPodGangNamesAssociatedWithPCLQ(ctx, pgs, pclq.ObjectMeta)
	// if err != nil {
	// 	return err
	// }
	// podGangToExistingPods, _, orphanedPods := segregateExistingPodsByPodGang(existingPodGangNames, pclqPods)
	// podsCreated, podsDeleted, err := r.syncPodGangPods(ctx, logger, pgs, pclq, podGangToExistingPods)
	// if err != nil {
	// 	return err
	// }

	// diff := len(pclqPods) + podsCreated - podsDeleted - int(pclq.Spec.Replicas) - len(orphanedPods)
	// if diff < 0 {
	// 	logger.Info("Too few replicas for PodClique, creating new pods", "need", pclq.Spec.Replicas, "existing", len(pclqPods), "creating", diff)
	// 	return r.doCreatePods(ctx, logger, pclq, nil, -diff)
	// } else {
	// 	// delete excess pods
	// }

	// if err := r.doDeletePods(ctx, logger, pclq, orphanedPods); err != nil {
	// 	return err
	// }

	return nil
}

// func (r _resource) syncPodGangPods(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet, pclq *grovecorev1alpha1.PodClique, podGangPods map[string][]*corev1.Pod) (int, int, error) {
// 	// compute expected number of pods in the PodGang from a particualr PodClique
// 	// 1. If PodClique is a part of a PCSG, then number expected in the PodGang is pgs.spec.templateSpec.cliques[x].spec.replicas
// 	// 2. If PodClique has its own scaleConfig, then number expected in the PodGang is pclq.spec.replicas <pgs-name>-<replica-index> : INDIVIDUAL HPA
// 	// 3. If PodClique has no autoscaling config, then number expected in the PodGang is pclq.spec.replicas
// 	var podsCreated, podsDeleted int
// 	expectedPodGangReplicas := computeExpectedPodGangReplicas(pgs, pclq)
// 	for podGangName, pods := range podGangPods {
// 		diff := len(pods) - int(expectedPodGangReplicas)
// 		if diff < 0 {
// 			if err := r.doCreatePods(ctx, logger, pclq, &podGangName, -diff); err != nil {
// 				return podsCreated, podsDeleted, err
// 			}
// 			podsCreated++
// 		} else if diff > 0 {
// 			// delete the pods based on TBD alg
// 			podsDeleted++
// 		}
// 	}
// 	return podsCreated, podsDeleted, nil
// }

//func segregateExistingPodsByPodGang(existingPodGangNames []string, pods []*corev1.Pod) (map[string][]*corev1.Pod, []*corev1.Pod, []*corev1.Pod) {
//	podGangToPod := make(map[string][]*corev1.Pod)
//	unassignedPods := make([]*corev1.Pod, 20) // TODO: @renormalize decide on a better value
//	// orphanedPods holds Pods whose PodGang was deleted
//	orphanedPods := make([]*corev1.Pod, 20)
//	for _, existingPodGangName := range existingPodGangNames {
//		podGangToPod[existingPodGangName] = make([]*corev1.Pod, 0)
//	}
//	for _, pod := range pods {
//		podGangName, ok := pod.GetLabels()[grovecorev1alpha1.LabelPodGangName]
//		if !ok {
//			unassignedPods = append(unassignedPods, pod)
//		}
//		if slices.Contains(existingPodGangNames, podGangName) {
//			podGangToPod[podGangName] = append(podGangToPod[podGangName], pod)
//		} else {
//			orphanedPods = append(orphanedPods, pod)
//		}
//	}
//	return podGangToPod, unassignedPods, orphanedPods
//}

// func (r _resource) doCreatePods(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique, podGangName *string, numPods int) error {
// 	createTasks := make([]utils.Task, 0, numPods)
// 	for i := range numPods {
// 		createTask := utils.Task{
// 			Name: fmt.Sprintf("CreatePod-%s-%d", pclq.Name, i),
// 			Fn: func(ctx context.Context) error {
// 				pod := &corev1.Pod{}
// 				if err := r.buildResource(pclq, pod, podGangName); err != nil {
// 					return groveerr.WrapError(err,
// 						errCodeSyncPod,
// 						component.OperationSync,
// 						fmt.Sprintf("failed to build Pod resource for PodClique %v", client.ObjectKeyFromObject(pclq)),
// 					)
// 				}
// 				if err := r.client.Create(ctx, pod); err != nil {
// 					r.eventRecorder.Eventf(pclq, corev1.EventTypeWarning, reasonPodCreationFailed, "Error creating pod: %v", err)
// 					return groveerr.WrapError(err,
// 						errCodeSyncPod,
// 						component.OperationSync,
// 						fmt.Sprintf("failed to create Pod: %v for PodClique %v", client.ObjectKeyFromObject(pod), client.ObjectKeyFromObject(pclq)),
// 					)
// 				}
// 				logger.Info("Created pod for PodClique", "pclqName", pclq.Name, "podName", pod.Name)
// 				r.eventRecorder.Eventf(pclq, corev1.EventTypeNormal, reasonPodCreationSuccessful, "Created Pod: %s", pod.Name)
// 				return nil
// 			},
// 		}
// 		createTasks = append(createTasks, createTask)
// 	}
// 	if runResult := utils.RunConcurrentlyWithSlowStart(ctx, logger, 1, createTasks); runResult.HasErrors() {
// 		err := runResult.GetAggregatedError()
// 		logger.Error(err, "Error creating pods for PodClique", "pclqName", pclq.Name, "run summary", runResult.GetSummary())
// 		return groveerr.WrapError(runResult.GetAggregatedError(),
// 			errCodeSyncPod,
// 			component.OperationSync,
// 			fmt.Sprintf("failed to create pods for PodClique %v", client.ObjectKeyFromObject(pclq)),
// 		)
// 	}
// 	return nil
// }

// func (r _resource) doDeletePods(ctx context.Context, logger logr.Logger, pclq *grovecorev1alpha1.PodClique, orphanedPods []*corev1.Pod) error {
// 	deleteTasks := make([]utils.Task, 0, len(orphanedPods))
// 	for i, pod := range orphanedPods {
// 		deleteTask := utils.Task{
// 			Name: fmt.Sprintf("DeletePod-%s-%d", pod.Name, i),
// 			Fn: func(ctx context.Context) error {
// 				logger.Info("Initiating delete of Pod", "Pod", client.ObjectKeyFromObject(pod))
// 				if err := r.client.Delete(ctx, pod); err != nil {
// 					return groveerr.WrapError(err,
// 						errCodeDeletePod,
// 						component.OperationSync,
// 						fmt.Sprintf("failed to delete Pod %v", client.ObjectKeyFromObject(pod)),
// 					)
// 				}
// 				logger.Info("Deleted pod for PodClique", "Pod", client.ObjectKeyFromObject(pod))
// 				r.eventRecorder.Eventf(pclq, corev1.EventTypeNormal, reasonPodDeletionSuccessful, "Deleted Pod: %s", pod.Name)
// 				return nil
// 			},
// 		}
// 		deleteTasks = append(deleteTasks, deleteTask)
// 	}
// 	if runResult := utils.RunConcurrentlyWithSlowStart(ctx, logger, 1, deleteTasks); runResult.HasErrors() {
// 		err := runResult.GetAggregatedError()
// 		logger.Error(err, "Error deleting pods for PodClique", "pclqName", pclq.Name, "run summary", runResult.GetSummary())
// 		return groveerr.WrapError(runResult.GetAggregatedError(),
// 			errCodeSyncPod,
// 			component.OperationSync,
// 			fmt.Sprintf("failed to delete pods for PodClique %v", client.ObjectKeyFromObject(pclq)),
// 		)
// 	}
// 	return nil
// }

// // shouldCreatePodsWithSchedulingGate
// func (r _resource) shouldCreatePodsWithSchedulingGate(ctx context.Context, pclq *grovecorev1alpha1.PodClique) (bool, error) {
// 	pgsName := k8sutils.GetFirstOwnerName(pclq.ObjectMeta)
// 	podGangName, err := constructPodGangNameFromPCLQ(pgsName, pclq)
// 	if err != nil {
// 		return false, groveerr.WrapError(err,
// 			errCodeCreatePodGangNameFromPCLQ,
// 			component.OperationSync,
// 			fmt.Sprintf("failed to construct pod gang name from PCLQ %v", pclq.Name),
// 		)
// 	}
// 	pgExists, err := r.checkPodGangExists(ctx, pclq.Namespace, pgsName, podGangName)
// 	if err != nil {
// 		return false, err
// 	}
// 	return !pgExists, nil
// }

// // constructPodGangNameFromPCLQ creates a PodGang name which are created for every PodGangSet replica.
// func constructPodGangNameFromPCLQ(pgsName string, pclq *grovecorev1alpha1.PodClique) (string, error) {
// 	pgsReplica, err := utils.GetPodGangSetReplicaIndexFromPodCliqueFQN(pgsName, pclq.Name)
// 	if err != nil {
// 		return "", err
// 	}
// 	return grovecorev1alpha1.GeneratePodGangName(grovecorev1alpha1.ResourceNameReplica{Name: pgsName, Replica: pgsReplica}, nil), nil
// }

// func (r _resource) checkPodGangExists(ctx context.Context, namespace, pgsName, podGangName string) (bool, error) {
// 	pgObjectKey := client.ObjectKey{Name: podGangName, Namespace: namespace}
// 	pgPartialObjMeta := &metav1.PartialObjectMetadata{}
// 	pgPartialObjMeta.SetGroupVersionKind(groveschedulerv1alpha1.SchemeGroupVersion.WithKind("PodGang"))
// 	if err := r.client.Get(ctx, pgObjectKey, pgPartialObjMeta); err != nil {
// 		if errors.IsNotFound(err) {
// 			return false, nil
// 		}
// 		return false, groveerr.WrapError(err,
// 			errCodeGetPodGangMetadata,
// 			component.OperationSync,
// 			fmt.Sprintf("failed to check pod gang existence: %v", podGangName),
// 		)
// 	}
// 	return k8sutils.GetFirstOwnerName(pgPartialObjMeta.ObjectMeta) == pgsName, nil
// }

func (r _resource) buildResource(pclq *grovecorev1alpha1.PodClique, pod *corev1.Pod, podGangName *string) error {
	pod.Spec = *pclq.Spec.PodSpec.DeepCopy()
	labels, err := getLabels(pclq.ObjectMeta)
	if podGangName != nil {
		labels[grovecorev1alpha1.LabelPodGangName] = *podGangName
	} else {
		pod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: podGangSchedulingGate}}
	}
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
	// TODO: Add init container as part of the PodSpec once it is ready.

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
			grovecorev1alpha1.LabelPodCliqueName: pclqObjectMeta.Name,
		},
	)
}

func getLabels(pclqObjectMeta metav1.ObjectMeta) (map[string]string, error) {
	pgsName := k8sutils.GetFirstOwnerName(pclqObjectMeta)
	pgsReplicaIndex, err := utils.GetPodGangSetReplicaIndexFromPodCliqueFQN(pgsName, pclqObjectMeta.Name)
	if err != nil {
		return nil, err
	}
	// TODO: Additional PodGang label is also required to be added to the pod labels.
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		map[string]string{
			grovecorev1alpha1.LabelPodCliqueName:          pclqObjectMeta.Name,
			grovecorev1alpha1.LabelPodGangSetReplicaIndex: strconv.Itoa(pgsReplicaIndex),
		}), nil
}
