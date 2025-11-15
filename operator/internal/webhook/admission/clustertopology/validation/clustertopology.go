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

package validation

import (
	"fmt"

	grovecorev1alpha1 "github.com/ai-dynamo/grove/operator/api/core/v1alpha1"

	"github.com/samber/lo"
	admissionv1 "k8s.io/api/admission/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// clusterTopologyValidator validates ClusterTopology resources for create and update operations.
type clusterTopologyValidator struct {
	operation       admissionv1.Operation
	clusterTopology *grovecorev1alpha1.ClusterTopology
}

// newClusterTopologyValidator creates a new ClusterTopology validator for the given operation.
func newClusterTopologyValidator(clusterTopology *grovecorev1alpha1.ClusterTopology, operation admissionv1.Operation) *clusterTopologyValidator {
	return &clusterTopologyValidator{
		operation:       operation,
		clusterTopology: clusterTopology,
	}
}

// ---------------------------- validate create of ClusterTopology -----------------------------------------------

// validate validates the ClusterTopology object.
func (v *clusterTopologyValidator) validate() error {
	allErrs := field.ErrorList{}

	// Note: Metadata validation is handled by the API server
	errs := v.validateClusterTopologySpec(field.NewPath("spec"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}

	return allErrs.ToAggregate()
}

// validateClusterTopologySpec validates the specification of a ClusterTopology object.
func (v *clusterTopologyValidator) validateClusterTopologySpec(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	levelsPath := fldPath.Child("levels")

	// Note: MinItems and MaxItems are enforced by kubebuilder validation
	// (+kubebuilder:validation:MinItems=1, +kubebuilder:validation:MaxItems=7)

	// First, validate each level individually (domain validity, key format, etc.)
	lo.ForEach(v.clusterTopology.Spec.Levels, func(level grovecorev1alpha1.TopologyLevel, i int) {
		levelPath := levelsPath.Index(i)
		allErrs = append(allErrs, v.validateTopologyLevel(level, levelPath)...)
	})

	// Then validate hierarchical order (assumes all domains are valid)
	// This also ensures domain uniqueness
	allErrs = append(allErrs, validateTopologyLevelOrder(v.clusterTopology.Spec.Levels, levelsPath)...)

	// Validate that all keys are unique
	allErrs = append(allErrs, validateTopologyLevelKeyUniqueness(v.clusterTopology.Spec.Levels, levelsPath)...)

	return allErrs
}

// validateTopologyLevel validates a single topology level in isolation.
func (v *clusterTopologyValidator) validateTopologyLevel(level grovecorev1alpha1.TopologyLevel, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Note: Domain validation is handled by kubebuilder enum validation
	// (+kubebuilder:validation:Enum=region;zone;datacenter;block;rack;host;numa)

	// Note: Key required and length validation is handled by kubebuilder validation
	// (+kubebuilder:validation:Required, +kubebuilder:validation:MinLength=1, +kubebuilder:validation:MaxLength=63)

	// Validate key format is a valid Kubernetes label key
	keyPath := fldPath.Child("key")
	allErrs = append(allErrs, metav1validation.ValidateLabelName(level.Key, keyPath)...)

	return allErrs
}

// validateTopologyLevelOrder validates that topology levels are in the correct hierarchical order
// (broadest to narrowest: Region > Zone > DataCenter > Block > Rack > Host > Numa).
// This function assumes all domains have already been validated to be valid topology domains.
// This validation also ensures domain uniqueness (duplicate domains would have the same order value).
func validateTopologyLevelOrder(levels []grovecorev1alpha1.TopologyLevel, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i := 1; i < len(levels); i++ {
		prevLevel := levels[i-1]
		currLevel := levels[i]

		// Current level must be narrower than previous level
		if currLevel.BroaderThan(prevLevel) {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("domain"), currLevel.Domain,
				fmt.Sprintf("topology levels must be in hierarchical order (broadest to narrowest). Domain '%s' at position %d cannot come after '%s' at position %d",
					currLevel.Domain, i, prevLevel.Domain, i-1)))
		} else if currLevel.Compare(prevLevel) == 0 {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("domain"), currLevel.Domain))
		}
	}

	return allErrs
}

// validateTopologyLevelKeyUniqueness validates that all keys in the topology levels are unique.
func validateTopologyLevelKeyUniqueness(levels []grovecorev1alpha1.TopologyLevel, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	seenKeys := make(map[string]int) // map key to first index where it appears

	for i, level := range levels {
		if firstIndex, exists := seenKeys[level.Key]; exists {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("key"), level.Key,
				fmt.Sprintf("duplicate key: already used at levels[%d]", firstIndex)))
		} else {
			seenKeys[level.Key] = i
		}
	}

	return allErrs
}

// ---------------------------- validate update of ClusterTopology -----------------------------------------------

// validateUpdate validates the update to a ClusterTopology object.
func (v *clusterTopologyValidator) validateUpdate(oldClusterTopology *grovecorev1alpha1.ClusterTopology) error {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, v.validateClusterTopologySpecUpdate(&v.clusterTopology.Spec, &oldClusterTopology.Spec, field.NewPath("spec"))...)
	return allErrs.ToAggregate()
}

// validateClusterTopologySpecUpdate validates updates to the ClusterTopology specification.
func (v *clusterTopologyValidator) validateClusterTopologySpecUpdate(oldSpec, newSpec *grovecorev1alpha1.ClusterTopologySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	levelsPath := fldPath.Child("levels")

	// Validate that the number of levels hasn't changed
	if len(newSpec.Levels) != len(oldSpec.Levels) {
		allErrs = append(allErrs, field.Forbidden(levelsPath, "not allowed to add or remove topology levels"))
		return allErrs
	}

	// Validate that the order and domains of levels haven't changed
	allErrs = append(allErrs, validateTopologyLevelImmutableFields(newSpec.Levels, oldSpec.Levels, levelsPath)...)

	return allErrs
}

// validateTopologyLevelImmutableFields validates that immutable fields in topology levels haven't changed during an update.
func validateTopologyLevelImmutableFields(newLevels, oldLevels []grovecorev1alpha1.TopologyLevel, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i := range newLevels {
		levelPath := fldPath.Index(i)

		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newLevels[i].Domain, oldLevels[i].Domain, levelPath.Child("domain"))...)
		// Note: Key is allowed to change (not in the requirements), but validation has already occurred in validate()
	}

	return allErrs
}
