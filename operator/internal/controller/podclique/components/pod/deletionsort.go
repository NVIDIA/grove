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
	corev1 "k8s.io/api/core/v1"
)

// DeletionSorter enables sorting of a slice of Pods according to preference for deletion
type DeletionSorter []*corev1.Pod

// Len returns the length of the DeletionSorter
func (s DeletionSorter) Len() int {
	return len(s)
}

// Swap swaps two elements in a DeletionSorter
func (s DeletionSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// podPhaseToOrdinal maps pod phases to deletion priority order (lower values are deleted first)
var podPhaseToOrdinal = map[corev1.PodPhase]int{corev1.PodPending: 0, corev1.PodUnknown: 1, corev1.PodRunning: 2}

// Less compares two pods and returns true if the first one should be preferred for deletion.
// Code partially adapted from https://github.com/kubernetes/kubernetes/blob/5a450884b127f7b8e477d48cf3967a2a5eca9126/pkg/controller/controller_utils.go#L702
// Only 4 conditions have been taken as is and used here.
func (s DeletionSorter) Less(i, j int) bool {
	// 1. Unassigned < assigned
	// If only one of the pods is unassigned, the unassigned one is smaller
	if s[i].Spec.NodeName != s[j].Spec.NodeName && (len(s[i].Spec.NodeName) == 0 || len(s[j].Spec.NodeName) == 0) {
		return len(s[i].Spec.NodeName) == 0
	}

	// 2. PodPending < PodUnknown < PodRunning
	if s[i].Status.Phase != s[j].Status.Phase {
		return podPhaseToOrdinal[s[i].Status.Phase] < podPhaseToOrdinal[s[j].Status.Phase]
	}

	// 3. Not ready < ready
	// If only one of the pods is not ready, the not ready one is smaller
	if isPodReady(s[i]) != isPodReady(s[j]) {
		return !isPodReady(s[i])
	}

	// 4. Empty creation time pods < newer pods < older pods
	if s[i].CreationTimestamp.IsZero() || s[j].CreationTimestamp.IsZero() {
		return s[i].CreationTimestamp.IsZero()
	}
	return s[i].CreationTimestamp.After(s[j].CreationTimestamp.Time)
}

// isPodReady checks if a pod is ready by looking for the PodReady condition with status True
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
