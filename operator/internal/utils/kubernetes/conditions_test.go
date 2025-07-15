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

package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHasConditionsChanged(t *testing.T) {
	testCases := []struct {
		description        string
		existingConditions []metav1.Condition
		newCondition       metav1.Condition
		expectedResult     bool
	}{
		{
			description:        "should return true when condition was never set before",
			existingConditions: []metav1.Condition{},
			newCondition:       metav1.Condition{Type: "Cond1", Status: metav1.ConditionFalse},
			expectedResult:     true,
		},
		{
			description: "should return true when the existing condition has a different status than the new condition",
			existingConditions: []metav1.Condition{
				{Type: "Cond1", Status: metav1.ConditionFalse},
				{Type: "Cond2", Status: metav1.ConditionUnknown},
			},
			newCondition:   metav1.Condition{Type: "Cond1", Status: metav1.ConditionTrue},
			expectedResult: true,
		},
		{
			description: "should return true when existing condition has a different reason than the new condition",
			existingConditions: []metav1.Condition{
				{Type: "Cond1", Status: metav1.ConditionFalse, Reason: "Cond1 reason"},
				{Type: "Cond2", Status: metav1.ConditionUnknown},
			},
			newCondition:   metav1.Condition{Type: "Cond1", Status: metav1.ConditionTrue, Reason: "Cond1 another reason"},
			expectedResult: true,
		},
		{
			description: "should return true when existing condition has a different message than the new condition",
			existingConditions: []metav1.Condition{
				{Type: "Cond1", Status: metav1.ConditionFalse, Message: "Cond1 msg"},
				{Type: "Cond2", Status: metav1.ConditionUnknown},
			},
			newCondition:   metav1.Condition{Type: "Cond1", Status: metav1.ConditionTrue, Message: "Cond1 another msg"},
			expectedResult: true,
		},
		{
			description: "should return false when existing and new condition match",
			existingConditions: []metav1.Condition{
				{Type: "Cond1", Status: metav1.ConditionFalse, Reason: "Cond1 reason", Message: "Cond1 msg"},
				{Type: "Cond2", Status: metav1.ConditionUnknown},
			},
			newCondition:   metav1.Condition{Type: "Cond1", Status: metav1.ConditionFalse, Reason: "Cond1 reason", Message: "Cond1 msg"},
			expectedResult: false,
		},
	}

	t.Parallel()
	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expectedResult, HasConditionChanged(testCase.existingConditions, testCase.newCondition))
		})
	}
}
