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
	"errors"
	"fmt"
	"reflect"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errSyncPod   v1alpha1.ErrorCode = "ERR_SYNC_POD"
	errDeletePod v1alpha1.ErrorCode = "ERR_DELETE_POD"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// podInfo contains info about pods created by a PodClique
type podInfo struct {
	pods map[string]*corev1.Pod
}

// New creates an instance of Pod component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[v1alpha1.PodClique] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the Pod Operator manages.
func (r _resource) GetExistingResourceNames(_ context.Context, _ logr.Logger, _ *v1alpha1.PodClique) ([]string, error) {
	//TODO Implement me
	return nil, nil
}

// Sync synchronizes all resources that the Pod Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique) error {
	info, err := r.listPods(ctx, logger, pclq.Name, pclq.Namespace)
	if err != nil {
		logger.Error(err, "failed to list pods")
		return err
	}
	logger.Info("Found existing pods", "count", len(info.pods))

	objectKeys := getObjectKeys(pclq)
	tasks := make([]utils.Task, 0, len(objectKeys))
	for _, objectKey := range objectKeys {
		createOrUpdateTask := utils.Task{
			Name: fmt.Sprintf("CreateOrUpdatePod-%s", objectKey),
			Fn: func(ctx context.Context) error {
				return r.doCreateOrUpdate(ctx, logger, pclq, objectKey, info)
			},
		}
		tasks = append(tasks, createOrUpdateTask)
	}
	if errs := utils.RunConcurrently(ctx, tasks); len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pclqObjectMeta metav1.ObjectMeta) error {
	info, err := r.listPods(ctx, logger, pclqObjectMeta.Name, pclqObjectMeta.Namespace)
	if err != nil {
		logger.Error(err, "failed to list pods")
		return err
	}
	logger.Info("Found pods to delete", "count", len(info.pods))

	tasks := make([]utils.Task, 0, len(info.pods))
	for _, pod := range info.pods {
		deleteTask := utils.Task{
			Name: fmt.Sprintf("DeletePod-%s", pod.Name),
			Fn: func(ctx context.Context) error {
				return r.doDelete(ctx, logger, pod)
			},
		}
		tasks = append(tasks, deleteTask)
	}
	if errs := utils.RunConcurrently(ctx, tasks); len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (r _resource) doCreateOrUpdate(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique, podObjectKey client.ObjectKey, info *podInfo) error {
	logger.Info("Running CreateOrUpdate Pod", "podObjectKey", podObjectKey)
	pod := emptyPod(podObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, pod, func() error {
		return r.buildResource(logger, pod, pclq, info)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncPod,
			component.OperationSync,
			fmt.Sprintf("Error syncing Pod: %v for PodClique: %v", podObjectKey, client.ObjectKeyFromObject(pclq)),
		)
	}
	logger.Info("triggered create or update of PodClique", "podObjectKey", podObjectKey, "result", opResult)
	return nil
}

func (r _resource) doDelete(ctx context.Context, logger logr.Logger, pod *corev1.Pod) error {
	logger.Info("Running Delete Pod", "name", pod.Name, "namespace", pod.Namespace)
	if err := r.client.Delete(ctx, pod); err != nil {
		return groveerr.WrapError(err,
			errDeletePod,
			component.OperationDelete,
			fmt.Sprintf("Error deleting Pod: %s/%s", pod.Namespace, pod.Name),
		)
	}
	return nil
}

func (r _resource) buildResource(logger logr.Logger, pod *corev1.Pod, pclq *v1alpha1.PodClique, info *podInfo) error {
	podObjectKey, pclqObjectKey := client.ObjectKeyFromObject(pod), client.ObjectKeyFromObject(pclq)
	if actual, ok := info.pods[pod.Name]; ok {
		pod.Labels = actual.Labels
		pod.Annotations = actual.Annotations
		pod.Spec = actual.Spec
		if updatePod(pod, pclq) {
			logger.Info("Update pod", "name", pod.Name)
		}
	} else {
		logger.Info("Create pod", "name", pod.Name)
		podSpec := findPodSpec(podObjectKey, pclq)
		if podSpec == nil {
			logger.Info("Pod spec not found in PodClique", "podObjectKey", podObjectKey, "pclqObjectKey", pclqObjectKey)
			return groveerr.New(errSyncPod,
				component.OperationSync,
				fmt.Sprintf("PodSpec for Pod: %v not found in PodClique: %v", podObjectKey, pclqObjectKey),
			)
		}
		if err := controllerutil.SetControllerReference(pclq, pod, r.scheme); err != nil {
			return err
		}
		pod.Labels = pclq.Labels
		pod.Annotations = pclq.Annotations
		pod.Spec = *podSpec
	}
	return nil
}

func findPodSpec(podObjectKey client.ObjectKey, pclq *v1alpha1.PodClique) *corev1.PodSpec {
	for replicaID := range pclq.Spec.Replicas {
		if createPodName(pclq.Name, replicaID) == podObjectKey.Name {
			return &pclq.Spec.PodSpec
		}
	}
	return nil
}

func getObjectKeys(pclq *v1alpha1.PodClique) []client.ObjectKey {
	podNames := getPodNames(pclq)
	podObjKeys := make([]client.ObjectKey, 0, len(podNames))
	for _, podName := range podNames {
		podObjKeys = append(podObjKeys, client.ObjectKey{
			Name:      podName,
			Namespace: pclq.Namespace,
		})
	}
	return podObjKeys
}

func getPodNames(pclq *v1alpha1.PodClique) []string {
	podPrefix := pclq.Name
	podNames := make([]string, 0, pclq.Spec.Replicas)
	for replicaID := range pclq.Spec.Replicas {
		podNames = append(podNames, createPodName(podPrefix, replicaID))
	}
	return podNames
}

// PC name : <PGS.Name>-<PGS.ReplicaID>-<PC.Name>
// Pod name : <PC.Name>-<PC.ReplicaID>
func createPodName(prefix string, suffix int32) string {
	return fmt.Sprintf("%s-%d", prefix, suffix)
}

func emptyPod(objKey client.ObjectKey) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}

// retrieve the existing pods created by the PodClique
func (r _resource) listPods(ctx context.Context, logger logr.Logger, pclqName, namespace string) (*podInfo, error) {
	var podList corev1.PodList
	if err := r.client.List(ctx, &podList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	info := &podInfo{pods: make(map[string]*corev1.Pod)}
	for i, pod := range podList.Items {
		if len(pod.OwnerReferences) == 1 {
			ref := pod.OwnerReferences[0]
			if ref.Kind == v1alpha1.PodCliqueKind && ref.Name == pclqName {
				logger.Info("Found existing pod", "name", pod.Name)
				info.pods[pod.Name] = &podList.Items[i]
			}
		}
	}
	return info, nil
}

// update the allowed pod fields with the new values only if they have changed
func updatePod(pod *corev1.Pod, pclq *v1alpha1.PodClique) bool {
	spec := &pclq.Spec.PodSpec
	if len(pod.Spec.Containers) != len(spec.Containers) || len(pod.Spec.InitContainers) != len(spec.InitContainers) {
		return false
	}

	var update bool
	for i, container := range spec.Containers {
		if container.Image != pod.Spec.Containers[i].Image {
			pod.Spec.Containers[i].Image = container.Image
			update = true
		}
	}

	for i, container := range spec.InitContainers {
		if container.Image != pod.Spec.InitContainers[i].Image {
			pod.Spec.InitContainers[i].Image = container.Image
			update = true
		}
	}

	if spec.ActiveDeadlineSeconds == nil {
		if pod.Spec.ActiveDeadlineSeconds != nil {
			pod.Spec.ActiveDeadlineSeconds = nil
			update = true
		}
	} else {
		if pod.Spec.ActiveDeadlineSeconds == nil || *pod.Spec.ActiveDeadlineSeconds != *spec.ActiveDeadlineSeconds {
			pod.Spec.ActiveDeadlineSeconds = spec.ActiveDeadlineSeconds
			update = true
		}
	}

	if len(spec.Tolerations) > len(pod.Spec.Tolerations) {
		for i := len(pod.Spec.Tolerations); i < len(spec.Tolerations); i++ {
			pod.Spec.Tolerations = append(pod.Spec.Tolerations, spec.Tolerations[i])
			update = true
		}
	}

	if !reflect.DeepEqual(pod.Annotations, pclq.Annotations) {
		pod.Annotations = pclq.Annotations
		update = true
	}

	if !reflect.DeepEqual(pod.Labels, pclq.Labels) {
		pod.Labels = pclq.Labels
		update = true
	}

	return update
}
