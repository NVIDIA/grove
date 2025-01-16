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
	"strings"

	"github.com/NVIDIA/grove/operator/internal/utils"
	"github.com/samber/lo"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/NVIDIA/grove/operator/api/core/v1alpha1"
)

var (
	allowedUpdateStrategyTypes   = sets.New[v1alpha1.GangUpdateStrategyType](v1alpha1.GangUpdateStrategyRecreate, v1alpha1.GangUpdateStrategyRolling)
	allowedStartupTypes          = sets.New[v1alpha1.CliqueStartupType](v1alpha1.CliqueStartupTypeInOrder, v1alpha1.CliqueStartupTypeAnyOrder, v1alpha1.CliqueStartupTypeExplicit)
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

func (v *validator) validatePodGangTemplateSpec(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.StartupType, allowedStartupTypes, fldPath.Child("cliqueStartupType"))...)
	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.RestartPolicy, allowedRestartPolicies, fldPath.Child("restartPolicy"))...)
	allErrs = append(allErrs, validateEnumType(v.pgs.Spec.Template.NetworkPackStrategy, allowedNetworkPackStrategies, fldPath.Child("networkPackStrategy"))...)

	// validate cliques
	warnings, errs := v.validatePodCliqueTemplates(fldPath.Child("cliques"))
	if len(errs) != 0 {
		allErrs = append(allErrs, errs...)
	}
	return warnings, allErrs
}

func (v *validator) validatePodCliqueTemplates(fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	var warnings []string
	cliqueTemplateSpecs := v.pgs.Spec.Template.Cliques
	isExplicit := v.pgs.Spec.Template.StartupType != nil && *v.pgs.Spec.Template.StartupType == v1alpha1.CliqueStartupTypeExplicit

	if len(cliqueTemplateSpecs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "at least on PodClique must be defined"))
	}

	cliqueNames := make([]string, 0, len(cliqueTemplateSpecs))
	for _, cliqueTemplateSpec := range cliqueTemplateSpecs {
		cliqueNames = append(cliqueNames, cliqueTemplateSpec.Name)
		warns, errs := v.validatePodCliqueTemplateSpec(cliqueTemplateSpec, fldPath)
		if len(errs) != 0 {
			allErrs = append(allErrs, errs...)
		}
		if len(warns) != 0 {
			warnings = append(warnings, warns...)
		}
	}

	duplicateCliqueNames := lo.FindDuplicates(cliqueNames)
	if len(duplicateCliqueNames) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"),
			strings.Join(duplicateCliqueNames, ","), "cliqueTemplateSpec names must be unique"))
	}

	if isExplicit {
		allErrs = append(allErrs, validateCliqueDependencies(cliqueTemplateSpecs, fldPath)...)
	}

	return warnings, allErrs
}

func (v *validator) validatePodCliqueTemplateSpec(cliqueTemplateSpec v1alpha1.PodCliqueTemplateSpec, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePodCliqueTemplateObjectMeta(cliqueTemplateSpec.ObjectMeta, fldPath.Child("metadata"))...)
	if cliqueTemplateSpec.Spec.Replicas <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), cliqueTemplateSpec.Spec.Replicas, "must be greater than 0"))
	}

	isStartupTypeExplicit := v.pgs.Spec.Template.StartupType != nil && *v.pgs.Spec.Template.StartupType == v1alpha1.CliqueStartupTypeExplicit
	if isStartupTypeExplicit && len(cliqueTemplateSpec.Spec.StartsAfter) > 0 {
		for _, dep := range cliqueTemplateSpec.Spec.StartsAfter {
			if utils.IsEmptyStringType(dep) {
				allErrs = append(allErrs, field.Required(fldPath.Child("startsAfter"), "clique dependency must not be empty"))
			}
			if dep == cliqueTemplateSpec.Name {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"), dep, "clique dependency cannot refer to itself"))
			}
		}
		duplicateStartAfterDeps := lo.FindDuplicates(cliqueTemplateSpec.Spec.StartsAfter)
		if len(duplicateStartAfterDeps) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"),
				strings.Join(duplicateStartAfterDeps, ","), "clique dependencies must be unique"))
		}
	}

	warnings, cliquePodSpecErrs := v.validatePodSpec(cliqueTemplateSpec.Spec.Spec, fldPath.Child("spec"))
	if len(cliquePodSpecErrs) != 0 {
		allErrs = append(allErrs, cliquePodSpecErrs...)
	}

	return warnings, allErrs
}

func (v *validator) validatePodSpec(spec corev1.PodSpec, fldPath *field.Path) ([]string, field.ErrorList) {
	allErrs := field.ErrorList{}
	var warnings []string

	if !utils.IsEmptyStringType(spec.RestartPolicy) {
		warnings = append(warnings, "restartPolicy will be ignored, it will be set to Never")
	}

	specFldPath := fldPath.Child("spec")
	if v.operation == admissionv1.Create {
		if spec.TopologySpreadConstraints != nil {
			allErrs = append(allErrs, field.Invalid(specFldPath.Child("topologySpreadConstraints"), spec.TopologySpreadConstraints, "must not be set"))
		}
		if !utils.IsEmptyStringType(spec.NodeName) {
			allErrs = append(allErrs, field.Invalid(specFldPath.Child("nodeName"), spec.NodeName, "must not be set"))
		}
	}
	if spec.NodeSelector != nil {
		allErrs = append(allErrs, field.Invalid(specFldPath.Child("nodeSelector"), spec.NodeSelector, "must not be set"))
	}

	return warnings, allErrs
}

func validatePodCliqueTemplateObjectMeta(cliqueObjMeta metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateNonEmptyStringField(cliqueObjMeta.Name, fldPath.Child("name"))...)
	allErrs = append(allErrs, metav1validation.ValidateLabels(cliqueObjMeta.Labels, fldPath.Child("labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(cliqueObjMeta.Annotations, fldPath.Child("annotations"))...)

	// Setting GenerateName is not allowed for a PodCliqueTemplateSpec when configuring a PodGangSet.
	if len(cliqueObjMeta.GetGenerateName()) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("generateName"), "not allowed on this type"))
	}
	// Setting Namespace is not allowed for a PodCliqueTemplateSpec when configuring a PodGangSet.
	if len(cliqueObjMeta.Namespace) != 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("namespace"), "not allowed on this type"))
	}

	// If you set any field other than Name, labels and Annotations, it will be ignored. We only explicitly disallow setting GenerateName and Namespace.
	// TODO revisit this with others and see if this is ok.

	return allErrs
}

func validateCliqueDependencies(cliques []v1alpha1.PodCliqueTemplateSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	depG := NewPodCliqueDependencyGraph()
	var discoveredCliqueNames []string
	for _, clique := range cliques {
		discoveredCliqueNames = append(discoveredCliqueNames, clique.Name)
		depG.AddDependencies(clique.Name, clique.Spec.StartsAfter)
	}

	unknownCliquesDeps := depG.GetUnknownCliques(discoveredCliqueNames)
	if len(unknownCliquesDeps) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("startsAfter"),
			strings.Join(unknownCliquesDeps, ","), "unknown clique names found, all clique dependencies must be defined as cliques"))
	}

	// check for strongly connected components a.k.a cycles in the directed graph of clique dependencies
	cycles := depG.GetStronglyConnectedCliques()
	if len(cycles) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, cycles, "clique must not have circular dependencies"))
	}

	return allErrs
}
