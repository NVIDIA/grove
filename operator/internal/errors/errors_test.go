package errors

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWrapError(t *testing.T) {
	cause := fmt.Errorf("test error")
	const (
		code      = "ERR_TEST"
		operation = "test-operation"
		message   = "test message"
	)
	err := WrapError(cause, code, operation, message)
	groveErr := &GroveError{}
	assert.True(t, errors.As(err, &groveErr))
	assert.Equal(t, cause, groveErr.Cause)
	assert.Equal(t, code, string(groveErr.Code))
	assert.Equal(t, operation, groveErr.Operation)
	assert.Equal(t, message, groveErr.Message)
}

func TestMapToLastErrors(t *testing.T) {
	err1 := &GroveError{
		Code:       v1alpha1.ErrorCode("ERR_TEST1"),
		Cause:      fmt.Errorf("test-error1"),
		Operation:  "test-op",
		Message:    "test-message1",
		ObservedAt: time.Now().UTC(),
	}
	err2 := &GroveError{
		Code:       v1alpha1.ErrorCode("ERR_TEST2"),
		Cause:      fmt.Errorf("test-error2"),
		Operation:  "test-op",
		Message:    "test-message2",
		ObservedAt: time.Now().UTC(),
	}
	testCases := []struct {
		name             string
		errs             []error
		expectedLastErrs []v1alpha1.LastError
	}{
		{
			name:             "No errors",
			errs:             []error{},
			expectedLastErrs: []v1alpha1.LastError{},
		},
		{
			name: "A slice of Grove errors",
			errs: []error{err1, err2},
			expectedLastErrs: []v1alpha1.LastError{
				{Code: err1.Code, Description: err1.Error(), ObservedAt: metav1.NewTime(err1.ObservedAt)},
				{Code: err2.Code, Description: err2.Error(), ObservedAt: metav1.NewTime(err2.ObservedAt)},
			},
		},
		{
			name: "A slice of Grove errors and other errors",
			errs: []error{err1, err2, fmt.Errorf("test-error3")},
			expectedLastErrs: []v1alpha1.LastError{
				{Code: err1.Code, Description: err1.Error(), ObservedAt: metav1.NewTime(err1.ObservedAt)},
				{Code: err2.Code, Description: err2.Error(), ObservedAt: metav1.NewTime(err2.ObservedAt)},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lastErrs := MapToLastErrors(tc.errs)
			assert.ElementsMatch(t, tc.expectedLastErrs, lastErrs)
		})
	}
}
