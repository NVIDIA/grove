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

package testutils

import (
	"time"

	grovecorev1alpha1 "github.com/NVIDIA/grove/operator/api/core/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ============================================================================
// PodCliqueScalingGroup Option Functions
// ============================================================================

// PCSGOption is a function that modifies a PodCliqueScalingGroup for testing.
type PCSGOption func(*grovecorev1alpha1.PodCliqueScalingGroup)

// WithPCSGHealthy sets the PCSG to a healthy state with MinAvailableBreached=False.
func WithPCSGHealthy() PCSGOption {
	return func(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) {
		pcsg.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypeMinAvailableBreached,
				Status: metav1.ConditionFalse,
				Reason: grovecorev1alpha1.ConditionReasonSufficientAvailablePCSGReplicas,
			},
		}
	}
}

// WithPCSGTerminating marks the PCSG for termination with a DeletionTimestamp.
func WithPCSGTerminating() PCSGOption {
	return func(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) {
		now := metav1.NewTime(time.Now())
		pcsg.DeletionTimestamp = &now
		pcsg.Finalizers = []string{"test-finalizer"}
	}
}

// WithPCSGMinAvailableBreached sets the PCSG to have MinAvailableBreached=True.
func WithPCSGMinAvailableBreached() PCSGOption {
	return func(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) {
		pcsg.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypeMinAvailableBreached,
				Status: metav1.ConditionTrue,
				Reason: grovecorev1alpha1.ConditionReasonInsufficientAvailablePCSGReplicas,
			},
		}
	}
}

// WithPCSGUnknownCondition sets the PCSG to have MinAvailableBreached=Unknown.
func WithPCSGUnknownCondition() PCSGOption {
	return func(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) {
		pcsg.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypeMinAvailableBreached,
				Status: metav1.ConditionUnknown,
				Reason: "UnknownState",
			},
		}
	}
}

// WithPCSGObservedGeneration sets the PCSG ObservedGeneration to enable status mutations.
func WithPCSGObservedGeneration(generation int64) PCSGOption {
	return func(pcsg *grovecorev1alpha1.PodCliqueScalingGroup) {
		pcsg.Status.ObservedGeneration = &generation
	}
}

// ============================================================================
// PodClique Option Functions
// ============================================================================

// PCLQOption is a function that modifies a PodClique for testing.
type PCLQOption func(*grovecorev1alpha1.PodClique)

// WithPCLQAvailable sets the PodClique to a healthy state with MinAvailableBreached=False and PodCliqueScheduled=True.
func WithPCLQAvailable() PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		pclq.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypeMinAvailableBreached,
				Status: metav1.ConditionFalse,
				Reason: "SufficientReadyReplicas",
			},
			{
				Type:   grovecorev1alpha1.ConditionTypePodCliqueScheduled,
				Status: metav1.ConditionTrue,
				Reason: "ScheduledSuccessfully",
			},
		}
		pclq.Status.ReadyReplicas = pclq.Spec.Replicas
	}
}

// WithPCLQTerminating marks the PodClique for termination with a DeletionTimestamp.
func WithPCLQTerminating() PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		now := metav1.NewTime(time.Now())
		pclq.DeletionTimestamp = &now
		pclq.Finalizers = []string{"test-finalizer"}
	}
}

// WithPCLQMinAvailableBreached sets the PodClique to have MinAvailableBreached=True but scheduled.
func WithPCLQMinAvailableBreached() PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		pclq.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypeMinAvailableBreached,
				Status: metav1.ConditionTrue,
				Reason: "InsufficientReadyReplicas",
			},
			{
				Type:   grovecorev1alpha1.ConditionTypePodCliqueScheduled,
				Status: metav1.ConditionTrue,
				Reason: "ScheduledSuccessfully",
			},
		}
	}
}

// WithPCLQScheduled sets the PodClique to be scheduled but with unknown availability.
func WithPCLQScheduled() PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		pclq.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypePodCliqueScheduled,
				Status: metav1.ConditionTrue,
				Reason: "ScheduledSuccessfully",
			},
		}
	}
}

// WithPCLQNotScheduled sets the PodClique to be not scheduled.
func WithPCLQNotScheduled() PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		pclq.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypePodCliqueScheduled,
				Status: metav1.ConditionFalse,
				Reason: "SchedulingFailed",
			},
		}
	}
}

// WithPCLQScheduledAndAvailable sets the PodClique to be both scheduled and available.
func WithPCLQScheduledAndAvailable() PCLQOption {
	return WithPCLQAvailable()
}

// WithPCLQScheduledButBreached sets the PodClique to be scheduled but with breached availability.
func WithPCLQScheduledButBreached() PCLQOption {
	return WithPCLQMinAvailableBreached()
}

// WithPECLMinAvailableInUnknown sets the PodClique to have MinAvailableBreached=Unknown.
func WithPECLMinAvailableInUnknown() PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		pclq.Status.Conditions = []metav1.Condition{
			{
				Type:   grovecorev1alpha1.ConditionTypeMinAvailableBreached,
				Status: metav1.ConditionUnknown,
				Reason: "UnknownState",
			},
		}
	}
}

// WithPCLQNoConditions removes all conditions from the PodClique status.
func WithPCLQNoConditions() PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		pclq.Status.Conditions = []metav1.Condition{}
	}
}

// WithPCLQCustomCondition adds a custom condition to the PodClique status.
func WithPCLQCustomCondition(conditionType string, status metav1.ConditionStatus, reason, message string) PCLQOption {
	return func(pclq *grovecorev1alpha1.PodClique) {
		condition := metav1.Condition{
			Type:    conditionType,
			Status:  status,
			Reason:  reason,
			Message: message,
		}
		pclq.Status.Conditions = append(pclq.Status.Conditions, condition)
	}
}

// ============================================================================
// PodGangSet Option Functions
// ============================================================================

// PGSOption is a function that modifies a PodGangSet for testing.
type PGSOption func(*grovecorev1alpha1.PodGangSet)
