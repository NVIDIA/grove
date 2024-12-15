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

func (v *validator) validate() (warnings []string, err error) {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&v.pgs.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	warnings, pgsSpecErrs := v.validatePodGangSetSpec()
	if pgsSpecErrs != nil {
		allErrs = append(allErrs, pgsSpecErrs...)
	}
	err = allErrs.ToAggregate()
	return
}

// validatePodGangSetSpec validates the specification of a PodGangSet object.
func (v *validator) validatePodGangSetSpec() (warnings []string, errs field.ErrorList) {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath("spec")

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(v.pgs.Spec.Replicas), fldPath.Child("replicas"))...)
	allErrs = append(allErrs, v.validateUpdateStrategy(fldPath.Child("updateStrategy"))...)
	warnings, pgTemplateSpecErrs := v.validatePodGangTemplateSpec(fldPath.Child("template"))
	if pgTemplateSpecErrs != nil {
		allErrs = append(allErrs, pgTemplateSpecErrs...)
	}

	return
}

func (v *validator) validatePodGangTemplateSpec(fldPath *field.Path) (warnings []string, errs field.ErrorList) {
	errs = field.ErrorList{}

	errs = append(errs, validateEnumType(v.pgs.Spec.Template.StartupType, allowedStartupTypes, fldPath.Child("cliqueStartupType"))...)
	errs = append(errs, validateEnumType(v.pgs.Spec.Template.RestartPolicy, allowedRestartPolicies, fldPath.Child("restartPolicy"))...)
	errs = append(errs, validateEnumType(v.pgs.Spec.Template.NetworkPackStrategy, allowedNetworkPackStrategies, fldPath.Child("networkPackStrategy"))...)

	// validate cliques
	warnings, cliqueErrs := v.validatePodCliques(fldPath.Child("cliques"))
	if cliqueErrs != nil {
		errs = append(errs, cliqueErrs...)
	}
	return
}

func (v *validator) validateUpdateStrategy(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	updateStrategy := v.pgs.Spec.UpdateStrategy
	if updateStrategy == nil {
		return append(allErrs, field.Required(fldPath, "field is required"))
	}
	allErrs = append(allErrs, validateEnumType(&updateStrategy.Type, allowedUpdateStrategyTypes, fldPath.Child("type"))...)
	if updateStrategy.Type == v1alpha1.GangUpdateStrategyRolling {
		allErrs = append(allErrs, v.validateRollingUpdateConfig(fldPath.Child("rollingUpdateConfig"))...)
	}

	return allErrs
}

func (v *validator) validateRollingUpdateConfig(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	rollingUpdateConfig := v.pgs.Spec.UpdateStrategy.RollingUpdateConfig
	replicas := v.pgs.Spec.Replicas

	if rollingUpdateConfig == nil {
		return append(allErrs, field.Required(fldPath, "field is required"))
	}
	var (
		maxUnavailable int
		maxSurge       int
		err            error
	)
	if maxUnavailable, err = intstr.GetScaledValueFromIntOrPercent(rollingUpdateConfig.MaxUnavailable, int(replicas), false); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, err.Error()))
	}
	if maxSurge, err = intstr.GetScaledValueFromIntOrPercent(rollingUpdateConfig.MaxSurge, int(replicas), false); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxSurge"), rollingUpdateConfig.MaxSurge, err.Error()))
	}

	// Ensure that MaxUnavailable and MaxSurge are non-negative.
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(maxUnavailable), fldPath.Child("maxUnavailable"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(maxSurge), fldPath.Child("maxSurge"))...)

	// Ensure that MaxUnavailable is not more than the replicas for the PodGangSet.
	if maxUnavailable > int(replicas) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, "cannot be greater than replicas"))
	}
	// Ensure that both MaxSurge and MaxUnavailable are not zero.
	if maxSurge == 0 && maxUnavailable == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdateConfig.MaxUnavailable, "cannot be 0 when maxSurge is 0"))
	}

	return allErrs
}
