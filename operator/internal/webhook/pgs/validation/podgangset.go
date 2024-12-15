// /*
// Copyright 2024 The Grove Authors.
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
	admissionv1 "k8s.io/api/admission/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
)

var (
	allowedUpdateStrategyTypes   = sets.New[v1alpha1.GangUpdateStrategyType](v1alpha1.GangUpdateStrategyRecreate, v1alpha1.GangUpdateStrategyRolling)
	allowedStartupTypes          = sets.New[v1alpha1.CliqueStartupType](v1alpha1.CliqueStartupTypeInOrder, v1alpha1.CliqueStartupTypeExplicit)
	allowedRestartPolicies       = sets.New[v1alpha1.PodGangRestartPolicy](v1alpha1.GangRestartPolicyNever, v1alpha1.GangRestartPolicyOnFailure, v1alpha1.GangRestartPolicyAlways)
	allowedNetworkPackStrategies = sets.New[v1alpha1.NetworkPackStrategy](v1alpha1.BestEffort, v1alpha1.Strict)
)

type validator struct {
	operation admissionv1.Operation
	pgs       *v1alpha1.PodGangSet
}

func newValidator(op admissionv1.Operation, pgs *v1alpha1.PodGangSet) *validator {
	return &validator{
		operation: op,
		pgs:       pgs,
	}
}

func (v *validator) validate() ([]string, error) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&v.pgs.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	warnings, errs := v.validatePodGangSetSpec()
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}

	return warnings, allErrs.ToAggregate()
}

// validatePodGangSetSpec validates the specification of a PodGangSet object.
func (v *validator) validatePodGangSetSpec() ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("spec")

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(v.pgs.Spec.Replicas), fldPath.Child("replicas"))...)
	allErrs = append(allErrs, v.validateUpdateStrategy(fldPath.Child("updateStrategy"))...)
	warnings, errs := v.validatePodGangTemplateSpec(fldPath.Child("template"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}

	return warnings, allErrs
}

func (v *validator) validatePodGangTemplateSpec(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.StartupType, allowedStartupTypes, fldPath.Child("cliqueStartupType"))...)
	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.RestartPolicy, allowedRestartPolicies, fldPath.Child("restartPolicy"))...)
	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.NetworkPackStrategy, allowedNetworkPackStrategies, fldPath.Child("networkPackStrategy"))...)

	// validate cliques
	warnings, errs := v.validatePodCliques(fldPath.Child("cliques"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return warnings, allErrs
}

func (v *validator) validateUpdateStrategy(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	updateStrategy := v.pgs.Spec.UpdateStrategy
	if updateStrategy == nil {
		return append(errs, field.Required(fldPath, "field is required"))
	}
	errs = append(errs, validateEnumType(&updateStrategy.Type, allowedUpdateStrategyTypes, fldPath.Child("type"))...)
	if updateStrategy.Type == v1alpha1.GangUpdateStrategyRolling {
		errs = append(errs, v.validateRollingUpdateConfig(fldPath.Child("rollingUpdateConfig"))...)
	}

	return errs
}

func (v *validator) validateRollingUpdateConfig(fldPath *field.Path) field.ErrorList {
	errs := field.ErrorList{}

	rollingUpdateConfig := v.pgs.Spec.UpdateStrategy.RollingUpdateConfig
	replicas := int(v.pgs.Spec.Replicas)

	if rollingUpdateConfig == nil {
		return append(errs, field.Required(fldPath, "field is required"))
	} else {
		maxUnavailable, err := intstr.GetScaledValueFromIntOrPercent(rollingUpdateConfig.MaxUnavailable, replicas, false)
		if err != nil {
			errs = append(errs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, err.Error()))
		}
		maxSurge, err := intstr.GetScaledValueFromIntOrPercent(rollingUpdateConfig.MaxSurge, replicas, false)
		if err != nil {
			errs = append(errs, field.Invalid(fldPath.Child("maxSurge"), rollingUpdateConfig.MaxSurge, err.Error()))
		}

		// Ensure that MaxUnavailable and MaxSurge are non-negative.
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(maxUnavailable), fldPath.Child("maxUnavailable"))...)
		errs = append(errs, apivalidation.ValidateNonnegativeField(int64(maxSurge), fldPath.Child("maxSurge"))...)

		// Ensure that MaxUnavailable is not more than the replicas for the PodGangSet.
		if maxUnavailable > replicas {
			errs = append(errs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, "cannot be greater than replicas"))
		}
		// Ensure that both MaxSurge and MaxUnavailable are not zero.
		if maxSurge == 0 && maxUnavailable == 0 {
			errs = append(errs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, "cannot be 0 when maxSurge is 0"))
		}
	}
	return errs
}
