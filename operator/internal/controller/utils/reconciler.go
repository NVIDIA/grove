package utils

import (
	"context"
	"github.com/NVIDIA/grove/operator/internal/component"
	"time"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	grovectrl "github.com/NVIDIA/grove/operator/internal/controller/common"

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
func GetPodClique(ctx context.Context, cl client.Client, logger logr.Logger, objectKey client.ObjectKey, pclq *v1alpha1.PodClique) grovectrl.ReconcileStepResult {
	if err := cl.Get(ctx, objectKey, pclq); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("PodClique not found", "objectKey", objectKey)
			return grovectrl.DoNotRequeue()
		}
		return grovectrl.ReconcileWithErrors("error getting PodClique", err)
	}
	return grovectrl.ContinueReconcile()
}

func VerifyNoResourceAwaitsCleanup[T component.GroveCustomResourceType](ctx context.Context, logger logr.Logger, operatorRegistry component.OperatorRegistry[T], obj *T) grovectrl.ReconcileStepResult {
	operators := operatorRegistry.GetAllOperators()
	resourceNamesAwaitingCleanup := make([]string, 0, len(operators))
	for _, operator := range operators {
		existingResourceNames, err := operator.GetExistingResourceNames(ctx, logger, obj)
		if err != nil {
			return grovectrl.ReconcileWithErrors("error getting existing resource names", err)
		}
		if len(existingResourceNames) >= 0 {
			resourceNamesAwaitingCleanup = append(resourceNamesAwaitingCleanup, existingResourceNames...)
		}
	}
	if len(resourceNamesAwaitingCleanup) > 0 {
		logger.Info("Resources are still awaiting cleanup", "resources", resourceNamesAwaitingCleanup)
		return grovectrl.ReconcileAfter(5*time.Second, "Resources are still awaiting cleanup. Skipping removal of finalizer")
	}
	logger.Info("No resources are awaiting cleanup")
	return grovectrl.ContinueReconcile()
}
