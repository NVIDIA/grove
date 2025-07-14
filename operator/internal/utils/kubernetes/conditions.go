package kubernetes

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HasConditionChanged checks if a specific condition passed as newCondition has either been set newly
// or it has changed when compared to the existing condition.
func HasConditionChanged(existingConditions []metav1.Condition, newCondition metav1.Condition) bool {
	existingCond := meta.FindStatusCondition(existingConditions, newCondition.Type)
	if existingCond == nil {
		return true
	}
	return existingCond.Status != newCondition.Status ||
		existingCond.Reason != newCondition.Reason ||
		existingCond.Message != newCondition.Message
}
