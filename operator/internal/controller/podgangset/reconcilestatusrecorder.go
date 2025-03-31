package podgangset

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/controller/common"
	ctrlcommon "github.com/NVIDIA/grove/operator/internal/controller/common"
	groveerr "github.com/NVIDIA/grove/operator/internal/errors"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type recorder struct {
	client        client.Client
	eventRecorder record.EventRecorder
}

// NewReconcileStatusRecorder returns a new reconcile status recorder for PodGangSet.
func NewReconcileStatusRecorder(client client.Client, eventRecorder record.EventRecorder) common.ReconcileStatusRecorder[v1alpha1.PodGangSet] {
	return &recorder{
		client:        client,
		eventRecorder: eventRecorder,
	}
}

func (r *recorder) RecordStart(ctx context.Context, pgs *v1alpha1.PodGangSet, operationType v1alpha1.LastOperationType) error {
	slog.Info("recording start", "pgs", pgs.Name, "operationType", operationType)
	eventReason := lo.Ternary[string](operationType == v1alpha1.LastOperationTypeReconcile, v1alpha1.EventReconciling, v1alpha1.EventDeleting)
	r.eventRecorder.Event(pgs, v1.EventTypeNormal, eventReason, fmt.Sprintf("Reconciling PodGangSet"))
	description := lo.Ternary(operationType == v1alpha1.LastOperationTypeReconcile, "PodGangSet reconciliation is in progress", "PodGangSet deletion is in progress")
	return r.recordLastOperationAndLastErrors(ctx, pgs, operationType, v1alpha1.LastOperationStateProcessing, description)
}

func (r *recorder) RecordCompletion(ctx context.Context, pgs *v1alpha1.PodGangSet, operationType v1alpha1.LastOperationType, operationResult *ctrlcommon.ReconcileStepResult) error {
	r.recordCompletionEvent(pgs, operationType, operationResult)
	description := getLastOperationCompletionDescription(operationType, operationResult)
	var (
		lastErrors  []v1alpha1.LastError
		lastOpState = v1alpha1.LastOperationStateSucceeded
	)
	if operationResult != nil && operationResult.HasErrors() {
		lastErrors = groveerr.MapToLastErrors(operationResult.GetErrors())
		lastOpState = v1alpha1.LastOperationStateError
	}
	return r.recordLastOperationAndLastErrors(ctx, pgs, operationType, lastOpState, description, lastErrors...)
}

func (r *recorder) recordCompletionEvent(pgs *v1alpha1.PodGangSet, operationType v1alpha1.LastOperationType, operationResult *ctrlcommon.ReconcileStepResult) {
	slog.Info("recording completion", "pgs", pgs.Name, "operationType", operationType)
	eventReason := getCompletionEventReason(operationType, operationResult)
	eventType := lo.Ternary(operationResult != nil && operationResult.HasErrors(), v1.EventTypeWarning, v1.EventTypeNormal)
	message := getCompletionEventMessage(operationType, operationResult)
	r.eventRecorder.Event(pgs, eventType, eventReason, message)
}

func getCompletionEventReason(operationType v1alpha1.LastOperationType, operationResult *ctrlcommon.ReconcileStepResult) string {
	if operationResult != nil && operationResult.HasErrors() {
		return lo.Ternary[string](operationType == v1alpha1.LastOperationTypeReconcile, v1alpha1.EventReconcileError, v1alpha1.EventDeleteError)
	}
	return lo.Ternary[string](operationType == v1alpha1.LastOperationTypeReconcile, v1alpha1.EventReconciled, v1alpha1.EventDeleted)
}

func getCompletionEventMessage(operationType v1alpha1.LastOperationType, operationResult *ctrlcommon.ReconcileStepResult) string {
	if operationResult != nil && operationResult.HasErrors() {
		return operationResult.GetDescription()
	}
	return lo.Ternary(operationType == v1alpha1.LastOperationTypeReconcile, "Reconciled PodGangSet", "Deleted PodGangSet")
}

func getLastOperationCompletionDescription(operationType v1alpha1.LastOperationType, operationResult *ctrlcommon.ReconcileStepResult) string {
	if operationResult != nil && operationResult.HasErrors() {
		return fmt.Sprintf("%s. Operation will be retried.", operationResult.GetDescription())
	}
	return lo.Ternary(operationType == v1alpha1.LastOperationTypeReconcile, "PodGangSet has been successfully reconciled", "PodGangSet has been successfully deleted")
}

func (r *recorder) recordLastOperationAndLastErrors(ctx context.Context,
	pgs *v1alpha1.PodGangSet,
	operationType v1alpha1.LastOperationType,
	operationStatus v1alpha1.LastOperationState,
	description string,
	lastErrors ...v1alpha1.LastError) error {

	originalPgs := pgs.DeepCopy()
	pgs.Status.LastOperation = &v1alpha1.LastOperation{
		Type:           operationType,
		State:          operationStatus,
		LastUpdateTime: metav1.NewTime(time.Now().UTC()),
		Description:    description,
	}
	pgs.Status.LastErrors = lastErrors
	return r.client.Status().Patch(ctx, pgs, client.MergeFrom(originalPgs))
}
