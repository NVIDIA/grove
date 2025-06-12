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

package common

import (
	"context"
	"fmt"
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconciledObject is an interface that defines the methods to track the status of a resource during reconciliation.
// It is expected that any resource that wishes to use the ReconcileStatusRecorder will need to have status fields for
// LastOperation and LastErrors and it should also be a client.Object.
type ReconciledObject interface {
	client.Object
	// SetLastOperation sets the <resource>.Status.LastOperation on the target resource.
	SetLastOperation(operation *grovecorev1alpha1.LastOperation)
	// SetLastErrors sets the <resource>.Status.LastErrors on the target resource.
	SetLastErrors(lastErrors ...grovecorev1alpha1.LastError)
}

// ReconcileStatusRecorder is an interface that defines the methods to record the start and completion of a reconcile operation.
// Reconcile progress will be recorded both as events and as <resource>.Status.LastOperation and <resource>.Status.LastErrors.
type ReconcileStatusRecorder interface {
	// RecordStart records the start of a reconcile operation.
	RecordStart(ctx context.Context, obj ReconciledObject, operationType grovecorev1alpha1.LastOperationType) error
	// RecordCompletion records the completion of a reconcile operation.
	// If the last reconciliation completed with errors then it will additionally record <resource>.Status.LastErrors.
	RecordCompletion(ctx context.Context, obj ReconciledObject, operationType grovecorev1alpha1.LastOperationType, operationResult *ReconcileStepResult) error
}

type recorder struct {
	client        client.Client
	eventRecorder record.EventRecorder
}

func (r *recorder) RecordStart(ctx context.Context, obj ReconciledObject, operationType grovecorev1alpha1.LastOperationType) error {
	eventReason := lo.Ternary[string](operationType == grovecorev1alpha1.LastOperationTypeReconcile, grovecorev1alpha1.EventReconciling, grovecorev1alpha1.EventDeleting)
	resourceKind := obj.GetObjectKind().GroupVersionKind().Kind
	r.eventRecorder.Event(obj, v1.EventTypeNormal, eventReason, fmt.Sprintf("Reconciling %s", resourceKind))
	description := lo.Ternary(operationType == grovecorev1alpha1.LastOperationTypeReconcile, fmt.Sprintf("%s reconciliation is in progress", resourceKind), fmt.Sprintf("%s deletion is in progress", resourceKind))
	return r.recordLastOperationAndLastErrors(ctx, obj, operationType, grovecorev1alpha1.LastOperationStateProcessing, description)
}

func (r *recorder) RecordCompletion(ctx context.Context, obj ReconciledObject, operationType grovecorev1alpha1.LastOperationType, errResult *ReconcileStepResult) error {
	r.recordCompletionEvent(obj, operationType, errResult)
	resourceKind := obj.GetObjectKind().GroupVersionKind().Kind
	description := getLastOperationCompletionDescription(operationType, resourceKind, errResult)
	var (
		lastErrors  []grovecorev1alpha1.LastError
		lastOpState = grovecorev1alpha1.LastOperationStateSucceeded
	)
	if errResult != nil && errResult.HasErrors() {
		lastErrors = groveerr.MapToLastErrors(errResult.GetErrors())
		lastOpState = grovecorev1alpha1.LastOperationStateError
	}
	return r.recordLastOperationAndLastErrors(ctx, obj, operationType, lastOpState, description, lastErrors...)
}

// NewReconcileStatusRecorder returns a new reconcile status recorder.
func NewReconcileStatusRecorder(client client.Client, eventRecorder record.EventRecorder) ReconcileStatusRecorder {
	return &recorder{
		client:        client,
		eventRecorder: eventRecorder,
	}
}

func (r *recorder) recordCompletionEvent(obj ReconciledObject, operationType grovecorev1alpha1.LastOperationType, operationResult *ReconcileStepResult) {
	eventReason := getCompletionEventReason(operationType, operationResult)
	eventType := lo.Ternary(operationResult != nil && operationResult.HasErrors(), v1.EventTypeWarning, v1.EventTypeNormal)
	resourceKind := obj.GetObjectKind().GroupVersionKind().Kind
	message := getCompletionEventMessage(operationType, operationResult, resourceKind)
	r.eventRecorder.Event(obj, eventType, eventReason, message)
}

func getCompletionEventReason(operationType grovecorev1alpha1.LastOperationType, operationResult *ReconcileStepResult) string {
	if operationResult != nil && operationResult.HasErrors() {
		return lo.Ternary[string](operationType == grovecorev1alpha1.LastOperationTypeReconcile, grovecorev1alpha1.EventReconcileError, grovecorev1alpha1.EventDeleteError)
	}
	return lo.Ternary[string](operationType == grovecorev1alpha1.LastOperationTypeReconcile, grovecorev1alpha1.EventReconciled, grovecorev1alpha1.EventDeleted)
}

func getCompletionEventMessage(operationType grovecorev1alpha1.LastOperationType, operationResult *ReconcileStepResult, resourceKind string) string {
	if operationResult != nil && operationResult.HasErrors() {
		return operationResult.GetDescription()
	}
	return lo.Ternary(operationType == grovecorev1alpha1.LastOperationTypeReconcile, fmt.Sprintf("Reconciled %s", resourceKind), fmt.Sprintf("Deleted %s", resourceKind))
}

func getLastOperationCompletionDescription(operationType grovecorev1alpha1.LastOperationType, resourceKind string, operationResult *ReconcileStepResult) string {
	if operationResult != nil && operationResult.HasErrors() {
		return fmt.Sprintf("%s. Operation will be retried.", operationResult.GetDescription())
	}
	return lo.Ternary(operationType == grovecorev1alpha1.LastOperationTypeReconcile, fmt.Sprintf("%s has been successfully reconciled", resourceKind), fmt.Sprintf("%s has been successfully deleted", resourceKind))
}

func (r *recorder) recordLastOperationAndLastErrors(ctx context.Context,
	obj ReconciledObject,
	operationType grovecorev1alpha1.LastOperationType,
	operationStatus grovecorev1alpha1.LastOperationState,
	description string,
	lastErrors ...grovecorev1alpha1.LastError) error {
	originalObj := obj.DeepCopyObject().(ReconciledObject)
	obj.SetLastOperation(&grovecorev1alpha1.LastOperation{
		Type:           operationType,
		State:          operationStatus,
		LastUpdateTime: metav1.NewTime(time.Now().UTC()),
		Description:    description,
	})
	obj.SetLastErrors(lastErrors...)
	return r.client.Status().Patch(ctx, obj, client.MergeFrom(originalObj))
}
