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
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
)

// validatePodGangSet validates a PodGangSet object.
func validatePodGangSet(pgs *v1alpha1.PodGangSet) error {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&pgs.ObjectMeta, false, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validatePodGangSetSpec(&pgs.Spec, field.NewPath("spec"))...)

	return allErrs.ToAggregate()
}

// validatePodGangSetSpec validates the specification of a PodGangSet object.
func validatePodGangSetSpec(spec *v1alpha1.PodGangSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePodGangTemplateSpec(&spec.Template, fldPath.Child("template"))...)

	if spec.Replicas == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("replicas"), "field is required"))
	} else if *spec.Replicas < 0 {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.Replicas), fldPath.Child("replicas"))...)
	} else {
		allErrs = append(allErrs, validateUpdateStrategy(spec.UpdateStrategy, *spec.Replicas, fldPath.Child("updateStragery"))...)
	}

	// TODO: validate GangSpreadConstraints
	//validateTopologySpreadConstraints(spec.GangSpreadConstraints, fldPath.Child("gangSpreadConstraints"), corevalidation.PodValidationOptions{})

	return allErrs
}

func validatePodGangTemplateSpec(spec *v1alpha1.PodGangTemplateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// validate startup type
	var isInOrder bool
	if spec.StartupType == nil || *spec.StartupType == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("cliqueStartupType"), "field is required"))
	} else {
		switch *spec.StartupType {
		case v1alpha1.Explicit:
		// nop
		case v1alpha1.InOrder:
			isInOrder = true
		default:
			allErrs = append(allErrs, field.Invalid(fldPath.Child("cliqueStartupType"), spec.StartupType, "invalid value"))
		}
	}

	// validate restart policy
	if spec.RestartPolicy == nil || *spec.RestartPolicy == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("restartPolicy"), "field is required"))
	} else {
		switch *spec.RestartPolicy {
		case v1alpha1.GangRestartPolicyNever, v1alpha1.GangRestartPolicyOnFailure, v1alpha1.GangRestartPolicyAlways:
			// nop
		default:
			allErrs = append(allErrs, field.Invalid(fldPath.Child("restartPolicy"), spec.RestartPolicy, "invalid value"))
		}
	}

	// validate networkPack strategy
	if spec.NetworkPackStrategy == nil || *spec.NetworkPackStrategy == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("networkPackStrategy"), "field is required"))
	} else {
		switch *spec.NetworkPackStrategy {
		case v1alpha1.BestEffort, v1alpha1.Strict:
			// nop
		default:
			allErrs = append(allErrs, field.Invalid(fldPath.Child("networkPackStrategy"), *spec.NetworkPackStrategy, "invalid value"))
		}
	}

	// validate cliques
	allErrs = append(allErrs, validateCliques(spec.Cliques, fldPath.Child("cliques"), isInOrder)...)

	return allErrs
}

func validateUpdateStrategy(obj *v1alpha1.GangUpdateStrategy, replicas int32, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if obj == nil {
		return append(allErrs, field.Required(fldPath, "field is required"))
	}

	switch obj.Type {
	case v1alpha1.GangUpdateStrategyRecreate, v1alpha1.GangUpdateStrategyRolling:
		// nop
	case "":
		allErrs = append(allErrs, field.Required(fldPath.Child("updateStrategy"), "field is required"))
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("updateStrategy"), obj.Type, "invalid value"))
	}

	if cfg := obj.RollingUpdateConfig; cfg == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("rollingUpdateConfig"), "field is required"))
	} else {
		maxUnavailable, err := intstr.GetScaledValueFromIntOrPercent(cfg.MaxUnavailable, int(replicas), false)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), cfg.MaxUnavailable, err.Error()))
		} else if maxUnavailable < 0 {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(maxUnavailable), fldPath.Child("maxUnavailable"))...)
		} else if maxUnavailable > int(replicas) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), cfg.MaxUnavailable, "cannot be greater than replicas"))
		}

		maxSurge, err := intstr.GetScaledValueFromIntOrPercent(cfg.MaxSurge, int(replicas), false)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("maxSurge"), cfg.MaxSurge, err.Error()))
		} else if maxSurge < 0 {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(maxSurge), fldPath.Child("maxSurge"))...)
		} else if maxSurge == 0 && maxUnavailable == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("maxSurge"), cfg.MaxSurge, "cannot be 0 when maxUnavailable is 0"))
		}
	}

	return allErrs
}
