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

package status

import (
	"github.com/NVIDIA/grove/operator/internal/component"
	"github.com/NVIDIA/grove/operator/internal/utils/kubernetes"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateReplicaAvailable validates that replica resources meet availability requirements
func ValidateReplicaAvailable[T component.GroveCustomResourceType, P interface {
	metav1.Object
	*T
}](
	logger logr.Logger,
	resources []T,
	expectedCount int,
	getConditions func(T) []metav1.Condition,
	resourceType string,
	replicaIndex int,
) bool {
	return validateReplicaState[T, P](
		logger, resources, expectedCount,
		getConditions, IsAvailable,
		resourceType, replicaIndex,
	)
}

// ValidateReplicaScheduled validates that replica resources are scheduled
func ValidateReplicaScheduled[T component.GroveCustomResourceType, P interface {
	metav1.Object
	*T
}](
	logger logr.Logger,
	resources []T,
	expectedCount int,
	getConditions func(T) []metav1.Condition,
	resourceType string,
	replicaIndex int,
) bool {
	return validateReplicaState[T, P](
		logger, resources, expectedCount,
		getConditions, IsScheduled,
		resourceType, replicaIndex,
	)
}

// validateReplicaState is internal helper for replica validation
func validateReplicaState[T component.GroveCustomResourceType, P interface {
	metav1.Object
	*T
}](
	logger logr.Logger,
	resources []T,
	expectedCount int,
	getConditions func(T) []metav1.Condition,
	stateCheckFunc func([]metav1.Condition) bool,
	resourceType string,
	replicaIndex int,
) bool {
	nonTerminated := lo.Filter(resources, func(r T, _ int) bool {
		var p P = &r
		return !kubernetes.IsResourceTerminating(p)
	})

	if len(nonTerminated) < expectedCount {
		logger.Info("Replica missing expected resources",
			"resourceType", resourceType, "replicaIndex", replicaIndex,
			"expected", expectedCount, "actual", len(nonTerminated))
		return false
	}

	allInState := lo.EveryBy(nonTerminated, func(r T) bool {
		return stateCheckFunc(getConditions(r))
	})

	logger.Info("Replica state validation result",
		"resourceType", resourceType, "replicaIndex", replicaIndex,
		"allInState", allInState)
	return allInState
}
