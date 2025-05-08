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

package hpa

import (
	"context"
	"fmt"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errSyncHpa   v1alpha1.ErrorCode = "ERR_SYNC_HPA"
	errDeleteHpa v1alpha1.ErrorCode = "ERR_DELETE_HPA"
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

// GetExistingResourceNames returns the names of all the existing resources that the HPA Operator manages.
func (r _resource) GetExistingResourceNames(_ context.Context, _ logr.Logger, _ *v1alpha1.PodClique) ([]string, error) {
	//TODO Implement me
	return nil, nil
}

// Sync synchronizes all resources that the HPA Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pclq *v1alpha1.PodClique) error {
	// Do not create HPA if ScaleConfig is nil
	if pclq.Spec.ScaleConfig == nil {
		return nil
	}
	objectKey := getObjectKey(pclq.ObjectMeta)
	hpa := emptyHPA(objectKey)
	logger.Info("Running CreateOrUpdate HPA", "objectKey", objectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, hpa, func() error {
		return r.buildResource(pclq, hpa)
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncHpa,
			component.OperationSync,
			fmt.Sprintf("Error syncing HPA: %v for PodClique: %v", objectKey, client.ObjectKeyFromObject(pclq)),
		)
	}
	logger.Info("triggered create or update of HPA", "objectKey", objectKey, "result", opResult)
	return nil
}

func (r _resource) Delete(ctx context.Context, logger logr.Logger, pclqObjectMeta metav1.ObjectMeta) error {
	objectKey := getObjectKey(pclqObjectMeta)
	logger.Info("Triggering delete of HPA", "objectKey", objectKey)
	if err := r.client.Delete(ctx, emptyHPA(objectKey)); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("HPA not found, deletion is a no-op", "objectKey", objectKey)
			return nil
		}
		return groveerr.WrapError(err,
			errDeleteHpa,
			component.OperationDelete,
			fmt.Sprintf("Error deleting HPA: %v for PodClique: %v", objectKey, pclqObjectMeta),
		)
	}
	logger.Info("deleted HPA", "objectKey", objectKey)
	return nil
}

func (r _resource) buildResource(pclq *v1alpha1.PodClique, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	objKey := client.ObjectKeyFromObject(pclq)
	hpaSpec := autoscalingv2.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			Kind:       v1alpha1.PodCliqueKind,
			Name:       objKey.Name,
			APIVersion: "grove.io/v1alpha1",
		},
		MinReplicas: pclq.Spec.ScaleConfig.MinReplicas,
		MaxReplicas: pclq.Spec.ScaleConfig.MaxReplicas,
		Metrics:     pclq.Spec.ScaleConfig.Metrics,
	}
	hpa.Spec = hpaSpec

	if err := controllerutil.SetControllerReference(pclq, hpa, r.scheme); err != nil {
		return groveerr.WrapError(err,
			errSyncHpa,
			component.OperationSync,
			fmt.Sprintf("Error setting controller reference for HPA: %v", objKey),
		)
	}

	return nil
}

func getObjectKey(pclqObjMeta metav1.ObjectMeta) client.ObjectKey {
	return client.ObjectKey{
		Name:      component.GeneratePcHpaName(pclqObjMeta),
		Namespace: pclqObjMeta.Namespace,
	}
}

func emptyHPA(objKey client.ObjectKey) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
