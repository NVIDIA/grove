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

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"
	"github.com/NVIDIA/grove/operator/internal/utils"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	errListHPA   grovecorev1alpha1.ErrorCode = "ERR_LIST_HPA"
	errSyncHPA   grovecorev1alpha1.ErrorCode = "ERR_SYNC_HPA"
	errDeleteHPA grovecorev1alpha1.ErrorCode = "ERR_DELETE_HPA"
)

type _resource struct {
	client client.Client
	scheme *runtime.Scheme
}

// New creates an instance of HPA component operator.
func New(client client.Client, scheme *runtime.Scheme) component.Operator[grovecorev1alpha1.PodGangSet] {
	return &_resource{
		client: client,
		scheme: scheme,
	}
}

// GetExistingResourceNames returns the names of all the existing resources that the HPA Operator manages.
func (r _resource) GetExistingResourceNames(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) ([]string, error) {
	logger.Info("Looking for existing HPA resources")
	objMetaList := &metav1.PartialObjectMetadataList{}
	objMetaList.SetGroupVersionKind(autoscalingv2.SchemeGroupVersion.WithKind("HorizontalPodAutoscaler"))
	if err := r.client.List(ctx,
		objMetaList,
		client.InNamespace(pgs.Namespace),
		client.MatchingLabels(getPodCliqueHPASelectorLabels(pgs.ObjectMeta)),
	); err != nil {
		return nil, groveerr.WrapError(err,
			errListHPA,
			component.OperationGetExistingResourceNames,
			fmt.Sprintf("Error listing HorizontalPodAutoscaler for PodCliques belonging to PodGangSet: %s", client.ObjectKeyFromObject(pgs)),
		)
	}
	return k8sutils.FilterMapOwnedResourceNames(pgs.ObjectMeta, objMetaList.Items), nil
}

// Sync synchronizes all resources that the HPA Operator manages.
func (r _resource) Sync(ctx context.Context, logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) error {
	tasks := make([]utils.Task, 0, len(pgs.Spec.TemplateSpec.Cliques)*int(pgs.Spec.Replicas)+len(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs))
	tasks = append(tasks, r.createPodCliqueHPATasks(logger, pgs)...)
	tasks = append(tasks, r.createPodCliqueScalingGroupHPATasks(logger, pgs)...)

	if runResult := utils.RunConcurrentlyWithSlowStart(ctx, logger, 1, tasks); runResult.HasErrors() {
		return groveerr.WrapError(runResult.GetAggregatedError(),
			errSyncHPA,
			component.OperationSync,
			fmt.Sprintf("Error CreateOrUpdate HorizontalPodAutoscalers for PodGangSet: %v, run summary: %s", client.ObjectKeyFromObject(pgs), runResult.GetSummary()),
		)
	}
	logger.Info("Successfully synced HorizontalPodAutoscalers for PodGangSet")
	return nil
}

// Delete deletes all resources that the HPA Operator manages.
func (r _resource) Delete(ctx context.Context, logger logr.Logger, pgsObjMeta metav1.ObjectMeta) error {
	logger.Info("Triggering delete of HPA(s)")
	if err := r.client.DeleteAllOf(ctx,
		&autoscalingv2.HorizontalPodAutoscaler{},
		client.InNamespace(pgsObjMeta.Namespace),
		client.MatchingLabels(getPodCliqueHPASelectorLabels(pgsObjMeta)),
	); err != nil {
		return groveerr.WrapError(err,
			errDeleteHPA,
			component.OperationDelete,
			fmt.Sprintf("Error deleting HPA for PodGangSet %v", pgsObjMeta.Name))
	}
	logger.Info("Deleted HPA(s)")
	return nil
}

func (r _resource) createPodCliqueHPATasks(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) []utils.Task {
	createOrUpdateTasks := make([]utils.Task, 0, len(pgs.Spec.TemplateSpec.Cliques)*int(pgs.Spec.Replicas))
	for replicaIndex := range pgs.Spec.Replicas {
		for _, pclqTemplateSpec := range pgs.Spec.TemplateSpec.Cliques {
			pclqFQN := grovecorev1alpha1.GeneratePodCliqueName(pgs.Name, int(replicaIndex), pclqTemplateSpec.Name)
			if pclqTemplateSpec.Spec.ScaleConfig == nil {
				logger.V(4).Info("Skipping HPA creation for PodClique since no ScaleConfig is defined", "podCliqueName", pclqFQN)
				continue
			}
			hpaObjectKey := client.ObjectKey{
				Name:      pclqFQN,
				Namespace: pgs.Namespace,
			}
			task := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdateHPA-%s", hpaObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdateHPA(ctx, logger, pgs.Name, pclqFQN, *pclqTemplateSpec.Spec.MinReplicas, *pclqTemplateSpec.Spec.ScaleConfig, hpaObjectKey)
				},
			}
			logger.V(4).Info("Adding task to create or update HPA for PodClique", "taskName", task.Name, "hpaObjectKey", hpaObjectKey)
			createOrUpdateTasks = append(createOrUpdateTasks, task)
		}
	}
	return createOrUpdateTasks
}

func (r _resource) createPodCliqueScalingGroupHPATasks(logger logr.Logger, pgs *grovecorev1alpha1.PodGangSet) []utils.Task {
	createOrUpdateTasks := make([]utils.Task, 0, int(pgs.Spec.Replicas)*len(pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs))
	for replicaIndex := range pgs.Spec.Replicas {
		for _, pclqScalingGrpConfig := range pgs.Spec.TemplateSpec.PodCliqueScalingGroupConfigs {
			pclqScalingGrpFQN := grovecorev1alpha1.GeneratePodCliqueScalingGroupName(pgs.Name, replicaIndex, pclqScalingGrpConfig.Name)
			if pclqScalingGrpConfig.ScaleConfig == nil {
				logger.V(4).Info("Skipping HPA creation for PodCliqueScalingGroup since no ScaleConfig is defined", "podCliqueScalingGroupName", pclqScalingGrpFQN)
				continue
			}
			hpaObjectKey := client.ObjectKey{
				Name:      pclqScalingGrpFQN,
				Namespace: pgs.Namespace,
			}
			task := utils.Task{
				Name: fmt.Sprintf("CreateOrUpdateHPA-%s", hpaObjectKey),
				Fn: func(ctx context.Context) error {
					return r.doCreateOrUpdateHPA(ctx, logger, pgs.Name, pclqScalingGrpFQN, 1, *pclqScalingGrpConfig.ScaleConfig, hpaObjectKey)
				},
			}
			logger.V(4).Info("Adding task to create or update HPA for PodCliqueScalingGroup", "taskName", task.Name, "hpaObjectKey", hpaObjectKey)
			createOrUpdateTasks = append(createOrUpdateTasks, task)
		}
	}
	return createOrUpdateTasks
}

func (r _resource) doCreateOrUpdateHPA(
	ctx context.Context,
	logger logr.Logger,
	pgsName, targetPCLQName string,
	minReplicas int32,
	scaleConfig grovecorev1alpha1.AutoScalingConfig,
	hpaObjectKey client.ObjectKey) error {
	logger.Info("Running CreateOrUpdate HPA for PodClique", "hpaObjectKey", hpaObjectKey)
	hpa := emptyHPA(hpaObjectKey)
	opResult, err := controllerutil.CreateOrPatch(ctx, r.client, hpa, func() error {
		buildResource(pgsName, hpa, grovecorev1alpha1.PodCliqueScalingGroupKind, targetPCLQName, minReplicas, scaleConfig)
		return nil
	})
	if err != nil {
		return groveerr.WrapError(err,
			errSyncHPA,
			component.OperationSync,
			fmt.Sprintf("Error creating or updating HPA for PodClique: %s", hpaObjectKey),
		)
	}
	logger.Info("triggered create or update of HPA", "hpaObjectKey", hpaObjectKey, "result", opResult)
	return nil
}

func buildResource(
	pgsName string,
	hpa *autoscalingv2.HorizontalPodAutoscaler,
	targetResourceKind string,
	targetResourceName string,
	minReplicas int32,
	scaleConfig grovecorev1alpha1.AutoScalingConfig) {
	hpa.Spec.MinReplicas = ptr.To(minReplicas)
	hpa.Spec.MaxReplicas = scaleConfig.MaxReplicas
	hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{
		Kind:       targetResourceKind,
		Name:       targetResourceName,
		APIVersion: grovecorev1alpha1.SchemeGroupVersion.Version,
	}
	hpa.Spec.Metrics = scaleConfig.Metrics
	hpa.Labels = getLabels(pgsName, hpa.Name)
}

func getLabels(pgsName, hpaName string) map[string]string {
	hpaComponentLabels := map[string]string{
		grovecorev1alpha1.LabelAppNameKey:   hpaName,
		grovecorev1alpha1.LabelComponentKey: component.NameHorizontalPodAutoscaler,
	}
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsName),
		hpaComponentLabels,
	)
}

func getPodCliqueHPASelectorLabels(pgsObjectMeta metav1.ObjectMeta) map[string]string {
	return lo.Assign(
		k8sutils.GetDefaultLabelsForPodGangSetManagedResources(pgsObjectMeta.Name),
		map[string]string{
			grovecorev1alpha1.LabelComponentKey: component.NameHorizontalPodAutoscaler,
		},
	)
}

func emptyHPA(objKey client.ObjectKey) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objKey.Name,
			Namespace: objKey.Namespace,
		},
	}
}
