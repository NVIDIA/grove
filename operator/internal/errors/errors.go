package errors

import (
	"errors"
	"fmt"
	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

var _ error = (*GroveError)(nil)

// GroveError is a custom error type that should be used throughout grove which encapsulates
// the underline error (cause), uniquely identifiable error code, contextual information captured
// as operation during which an error occurred and any custom message.
type GroveError struct {
	// Code indicates the category of error.
	Code v1alpha1.ErrorCode
	// Cause is the underline error.
	Cause error
	// Operation is the semantic operation during which this error is created/wrapped.
	Operation string
	// Message is the custom message providing additional context for the error.
	Message string
	// ObservedAt is the time at which the error was observed.
	ObservedAt time.Time
}

func (e *GroveError) Error() string {
	errMsg := fmt.Sprintf("[Operation: %s, Code: %s] message: %s", e.Operation, e.Code, e.Message)
	if e.Cause != nil {
		errMsg += fmt.Sprintf(", cause: %s", e.Cause.Error())
	}
	return errMsg
}

// New creates a new GroveError with the given error code, operation and message.
// This function should be used to create a new error. If you wish to wrap an existing error then use WrapError instead.
func New(code v1alpha1.ErrorCode, operation string, message string) error {
	return &GroveError{
		Code:       code,
		Operation:  operation,
		Message:    message,
		ObservedAt: time.Now().UTC(),
	}
}

// WrapError wraps the given error with the provided error code, operation and message.
// If the err is nil then it returns nil.
func WrapError(err error, code v1alpha1.ErrorCode, operation string, message string) error {
	if err == nil {
		return nil
	}
	return &GroveError{
		Code:       code,
		Cause:      err,
		Operation:  operation,
		Message:    message,
		ObservedAt: time.Now().UTC(),
	}
}

// MapToLastErrors maps the given errors (that are GroveError's) to LastError slice.
func MapToLastErrors(errs []error) []v1alpha1.LastError {
	lastErrs := make([]v1alpha1.LastError, 0, len(errs))
	for _, err := range errs {
		groveErr := &GroveError{}
		if errors.As(err, &groveErr) {
			lastErr := v1alpha1.LastError{
				Code:        groveErr.Code,
				Description: err.Error(),
				ObservedAt:  metav1.NewTime(groveErr.ObservedAt),
			}
			lastErrs = append(lastErrs, lastErr)
		}
	}
	return lastErrs
}
