package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// ReconcileStatusRecorder is an interface that defines the methods to record the start and completion of a reconcile operation.
// Reconcile progress will be recorded both as events and as <resource>.Status.LastOperation and <resource>.Status.LastErrors.
type ReconcileStatusRecorder[T component.GroveCustomResourceType] interface {
	// RecordStart records the start of a reconcile operation.
	RecordStart(ctx context.Context, obj *T, operationType v1alpha1.LastOperationType) error
	// RecordCompletion records the completion of a reconcile operation.
	// If the last reconciliation completed with errors then it will additionally record <resource>.Status.LastErrors.
	RecordCompletion(ctx context.Context, obj *T, operationType v1alpha1.LastOperationType, operationResult *ReconcileStepResult) error
}

// ReconcileStepFn is a function that performs a step in the reconcile flow.
type ReconcileStepFn[T component.GroveCustomResourceType] func(ctx context.Context, logger logr.Logger, obj *T) ReconcileStepResult

// ReconcileStepResult holds the result of a reconcile step.
type ReconcileStepResult struct {
	result            ctrl.Result
	errs              []error
	continueReconcile bool
	description       string
}

// Result returns the result and error from the reconcile step.
func (r ReconcileStepResult) Result() (ctrl.Result, error) {
	return r.result, errors.Join(r.errs...)
}

// NeedsRequeue returns true if the reconcile step needs to be requeued.
// This will happen if there is an error or if the result is marked to be requeued.
func (r ReconcileStepResult) NeedsRequeue() bool {
	return len(r.errs) > 0 || r.result.Requeue || r.result.RequeueAfter > 0
}

// HasErrors returns true if there are errors from the reconcile step.
func (r ReconcileStepResult) HasErrors() bool {
	return len(r.errs) > 0
}

// GetErrors returns the errors from the reconcile step.
func (r ReconcileStepResult) GetErrors() []error {
	return r.errs
}

// GetDescription returns the description from the reconcile step.
func (r ReconcileStepResult) GetDescription() string {
	return r.description
}

// DoNotRequeue returns a ReconcileStepResult that does not requeue the reconciliation.
func DoNotRequeue() ReconcileStepResult {
	return ReconcileStepResult{
		continueReconcile: false,
		result:            ctrl.Result{Requeue: false},
	}
}

// ContinueReconcile returns a ReconcileStepResult that continues the reconciliation to the next step.
func ContinueReconcile() ReconcileStepResult {
	return ReconcileStepResult{
		continueReconcile: true,
	}
}

// ReconcileWithErrors returns a ReconcileStepResult that re-queues the reconciliation with the given errors.
func ReconcileWithErrors(errs ...error) ReconcileStepResult {
	return ReconcileStepResult{
		result:            ctrl.Result{Requeue: true},
		errs:              errs,
		continueReconcile: false,
		description:       fmt.Sprintf("%s", errors.Join(errs...).Error()),
	}
}

// ReconcileAfter returns a ReconcileStepResult that re-queues the reconciliation after the given duration.
func ReconcileAfter(duration time.Duration, description string) ReconcileStepResult {
	return ReconcileStepResult{
		result: ctrl.Result{
			RequeueAfter: duration,
		},
		continueReconcile: false,
		description:       description,
	}
}

// ShortCircuitReconcileFlow returns true if the reconcile flow should be short-circuited and not continue.
func ShortCircuitReconcileFlow(result ReconcileStepResult) bool {
	return !result.continueReconcile
}
