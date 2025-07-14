package kubernetes

import (
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
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
