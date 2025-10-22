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
	"errors"
	"testing"

	groveerr "github.com/ai-dynamo/grove/operator/internal/errors"

	"github.com/stretchr/testify/assert"
)

func TestShouldRequeueAfter(t *testing.T) {
	testCases := []struct {
		description string
		err         error
		expected    bool
	}{
		{
			description: "should return true for GroveError with RequeueAfter code",
			err: &groveerr.GroveError{
				Code:    groveerr.ErrCodeRequeueAfter,
				Message: "test requeue after error",
			},
			expected: true,
		},
		{
			description: "should return false for GroveError with ContinueReconcileAndRequeue code",
			err: &groveerr.GroveError{
				Code:    groveerr.ErrCodeContinueReconcileAndRequeue,
				Message: "test continue and requeue error",
			},
			expected: false,
		},
		{
			description: "should return false for GroveError with other code",
			err: &groveerr.GroveError{
				Code:    "ERR_OTHER_CODE",
				Message: "test other error",
			},
			expected: false,
		},
		{
			description: "should return false for non-GroveError",
			err:         errors.New("standard error"),
			expected:    false,
		},
		{
			description: "should return false for nil error",
			err:         nil,
			expected:    false,
		},
		{
			description: "should return false for wrapped standard error",
			err:         errors.New("wrapped: standard error"),
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := ShouldRequeueAfter(tc.err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestShouldContinueReconcileAndRequeue(t *testing.T) {
	testCases := []struct {
		description string
		err         error
		expected    bool
	}{
		{
			description: "should return true for GroveError with ContinueReconcileAndRequeue code",
			err: &groveerr.GroveError{
				Code:    groveerr.ErrCodeContinueReconcileAndRequeue,
				Message: "test continue and requeue error",
			},
			expected: true,
		},
		{
			description: "should return false for GroveError with RequeueAfter code",
			err: &groveerr.GroveError{
				Code:    groveerr.ErrCodeRequeueAfter,
				Message: "test requeue after error",
			},
			expected: false,
		},
		{
			description: "should return false for GroveError with other code",
			err: &groveerr.GroveError{
				Code:    "ERR_OTHER_CODE",
				Message: "test other error",
			},
			expected: false,
		},
		{
			description: "should return false for non-GroveError",
			err:         errors.New("standard error"),
			expected:    false,
		},
		{
			description: "should return false for nil error",
			err:         nil,
			expected:    false,
		},
		{
			description: "should return false for wrapped standard error",
			err:         errors.New("wrapped: standard error"),
			expected:    false,
		},
		{
			description: "should return false for GroveError with unknown code",
			err: &groveerr.GroveError{
				Code:    "unknown-code",
				Message: "test unknown code error",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := ShouldContinueReconcileAndRequeue(tc.err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
