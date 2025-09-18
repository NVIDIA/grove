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

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveevents "github.com/NVIDIA/grove/operator/internal/component/events"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// createPodCreationTask creates a utils.Task that handles pod creation with proper error handling and event recording.
// It builds the pod resource, creates it in the cluster, records expectations for tracking,
// and emits appropriate events to indicate success or failure.
func (r _resource) createPodCreationTask(logger logr.Logger, pcs *grovecorev1alpha1.PodCliqueSet, pclq *grovecorev1alpha1.PodClique, podGangName, pclqExpectationsKey string, taskIndex, podHostNameIndex int) utils.Task {
	pclqObjKey := client.ObjectKeyFromObject(pclq)
	return utils.Task{
		Name: fmt.Sprintf("CreatePod-%s-%d", pclq.Name, taskIndex),
		Fn: func(ctx context.Context) error {
			pod := &corev1.Pod{}

			// Build the Pod resource with all necessary configuration
			if err := r.buildResource(pcs, pclq, podGangName, pod, podHostNameIndex); err != nil {
				return groveerr.WrapError(err,
					errCodeBuildPodResource,
					component.OperationSync,
					fmt.Sprintf("failed to build Pod resource for PodClique %v", pclqObjKey),
				)
			}

			// Create the Pod in the Kubernetes cluster
			if err := r.client.Create(ctx, pod); err != nil {
				r.eventRecorder.Eventf(pclq, corev1.EventTypeWarning, groveevents.ReasonPodCreateFailed, "Error creating pod %v: %v", pod.Name, err)
				return groveerr.WrapError(err,
					errCodeCreatePod,
					component.OperationSync,
					fmt.Sprintf("failed to create Pod: %s for PodClique %v", pod.Name, pclqObjKey),
				)
			}

			logger.Info("Created Pod for PodClique", "podName", pod.Name, "podUID", pod.GetUID())

			// Record creation expectation for tracking purposes
			if err := r.expectationsStore.ExpectCreations(logger, pclqExpectationsKey, pod.GetUID()); err != nil {
				utilruntime.HandleErrorWithLogger(logger, err, "could not record create expectations for Pod", "pclqObjKey", pclqObjKey, "pod", pod.Name)
			}

			// Emit success event
			r.eventRecorder.Eventf(pclq, corev1.EventTypeNormal, groveevents.ReasonPodCreateSuccessful, "Created Pod: %s", pod.Name)
			return nil
		},
	}
}

// createPodDeletionTask creates a utils.Task that handles pod deletion with proper error handling and event recording.
// It attempts to delete the specified pod, handles already-deleted scenarios gracefully,
// records deletion expectations for tracking, and emits appropriate events.
func (r _resource) createPodDeletionTask(logger logr.Logger, pclq *grovecorev1alpha1.PodClique, podToDelete *corev1.Pod, pclqExpectationsKey string) utils.Task {
	podObjKey := client.ObjectKeyFromObject(podToDelete)
	pclqObjKey := client.ObjectKeyFromObject(pclq)
	return utils.Task{
		Name: fmt.Sprintf("DeletePod-%s", podToDelete.Name),
		Fn: func(ctx context.Context) error {
			// Attempt to delete the pod
			if err := r.client.Delete(ctx, podToDelete); err != nil {
				// Handle case where pod was already deleted
				if apierrors.IsNotFound(err) {
					logger.Info("pod has already been deleted", "pod", podObjKey)
					r.expectationsStore.ObserveDeletions(logger, pclqExpectationsKey, podToDelete.GetUID())
					return nil
				}

				// Handle actual deletion errors
				r.eventRecorder.Eventf(pclq, corev1.EventTypeWarning, groveevents.ReasonPodDeleteFailed, "Error deleting pod: %v", err)
				return groveerr.WrapError(err,
					errCodeDeletePod,
					component.OperationSync,
					fmt.Sprintf("failed to delete Pod: %v for PodClique %v", podObjKey, pclqObjKey),
				)
			}

			logger.Info("Deleted Pod", "podObjectKey", podObjKey)

			// Record deletion expectation for tracking purposes
			if err := r.expectationsStore.ExpectDeletions(logger, pclqExpectationsKey, podToDelete.GetUID()); err != nil {
				utilruntime.HandleErrorWithLogger(logger, err, "could not record delete expectation", "pclq", pclqObjKey, "pod", podObjKey)
			}

			// Emit success event
			r.eventRecorder.Eventf(pclq, corev1.EventTypeNormal, groveevents.ReasonPodDeleteSuccessful, "Deleted Pod: %s", podToDelete.Name)
			return nil
		},
	}
}

// createPodDeletionTasks creates multiple pod deletion tasks for batch operations.
// This is a convenience function for creating deletion tasks for multiple pods
// that need to be deleted as part of scale-down or rolling update operations.
func (r _resource) createPodDeletionTasks(logger logr.Logger, pclq *grovecorev1alpha1.PodClique, podsToDelete []*corev1.Pod, pclqExpectationsKey string) []utils.Task {
	deletionTasks := make([]utils.Task, 0, len(podsToDelete))
	for _, podToDelete := range podsToDelete {
		deletionTasks = append(deletionTasks, r.createPodDeletionTask(logger, pclq, podToDelete, pclqExpectationsKey))
	}
	return deletionTasks
}
