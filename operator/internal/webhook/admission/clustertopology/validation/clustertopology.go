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

	admissionv1 "k8s.io/api/admission/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// topologyDomainOrder defines the hierarchical order of topology domains from broadest to narrowest.
// Lower index = broader scope (e.g., Region is broader than Zone).
var topologyDomainOrder = map[grovecorev1alpha1.TopologyDomain]int{
	grovecorev1alpha1.TopologyDomainRegion:     0,
	grovecorev1alpha1.TopologyDomainZone:       1,
	grovecorev1alpha1.TopologyDomainDataCenter: 2,
	grovecorev1alpha1.TopologyDomainBlock:      3,
	grovecorev1alpha1.TopologyDomainRack:       4,
	grovecorev1alpha1.TopologyDomainHost:       5,
	grovecorev1alpha1.TopologyDomainNuma:       6,
}

// ctValidator validates ClusterTopology resources for create and update operations.
type ctValidator struct {
	operation admissionv1.Operation
	ct        *grovecorev1alpha1.ClusterTopology
}

// newCTValidator creates a new ClusterTopology validator for the given operation.
func newCTValidator(ct *grovecorev1alpha1.ClusterTopology, operation admissionv1.Operation) *ctValidator {
	return &ctValidator{
		operation: operation,
		ct:        ct,
	}
}

// ---------------------------- validate create of ClusterTopology -----------------------------------------------

// validate validates the ClusterTopology object.
func (v *ctValidator) validate() ([]string, error) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&v.ct.ObjectMeta, false,
		apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	fldPath := field.NewPath("spec")
	warnings, errs := v.validateClusterTopologySpec(fldPath)
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}

	return warnings, allErrs.ToAggregate()
}

// validateClusterTopologySpec validates the specification of a ClusterTopology object.
func (v *ctValidator) validateClusterTopologySpec(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	var warnings []string

	levelsPath := fldPath.Child("levels")

	// Validate that at least one level is defined
	if len(v.ct.Spec.Levels) == 0 {
		allErrs = append(allErrs, field.Required(levelsPath, "at least one topology level must be defined"))
		return warnings, allErrs
	}

	// Validate max items (should be enforced by kubebuilder validation, but check here too)
	if len(v.ct.Spec.Levels) > len(topologyDomainOrder) {
		allErrs = append(allErrs, field.TooMany(levelsPath, len(v.ct.Spec.Levels), len(topologyDomainOrder)))
	}

	// First, validate each level individually (domain validity, key format, etc.)
	for i, level := range v.ct.Spec.Levels {
		levelPath := levelsPath.Index(i)
		allErrs = append(allErrs, v.validateTopologyLevel(level, levelPath)...)
	}

	// Then validate hierarchical order (assumes all domains are valid)
	// This also ensures domain uniqueness
	allErrs = append(allErrs, validateTopologyLevelOrder(v.ct.Spec.Levels, levelsPath)...)

	return warnings, allErrs
}

// validateTopologyLevel validates a single topology level in isolation.
func (v *ctValidator) validateTopologyLevel(level grovecorev1alpha1.TopologyLevel, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Validate domain is one of the allowed values
	domainPath := fldPath.Child("domain")
	if _, exists := topologyDomainOrder[level.Domain]; !exists {
		// Build list of allowed domains from the map keys
		allowedDomains := make([]string, 0, len(topologyDomainOrder))
		for domain := range topologyDomainOrder {
			allowedDomains = append(allowedDomains, string(domain))
		}
		allErrs = append(allErrs, field.NotSupported(domainPath, level.Domain, allowedDomains))
	}

	// Validate key is a valid Kubernetes label key
	keyPath := fldPath.Child("key")
	if level.Key == "" {
		allErrs = append(allErrs, field.Required(keyPath, "key is required"))
	} else {
		// Use Kubernetes label name validation
		allErrs = append(allErrs, metav1validation.ValidateLabelName(level.Key, keyPath)...)
	}

	return allErrs
}

// validateTopologyLevelOrder validates that topology levels are in the correct hierarchical order
// (broadest to narrowest: Region > Zone > DataCenter > Block > Rack > Host > Numa).
// This function assumes all domains have already been validated to be valid topology domains.
// This validation also ensures domain uniqueness (duplicate domains would have the same order value).
func validateTopologyLevelOrder(levels []grovecorev1alpha1.TopologyLevel, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i := 1; i < len(levels); i++ {
		prevDomain := levels[i-1].Domain
		currDomain := levels[i].Domain

		prevOrder := topologyDomainOrder[prevDomain]
		currOrder := topologyDomainOrder[currDomain]

		// Current level must have a higher order (narrower scope) than previous level
		if currOrder <= prevOrder {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("domain"), currDomain,
				fmt.Sprintf("topology levels must be in hierarchical order (broadest to narrowest). Domain '%s' at position %d cannot come after '%s' at position %d",
					currDomain, i, prevDomain, i-1)))
		}
	}

	return allErrs
}

// ---------------------------- validate update of ClusterTopology -----------------------------------------------

// validateUpdate validates the update to a ClusterTopology object.
func (v *ctValidator) validateUpdate(oldCT *grovecorev1alpha1.ClusterTopology) error {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("spec")
	allErrs = append(allErrs, v.validateClusterTopologySpecUpdate(&v.ct.Spec, &oldCT.Spec, fldPath)...)
	return allErrs.ToAggregate()
}

// validateClusterTopologySpecUpdate validates updates to the ClusterTopology specification.
func (v *ctValidator) validateClusterTopologySpecUpdate(newSpec, oldSpec *grovecorev1alpha1.ClusterTopologySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	levelsPath := fldPath.Child("levels")

	// Validate that the number of levels hasn't changed
	if len(newSpec.Levels) != len(oldSpec.Levels) {
		allErrs = append(allErrs, field.Forbidden(levelsPath, "not allowed to add or remove topology levels"))
		return allErrs
	}

	// Validate that the order and domains of levels haven't changed
	for i := range newSpec.Levels {
		levelPath := levelsPath.Index(i)
		newLevel := newSpec.Levels[i]
		oldLevel := oldSpec.Levels[i]

		// Validate that domain is immutable
		if newLevel.Domain != oldLevel.Domain {
			allErrs = append(allErrs, field.Invalid(levelPath.Child("domain"), newLevel.Domain,
				fmt.Sprintf("domain is immutable, cannot change from '%s' to '%s'", oldLevel.Domain, newLevel.Domain)))
		}

		// Note: Key is allowed to change (not in the requirements), but we validate it's still a valid label key
		// The validate() function already checked this during the general validation
	}

	return allErrs
}
