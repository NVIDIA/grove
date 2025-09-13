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

	apicommon "github.com/NVIDIA/grove/operator/api/common"
	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPCLQPods lists all Pods that belong to a PodClique
func GetPCLQPods(ctx context.Context, cl client.Client, pcsName string, pclq *grovecorev1alpha1.PodClique) ([]*corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cl.List(ctx,
		podList,
		client.InNamespace(pclq.Namespace),
		client.MatchingLabels(
			lo.Assign(
				apicommon.GetDefaultLabelsForPodCliqueSetManagedResources(pcsName),
				map[string]string{
					apicommon.LabelPodClique: pclq.Name,
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

// PodsToObjectNames converts a slice of Pods to a slice of string representations in "namespace/name" format.
func PodsToObjectNames(pods []*corev1.Pod) []string {
	return lo.Map(pods, func(pod *corev1.Pod, _ int) string {
		return cache.NamespacedNameAsObjectName(client.ObjectKeyFromObject(pod)).String()
	})
}
