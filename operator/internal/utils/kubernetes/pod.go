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

package kubernetes

import (
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// constants for extension PodConditionTypes.
const (
	// PodHasAtleastOneContainerWithNonZeroExitCode is a custom corev1.PodConditionType that represents a Pod which is NotReady and has at least one of its containers that
	// have exited with an exit code != 0. This condition type will be used towards setting of MinAvailableBreached condition
	// on the owner resource of a Pod.
	PodHasAtleastOneContainerWithNonZeroExitCode corev1.PodConditionType = "PodHasAtleastOneContainerWithNonZeroExitCode"
	// ScheduleGatedPod is a custom corev1.PodConditionType that represents a Pod which has one or more scheduling gates set.
	ScheduleGatedPod corev1.PodConditionType = "ScheduleGatedPod"
	// TerminatingPod is a custom corev1.PodConditionType that represents that this Pod has deletionTimestamp set on it.
	TerminatingPod corev1.PodConditionType = "TerminatingPod"
	// PodStartedButNotReady is a custom corev1.PodConditionType that represents a Pod that has at least container whose
	// status has started=true and ready=false.
	// NOTE: We are currently NOT supporting any thresholds/initialDelays as defined in container probes. Support for that might come later.
	PodStartedButNotReady corev1.PodConditionType = "PodStartedButNotReady"
)

// CategorizePodsByConditionType groups pods by their pod condition. Three condition types a.k.a. categories are of interest:
// 1. Ready pods, 2. ScheduleGated pods and 3. Pods that are NotReady and have at least one container with non-zero exit code.
func CategorizePodsByConditionType(logger logr.Logger, pods []*corev1.Pod) map[corev1.PodConditionType][]*corev1.Pod {
	podCategories := make(map[corev1.PodConditionType][]*corev1.Pod)
	for _, pod := range pods {
		// Check if the pod has been scheduled or is schedule gated.
		if IsPodScheduled(pod) {
			podCategories[corev1.PodScheduled] = append(podCategories[corev1.PodScheduled], pod)
		} else if IsPodScheduleGated(pod) {
			podCategories[ScheduleGatedPod] = append(podCategories[ScheduleGatedPod], pod)
		}
		// Check if the pod has a deletion timestamp set, which indicates that the pod is terminating.
		if IsResourceTerminating(pod.ObjectMeta) {
			podCategories[TerminatingPod] = append(podCategories[TerminatingPod], pod)
		}
		// check if the pod is ready, has at least one container with a non-zero exit code or has started but not ready containers.
		if IsPodReady(pod) {
			podCategories[corev1.PodReady] = append(podCategories[corev1.PodReady], pod)
		} else if HasAnyContainerExitedErroneously(logger, pod) {
			podCategories[PodHasAtleastOneContainerWithNonZeroExitCode] = append(podCategories[PodHasAtleastOneContainerWithNonZeroExitCode], pod)
		} else if HasAnyStartedButNotReadyContainer(pod) {
			podCategories[PodStartedButNotReady] = append(podCategories[PodStartedButNotReady], pod)
		}
	}
	return podCategories
}

// IsPodReady checks the PodReady condition. If the condition is not set
// it returns false else it returns the condition status value.
func IsPodReady(pod *corev1.Pod) bool {
	podReadyCond, ok := lo.Find(pod.Status.Conditions, func(cond corev1.PodCondition) bool {
		return cond.Type == corev1.PodReady
	})
	if !ok {
		return false
	}
	return podReadyCond.Status == corev1.ConditionTrue
}

// IsPodScheduleGated checks if there are scheduling gates added to the Pod.
func IsPodScheduleGated(pod *corev1.Pod) bool {
	scheduledCond, ok := lo.Find(pod.Status.Conditions, func(cond corev1.PodCondition) bool {
		return cond.Type == corev1.PodScheduled
	})
	if !ok {
		return false
	}
	return scheduledCond.Status == corev1.ConditionFalse && scheduledCond.Reason == corev1.PodReasonSchedulingGated
}

// IsPodScheduled checks if the Pod has been scheduled by the scheduler.
func IsPodScheduled(pod *corev1.Pod) bool {
	scheduledCond, ok := lo.Find(pod.Status.Conditions, func(cond corev1.PodCondition) bool {
		return cond.Type == corev1.PodScheduled
	})
	if !ok {
		return false
	}
	return scheduledCond.Status == corev1.ConditionTrue
}

// HasAnyContainerExitedErroneously checks if any container has been terminated with a non-zero exit code in a Pod.
func HasAnyContainerExitedErroneously(logger logr.Logger, pod *corev1.Pod) bool {
	podObjKey := client.ObjectKeyFromObject(pod)
	// check init container statuses
	erroneousInitContainerStatus := GetContainerStatusIfTerminatedErroneously(pod.Status.InitContainerStatuses)
	if erroneousInitContainerStatus != nil {
		logTerminatedErroneouslyPodContainerStatus(logger, podObjKey, erroneousInitContainerStatus)
		return true
	}
	// check non-init container statuses
	erroneousContainerStatus := GetContainerStatusIfTerminatedErroneously(pod.Status.ContainerStatuses)
	if erroneousContainerStatus != nil {
		logTerminatedErroneouslyPodContainerStatus(logger, podObjKey, erroneousContainerStatus)
		return true
	}
	return false
}

// HasAnyStartedButNotReadyContainer checks if there is at least one container which has started but not ready.
func HasAnyStartedButNotReadyContainer(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Started != nil && *containerStatus.Started && !containerStatus.Ready {
			return true
		}
	}
	return false
}

// GetContainerStatusIfTerminatedErroneously gets the first occurrence of corev1.ContainerStatus (across init, sidecar and main containers)
// that has a non-zero LastTerminationState.Terminated.ExitCode. The reason to choose `containerStatus.LastTerminationState` instead of `containerStatus.State` is that
// the `containerStatus.State` oscillates between waiting and terminating in case of containers exiting with non-zero exit code, while the `containerStatus.LastTerminationState`
// captures the last termination state and only changes when the container starts properly after multiple attempts, thus
// it is a more stable target state to observe.
func GetContainerStatusIfTerminatedErroneously(containerStatuses []corev1.ContainerStatus) *corev1.ContainerStatus {
	containerStatus, ok := lo.Find(containerStatuses, func(containerStatus corev1.ContainerStatus) bool {
		return containerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated.ExitCode != 0
	})
	if !ok {
		return nil
	}
	return &containerStatus
}

func logTerminatedErroneouslyPodContainerStatus(logger logr.Logger, podObjKey client.ObjectKey, containerStatus *corev1.ContainerStatus) {
	if containerStatus != nil {
		logger.Info("container exited with a non-zero exit code",
			"pod", podObjKey,
			"container", containerStatus.Name,
			"exitCode", containerStatus.State.Terminated.ExitCode,
			"exitReason", containerStatus.State.Terminated.Reason)
	}
}

// IsPodActive determines if a pod is active (running, pending, or will be restarted)
// Returns true if the pod is active
// Returns false if the pod is terminating or has failed permanently
func IsPodActive(pod *corev1.Pod) bool {
	if IsResourceTerminating(pod.ObjectMeta) {
		return false
	}

	if pod.Status.Phase == corev1.PodFailed && pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
		return false
	}

	if pod.Status.Phase == corev1.PodSucceeded {
		return false
	}

	return true
}
