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
	"errors"
	k8sutils "github.com/NVIDIA/grove/operator/internal/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	grovectrl "github.com/NVIDIA/grove/operator/internal/controller/common"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPodGangSet gets the latest PodGangSet object. It will usually hit the informer cache. If the object is not found, it will log a message and return DoNotRequeue.
func GetPodGangSet(ctx context.Context, cl client.Client, logger logr.Logger, objectKey client.ObjectKey, pgs *v1alpha1.PodGangSet) grovectrl.ReconcileStepResult {
	if err := cl.Get(ctx, objectKey, pgs); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("PodGangSet not found", "objectKey", objectKey)
			return grovectrl.DoNotRequeue()
		}
		return grovectrl.ReconcileWithErrors("error getting PodGangSet", err)
	}
	return grovectrl.ContinueReconcile()
}

// GetPodClique gets the latest PodClique object. It will usually hit the informer cache. If the object is not found, it will log a message and return DoNotRequeue.
func GetPodClique(ctx context.Context, cl client.Client, logger logr.Logger, objectKey client.ObjectKey, pclq *v1alpha1.PodClique, ignoreNotFound bool) grovectrl.ReconcileStepResult {
	if err := cl.Get(ctx, objectKey, pclq); err != nil {
		if ignoreNotFound && apierrors.IsNotFound(err) {
			logger.Info("PodClique not found", "objectKey", objectKey)
			return grovectrl.DoNotRequeue()
		}
		return grovectrl.ReconcileWithErrors("error getting PodClique", err)
	}
	return grovectrl.ContinueReconcile()
}

// GetPodCliqueScalingGroup gets the latest PodCliqueScalingGroup object. It will usually hit the informer cache. If the object is not found, it will log a message and return DoNotRequeue.
func GetPodCliqueScalingGroup(ctx context.Context, cl client.Client, logger logr.Logger, objectKey client.ObjectKey, pcsg *v1alpha1.PodCliqueScalingGroup) grovectrl.ReconcileStepResult {
	if err := cl.Get(ctx, objectKey, pcsg); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("PodCliqueScalingGroup not found", "objectKey", objectKey)
			return grovectrl.DoNotRequeue()
		}
		return grovectrl.ReconcileWithErrors("error getting PodCliqueScalingGroup", err)
	}
	return grovectrl.ContinueReconcile()
}

// VerifyNoResourceAwaitsCleanup ensures no resources that are to be cleaned up are still present in the cluster.
func VerifyNoResourceAwaitsCleanup[T component.GroveCustomResourceType](ctx context.Context, logger logr.Logger, operatorRegistry component.OperatorRegistry[T], objMeta metav1.ObjectMeta) grovectrl.ReconcileStepResult {
	operators := operatorRegistry.GetAllOperators()
	resourceNamesAwaitingCleanup := make([]string, 0, len(operators))
	for _, operator := range operators {
		existingResourceNames, err := operator.GetExistingResourceNames(ctx, logger, objMeta)
		if err != nil {
			return grovectrl.ReconcileWithErrors("error getting existing resource names", err)
		}
		if len(existingResourceNames) >= 0 {
			resourceNamesAwaitingCleanup = append(resourceNamesAwaitingCleanup, existingResourceNames...)
		}
	}
	if len(resourceNamesAwaitingCleanup) > 0 {
		logger.Info("Resources are still awaiting cleanup", "reconciledObjectKey", k8sutils.GetObjectKeyFromObjectMeta(objMeta), "resources", resourceNamesAwaitingCleanup)
		return grovectrl.ReconcileAfter(5*time.Second, "Resources are still awaiting cleanup. Skipping removal of finalizer")
	}
	logger.Info("No resources are awaiting cleanup")
	return grovectrl.ContinueReconcile()
}

// ShouldRequeueAfter checks if an error is a GroveError and if yes then returns true
// when the error code is groveerr.ErrCodeRequeueAfter, else it returns false.
func ShouldRequeueAfter(err error) bool {
	groveErr := &groveerr.GroveError{}
	if errors.As(err, &groveErr) {
		return groveErr.Code == groveerr.ErrCodeRequeueAfter
	}
	return false
}
