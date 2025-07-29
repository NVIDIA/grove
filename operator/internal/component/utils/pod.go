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

package utils

import (
	"context"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPCLQPods lists all Pods that belong to a PodClique
func GetPCLQPods(ctx context.Context, cl client.Client, pgsName string, pclq *grovecorev1alpha1.PodClique) ([]*corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cl.List(ctx,
		podList,
		client.InNamespace(pclq.Namespace),
		client.MatchingLabels(
			lo.Assign(
				k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
				map[string]string{
					grovecorev1alpha1.LabelPodClique: pclq.Name,
				},
			),
		)); err != nil {
		return nil, err
	}
	ownedPods := make([]*corev1.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if metav1.IsControlledBy(&pod, pclq) {
			ownedPods = append(ownedPods, &pod)
		}
	}
	return ownedPods, nil
}

// AddEnvVarsToContainers adds the given environment variables to the Pod containers.
func AddEnvVarsToContainers(containers []corev1.Container, envVars []corev1.EnvVar) {
	for i := range containers {
		containers[i].Env = append(containers[i].Env, envVars...)
	}
}
